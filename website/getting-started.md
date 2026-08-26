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
- [helmfile](https://helmfile.readthedocs.io/en/latest/#installation) — declarative wrapper around helm (v1.x)
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
          authn:
            apiKey:
              secretLabelSelector:
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

Open WebUI connects to the inference API internally and also requires a token. Set the following in your cluster values to point Open WebUI at the secret:

```yaml
open-webui:
  openaiApiKeyExistingSecret: apikey-openwebui
  openaiApiKeyExistingSecretKey: api-key
```

## Step 3 — Deploy the stack

The `helm/helmfile.yaml.gotmpl` manifest installs the full stack as a set of
ordered helmfile releases: the Gateway API and Inference Extension CRDs, the
Thalamus CRDs, the GPU operator and node feature discovery, the agentgateway
with its CRDs, `kube-prometheus-stack` for observability, the `thalamus` chart
(operator + gateway), and finally `open-webui`. Helmfile registers the required
helm repositories and applies the releases in dependency order.

Deploy with chart defaults:

```bash
helmfile --file helm/helmfile.yaml.gotmpl apply
```

To customize values for your cluster, write a release-keyed values file and
pass it via `--state-values-file`. The top-level keys are helmfile release
names (e.g. `thalamus`, `open-webui`, `gpu-operator`, `kube-prometheus-stack`,
`agentgateway`); everything underneath is forwarded to that release as chart
values.

```bash
helmfile --file helm/helmfile.yaml.gotmpl apply \
  --state-values-file my-cluster.yaml
```

To disable optional components (e.g. on a local cluster without GPUs):

```bash
helmfile --file helm/helmfile.yaml.gotmpl apply \
  --state-values-set node-feature-discovery.enabled=false \
  --state-values-set gpu-operator.enabled=false
```

To preview changes before applying, use `helmfile diff` in place of `apply`.

## Step 4 — Deploy a model

Models are declared as `thalamus.cloud/v1alpha1 Model` resources and applied
independently of the helm release. See [`helm/model.yaml`](../helm/model.yaml)
for a GPU example and [`helm/model.cpu.yaml`](../helm/model.cpu.yaml) for a
CPU example.

```bash
kubectl apply -f helm/model.yaml
```

Wait for the model to become ready:

```bash
kubectl wait model/qwen3-6-27b --namespace thalamus --for=condition=Ready --timeout=600s
```

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
# Open WebUI (browser)
kubectl port-forward svc/inference-gateway 3000:8080 -n thalamus
```

### Open WebUI

`thalamus` includes [Open WebUI](https://github.com/open-webui/open-webui),
a browser-based chat interface. It is reachable via the hostname configured in
your `open-webui.route.hostnames` value, or via the port-forward above for local access. Open `http://localhost:3000` in your browser.

## Local development (CPU-only)

For a lightweight local setup without a GPU, disable the GPU-specific components
and apply the CPU model example:

```bash
helmfile --file helm/helmfile.yaml.gotmpl apply \
  --state-values-set node-feature-discovery.enabled=false \
  --state-values-set gpu-operator.enabled=false

kubectl apply -f helm/model.cpu.yaml
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
- Read about the [planned architecture](/concepts/architecture).
