# Thalamus: Abstraction over Inference Platforms

## Context and Problem Statement

Thalamus is a vendor-neutral, Kubernetes-native inference service for sovereign LLM deployments.
The open-source community organized the technology stack around llm-d, vLLM, the Kubernetes Gateway API Inference Extension, KServe, KAITO, and other projects.
Each has its own API, deployment model, and semantics.

Thalamus shall provide a unified abstraction to the underlying stack.
This ADR evaluates the options to implement the abstraction.


## Decision Drivers

* **D1: Platform and stack independence**. Model maintainers and consumers must not be locked into a specific platform, and usage must not require platform-specific knowledge.
* **D2: Operational consistency**. Provide a uniform and coherent management across the underlying stack.
* **D3: Extensibility**. Adding a new platform must not require changes to the user-facing API.

## Considered Options

* Option 1: Model CRD
* Option 2: Thin pass-through
* Option 3: Model CRD with single backend

## Decision Outcome

Chosen option: **Model CRD** (with multiple backends), because it is the only option that satisfies platform and stack independence and extensibility together, keeps the user-facing API simple and controlled, and allows plugging in additional backends in response to user demand.

The Model CRD is a Thalamus-owned, user-facing API that describes what to serve; per-backend adapters translate it into backend-specific resources and normalize their status back. This decouples model maintainers and consumers from any single backend: switching backends is a one-field change, and adding or removing a backend is pluggable without touching the user-facing API. This supports users who already trust and run a specific stack (e.g. KServe) without forcing a custom implementation.

Direct backend exposure (Option 2) was rejected because it loses platform independence: every consumer must understand N backends, Thalamus-specific requirements have no API home, and switching backends becomes a major effort.

Model CRD with a single backend (Option 3) was rejected because tight single-backend lock-in weakens independence and makes later integrations harder.

### Consequences

* (D1) Good, because there is no single backend lock-in, which provides platform and stack independence, essential for sovereign environments and community development.
* (D1) Good, because it opens collaboration opportunities across the community, drawing expertise in LLMs and operators.
* (D1) Good, because model definitions are portable and switching backends is a one-field change.
* (D1) Good, because Thalamus controls the API rather than depending on the backend provider project and community.
* (D3) Good, because adding or removing backends is pluggable and does not change the user-facing API: users do not have to think about the backend implementation details.
* (D3) Neutral, because new backends can be integrated on-demand, although with major effort.
* (D2) Neutral, because the CRD is alpha — cheap initially, but may grow more complex over time.
* (D1) Neutral, because when the backend choice is exposed to users, it is not obvious which backend supports which features; and if only features shared by all backends are exposed, there is little advantage to supporting multiple backends.
* (D2) Bad, because every backend might need a maintained and fully validated translation with test coverage.
* (D2) Bad, because backends do not have feature parity: features must either be rebuilt across backends, integrated manually in the controller, or left unsupported for a backend.

## Pros and Cons of the Options

### Option 1: Model CRD

A Thalamus-owned Model CRD is the user-facing platform API describing what to serve across multiple backends.
Backend-specific adapters translate the Model CRD into backend-specific resources, observe their statuses, and report them back in normalized fields.

* (D1) Good, because model definitions are portable and switching backends is a one-field change
* (D1) Good, because Thalamus controls the API rather than the backend provider project and community
* (D3) Good, because adding or removing backends is pluggable and doesn't change the user-facing API
* (D3) Good, because users do not have to think about the backend
* (D2) Neutral, because the CRD is alpha, low-cost initially, but might get more complex over time
* (D2) Bad, because every backend needs a maintained and fully validated translation and test coverage
* (D2) Bad, because not all backends have feature parity. Features missing from a backend would need to be rebuilt or integrated manually in the controller
* (D1) Bad, because it is not obvious to users which backend supports what features. If only features shared by all backends are supported, there is little benefit to supporting multiple backends


```
┌─────────────────────────┐
│        Model CR         │  user-facing, stable
└─────────────────────────┘
            ▲
     spec   │   status (normalized phases / conditions)
            ▼
┌─────────────────────────┐
│    Model controller     │
│  (per-backend adapter)  │
└──┬─────────┬─────────┬──┘
backend: │ llm-d   │ kserve  │ kaito | ...
 ▼         ▼         ▼
InferencePool  Inference   Workspace
+ Deployments  Service
+ EPP
```


```yaml
apiVersion: thalamus.dev/v1alpha1
kind: Model
metadata:
  name: llama-3-70b
spec:
  weights:
    type: huggingFace  # types={huggingFace, s3, ceph, ..}
    huggingFace:
      repoID: meta-llama/Llama-3.3-70B-Instruct
      tokenSecretRef:
        name: hf-token-secret
        key: token
  serving:
    backend: kserve  # backends={llm-d, kserve, kaito, ..}. Cluster-wide default if omitted.
    engine:
      image: vllm/vllm-openai:v0.25.0
      args:
        tensor-parallel-size: 4
        max-model-len: 16384
      env:
        NIM_LOW_MEMORY_MODE: "1"
      resources:
        limits:
          nvidia.com/gpu: "4"
  scheduling:
    nodeSelector:
      gpu.nvidia.com/class: H100
  accessPolicies:  # Who may call this model
    namespaceSelector:
    - teamA
status:
  phase: Ready
  conditions:
  - type: Ready
    status: "True"
    reason: BackendReady
    message: KServe InferenceService llama-3-70b is ready
    timestamp: "<timestamp>"
```


### Option 2: Direct backend exposure

Direct integration with the backend resources without any abstraction.


* (D2) Good, because there's no translation layer to build and maintain. Each backend matches upstream.
* (D1) Bad, because platform independence is lost. Every consumer must understand N backends.
* (D1) Bad, because Thalamus-specific requirements have no API home and must be proposed to and accepted by upstream.
* (D3) Bad, because switching backends is a major effort.
* (D2) Bad, because there's no consistent tooling across backends.
* (D2) Bad, because implementing hardware-specific features still requires changes to the stack, potentially across many backends.


### Option 3: Model CRD with single backend

* (D2) Good, because over time Thalamus gains increasing control over the backend as development efforts remain focused there
* (D1) Good, because model definitions are portable
* (D1) Good, because users do not have to think about the backend used
* (D1) Neutral, because single-backend lock-in is not inherently bad; it only becomes a problem if upstream lacks a fundamental feature, and even then it can be implemented on top of the chosen backend
* (D2) Neutral, because the CRD is alpha, low-cost initially, but might get more complex over time
* (D1, D3) Bad, because a tight single-backend lock-in negatively affects independence and makes further integrations with other backends harder

### Decision Matrix

To ensure an objective and transparent comparison, the following table evaluates all options using the same decision drivers.

🟢 = meets the driver fully · 🟡 = partially meets / caveats apply · 🔴 = does not meet the driver

| Decision Driver                  | Model CRD | Direct backend exposure | Model CRD with single backend |
|----------------------------------|-----------|-------------------------|-------------------------------|
| D1 Platform and stack independence  | 🟢        | 🔴                      | 🟢                            |
| D2 Operational consistency          | 🟡        | 🔴                      | 🟢                            |
| D3 Extensibility                    | 🟢        | 🟢                      | 🟡                            |

