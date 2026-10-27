---
title: Model Artifacts and Aliases
---

# Model artifacts and aliases

A `ModelArtifact` stores weight locations. A `Model` references it in the same
namespace.

## Base weights

```yaml
apiVersion: thalamus.cloud/v1alpha1
kind: ModelArtifact
metadata:
  name: qwen3-6-27b
  namespace: thalamus
spec:
  base: hf://Qwen/Qwen3.6-27B
---
apiVersion: thalamus.cloud/v1alpha1
kind: Model
metadata:
  name: qwen3-6-27b
  namespace: thalamus
spec:
  artifactRef:
    name: qwen3-6-27b
```

`spec.base` is required. The `hf://` scheme is supported today; more protocols
are planned. The operator passes the repository ID to vLLM.

## Draft model

`spec.draft` adds a separate draft model. For example, Gemma 4 MTP uses its
matching draft checkpoint. Unrelated fields are omitted:

```yaml
apiVersion: thalamus.cloud/v1alpha1
kind: ModelArtifact
metadata:
  name: gemma-4-26b-a4b-it
spec:
  base: hf://google/gemma-4-26B-A4B-it
  draft: hf://google/gemma-4-26B-A4B-it-assistant
---
apiVersion: thalamus.cloud/v1alpha1
kind: Model
metadata:
  name: gemma-4-26b-a4b-it
spec:
  artifactRef:
    name: gemma-4-26b-a4b-it
  serving:
    engine:
      containers:
        - name: engine
          image: ...
          args:
            - --speculative-config={"num_speculative_tokens":4,"moe_backend":"triton"}
    router:
      containers:
        - name: router
          image: ...
```

The operator passes `spec.draft` to vLLM as `--spec-model`.
`--speculative-config` takes the remaining speculative settings; omit its
`model` key.

Full field reference: [Model CRD API](/reference/model-crd-api).
