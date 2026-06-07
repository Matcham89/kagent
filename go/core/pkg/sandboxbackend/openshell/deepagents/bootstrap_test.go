package deepagents_test

import (
	"context"
	"testing"

	"github.com/kagent-dev/kagent/go/api/v1alpha2"
	"github.com/kagent-dev/kagent/go/core/pkg/sandboxbackend/openshell/deepagents"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildDeepAgentsConfigTOML(t *testing.T) {
	mc := &v1alpha2.ModelConfig{
		Spec: v1alpha2.ModelConfigSpec{
			Model:    "claude-opus-4-7",
			Provider: v1alpha2.ModelProviderAnthropic,
		},
	}
	raw, err := deepagents.BuildDeepAgentsConfigTOML(mc)
	require.NoError(t, err)
	s := string(raw)
	require.Contains(t, s, `default = "openai:claude-opus-4-7"`)
	require.Contains(t, s, "[models.providers.openai]")
	require.Contains(t, s, `base_url = "https://inference.local/v1"`)
	require.Contains(t, s, `api_key_env = "OPENAI_API_KEY"`)
	require.Contains(t, s, `models = ["claude-opus-4-7"]`)
}

func TestBuildDeepAgentsConfigTOML_RequiresModel(t *testing.T) {
	_, err := deepagents.BuildDeepAgentsConfigTOML(&v1alpha2.ModelConfig{})
	require.Error(t, err)
	_, err = deepagents.BuildDeepAgentsConfigTOML(nil)
	require.Error(t, err)
}

func TestBuildDeepAgentsEnvFile(t *testing.T) {
	mc := &v1alpha2.ModelConfig{Spec: v1alpha2.ModelConfigSpec{Model: "gpt-5.5"}}
	raw, err := deepagents.BuildDeepAgentsEnvFile(mc)
	require.NoError(t, err)
	env := string(raw)
	require.Contains(t, env, "OPENAI_BASE_URL=https://inference.local/v1")
	require.Contains(t, env, "OPENAI_API_KEY=openshell-inference")
	require.Contains(t, env, "DEEPAGENTS_MODEL=openai:gpt-5.5")
	require.Contains(t, env, "DEEPAGENTS_CODE_NO_UPDATE_CHECK=1")
	require.Contains(t, env, "LANGSMITH_TRACING=false")
}

func TestBuildBootstrapArtifacts(t *testing.T) {
	mc := &v1alpha2.ModelConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "mc1", Namespace: "default"},
		Spec:       v1alpha2.ModelConfigSpec{Model: "gpt-5.5"},
	}
	ah := &v1alpha2.AgentHarness{
		ObjectMeta: metav1.ObjectMeta{Name: "h1", Namespace: "default"},
		Spec:       v1alpha2.AgentHarnessSpec{Backend: v1alpha2.AgentHarnessBackendDeepAgents},
	}
	configTOML, envFile, execEnv, err := deepagents.BuildBootstrapArtifacts(context.Background(), nil, "default", ah, mc)
	require.NoError(t, err)
	require.Contains(t, string(configTOML), "[models.providers.openai]")
	require.Contains(t, string(envFile), "DEEPAGENTS_MODEL=openai:gpt-5.5")
	require.Empty(t, execEnv)
}
