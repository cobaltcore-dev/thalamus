---
title: Gateway Integrations
---

# Gateway Integrations

Thalamus uses [agentgateway](https://agentgateway.dev) as its data plane, and the helm release ships its controller and proxies. Other data planes may follow as opt-in integrations, so nothing on this page is part of the Thalamus API. See the [Architecture overview](/concepts/architecture) for how the pieces fit together.

This guide shows where to attach common options: authentication, token counting, and rate limiting. Everything beyond these examples is described in the [agentgateway documentation](https://agentgateway.dev/docs/kubernetes/latest/).

All options are configured through `AgentgatewayPolicy` resources. The operator already creates a few of them for model routing and the model list (`body-based-routing`, `model-list`), so use your own names.

## Prerequisites

- The Thalamus stack from [Getting Started](/getting-started) is installed. agentgateway is part of the release and enabled by default.
- The `inference-gateway` Gateway in the `thalamus` namespace.

## Authentication

By default the inference API accepts unauthenticated requests. To require authentication, apply a policy that targets the gateway listener:

```yaml
# apikey-auth.yaml
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayPolicy
metadata:
  name: apikey-auth
  namespace: thalamus
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: inference-gateway
      sectionName: api
  traffic:
    apiKeyAuthentication:
      mode: Strict
      configMapSelector:
        matchLabels:
          thalamus.cloud/apikey: "true"
```

Keys are stored as SHA-256 hashes in ConfigMaps, one entry per user or client. Only the hash lives in the cluster, the raw key is handed to the client once:

```bash
KEY=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')
HASH="sha256:$(printf '%s' "$KEY" | openssl dgst -sha256 | awk '{print $NF}')"
kubectl create configmap apikey-alice \
  --namespace thalamus \
  --from-literal=alice='{"keyHash":"'$HASH'","metadata":{"tenant":"team-a"}}'
kubectl label configmap apikey-alice --namespace thalamus thalamus.cloud/apikey=true
echo "key for alice: $KEY"
```

The optional metadata is readable by other policies and by metrics as `apiKey.tenant`.

Keys can also be stored in Secrets instead of ConfigMaps. Both ways, plus other methods such as JWT, basic auth, external authorization services, and CEL based authorization rules, are described in [Security](https://agentgateway.dev/docs/kubernetes/latest/documentation/security/) in the agentgateway docs.

## Token counting

For every `Model`, the operator creates an `AgentgatewayBackend` of type `ai`. The gateway counts input and output tokens of every request against these backends, including streamed responses. No configuration is needed for this.

Counts are exposed as the `agentgateway_gen_ai_client_token_usage` metric on the gateway proxies, labeled by model and token type. To scrape them, enable agentgateway metrics in the thalamus release (see [Perses Dashboards](/perses-dashboards) for the full setup):

```yaml
agentgateway:
  monitoring:
    enabled: true
    serviceMonitor:
      extraLabels:
        release: monitoring
```

To break down usage per tenant, attach the key metadata as a metric label by extending the `apikey-auth` policy from [Authentication](#authentication):

```yaml
spec:
  # targetRefs as in the apikey-auth policy
  frontend:
    metrics:
      attributes:
        add:
          - name: tenant
            expression: apiKey.tenant
```

Usage can then be grouped by tenant, for example with `sum by (tenant) (agentgateway_gen_ai_client_token_usage_sum{gen_ai_token_type=~"input|output"})`. The full per-key setup is described in [Virtual keys](https://agentgateway.dev/docs/kubernetes/latest/documentation/llm/cost-controls/virtual-keys/) in the agentgateway docs.

## Rate limiting

Policies can enforce request limits and token budgets. Attach them to the gateway, or to the `HTTPRoute` of a single model. The operator creates one `HTTPRoute` per `Model`, named `<model>-engine`, for example `qwen3-6-27b-engine`.

```yaml
# rate-limit.yaml
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayPolicy
metadata:
  name: rate-limit
  namespace: thalamus
spec:
  # targetRefs as in the apikey-auth policy
  traffic:
    rateLimit:
      local:
        - requests: 100
          unit: Minutes
          burst: 20
        - tokens: 10000
          unit: Hours
          key: apiKey.tenant
```

Each entry sets one limit. The `key` field scopes an entry: every distinct value of the expression gets its own bucket, here one token budget per tenant from the key metadata. Requests without a tenant share a single bucket.

Local limits run in each gateway proxy replica. How limits and token budgets behave is described in [Rate limiting for LLMs](https://agentgateway.dev/docs/kubernetes/latest/documentation/llm/rate-limit/) in the agentgateway docs, including [claim-level limits](https://agentgateway.dev/docs/kubernetes/latest/documentation/llm/rate-limit/#claim-level). For budgets shared across replicas, use [global rate limiting](https://agentgateway.dev/docs/kubernetes/latest/documentation/security/rate-limit-global/) through an external service.

## More options

This page covers what Thalamus users most commonly ask for. The agentgateway docs describe the rest:

| Topic | agentgateway docs |
| --- | --- |
| Authentication and authorization | [Security](https://agentgateway.dev/docs/kubernetes/latest/documentation/security/) |
| LLM routing, failover, prompt handling | [LLM consumption](https://agentgateway.dev/docs/kubernetes/latest/documentation/llm/) |
| Request and token rate limits | [Rate limiting for LLMs](https://agentgateway.dev/docs/kubernetes/latest/documentation/llm/rate-limit/) |
| Metrics, access logs, traces | [Observability](https://agentgateway.dev/docs/kubernetes/latest/documentation/observability/) |

## Related

- [Architecture overview](/concepts/architecture)
- [Request flow](/concepts/backends/native/request-flow)
- [Perses Dashboards](/perses-dashboards)
- [Getting Started](/getting-started)
