---
title: Models
---

# Models

A `Model` is one inference workload. It combines the weights to serve with
the configuration for serving them. While the object exists, the operator
reconciles the serving resources needed to run it.

A model deployment has two Kubernetes resources:

| Resource | Purpose |
| --- | --- |
| `ModelArtifact` | Identifies the base weights and, optionally, draft weights |
| `Model` | References an artifact and defines how to serve it |

Create both resources in the same namespace. A `ModelArtifact` can be shared
by more than one `Model`.

## Base model

Set `ModelArtifact.spec.base` to the location of the base weights. Thalamus
currently supports the `hf://` scheme and passes the repository ID to vLLM as
the model path.

```yaml
apiVersion: thalamus.cloud/v1alpha1
kind: ModelArtifact
metadata:
  name: gemma-4-26b-a4b-it
  namespace: thalamus
spec:
  base: hf://google/gemma-4-26B-A4B-it
```

For gated or private models, add `HF_TOKEN` to the engine container; see
[Hugging Face authentication](#hugging-face-authentication).

## Draft model

Set `ModelArtifact.spec.draft` to add a draft model for speculative decoding.
The operator passes it to vLLM as `--spec-model`. Configure the remaining
speculative-decoding options in the engine arguments.

```yaml
spec:
  base: hf://google/gemma-4-26B-A4B-it
  draft: hf://google/gemma-4-26B-A4B-it-assistant
```

## Configure the model

The native backend is the default backend.

| Field | Purpose |
| --- | --- |
| `spec.artifactRef` | Selects the `ModelArtifact` to serve |
| `spec.replicas` | Sets the desired number of engine replicas; defaults to 1 |
| `spec.aliases` | Exposes alternate model names for routing and listing |
| `spec.serving.engine` | Engine pod spec: image, args, resources, scheduling |
| `spec.serving.router` | Router pod spec: image, args, resources, scheduling |

### Model names

By default, clients address a model by its artifact's base repository ID. Set
`spec.aliases` to expose other names. Aliases replace the base ID in gateway
routing and `GET /v1/models`, so requests using the base ID no longer match.

A common use case is keeping a stable client-facing name while switching
models. To switch clients from Gemma 4-26B to Gemma 4-31B without changing
their model name, apply the Gemma 4-31B Model without the shared alias, wait
for it to become `Ready`, then move `gemma-4-latest` from the 26B Model to the
31B Model.

### Hugging Face authentication

Gated or private models need a Hugging Face token. The token is not part of
`ModelArtifact`; create a Secret in the Model's namespace and add `HF_TOKEN` to
the engine container.

```bash
kubectl create secret generic hf-token --namespace thalamus \
  --from-literal=HF_TOKEN="$HF_TOKEN"
```

```yaml
spec:
  serving:
    engine:
      containers:
        - name: engine
          env:
            - name: HF_TOKEN
              valueFrom:
                secretKeyRef:
                  name: hf-token
                  key: HF_TOKEN
```

## Model example

This Model references the artifact with `spec.draft` set and serves Gemma 4
with its assistant model:

```yaml
apiVersion: thalamus.cloud/v1alpha1
kind: Model
metadata:
  name: gemma-4-26b-a4b-it
  namespace: thalamus
spec:
  artifactRef:
    name: gemma-4-26b-a4b-it
  aliases:
    - gemma-4-latest
  serving:
    engine:
      containers:
        - name: engine
          image: docker.io/vllm/vllm-openai:v0.27.0
          args:
            - --speculative-config={"method":"mtp","num_speculative_tokens":1}
          resources:
            requests:
              nvidia.com/gpu: 1
            limits:
              nvidia.com/gpu: 1
    router:
      containers:
        - name: router
          image: ghcr.io/llm-d/llm-d-router-endpoint-picker:v0.9.0
```

## Observe a model

```bash
kubectl get models --namespace thalamus
```

| Phase | Meaning |
| --- | --- |
| `Creating` | Serving resources are being created or are not ready yet |
| `Ready` | The model serves traffic through the gateway |
| `Inactive` | `spec.replicas` is 0 and the artifact is available; no serving resources are deployed |
| `Failed` | The model cannot run; inspect the `Ready` condition |

For the native backend, `Ready` means the engine and router deployments have
their requested replicas, the inference pool is accepted, and every model
route is accepted by the gateway. Wait for this condition before sending
inference traffic:

```bash
kubectl wait model/gemma-4-26b-a4b-it --namespace thalamus \
  --for=condition=Ready --timeout=600s
```

`Creating` is a normal transitional state. For any phase other than `Ready`,
inspect the `Ready` condition for the controller's current reason and message:

```bash
kubectl describe model/gemma-4-26b-a4b-it --namespace thalamus
```

For example, `EngineNotReady` usually means that the engine deployment is
still starting. `HTTPRouteNotAccepted` means the gateway has not accepted a
route yet. A `Failed` phase signals a condition that needs action, such as an
invalid route, a failed deployment, or an unavailable artifact.

## Manage a model

Scale a model to change its capacity or temporarily release its resources.
Delete it when the model is no longer needed.

### Scale a model

**Scale above zero.** Set `spec.replicas` to the desired number of engine
replicas. It defaults to 1. Set a higher number to serve more replicas, then
wait for `Ready` again.

**Scale to zero.** Set `replicas` to 0 to remove serving resources while
preserving the model configuration. The model reports `Inactive`. Set it to a
positive value to recreate the resources and move the model through
`Creating` to `Ready`.

### Delete a model

```bash
kubectl delete model/gemma-4-26b-a4b-it --namespace thalamus
```

Deleting a model removes its serving resources but leaves its referenced
artifact intact.

## Artifact availability

A model requires its referenced artifact at every replica count, including
zero. An unavailable artifact prevents the operator from creating serving
resources, but never prevents it from removing them.

### Recover from a missing artifact

If the artifact is missing, the model reports `Failed` with the
`ArtifactNotFound` reason. Recreate the artifact under the same name to
recover automatically, or delete the model.

A missing artifact does not block scale-to-zero. Setting `replicas` to 0
removes serving resources, but the model remains `Failed` because its
reference is still broken. Scaling it back up will not recreate serving
resources until the artifact is restored.

### Delete an artifact in use

An artifact cannot finish deletion while a model references it. When deletion
is pending, referencing models report `Failed` with the
`ArtifactTerminating` reason.

To complete the deletion, delete every referencing model or update each
`spec.artifactRef`. Running serving resources remain until the model is scaled
to zero or deleted.

For every available field, see the [Model CRD API](/reference/model-crd-api).
