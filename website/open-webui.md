---
title: Open WebUI
---

# Open WebUI

[Open WebUI](https://github.com/open-webui/open-webui) is a self-hostable web interface for LLMs.
The `thalamus` chart does not bundle Open WebUI. Install it separately once Thalamus and your Gateway are running.

## Prerequisites

1. Thalamus installed per [Getting Started, Step 1](/getting-started).
2. A Gateway with a `frontend` listener (already included in the example). If you use your own Gateway, make sure to
   adjust any `route` related configuration below.

## Install

```bash
helm upgrade --install open-webui oci://ghcr.io/open-webui/helm-charts/open-webui \
  --namespace open-webui --create-namespace \
  --version 16.6.0 \
  --set ollama.enabled=false \
  --set pipelines.enabled=false \
  --set extraEnvVars[0].name=WEBUI_AUTH \
  --set-string extraEnvVars[0].value="False" \
  --set openaiBaseApiUrl="http://inference-gateway.thalamus.svc.cluster.local/v1" \
  --set route.enabled=true \
  --set route.parentRefs[0].name=inference-gateway \
  --set route.parentRefs[0].namespace=thalamus \
  --set route.parentRefs[0].sectionName=frontend
```

## Access

When using the example Gateway Open WebUI is served on the gateway's `frontend` listener (port 8080). For
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

Check that the route was created and accepted:

```bash
kubectl get httproute -n open-webui
kubectl wait httproute/open-webui \
  --namespace open-webui \
  --for=condition=Accepted \
  --timeout=120s
```

Check that the Gateway has the `frontend` listener:

```bash
kubectl get gateway inference-gateway -n thalamus -o yaml | grep -A5 listeners
```
