status: accepted

---
# Inference Gateway: Agentgateway vs. Envoy AI Gateway

## Context and Problem Statement

Thalamus needs a single gateway entry point for all inference traffic: TLS termination, authentication, rate limiting, and model-aware routing to the correct InferencePool. 

Thalamus currently uses **Agentgateway**; SAP AI-Core uses **Envoy AI Gateway**.

## Decision Drivers

**A. Operational simplicity**
- Fewer components means fewer failure points and simpler on-call
- No stateful dependencies unless strictly required

**B. Standards compliance**
- Gateway API conformance matters for ecosystem compatibility and future portability

**C. Observability and auth**
- LLM-specific token telemetry and rich auth (OIDC, multi-provider JWT, CEL RBAC) are useful from day one

**D. Token quota enforcement**
- Whether hard per-user daily token quotas (exact counts from response bodies) are a near-term requirement

**E. Team alignment**
- Shared stack with AI-Core only delivers value if there is a concrete shared maintenance or roadmap commitment — using the same component alone is not sufficient justification

## Considered Options

### Option 1: Agentgateway

[agentgateway.dev](https://agentgateway.dev) — unified multi-protocol data plane (HTTP, gRPC, LLM, MCP, A2A) written in Rust. Single process, no sidecars, no stateful dependencies.

**Key characteristics:**
- `AgentgatewayPolicy` CRD for per-route auth, rate limiting, and transformation
- API key auth via labelled Kubernetes Secrets; JWT/OIDC with multiple simultaneous providers; CEL-based RBAC
- Token rate limiting: upfront estimation (approximate) for local enforcement; exact hard limits possible via remote Envoy Rate Limit gRPC service
- LLM cost tracking and per-request token telemetry as Prometheus metrics
- MCP and A2A protocol support (only relevant if Thalamus hosts and exposes its own MCP servers or AI agents)
- Fully conformant with Gateway API v1.5.0

### Option 2: Envoy AI Gateway

[aigateway.envoyproxy.io](https://aigateway.envoyproxy.io) — LLM traffic layer on top of Envoy Gateway (v0.7.0, pre-GA since October 2024). Adds token quota enforcement and request/response transformation on top of the battle-tested Envoy proxy.

**Key characteristics:**
- `AIGatewayRoute` CRD for multi-model routing with token quota enforcement
- `ai-gateway-extproc` sidecar injected into each proxy pod via mutating webhook — handles all LLM-specific logic over a Unix domain socket; Redis-backed state required
- Exact token quota enforcement: extracts `InputToken`, `OutputToken`, `CachedInputToken` from response bodies; HTTP 429 on quota breach — suitable for hard per-user daily token budgets
- JWT/OIDC via Envoy Gateway `SecurityPolicy`; no LLM-specific auth primitives
- Partial Gateway API conformance (`GatewayInfrastructurePropagation` and `GatewayHTTPSListenerDetectMisdirectedRequests` not implemented)

## Decision Outcome

Chosen option: **Option 1: Agentgateway**

Agentgateway meets all current requirements with significantly lower operational complexity. The only compelling argument for Envoy AI Gateway — precise per-user token quota enforcement directly from response bodies — addresses a requirement Thalamus does not yet have. Hard limits are achievable with Agentgateway via remote rate limiting when needed.

**This decision should be revisited if:**
1. Hard per-user daily token quotas (exact response-body counts) become a confirmed near-term requirement
2. There is a concrete shared maintenance or roadmap with AI-Core that justifies the migration cost

### Consequences

* Good, because significantly lower operational complexity — single process, no stateful dependencies
* Good, because stronger LLM observability and auth out of the box
* Bad, because diverges from AI-Core's gateway stack
* Bad, because no published large-scale production case study

## Pros and Cons of the Options

### Decision Matrix

| Decision Driver | Option 1: Agentgateway | Option 2: Envoy AI Gateway |
|---|---|---|
| A. Operational simplicity | ✅ single process, no deps | ❌ sidecar + webhook + Redis |
| B. Gateway API conformance | ✅ full (v1.5.0) | ⚠️ partial (2 optional features missing) |
| C. Observability and auth | ✅ LLM telemetry, OIDC, CEL RBAC | ⚠️ token counts for rate limiting only |
| D. Token quota enforcement | ⚠️ exact counts require remote rate limiter | ✅ exact (from response body, built-in) |
| E. Team alignment | ⚠️ no concrete shared commitment yet | ✅ same stack as AI-Core |

Legend: ✅ = fully addressed, ⚠️ = partially addressed / conditional, ❌ = not addressed / major drawback

### Option 1: Agentgateway

- Good, because single Rust process — no sidecar injection, no mutating webhook, no Redis required
- Good, because fully conformant with Gateway API v1.5.0
- Good, because LLM cost tracking and per-request token telemetry built into observability stack
- Good, because OIDC browser flow, multiple simultaneous JWT providers, and CEL-based RBAC available
- Good, because MCP and A2A protocol support available if Thalamus expands to hosting its own agentic endpoints
- Neutral, because upfront token estimation is approximate — exact hard quotas require additional remote rate limiter deployment
- Bad, because no large-scale production case study publicly documented
- Bad, because diverges from AI-Core's stack

### Option 2: Envoy AI Gateway

- Good, because exact token quota enforcement built-in — no additional components required for hard per-user daily budgets
- Good, because Envoy Gateway base is production-proven at scale
- Good, because aligns with AI-Core's gateway stack
- Neutral, because v0.7.0 pre-GA (created October 2024) — production maturity of the AI Gateway layer specifically is unproven
- Bad, because `ai-gateway-extproc` sidecar injected via mutating webhook — sidecar can crash independently of the proxy; webhook must be healthy for pods to start
- Bad, because Redis is a required stateful dependency with no in-process fallback
- Bad, because partial Gateway API conformance
- Bad, because no LLM-specific cost/telemetry metrics — token counts surface only for rate limiting purposes

## Additional Information
