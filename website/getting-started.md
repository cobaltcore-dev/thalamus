---
title: Getting Started
---

# Getting Started

Thalamus is a vendor-neutral, Kubernetes-native inference service based on
[llm-d](https://llm-d.ai/), the [Gateway API inference extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension),
and [Cortex](https://github.com/cobaltcore-dev/cortex).

## Prerequisites

- A Kubernetes cluster with GPU nodes (NVIDIA)
- `kubectl` and `helm` configured for your cluster
- A [Hugging Face](https://huggingface.co) account with access to the models you want to serve

## Step 1 — Create the Hugging Face secret

Thalamus pulls model weights from Hugging Face at pod startup. Create a secret
with your Hugging Face token in the `thalamus` namespace:

```bash
kubectl create namespace thalamus

kubectl create secret generic huggingface \
  --namespace thalamus \
  --from-literal=token=<your-huggingface-token>
```

## Step 2 — Install `thalamus-infra`

`thalamus-infra` bundles the infrastructure dependencies: GPU operator,
node feature discovery, monitoring, and the Gateway API inference extension.

```bash
helm install thalamus-infra ./helm/thalamus-infra \
  --namespace thalamus
```

## Step 3 — Install `thalamus`

The `thalamus` chart installs the operator and registers the `Model` CRD.
Models are declared under the `models:` key in your values file.

Create a `my-values.yaml`:
> **Thalamus operator — under development**
>
> The Thalamus operator will automate model instance management and move model
> declaration from Helm values to the `thalamus.cloud/v1alpha1 Model` CRD,
> enabling fully declarative, per-resource lifecycle control. Until then, models
> are managed through the `models:` values key described below.

```yaml
accelerators:
  nvidia:
    image: vllm/vllm-openai:v0.9.1

models:
  - slug: qwen3-6-27b
    model: Qwen/Qwen3.6-27B
    accelerator: nvidia
    extraArgs:
      - "--tensor-parallel-size=2"
      - "--reasoning-parser=qwen3"
    resources:
      requests:
        nvidia.com/gpu: "2"
      limits:
        nvidia.com/gpu: "2"
```

Then install:

```bash
helm install thalamus ./helm/thalamus \
  --namespace thalamus \
  --values my-values.yaml
```

## Step 4 — Access the stack

Once the pods are running, the stack is reachable in two ways.

### Gateway API (OpenAI-compatible endpoint)

The inference gateway exposes an OpenAI-compatible API. Use the `LoadBalancer`
IP or internal service address to send requests:

```bash
curl http://<gateway-ip>/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3-6-27b",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Open WebUI

`thalamus` includes [Open WebUI](https://github.com/open-webui/open-webui),
a browser-based chat interface. It is reachable via the hostname configured in
your `open-webui.route.hostnames` value, or via port-forward for local access:

```bash
kubectl port-forward svc/open-webui 8080:80 -n thalamus
```

Then open `http://localhost:8080` in your browser.

## Next Steps

- Browse the [Model CRD API Reference](/reference/model-crd-api) for all available fields.
- Read about the [planned architecture](/concepts/architecture).
