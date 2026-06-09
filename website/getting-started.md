---
title: Getting Started
---

# Getting Started

Thalamus is a vendor-neutral, Kubernetes-native inference service based on
[llm-d](https://llm-d.ai/), the [Gateway API inference extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension),
and [Cortex](https://github.com/cobaltcore-dev/cortex).

## Prerequisites

### Tools

- [kubectl](https://kubernetes.io/docs/tasks/tools/) — Kubernetes CLI
- [helm](https://helm.sh/docs/intro/install/) — Kubernetes package manager (v3.x)
- A Kubernetes cluster with GPU nodes (NVIDIA), or [minikube](https://minikube.sigs.k8s.io/docs/start/) / any other local cluster for development

### Accounts

- A [Hugging Face](https://huggingface.co) account with a [read token](https://huggingface.co/settings/tokens) and access to the models you want to serve

## Step 1 — Create the Hugging Face secret

Thalamus pulls model weights from Hugging Face at pod startup. Create a secret
with your Hugging Face token in the `thalamus` namespace:

```bash
kubectl create namespace thalamus
```

Then create the secret. The chart expects a secret named `hf-token` with key `HF_TOKEN`.

**Option A — one-time (token stays out of shell history):**

```bash
read -s HF_TOKEN
kubectl create secret generic hf-token \
  --from-literal=HF_TOKEN="$HF_TOKEN" \
  --namespace thalamus
unset HF_TOKEN
```

**Option B — from environment (if `HF_TOKEN` is already exported in your shell profile):**

```bash
kubectl create secret generic hf-token \
  --from-literal=HF_TOKEN="$HF_TOKEN" \
  --namespace thalamus
```

## Step 2 — Add Helm repositories

The charts depend on repositories that must be added before building dependencies:

```bash
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia
helm repo add open-webui https://helm.openwebui.com/
```

## Step 3 — Build chart dependencies

Run from the repo root:

```bash
helm dependency build helm/thalamus-infra
helm dependency build helm/thalamus
```

## Step 4 — Install `thalamus-infra`

`thalamus-infra` bundles the infrastructure dependencies: GPU operator,
node feature discovery, monitoring, and the Gateway API inference extension.

```bash
helm install thalamus-infra ./helm/thalamus-infra \
  --namespace thalamus
```

## Step 5 — Install `thalamus`

The `thalamus` chart installs the operator and registers the `Model` CRD.
Models are declared under the `models:` key in your values file.

> **Thalamus operator — under development**
>
> The Thalamus operator will automate model instance management and move model
> declaration from Helm values to the `thalamus.cloud/v1alpha1 Model` CRD,
> enabling fully declarative, per-resource lifecycle control. Until then, models
> are managed through the `models:` values key described below.

Write your own values file based on `example.values.yaml` or use it directly.

Then install:

```bash
helm install thalamus ./helm/thalamus \
  --namespace thalamus \
  --values example.values.yaml
```

**Caveats for values file:**
- Adjust `resources` for your selected model. If it fails without a visible
error, it might be OOM-killed due to RAM overflowing the specified `limit`.
- If your resources are limited, you may try setting up
`- "--max-model-len=8192` under `baseArgs` and explore other options to
optimize the model.
- Model slugs must be valid DNS-1035 labels: lowercase alphanumeric and hyphens
only, starting with a letter. Dots and underscores are not allowed (e.g. use
`qwen3-0-6b`, not `qwen3-0.6b`).

## Step 6 — Access the stack

Once the pods are running, the stack is reachable in two ways.

### Gateway API (OpenAI-compatible endpoint)

The inference gateway exposes an OpenAI-compatible API. Use the `LoadBalancer`
IP or internal service address to send requests:

```bash
curl http://<gateway-ip>/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Qwen/Qwen3-27B",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

For local clusters without a `LoadBalancer`, use port-forward:

```bash
kubectl port-forward svc/inference-gateway 8080:80 -n thalamus
```

### Open WebUI

`thalamus` includes [Open WebUI](https://github.com/open-webui/open-webui),
a browser-based chat interface. It is reachable via the hostname configured in
your `open-webui.route.hostnames` value, or via port-forward for local access:

```bash
kubectl port-forward svc/thalamus-open-webui 8080:80 -n thalamus
```

Then open `http://localhost:8080` in your browser.

## Local development (CPU, no GPU)

The default model and config provided are for the GPU setup. If you want the
most lightweight LLM deploy fast out of box on your laptop, replace the
configuration with the commented one and choose CPU-based lightweight models.

> **Note:** The CPU image has no Apple Silicon / Metal acceleration. Inference
> will be significantly slower than on a GPU or native macOS runtimes like
> Ollama.

> **Note:** When using the Docker driver (default on macOS), Docker does not
> fully virtualize memory — vLLM sees the entire host RAM and will attempt to
> allocate a large fraction of it, exceeding your container limits and causing
> an OOM kill. Set `--gpu-memory-utilization` explicitly to avoid this.

Observed peak usage for small CPU models is ~8 cores and ~24–25 GiB RAM,
driven primarily by KV cache pre-allocation rather than model size.

## Next Steps

- Browse the [Model CRD API Reference](/reference/model-crd-api) for all available fields.
- Read about the [planned architecture](/concepts/architecture).
