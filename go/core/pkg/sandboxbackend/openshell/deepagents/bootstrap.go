package deepagents

import (
	"context"
	"fmt"
	"strings"

	"github.com/kagent-dev/kagent/go/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// modelSpec returns the dcode "provider:model" spec for the inference.local route.
func modelSpec(model string) string {
	return inferenceProvider + ":" + model
}

// BuildDeepAgentsConfigTOML returns config.toml bytes (written to ~/.deepagents/config.toml)
// for the given ModelConfig. dcode routes through the built-in openai provider with the
// base_url overridden to the OpenShell inference gateway (inference.local), mirroring how the
// Hermes harness routes inference through the gateway.
func BuildDeepAgentsConfigTOML(mc *v1alpha2.ModelConfig) ([]byte, error) {
	if mc == nil {
		return nil, fmt.Errorf("ModelConfig is required")
	}
	modelID := strings.TrimSpace(mc.Spec.Model)
	if modelID == "" {
		return nil, fmt.Errorf("ModelConfig.spec.model is required for Deep Agents bootstrap")
	}
	spec := modelSpec(modelID)

	var b strings.Builder
	b.WriteString("[models]\n")
	fmt.Fprintf(&b, "default = %q\n", spec)
	fmt.Fprintf(&b, "recent = %q\n\n", spec)
	fmt.Fprintf(&b, "[models.providers.%s]\n", inferenceProvider)
	fmt.Fprintf(&b, "base_url = %q\n", DefaultInferenceBaseURL)
	fmt.Fprintf(&b, "api_key_env = %q\n", InferenceAPIKeyEnv)
	fmt.Fprintf(&b, "models = [%q]\n", modelID)
	return []byte(b.String()), nil
}

// BuildDeepAgentsEnvFile returns the ~/.deepagents/.env bytes. It points the OpenAI-compatible
// SDK at the inference gateway (so both dcode and the baked langgraph gateway graph route
// identically), supplies a placeholder key the gateway resolves, names the default model for the
// gateway graph, and disables update checks / LangSmith tracing inside the sandbox.
func BuildDeepAgentsEnvFile(mc *v1alpha2.ModelConfig) ([]byte, error) {
	if mc == nil {
		return nil, fmt.Errorf("ModelConfig is required")
	}
	modelID := strings.TrimSpace(mc.Spec.Model)
	if modelID == "" {
		return nil, fmt.Errorf("ModelConfig.spec.model is required for Deep Agents bootstrap")
	}
	lines := []string{
		"OPENAI_BASE_URL=" + DefaultInferenceBaseURL,
		// Placeholder credential; the OpenShell inference gateway injects/handles real auth.
		InferenceAPIKeyEnv + "=openshell-inference",
		// Consumed by the baked gateway graph (/opt/deepagents/server_graph.py).
		"DEEPAGENTS_MODEL=" + modelSpec(modelID),
		"DEEPAGENTS_CODE_NO_UPDATE_CHECK=1",
		"DEEPAGENTS_CODE_AUTO_UPDATE=0",
		"LANGSMITH_TRACING=false",
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// BuildBootstrapArtifacts builds config.toml, .env, and the exec environment for Deep Agents
// bootstrap. Deep Agents harnesses have no messaging channels, so execEnv is always empty.
func BuildBootstrapArtifacts(_ context.Context, _ client.Client, _ string, _ *v1alpha2.AgentHarness, mc *v1alpha2.ModelConfig) (configTOML, envFile []byte, execEnv map[string]string, err error) {
	configTOML, err = BuildDeepAgentsConfigTOML(mc)
	if err != nil {
		return nil, nil, nil, err
	}
	envFile, err = BuildDeepAgentsEnvFile(mc)
	if err != nil {
		return nil, nil, nil, err
	}
	return configTOML, envFile, map[string]string{}, nil
}
