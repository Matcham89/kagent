package deepagents

const (
	// DeepAgentsSandboxBaseImage is the default OpenShell VM image for Deep Agents harnesses.
	// Built in-repo from docker/deepagents-sandbox-base/Dockerfile (dcode + deepagents SDK +
	// langgraph-cli + the baked gateway graph under /opt/deepagents). Defaults to a private
	// Docker Hub repo for local dev; override per-resource via AgentHarness spec.image.
	DeepAgentsSandboxBaseImage = "docker.io/matcham89/deepagents-sandbox-base:latest"

	// DeepAgentsHome is the HOME set for dcode and the gateway so that ~/.deepagents resolves
	// to DeepAgentsConfigDir below.
	DeepAgentsHome = "/sandbox"

	// DeepAgentsConfigDir is the in-sandbox Deep Agents config root ($HOME/.deepagents).
	// dcode reads $HOME/.deepagents/{config.toml,.env}, so it must equal DeepAgentsHome + "/.deepagents".
	DeepAgentsConfigDir = "/sandbox/.deepagents"

	// DeepAgentsGatewayWorkdir holds the baked langgraph gateway app (server_graph.py + langgraph.json).
	DeepAgentsGatewayWorkdir = "/opt/deepagents"

	// DeepAgentsConfigHashFile is the root-owned integrity anchor written at bootstrap.
	DeepAgentsConfigHashFile = "/etc/nemoclaw/deepagents.config-hash"

	// DeepAgentsInternalGatewayPort is where the langgraph dev server binds (127.0.0.1 only).
	DeepAgentsInternalGatewayPort = 2024

	// DeepAgentsPublicGatewayPort is exposed via socat for OpenShell port forwarding.
	DeepAgentsPublicGatewayPort = 8624

	// DefaultInferenceBaseURL is the model base_url when routing through OpenShell inference.local.
	// inference.local exposes an OpenAI-compatible API, so dcode routes via the built-in
	// openai provider with this base_url override.
	DefaultInferenceBaseURL = "https://inference.local/v1"

	// InferenceAPIKeyEnv is the env var dcode/openai SDK reads for the (gateway-injected) API key.
	InferenceAPIKeyEnv = "OPENAI_API_KEY"

	// inferenceProvider is the dcode provider key used for the inference.local OpenAI-compatible route.
	inferenceProvider = "openai"
)
