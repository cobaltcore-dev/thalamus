---
status: accepted
---
# Inference Gateway: Agentgateway vs. Envoy AI Gateway

## Context and Problem Statement

Thalamus needs a single gateway entry point for all inference traffic: TLS termination, authentication, rate limiting, and model-aware routing to the correct InferencePool. 

Thalamus currently uses **Agentgateway**; another SAP internal team uses **Envoy AI Gateway**.

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

**E. Cross-team alignment**
- A shared stack with the other team only delivers value if there is a concrete shared maintenance or roadmap commitment — using the same component alone is not sufficient justification

## Decision Outcome

Chosen option: **Option 1: Agentgateway**

Agentgateway meets all current requirements with significantly lower operational complexity. The only compelling argument for Envoy AI Gateway — precise per-user token quota enforcement directly from response bodies — addresses a requirement Thalamus does not yet have. Hard limits are achievable with Agentgateway via remote rate limiting when needed.

**This decision should be revisited if:**
1. Hard per-user daily token quotas (exact response-body counts) become a confirmed near-term requirement
2. There is a concrete shared maintenance or roadmap with the other team that justifies the migration cost

### Consequences

* Good, because significantly lower operational complexity — single process, no stateful dependencies
* Good, because stronger LLM observability and auth out of the box
* Bad, because it diverges from the other team's gateway stack
* Bad, because no published large-scale production case study

## Pros and Cons of the Options

### Option 1: Agentgateway

- Good, because single Rust process — no sidecar injection, no mutating webhook, no Redis required
- Good, because fully conformant with Gateway API v1.5.0
- Good, because LLM cost tracking and per-request token telemetry built into observability stack
- Good, because OIDC browser flow, multiple simultaneous JWT providers, and CEL-based RBAC available
- Good, because MCP and A2A protocol support available if Thalamus expands to hosting its own agentic endpoints
- Neutral, because upfront token estimation is approximate — exact hard quotas require additional remote rate limiter deployment
- Bad, because no large-scale production case study publicly documented
- Bad, because it diverges from the other team's stack

### Option 2: Envoy AI Gateway

- Good, because exact token quota enforcement is built-in — no separate remote rate-limiting service is required for hard per-user daily budgets
- Good, because Envoy Gateway base is production-proven at scale
- Good, because it aligns with the other team's gateway stack
- Neutral, because v0.7.0 pre-GA (created October 2024) — production maturity of the AI Gateway layer specifically is unproven
- Bad, because `ai-gateway-extproc` sidecar injected via mutating webhook — sidecar can crash independently of the proxy; webhook must be healthy for pods to start
- Bad, because Redis is a required stateful dependency with no in-process fallback
- Bad, because Gateway API conformance is partial (`GatewayInfrastructurePropagation` and `GatewayHTTPSListenerDetectMisdirectedRequests` not implemented)
- Bad, because no LLM-specific cost/telemetry metrics — token counts surface only for rate limiting purposes

### Decision Matrix

| Decision Driver | Option 1: Agentgateway | Option 2: Envoy AI Gateway |
|---|---|---|
| A. Operational simplicity | ✅ single process, no deps | ❌ sidecar + webhook + Redis |
| B. Gateway API conformance | ✅ full (v1.5.0) | ⚠️ partial (2 optional features missing) |
| C. Observability and auth | ✅ LLM telemetry, OIDC, CEL RBAC | ⚠️ token counts for rate limiting only |
| D. Token quota enforcement | ⚠️ exact counts require remote rate limiter | ✅ exact (from response body, built-in) |
| E. Cross-team alignment | ⚠️ no concrete shared commitment yet | ✅ same stack as the other team |

Legend: ✅ = fully addressed, ⚠️ = partially addressed / conditional, ❌ = not addressed / major drawback
