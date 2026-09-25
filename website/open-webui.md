---
title: Open WebUI
---

# Open WebUI

[Open WebUI](https://github.com/open-webui/open-webui) is a self-hostable web interface for LLMs.
It is disabled by default. Enable it by adding `--set open-webui.enabled=true`
to the `thalamus` install command from [Getting Started, Step 1](/getting-started):

```bash
helm upgrade --install thalamus oci://ghcr.io/cobaltcore-dev/charts/thalamus \
  --namespace thalamus --wait --reuse-values \
  --version @@CHART_VERSION@@ \
  --set open-webui.enabled=true
```

Open WebUI is served on the gateway's `frontend` listener (port 8080). For
local clusters without a `LoadBalancer`, use port-forward:

```bash
kubectl port-forward svc/inference-gateway 8080:8080 -n thalamus
```

Then open `http://localhost:8080` in your browser.

::: warning
[API key authentication](/getting-started#api-key-authentication-optional) does
not work with Open WebUI out of the box. The API key policy requires an
`Authorization: Bearer <key>` header on every request to the gateway, but
Open WebUI does not send one by default. If you enable both, you need to
configure Open WebUI to attach the bearer token to its requests.
:::
