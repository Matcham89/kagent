package deepagents

import (
	"maps"

	sandboxv1 "github.com/kagent-dev/kagent/go/api/openshell/gen/sandboxv1"
	"github.com/kagent-dev/kagent/go/api/v1alpha2"
)

// Network policy map keys for Deep Agents sandbox egress.
const (
	NetworkPolicyKeyNVIDIA    = "nvidia"
	NetworkPolicyKeyGitHub    = "github"
	NetworkPolicyKeyPyPI      = "pypi"
	NetworkPolicyKeyLangChain = "langchain"
)

const (
	endpointProtocolREST = "rest"
	endpointEnforcement  = "enforce"
	endpointAccessFull   = "full"
)

// SandboxPolicyVersion is OpenShell SandboxPolicy.version for Deep Agents fragments.
const SandboxPolicyVersion = 1

// deepAgentsCoreBinaries are the executables allowed to reach inference / research endpoints.
// Paths match the in-repo sandbox base image (docker/deepagents-sandbox-base/Dockerfile): the
// dcode/langgraph entrypoints exec the venv interpreter, so that is the process that issues
// network calls.
var deepAgentsCoreBinaries = []*sandboxv1.NetworkBinary{
	{Path: "/opt/deepagents/.venv/bin/python"},
	{Path: "/opt/deepagents/.venv/bin/python3"},
	{Path: "/usr/local/bin/python3*"},
	{Path: "/usr/bin/node"},
	{Path: "/usr/bin/nodejs"},
}

var inferenceRules = []*sandboxv1.L7Rule{
	{Allow: &sandboxv1.L7Allow{Method: "POST", Path: "/v1/chat/completions"}},
	{Allow: &sandboxv1.L7Allow{Method: "POST", Path: "/v1/completions"}},
	{Allow: &sandboxv1.L7Allow{Method: "POST", Path: "/v1/embeddings"}},
	{Allow: &sandboxv1.L7Allow{Method: "POST", Path: "/v1/responses"}},
	{Allow: &sandboxv1.L7Allow{Method: "GET", Path: "/v1/models"}},
	{Allow: &sandboxv1.L7Allow{Method: "GET", Path: "/v1/models/**"}},
}

var getOnlyRules = []*sandboxv1.L7Rule{
	{Allow: &sandboxv1.L7Allow{Method: "GET", Path: "/**"}},
}

func restEndpoint(host string, rules []*sandboxv1.L7Rule) *sandboxv1.NetworkEndpoint {
	return &sandboxv1.NetworkEndpoint{
		Host:        host,
		Ports:       []uint32{443},
		Protocol:    endpointProtocolREST,
		Enforcement: endpointEnforcement,
		Rules:       rules,
	}
}

func restEndpointFullAccess(host string) *sandboxv1.NetworkEndpoint {
	return &sandboxv1.NetworkEndpoint{
		Host:        host,
		Ports:       []uint32{443},
		Protocol:    endpointProtocolREST,
		Enforcement: endpointEnforcement,
		Access:      endpointAccessFull,
	}
}

// IsDeepAgentsSandboxBackend reports backends that use the Deep Agents sandbox baseline.
func IsDeepAgentsSandboxBackend(b v1alpha2.AgentHarnessBackendType) bool {
	return b == v1alpha2.AgentHarnessBackendDeepAgents
}

func defaultDeepAgentsFilesystemPolicy() *sandboxv1.FilesystemPolicy {
	return &sandboxv1.FilesystemPolicy{
		IncludeWorkdir: true,
		ReadOnly: []string{
			"/usr",
			"/lib",
			"/opt/deepagents",
			"/proc",
			"/dev/urandom",
			"/app",
			"/etc",
			"/var/log",
		},
		ReadWrite: []string{
			"/sandbox",
			"/tmp",
			"/dev/null",
			DeepAgentsConfigDir,
		},
	}
}

func defaultDeepAgentsLandlockPolicy() *sandboxv1.LandlockPolicy {
	return &sandboxv1.LandlockPolicy{Compatibility: "best_effort"}
}

func defaultDeepAgentsProcessPolicy() *sandboxv1.ProcessPolicy {
	return &sandboxv1.ProcessPolicy{
		RunAsUser:  "sandbox",
		RunAsGroup: "sandbox",
	}
}

func defaultDeepAgentsNetworkPolicies() map[string]*sandboxv1.NetworkPolicyRule {
	return map[string]*sandboxv1.NetworkPolicyRule{
		NetworkPolicyKeyNVIDIA: {
			Name: "nvidia",
			Endpoints: []*sandboxv1.NetworkEndpoint{
				restEndpoint("integrate.api.nvidia.com", inferenceRules),
				restEndpoint("inference-api.nvidia.com", inferenceRules),
			},
			Binaries: deepAgentsCoreBinaries,
		},
		NetworkPolicyKeyGitHub: {
			Name: "github",
			Endpoints: []*sandboxv1.NetworkEndpoint{
				restEndpointFullAccess("github.com"),
				restEndpointFullAccess("api.github.com"),
			},
			Binaries: []*sandboxv1.NetworkBinary{
				{Path: "/usr/bin/git"},
				{Path: "/opt/deepagents/.venv/bin/python"},
			},
		},
		NetworkPolicyKeyPyPI: {
			Name: "pypi",
			Endpoints: []*sandboxv1.NetworkEndpoint{
				restEndpoint("pypi.org", getOnlyRules),
				restEndpoint("files.pythonhosted.org", getOnlyRules),
			},
			Binaries: []*sandboxv1.NetworkBinary{
				{Path: "/opt/deepagents/.venv/bin/pip"},
				{Path: "/opt/deepagents/.venv/bin/python"},
				{Path: "/usr/local/bin/python3*"},
			},
		},
		// langch.in hosts the dcode install/update script; langchain docs/registry for skills.
		NetworkPolicyKeyLangChain: {
			Name: "langchain",
			Endpoints: []*sandboxv1.NetworkEndpoint{
				restEndpoint("langch.in", getOnlyRules),
				restEndpointFullAccess("smith.langchain.com"),
				restEndpointFullAccess("api.smith.langchain.com"),
			},
			Binaries: deepAgentsCoreBinaries,
		},
	}
}

// BaselineDeepAgentsSandboxPolicy returns the fixed Deep Agents baseline (nvidia inference,
// github, pypi, langchain, plus filesystem / landlock / process policies).
func BaselineDeepAgentsSandboxPolicy() *sandboxv1.SandboxPolicy {
	net := map[string]*sandboxv1.NetworkPolicyRule{}
	maps.Copy(net, defaultDeepAgentsNetworkPolicies())
	return &sandboxv1.SandboxPolicy{
		Version:         SandboxPolicyVersion,
		NetworkPolicies: net,
		Filesystem:      defaultDeepAgentsFilesystemPolicy(),
		Landlock:        defaultDeepAgentsLandlockPolicy(),
		Process:         defaultDeepAgentsProcessPolicy(),
	}
}

// AllowedDomainsBinaries returns executables allowed to use kagent_allowed_domains for Deep Agents harnesses.
func AllowedDomainsBinaries() []*sandboxv1.NetworkBinary {
	return []*sandboxv1.NetworkBinary{
		{Path: "/opt/deepagents/.venv/bin/dcode"},
		{Path: "/opt/deepagents/.venv/bin/langgraph"},
		{Path: "/opt/deepagents/.venv/bin/python"},
		{Path: "/opt/deepagents/.venv/bin/python3"},
		{Path: "/usr/local/bin/python3*"},
		{Path: "/usr/bin/node"},
		{Path: "/usr/bin/nodejs"},
		{Path: "/usr/bin/curl"},
		{Path: "/usr/bin/git"},
		{Path: "/sandbox/**"},
	}
}
