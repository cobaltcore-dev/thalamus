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

## Configure weights

Set `ModelArtifact.spec.base` to the location of the base weights. Thalamus
currently supports the `hf://` scheme and passes the repository ID to vLLM.

Set `ModelArtifact.spec.draft` to add a draft model for speculative decoding.
The operator passes it to vLLM as `--spec-model`. Configure the remaining
speculative-decoding options in the engine arguments.

## Configure a model

The native backend is the default backend.

| Field | Purpose |
| --- | --- |
| `spec.artifactRef` | Selects the `ModelArtifact` to serve |
| `spec.serving` | Configures the engine and router |
| `spec.replicas` | Sets the desired number of engine replicas; defaults to 1 |
| `spec.scheduling` | Constrains where the model's pods can run |

### Model names

Clients address a model by the base repository ID in its artifact. The
operator uses that name for gateway routing and in `GET /v1/models`.

## Observe a model

```bash
kubectl get models --namespace thalamus
```

| Phase | Meaning |
| --- | --- |
| `Creating` | Serving resources are being created or are not ready yet |
| `Ready` | The model serves traffic through the gateway |
| `Inactive` | `spec.replicas` is 0; no serving resources are deployed |
| `Failed` | The model cannot run; inspect the `Ready` condition |

For the native backend, `Ready` means the engine and router deployments have
their requested replicas, the inference pool is accepted, and every model
route is accepted by the gateway. Wait for this condition before sending
inference traffic:

```bash
kubectl wait model/qwen3-6-27b --namespace thalamus \
  --for=condition=Ready --timeout=600s
```

`Creating` is a normal transitional state. For any phase other than `Ready`,
inspect the `Ready` condition for the controller's current reason and message:

```bash
kubectl describe model/qwen3-6-27b --namespace thalamus
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
kubectl delete model/qwen3-6-27b --namespace thalamus
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
