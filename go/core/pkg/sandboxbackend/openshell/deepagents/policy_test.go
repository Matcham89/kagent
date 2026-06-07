package deepagents_test

import (
	"testing"

	"github.com/kagent-dev/kagent/go/api/v1alpha2"
	"github.com/kagent-dev/kagent/go/core/pkg/sandboxbackend/openshell/deepagents"
	"github.com/stretchr/testify/require"
)

func TestBaselineDeepAgentsSandboxPolicy(t *testing.T) {
	pol := deepagents.BaselineDeepAgentsSandboxPolicy()
	require.NotNil(t, pol)
	net := pol.GetNetworkPolicies()
	require.Contains(t, net, deepagents.NetworkPolicyKeyNVIDIA)
	require.Contains(t, net, deepagents.NetworkPolicyKeyGitHub)
	require.Contains(t, net, deepagents.NetworkPolicyKeyPyPI)
	require.Contains(t, net, deepagents.NetworkPolicyKeyLangChain)

	fs := pol.GetFilesystem()
	require.True(t, fs.GetIncludeWorkdir())
	require.Contains(t, fs.GetReadWrite(), deepagents.DeepAgentsConfigDir)
	require.Contains(t, fs.GetReadOnly(), "/opt/deepagents")

	require.Equal(t, "sandbox", pol.GetProcess().GetRunAsUser())
}

func TestIsDeepAgentsSandboxBackend(t *testing.T) {
	require.True(t, deepagents.IsDeepAgentsSandboxBackend(v1alpha2.AgentHarnessBackendDeepAgents))
	require.False(t, deepagents.IsDeepAgentsSandboxBackend(v1alpha2.AgentHarnessBackendHermes))
	require.False(t, deepagents.IsDeepAgentsSandboxBackend(v1alpha2.AgentHarnessBackendOpenClaw))
}

func TestAllowedDomainsBinaries(t *testing.T) {
	bins := deepagents.AllowedDomainsBinaries()
	require.NotEmpty(t, bins)
	var paths []string
	for _, b := range bins {
		paths = append(paths, b.GetPath())
	}
	require.Contains(t, paths, "/opt/deepagents/.venv/bin/dcode")
	require.Contains(t, paths, "/sandbox/**")
}
