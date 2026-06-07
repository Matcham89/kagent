package openshell

import (
	"context"
	"fmt"
	"strings"
	"time"

	openshellv1 "github.com/kagent-dev/kagent/go/api/openshell/gen/openshellv1"
	"github.com/kagent-dev/kagent/go/api/v1alpha2"
	"github.com/kagent-dev/kagent/go/core/internal/utils"
	"github.com/kagent-dev/kagent/go/core/pkg/sandboxbackend"
	"github.com/kagent-dev/kagent/go/core/pkg/sandboxbackend/openshell/deepagents"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// DeepAgentsBackend implements AsyncBackend for Deep Agents (deepagents-code / dcode)
// AgentHarness resources on the OpenShell runtime.
type DeepAgentsBackend struct {
	*agentHarnessOpenShellBackend
}

var _ sandboxbackend.AsyncBackend = (*DeepAgentsBackend)(nil)

// NewDeepAgentsBackend returns the Deep Agents harness backend.
func NewDeepAgentsBackend(kubeClient client.Client, clients *OpenShellClients, cfg Config, recorder record.EventRecorder) *DeepAgentsBackend {
	return &DeepAgentsBackend{
		agentHarnessOpenShellBackend: newAgentHarnessOpenShellBackend(
			kubeClient, clients, cfg, recorder,
			v1alpha2.AgentHarnessBackendDeepAgents,
		),
	}
}

// EnsureAgentHarness syncs ModelConfig then creates the Deep Agents sandbox.
func (b *DeepAgentsBackend) EnsureAgentHarness(ctx context.Context, ah *v1alpha2.AgentHarness) (sandboxbackend.EnsureResult, error) {
	if ah == nil {
		return sandboxbackend.EnsureResult{}, fmt.Errorf("AgentHarness is required")
	}
	ctx, cancel := b.CallCtx(ctx)
	defer cancel()
	ctx = withAuth(ctx, b.cfg.Token)

	if res, found, err := b.findExistingSandbox(ctx, ah); err != nil || found {
		return res, err
	}
	return b.ensureAgentHarnessSandbox(ctx, ah, buildDeepAgentsCreateRequest)
}

// OnAgentHarnessReady writes ~/.deepagents/config.toml and .env, updates the config hash, starts
// the langgraph gateway, and forwards the public port.
func (b *DeepAgentsBackend) OnAgentHarnessReady(ctx context.Context, ah *v1alpha2.AgentHarness, h sandboxbackend.Handle) error {
	ref := strings.TrimSpace(ah.Spec.ModelConfigRef)
	if ref == "" {
		return nil
	}
	if h.ID == "" {
		return fmt.Errorf("sandbox backend handle id is empty")
	}
	if b.kubeClient == nil {
		return fmt.Errorf("kubernetes client is required for deepagents bootstrap")
	}

	modelConfigRef, err := utils.ParseRefString(ref, ah.Namespace)
	if err != nil {
		return fmt.Errorf("parse modelConfigRef: %w", err)
	}
	mc := &v1alpha2.ModelConfig{}
	if err := b.kubeClient.Get(ctx, modelConfigRef, mc); err != nil {
		return fmt.Errorf("get ModelConfig: %w", err)
	}

	configTOML, envFile, execEnv, err := deepagents.BuildBootstrapArtifacts(ctx, b.kubeClient, ah.Namespace, ah, mc)
	if err != nil {
		return fmt.Errorf("build deepagents config: %w", err)
	}

	token := b.cfg.Token
	idCtx, cancelID := b.CallCtx(ctx)
	defer cancelID()
	execID, err := b.ExecSandboxID(withAuth(idCtx, token), h.ID)
	if err != nil {
		return fmt.Errorf("resolve sandbox exec id: %w", err)
	}

	mkdirScript := fmt.Sprintf(`mkdir -p %s`, deepagents.DeepAgentsConfigDir)
	installCtx, cancelInstall := context.WithTimeout(ctx, 120*time.Second+15*time.Second)
	defer cancelInstall()
	code, stderr, err := b.ExecSandbox(withAuth(installCtx, token), execID, []string{"sh", "-c", mkdirScript}, nil, execEnv, 30)
	if err != nil {
		return fmt.Errorf("mkdir deepagents config dir: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("mkdir deepagents config dir: exit %d: %s", code, strings.TrimSpace(stderr))
	}

	configInstall := []string{"sh", "-c", fmt.Sprintf(`cat > %s/config.toml`, deepagents.DeepAgentsConfigDir)}
	code, stderr, err = b.ExecSandbox(withAuth(installCtx, token), execID, configInstall, configTOML, execEnv, 60)
	if err != nil {
		return fmt.Errorf("install deepagents config.toml: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("install deepagents config.toml: exit %d: %s", code, strings.TrimSpace(stderr))
	}

	envInstall := []string{"sh", "-c", fmt.Sprintf(`cat > %s/.env`, deepagents.DeepAgentsConfigDir)}
	code, stderr, err = b.ExecSandbox(withAuth(installCtx, token), execID, envInstall, envFile, execEnv, 60)
	if err != nil {
		return fmt.Errorf("install deepagents .env: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("install deepagents .env: exit %d: %s", code, strings.TrimSpace(stderr))
	}

	hashScript := fmt.Sprintf(
		`mkdir -p /etc/nemoclaw && sha256sum %s/config.toml %s/.env > %s && chmod 444 %s 2>/dev/null || true`,
		deepagents.DeepAgentsConfigDir, deepagents.DeepAgentsConfigDir, deepagents.DeepAgentsConfigHashFile, deepagents.DeepAgentsConfigHashFile,
	)
	hashCtx, cancelHash := context.WithTimeout(ctx, 30*time.Second)
	defer cancelHash()
	code, stderr, err = b.ExecSandbox(withAuth(hashCtx, token), execID, []string{"sh", "-c", hashScript}, nil, execEnv, 30)
	if err != nil {
		return fmt.Errorf("write deepagents config hash: %w", err)
	}
	if code != 0 {
		ctrllog.FromContext(ctx).Info("deepagents config hash write skipped (non-fatal)", "stderr", strings.TrimSpace(stderr))
	}

	// langgraph dev boots heavier than the hermes gateway (graph import + ASGI startup), so allow
	// a longer window than hermes before giving up.
	gwCtx, cancelGW := context.WithTimeout(ctx, 150*time.Second+15*time.Second)
	defer cancelGW()
	// Source the bootstrapped .env (model, base_url, key), set HOME so dcode/langgraph resolve
	// ~/.deepagents, then start the baked langgraph gateway app bound to localhost. The CWD must be
	// writable because `langgraph dev` creates a .langgraph_api state dir there, so run from the
	// config dir (RW) while pointing --config at the read-only baked app (its langgraph.json uses an
	// absolute graph path so it resolves regardless of CWD). Flags and LANGGRAPH_AUTH_TYPE=noop
	// mirror how dcode itself launches `langgraph dev` (deepagents_code/server.py).
	gatewayStart := fmt.Sprintf(
		`cd %s && set -a && . %s/.env && set +a && HOME=%s LANGGRAPH_AUTH_TYPE=noop PYTHONDONTWRITEBYTECODE=1 nohup langgraph dev --host 127.0.0.1 --port %d --no-browser --no-reload --config %s/langgraph.json >>/tmp/gateway.log 2>&1 &`,
		deepagents.DeepAgentsConfigDir,
		deepagents.DeepAgentsConfigDir,
		deepagents.DeepAgentsHome,
		deepagents.DeepAgentsInternalGatewayPort,
		deepagents.DeepAgentsGatewayWorkdir,
	)
	code, stderr, err = b.ExecSandbox(withAuth(gwCtx, token), execID, []string{"sh", "-c", gatewayStart}, nil, execEnv, 30)
	if err != nil {
		return fmt.Errorf("start deepagents gateway: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("start deepagents gateway: exit %d: %s", code, strings.TrimSpace(stderr))
	}

	if err := b.waitDeepAgentsGatewayListen(withAuth(gwCtx, token), execID, deepagents.DeepAgentsInternalGatewayPort, execEnv); err != nil {
		return fmt.Errorf("wait for deepagents gateway listen: %w", err)
	}

	socatStart := fmt.Sprintf(
		`command -v socat >/dev/null 2>&1 && nohup socat TCP-LISTEN:%d,bind=0.0.0.0,fork,reuseaddr TCP:127.0.0.1:%d >>/tmp/socat.log 2>&1 &`,
		deepagents.DeepAgentsPublicGatewayPort,
		deepagents.DeepAgentsInternalGatewayPort,
	)
	code, stderr, err = b.ExecSandbox(withAuth(gwCtx, token), execID, []string{"sh", "-c", socatStart}, nil, execEnv, 30)
	if err != nil {
		return fmt.Errorf("start deepagents socat forwarder: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("start deepagents socat forwarder: exit %d: %s", code, strings.TrimSpace(stderr))
	}

	ctrllog.FromContext(ctx).Info("deepagents bootstrap completed", "agentHarness", ah.Namespace+"/"+ah.Name)
	return nil
}

func (b *DeepAgentsBackend) waitDeepAgentsGatewayListen(ctx context.Context, execID string, port int, execEnv map[string]string) error {
	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)
	var lastResult ExecSandboxResult
	for range 150 {
		result, err := b.ExecSandboxOutput(ctx, execID, []string{"ss", "-tln"}, nil, execEnv, 5)
		if err != nil {
			return err
		}
		lastResult = result
		if result.ExitCode != 0 {
			return fmt.Errorf("ss -tln exit %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
		}
		if strings.Contains(result.Stdout, listenAddr) {
			return nil
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf(
		"timed out after 150s waiting for %s; last ss output: %s; stderr: %s",
		listenAddr,
		strings.TrimSpace(lastResult.Stdout),
		strings.TrimSpace(lastResult.Stderr),
	)
}

func buildDeepAgentsCreateRequest(ah *v1alpha2.AgentHarness, _ []string) (*openshellv1.CreateSandboxRequest, []string) {
	req, unsupported := buildAgentHarnessOpenshellCreateRequest(ah)
	if req.GetSpec().GetTemplate() == nil {
		req.Spec.Template = &openshellv1.SandboxTemplate{}
	}
	if ah.Spec.Image == "" {
		req.Spec.Template.Image = deepagents.DeepAgentsSandboxBaseImage
	}
	return req, unsupported
}
