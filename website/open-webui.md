---
title: Open WebUI
---

# Open WebUI

[Open WebUI](https://github.com/open-webui/open-webui) is a self-hostable web interface for LLMs.
The `thalamus` chart does not bundle Open WebUI. Install it separately once Thalamus and your Gateway are running.

## Prerequisites

1. Thalamus installed per [Getting Started, Step 1](/getting-started) (this also
   installs the Gateway API, inference extension, and agentgateway CRDs).
2. A Gateway with a `frontend` listener, e.g. the example Gateway:

```bash
kubectl apply -f examples/gateway.yaml
kubectl wait gateway/inference-gateway \
  --namespace thalamus \
  --for=condition=Programmed=True \
  --timeout=120s
```

The example manifest already contains the `api` listener (used by the operator,
see `gateway.listener`) and the `frontend` listener (used below).
If you use your own Gateway, make sure it has an HTTP `frontend` listener with
`allowedRoutes.namespaces.from: All`.

## Install

```bash
helm upgrade --install open-webui oci://ghcr.io/open-webui/helm-charts/open-webui \
  --namespace open-webui --create-namespace \
  --version 16.6.0 \
  --set ollama.enabled=false \
  --set pipelines.enabled=false \
  --set extraEnvVars[0].name=WEBUI_AUTH \
  --set extraEnvVars[0].value="False" \
  --set openaiBaseApiUrl="http://inference-gateway.thalamus.svc.cluster.local/v1" \
  --set route.enabled=true \
  --set route.parentRefs[0].name=inference-gateway \
  --set route.parentRefs[0].namespace=thalamus \
  --set route.parentRefs[0].sectionName=frontend
```

::: warning Renaming
`openaiBaseApiUrl` and `route.parentRefs` must stay in sync with your Gateway:
if you rename `gateway.name` (`inference-gateway`), change the release namespace
(`thalamus`), or use a different listener than `frontend`, update both values.
:::

## Access

Open WebUI is served on the gateway's `frontend` listener (port 8080). For
local clusters without a `LoadBalancer`, use port-forward:

```bash
kubectl port-forward svc/inference-gateway 8081:8080 -n thalamus
```

Then open `http://localhost:8081` in your browser.

## API key authentication

::: warning
[API key authentication](/getting-started#api-key-authentication-optional) does
not work with Open WebUI out of the box. The API key policy requires an
`Authorization: Bearer <key>` header on every request to the gateway, but
Open WebUI does not send one by default. If you enable both, configure Open WebUI
to attach the bearer token to its requests.
:::

## Troubleshooting

```bash
# Route created and accepted?
kubectl get httproute -n open-webui
kubectl wait httproute/open-webui \
  --namespace open-webui \
  --for=condition=Accepted \
  --timeout=120s

# Does the Gateway have the frontend listener?
kubectl get gateway inference-gateway -n thalamus -o yaml | grep -A5 listeners
```
