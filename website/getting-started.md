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

The Gateway API, Gateway API inference extension, and agentgateway CRDs are
bundled with the install (Step 3).

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

By default, the Thalamus API accepts unauthenticated requests. To enable token-based authentication, save an `AgentgatewayPolicy` as a values file. The example policy below requires every request to include a valid `Authorization: Bearer <key>` header. API keys are loaded from Kubernetes secrets labeled `thalamus-apikey: "true"`.

```yaml
# apikey-auth.yaml
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

Pass it with `-f apikey-auth.yaml` when installing in Step 3.

Create one secret per user or client:

```bash
kubectl create secret generic apikey-<name> \
  --namespace thalamus \
  --from-literal=api-key=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')
kubectl label secret apikey-<name> --namespace thalamus thalamus-apikey=true
```

## Step 3 — Deploy the stack

Thalamus installs as two Helm charts. The CRDs go first: Helm cannot create
custom resources in the same release as the CRDs they depend on, so one
release installs all CRDs and a second installs the operator and gateway.

```bash
helm upgrade --install thalamus-crds oci://ghcr.io/cobaltcore-dev/charts/thalamus-crds \
  --namespace thalamus --create-namespace --wait \
  --version @@CHART_VERSION@@
helm upgrade --install thalamus oci://ghcr.io/cobaltcore-dev/charts/thalamus \
  --namespace thalamus --wait \
  --version @@CHART_VERSION@@
```

The `thalamus-crds` chart installs the Thalamus CRDs plus the pinned Gateway
API, Gateway API inference extension, and agentgateway CRDs. The `thalamus`
chart installs the operator, the inference gateway, and the agentgateway
data-plane controller. Everything is enabled by default — no extra flags needed.

To pin versions or override values, add `--version` / `--set` (or `-f values.yaml`):

```bash
helm upgrade --install thalamus oci://ghcr.io/cobaltcore-dev/charts/thalamus \
  --namespace thalamus --version 0.1.0 \
  --set operator.image.tag=0.1.0
```

### Already have some of these components?

If your cluster already provides the Gateway API CRDs (common on managed
clusters), skip the bundled copies on the `thalamus-crds` release:

```bash
helm upgrade --install thalamus-crds oci://ghcr.io/cobaltcore-dev/charts/thalamus-crds \
  --namespace thalamus --create-namespace --wait \
  --version @@CHART_VERSION@@ \
  --set gateway-api.enabled=false \
  --set gateway-api-inference-extension.enabled=false
```

Likewise, if you run the agentgateway controller as a separate release,
disable the bundled copy on the `thalamus` release with
`--set agentgateway.enabled=false`.

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

Once the pods are running, the inference gateway exposes an OpenAI-compatible
API. Use the `LoadBalancer`
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

Install Thalamus with the commands in Step 3, then apply the CPU model example
on a local cluster without GPUs:

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
