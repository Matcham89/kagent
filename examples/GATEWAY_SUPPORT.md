# AgentGateway Support for Multi-Cluster Agent Communication

## Overview

This feature enables Kagent agents to communicate with agents in remote clusters via AgentGateway. Gateways are exposed as tools that the LLM can explicitly invoke, enabling sophisticated multi-cluster operations.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  Local Cluster (Orchestrator Agent)                 │
│  ┌──────────────────────────────────────┐          │
│  │ k8s-orchestrator Agent                │          │
│  │ - Skills: multi-cluster management    │          │
│  │ - Tools:                              │          │
│  │   • dev_cluster_gateway               │──────┐   │
│  │   • test_cluster_gateway              │──┐   │   │
│  │   • prod_cluster_gateway              │──┼───┼───┤
│  └──────────────────────────────────────┘  │   │   │
└─────────────────────────────────────────────┼───┼───┘
                                              │   │
                  JSON-RPC 2.0 + SSE          │   │
                  via AgentGateway :8080      │   │
                                              │   │
        ┌─────────────────────────────────────┘   │
        │                                         │
        ▼                                         ▼
┌────────────────────┐                 ┌────────────────────┐
│ Dev Cluster        │                 │ Test Cluster       │
│ 192.168.1.200:8080 │                 │ 192.168.1.201:8080 │
│                    │                 │                    │
│ AgentGateway       │                 │ AgentGateway       │
│        │           │                 │        │           │
│        ▼           │                 │        ▼           │
│  k8s-agent         │                 │  k8s-agent         │
│  (with kubectl,    │                 │  (with kubectl,    │
│   helm tools)      │                 │   helm tools)      │
└────────────────────┘                 └────────────────────┘
```

## How It Works

1. **Gateway Configuration**: Each agent defines gateways in `spec.declarative.a2aConfig.gateways`
2. **Tool Generation**: The kagent controller translates gateways into remote agent tools
3. **LLM Invocation**: The LLM sees gateways as tools (e.g., `dev_cluster_gateway`)
4. **A2A Routing**: When invoked, the request routes through the gateway URL to the target agent
5. **Response Handling**: The remote agent processes the request using its tools and returns results

## Configuration

### Agent CRD Structure

```yaml
apiVersion: kagent.dev/v1alpha2
kind: Agent
metadata:
  name: k8s-orchestrator
  namespace: kagent
spec:
  type: Declarative
  declarative:
    a2aConfig:
      skills:
        - id: multi-cluster-management
          name: Multi-Cluster Kubernetes Management
          # ... skill definition

      # Gateway configuration
      gateways:
        - id: dev-cluster                    # Gateway ID (becomes "dev_cluster_gateway" tool)
          address: https://192.168.1.200:8080 # AgentGateway base URL
          namespace: kagent                   # Target namespace on remote cluster
          agentName: k8s-agent                # Target agent name on remote cluster
          description: Gateway to dev cluster # Description for the LLM
          headersFrom:                        # Optional authentication
            - name: Authorization
              secretRef:
                name: dev-cluster-token
                key: token
```

### Field Descriptions

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique identifier for the gateway. Tool name will be `{id}_gateway` |
| `address` | Yes | Base URL of the AgentGateway (e.g., `https://192.168.1.200:8080`) |
| `namespace` | Yes | Namespace where the target agent lives on the remote cluster |
| `agentName` | Yes | Name of the target agent on the remote cluster |
| `description` | No | Context about this gateway for the LLM |
| `headersFrom` | No | Headers for authentication (supports secrets/configmaps) |

### Authentication

Gateways support the same authentication pattern as agent tools:

```yaml
gateways:
  - id: prod-cluster
    address: https://192.168.1.202:8080
    namespace: kagent
    agentName: k8s-agent
    headersFrom:
      # Bearer token from secret
      - name: Authorization
        secretRef:
          name: prod-cluster-token
          key: token
      # API key from configmap
      - name: X-API-Key
        configMapRef:
          name: prod-cluster-config
          key: api-key
```

## Example Usage

### Example 1: Single Cluster Query

**User**: "How many pods are running in the dev cluster?"

**LLM Action**: Invoke `dev_cluster_gateway` tool with message: "Count all running pods"

**Flow**:
1. Orchestrator agent receives user request
2. LLM identifies it needs the `dev_cluster_gateway` tool
3. Request sent to `https://192.168.1.200:8080/api/a2a/kagent/k8s-agent/`
4. Dev cluster's k8s-agent counts pods using its kubectl tool
5. Response returned to orchestrator
6. Orchestrator responds to user

### Example 2: Multi-Cluster Comparison

**User**: "Compare the number of deployments in dev vs test"

**LLM Actions**:
1. Invoke `dev_cluster_gateway` with "Count deployments"
2. Invoke `test_cluster_gateway` with "Count deployments"
3. Compare results and respond

**Response**:
```
I've checked both clusters:

Dev Cluster: 15 deployments
Test Cluster: 12 deployments

The dev cluster has 3 more deployments than the test cluster.
```

### Example 3: Cross-Cluster Health Check

**User**: "Is the api-gateway service healthy in all environments?"

**LLM Actions**:
1. Invoke `dev_cluster_gateway` with "Check api-gateway service status"
2. Invoke `test_cluster_gateway` with "Check api-gateway service status"
3. Invoke `prod_cluster_gateway` with "Check api-gateway service status"
4. Summarize health across all clusters

## Implementation Details

### Code Changes

#### 1. Agent CRD Types (`go/api/v1alpha2/agent_types.go`)

Added new types:
- `AgentGateway`: Defines gateway configuration
- Helper methods:
  - `ResolveHeaders()`: Resolve authentication headers from secrets/configmaps
  - `GetGatewayURL()`: Construct full A2A endpoint URL
  - `GetToolName()`: Generate tool name (`{id}_gateway`)

```go
type A2AConfig struct {
    Skills   []AgentSkill   `json:"skills,omitempty"`
    Gateways []AgentGateway `json:"gateways,omitempty"` // NEW
}

type AgentGateway struct {
    ID          string     `json:"id"`
    Address     string     `json:"address"`
    Namespace   string     `json:"namespace"`
    AgentName   string     `json:"agentName"`
    Description string     `json:"description,omitempty"`
    HeadersFrom []ValueRef `json:"headersFrom,omitempty"`
}
```

#### 2. Translator (`go/internal/controller/translator/agent/adk_api_translator.go`)

Added gateway translation logic in `translateInlineAgent()`:

```go
// Translate gateways into remote agent tools
if agent.Spec.Declarative.A2AConfig != nil {
    for _, gateway := range agent.Spec.Declarative.A2AConfig.Gateways {
        headers, err := gateway.ResolveHeaders(ctx, a.kube, agent.Namespace)
        if err != nil {
            return nil, nil, nil, fmt.Errorf("failed to resolve headers for gateway %s: %w", gateway.ID, err)
        }

        cfg.RemoteAgents = append(cfg.RemoteAgents, adk.RemoteAgentConfig{
            Name:        gateway.GetToolName(),
            Url:         gateway.GetGatewayURL(),
            Headers:     headers,
            Description: gateway.Description,
        })
    }
}
```

### Protocol Details

Gateways use the standard A2A protocol:

- **Protocol**: JSON-RPC 2.0 over HTTP
- **Transport**: SSE (Server-Sent Events) for streaming responses
- **Endpoint Pattern**: `{address}/api/a2a/{namespace}/{agentName}/`
- **Port**: AgentGateway runs on port 8080 (vs direct agent on 8083)
- **Authentication**: Headers resolved from secrets/configmaps

### Tool Naming Convention

Gateway tools follow the naming pattern: `{gateway.id}_gateway`

Examples:
- `id: dev-cluster` → tool name: `dev_cluster_gateway`
- `id: test` → tool name: `test_gateway`
- `id: us-west-2-prod` → tool name: `us_west_2_prod_gateway`

## Complete Example

See [`multi-cluster-agent-with-gateways.yaml`](./multi-cluster-agent-with-gateways.yaml) for a complete working example with:
- Gateway configurations for dev, test, and prod clusters
- Authentication secrets
- System message with usage instructions
- Skills definition

## Testing

### Manual Test

1. **Deploy AgentGateway on remote clusters**:
   ```bash
   # On each remote cluster (dev, test, prod)
   kubectl apply -f agentgateway-deployment.yaml
   ```

2. **Create authentication secrets**:
   ```bash
   kubectl create secret generic dev-cluster-token \
     --from-literal=token="Bearer your-token-here" \
     -n kagent
   ```

3. **Deploy orchestrator agent**:
   ```bash
   kubectl apply -f examples/multi-cluster-agent-with-gateways.yaml
   ```

4. **Test with kagent CLI**:
   ```bash
   kagent agent invoke k8s-orchestrator \
     -m "List all pods in the dev cluster"
   ```

### Expected Behavior

- Gateways appear as tools in the agent's tool list
- LLM can reason about which gateway to use based on cluster references
- Requests route through gateway URL to target agent
- Responses stream back via SSE
- Authentication headers are attached to requests

## Troubleshooting

### Gateway Not Appearing as Tool

**Check**:
1. Agent has `a2aConfig.gateways` defined
2. CRDs are up to date (run `make manifests` in `go/` directory)
3. Agent controller logs for translation errors

### Authentication Failures

**Check**:
1. Secrets exist in the agent's namespace
2. Secret keys match `headersFrom` configuration
3. AgentGateway accepts the provided authentication

### Connection Timeouts

**Check**:
1. Gateway address is reachable from the orchestrator cluster
2. Network policies allow outbound connections
3. AgentGateway is running and healthy on remote cluster

### Tool Not Being Invoked

**Check**:
1. System message mentions the gateway tools
2. Tool description is clear about when to use it
3. User query explicitly mentions cluster or comparison

## Next Steps

1. **Generate CRDs**: Run `make manifests` in `go/` directory after Go is installed
2. **Test Implementation**: Deploy example agent and verify gateway tools work
3. **Documentation**: Update main README with gateway feature
4. **Helm Charts**: Consider creating a Helm chart for multi-cluster setup

## Related Files

- **CRD Types**: `go/api/v1alpha2/agent_types.go` (lines 236-311)
- **Translator**: `go/internal/controller/translator/agent/adk_api_translator.go` (lines 565-580)
- **Example**: `examples/multi-cluster-agent-with-gateways.yaml`
- **Python ADK Types**: `python/packages/kagent-adk/src/kagent/adk/types.py` (lines 35-41, 94-131)

## Protocol Reference

AgentGateway follows the same A2A protocol as slackbot-agent:
- See `/Users/chris/Documents/github/slackbot-agent/slack_bot.py` for reference implementation
- JSON-RPC 2.0 with method `message/stream`
- SSE streaming with `text/event-stream` Accept header
- Context IDs for conversation continuity
