---
title: Getting Started
---

# Getting Started

Thalamus is a vendor-neutral, Kubernetes-native inference service based on
[llm-d](https://llm-d.ai/), the [Gateway API inference extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension),
and [Cortex](https://github.com/cobaltcore-dev/cortex).

Inference instances are declared as [`thalamus.cloud/v1alpha1 Model`](/reference/model-crd-api) resources.
The Thalamus operator reconciles each `Model` into the required engine, endpoint
picker, routing, and service resources.

## Prerequisites

### Tools

- [kubectl](https://kubernetes.io/docs/tasks/tools/) — Kubernetes CLI
- [helm](https://helm.sh/docs/intro/install/) — Kubernetes package manager (v3.x)
- A Kubernetes cluster, or [minikube](https://minikube.sigs.k8s.io/docs/start/) / any other local cluster for development

### Cluster requirements

Thalamus is vendor-neutral and does not install platform infrastructure. The
following are expected to already be present in the cluster:

- **GPU workloads:** a GPU driver/operator and node feature discovery for your
  hardware vendor (e.g. the [NVIDIA GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/) for NVIDIA).
- **Observability (optional):** a monitoring stack such as
  [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts) if you want Thalamus metrics scraped.

The Gateway API and Gateway API inference extension CRDs are applied as part of
the install (Step 3).

### Accounts

- A [Hugging Face](https://huggingface.co) account with a [read token](https://huggingface.co/settings/tokens) and access to the models you want to serve

## Step 1 — Create the Hugging Face secret

Thalamus pulls model weights from Hugging Face at pod startup. Create a secret
with your Hugging Face token in the `thalamus` namespace:

```bash
kubectl create namespace thalamus
```

Then create the secret. The operator expects a secret named `hf-token` with key `HF_TOKEN`.

```bash
kubectl create secret generic hf-token \
  --from-literal=HF_TOKEN="$HF_TOKEN" \
  --namespace thalamus
```

## Step 2 — Create API key secrets (optional)

By default, the Thalamus API accepts unauthenticated requests. To enable token-based authentication, deploy an `AgentgatewayPolicy` through the extraDeploy section in your Helm values. The example policy below requires every request to include a valid `Authorization: Bearer <key>` header. API keys are loaded from Kubernetes secrets labeled `thalamus-apikey: "true"`.

```yaml
thalamus:
  extraDeploy:
    apikey-auth:
      apiVersion: agentgateway.dev/v1alpha1
      kind: AgentgatewayPolicy
      metadata:
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
            secretSelector:
              matchLabels:
                thalamus-apikey: "true"
```

Create one secret per user or client:

```bash
kubectl create secret generic apikey-<name> \
  --namespace thalamus \
  --from-literal=api-key=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')
kubectl label secret apikey-<name> --namespace thalamus thalamus-apikey=true
```

## Step 3 — Deploy the stack

Thalamus builds on the [Gateway API](https://gateway-api.sigs.k8s.io/) and the
[Gateway API inference extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension).
Apply their CRDs first (pinned versions):

```bash
kubectl apply -f \
  "https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.6.2/standard-install.yaml"
kubectl apply -f \
  "https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/v1.6.0/v1-manifests.yaml"
```

Then install the Thalamus chart. By default it installs the Thalamus CRDs and
the agentgateway data plane ; everything is enabled, no extra flags needed:

```bash
helm install thalamus oci://ghcr.io/cobaltcore-dev/charts/thalamus \
  --namespace thalamus --create-namespace
```

To pin a release or override values, add `--version` / `--set` (or a values file):

```bash
helm install thalamus oci://ghcr.io/cobaltcore-dev/charts/thalamus \
  --namespace thalamus --version 2.0.0 \
  --set operator.image.tag=2.0.0
```

If you instead run the CRDs or agentgateway as separate releases, set
`--set crds.enabled=false` and/or `--set agentgateway.enabled=false`.

## Step 4 — Deploy a model

Models are declared as `thalamus.cloud/v1alpha1 Model` resources and applied
independently of the helm release. See [`examples/model-qwen3-6-27b-gpu.yaml`](https://raw.githubusercontent.com/cobaltcore-dev/thalamus/@@DOCS_VERSION@@/examples/model-qwen3-6-27b-gpu.yaml)
for a GPU example and [`examples/model-smollm2-cpu.yaml`](https://raw.githubusercontent.com/cobaltcore-dev/thalamus/@@DOCS_VERSION@@/examples/model-smollm2-cpu.yaml) for a
CPU example.

```bash
kubectl apply -f examples/model-qwen3-6-27b-gpu.yaml
```

Wait for the model to become ready:

```bash
kubectl wait model/qwen3-6-27b --namespace thalamus --for=condition=Ready --timeout=600s
```

::: details GPU example manifest
<<< ../examples/model-qwen3-6-27b-gpu.yaml{yml}
:::

For a CPU-only or local development setup, use the SmolLM2 example:

```bash
kubectl apply -f examples/model-smollm2-cpu.yaml
```

Wait for the model to become ready:

```bash
kubectl wait model/smollm2-135m --namespace thalamus --for=condition=Ready --timeout=600s
```

::: details CPU example manifest
<<< ../examples/model-smollm2-cpu.yaml{yml}
:::

## Step 5 — Access the stack

Once the pods are running, the stack is reachable in two ways.

### Gateway API (OpenAI-compatible endpoint)

The inference gateway exposes an OpenAI-compatible API. Use the `LoadBalancer`
IP or internal service address to send requests:

```bash
curl http://<gateway-ip>/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Qwen/Qwen3.6-27B",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

For local clusters without a `LoadBalancer`, use port-forward:

```bash
# OpenAI-compatible API
kubectl port-forward svc/inference-gateway 8080:80 -n thalamus
```

## Local development (CPU-only)

On a local cluster without GPUs apply the CPU model example:

```bash
kubectl apply -f examples/model-smollm2-cpu.yaml
```

> **Note:** The CPU image has no Apple Silicon / Metal acceleration. Inference
> will be significantly slower than on a GPU or native macOS runtimes like
> Ollama.

> **Note:** When using the Docker driver (default on macOS), Docker does not
> fully virtualize memory — vLLM sees the entire host RAM and will attempt to
> allocate a large fraction of it, exceeding your container limits and causing
> an OOM kill. Set `--gpu-memory-utilization` explicitly to avoid this. If a
> model fails to start without a visible error, it was most likely OOM-killed;
> adjust its `resources` for the selected model.

## Next Steps

- Browse the [Model CRD API Reference](/reference/model-crd-api) for all available fields.
- Read the [Architecture overview](/concepts/architecture) to understand how the
  operator, gateway, and endpoint picker fit together.
- Watch the [Demo](/demo) for a visual walkthrough.
