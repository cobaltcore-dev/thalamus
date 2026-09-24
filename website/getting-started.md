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
bundled with the install (Step 1).

### Accounts

- A [Hugging Face](https://huggingface.co) account with a [read token](https://huggingface.co/settings/tokens) and access to the models you want to serve

## Step 1 — Deploy the stack

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

> **Caution:** the CRDs are installed as ordinary release resources, not via
> Helm's protected `crds/` mechanism. `helm uninstall thalamus-crds` therefore
> deletes every CRD and cascades into deleting all custom resources they own —
> including every `Model` and its inference workloads. Only uninstall it when
> you intend to tear down Thalamus.

### Already have some of these components?

If your cluster already provides some of the CRDs bundled in `thalamus-crds`,
disable the matching dependencies:

- `gateway-api` — skip with `--set gateway-api.enabled=false` (common on
  managed clusters)
- `gateway-api-inference-extension` — provides the `InferencePool` CRD
  Thalamus needs; disable only if your cluster already provides it

```bash
helm upgrade --install thalamus-crds oci://ghcr.io/cobaltcore-dev/charts/thalamus-crds \
  --namespace thalamus --create-namespace --wait \
  --version @@CHART_VERSION@@ \
  --set gateway-api.enabled=false
```

Likewise, if you run the agentgateway controller as a separate release,
disable the bundled copy on the `thalamus` release with
`--set agentgateway.enabled=false`.

## Step 2 — Create the Hugging Face secret

Model pods pull their weights from Hugging Face at startup, so the secret must
exist before you deploy a model in Step 3. Create a secret named `hf-token`
with key `HF_TOKEN` in the `thalamus` namespace:

```bash
kubectl create secret generic hf-token \
  --from-literal=HF_TOKEN="$HF_TOKEN" \
  --namespace thalamus
```

## Step 3 — Deploy a model

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

## Step 4 — Access the stack

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

## API key authentication (optional)

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

Add `-f apikey-auth.yaml` to the install command in Step 1.

Create one secret per user or client:

```bash
kubectl create secret generic apikey-<name> \
  --namespace thalamus \
  --from-literal=api-key=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')
kubectl label secret apikey-<name> --namespace thalamus thalamus-apikey=true
```

## Local development (CPU-only)

Install Thalamus with the commands in Step 1, then apply the CPU model example
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
