---
title: Open WebUI
---

# Open WebUI <VerifiedBadge page="open-webui" />

[Open WebUI](https://github.com/open-webui/open-webui) is a self-hostable web interface for LLMs.
It is disabled by default. Enable it by adding `--set open-webui.enabled=true`
to the `thalamus` install command from [Getting Started, Step 1](/getting-started):

```bash test
helm upgrade --install thalamus oci://ghcr.io/cobaltcore-dev/charts/thalamus \
  --namespace thalamus --wait --reuse-values \
  --version @@CHART_VERSION@@ \
  --set open-webui.enabled=true
```

Wait for the WebUI and its gateway route to become ready:

```bash test
kubectl wait deployment/thalamus-open-webui --namespace thalamus --for=condition=Available --timeout=300s
kubectl get httproute thalamus-open-webui --namespace thalamus
```

Open WebUI is served on the gateway's `frontend` listener (port 8080). For
local clusters without a `LoadBalancer`, use port-forward:

```bash
kubectl port-forward svc/inference-gateway 8081:8080 -n thalamus
```

Then open `http://localhost:8081` in your browser.

Verify the frontend answers with the WebUI shell (Open WebUI renders
client-side, so this asserts the HTML shell, not chat content):

```bash test
curl -sf http://localhost:8081/ | grep -qi '<!doctype html'
curl -s http://localhost:8081/ | grep -qi 'open-webui\|open webui'
```

::: warning
[API key authentication](/getting-started#api-key-authentication-optional) does
not work with Open WebUI out of the box. The API key policy requires an
`Authorization: Bearer <key>` header on every request to the gateway, but
Open WebUI does not send one by default. If you enable both, you need to
configure Open WebUI to attach the bearer token to its requests.
:::
