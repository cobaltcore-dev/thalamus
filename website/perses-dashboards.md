---
title: Perses Dashboards
---

# Perses Dashboards

Thalamus ships a set of [Perses](https://perses.dev) dashboards in
[`examples/perses-dashboards`](https://github.com/cobaltcore-dev/thalamus/tree/@@DOCS_VERSION@@/examples/perses-dashboards):

| Dashboard | Purpose |
| --- | --- |
| `thalamus-slo.json` | User-facing latency SLOs (TTFT, TPOT, E2E, ITL), queue depth, endpoints |
| `thalamus-usage.json` | Token volume and request parameters |
| `thalamus-gpu.json` | GPU utilization, memory, and health via DCGM **(only with GPU operator installed)** |
| `thalamus-vllm.json` | Engine-level debugging: scheduling, prefill/decode, KV cache |
| `thalamus-llmd.json` | Router (EPP) debugging: queueing, prefix routing, payload sizes |

![The SLO dashboard in Perses](./public/perses-dashboards.png)

_The SLO dashboard: user-facing latency per model (TTFT, TPOT, E2E, ITL),
queue depth, and endpoint availability._

## Prerequisites

- Prometheus,
  e.g. through [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack),
  which also provides [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics)
  (required for some panels).
- [Perses](https://perses.dev/helm-charts/docs/installation/) with a Prometheus datasource pointing at your Prometheus.

## Enable metric scraping

The `thalamus` chart creates `ServiceMonitor`/`PodMonitor` resources for the
operator, the vLLM engine, and the endpoint picker, and the bundled
`agentgateway` chart creates monitors for the gateway controller and its
proxies. Enable both with values passed to the `thalamus` release:

```yaml
# my-cluster.yaml (values for the thalamus release)
monitoring:
  enabled: true
agentgateway:
  monitoring:
    enabled: true
```

```bash
helm upgrade --install thalamus oci://ghcr.io/cobaltcore-dev/charts/thalamus \
  --namespace thalamus --wait \
  --version @@CHART_VERSION@@ \
  -f my-cluster.yaml
```

> [!NOTE]
> If your Prometheus selects monitors by label (e.g. the `release:` label of
> kube-prometheus-stack), add it via `monitoring.additionalLabels` for the
> operator, engine, and endpoint picker monitors and via
> `agentgateway.monitoring.serviceMonitor.extraLabels` for the gateway
> monitors.

## Load the dashboards

You can add all the dashboards manually:
1. Run `kubectl port-forward -n <perses-namespace> svc/perses 9090:8080` depending on your Perses installation and go to `localhost:9090` or the respective port.
2. Create the project named `thalamus`.
3. For each dashboard in `examples/perses-dashboards`, click "Add dashboard", specity name, click "Edit JSON" button with **"{}"** symbol, copy and paste the full dashboard code.
4. Save.

Alternatively, use [percli](https://github.com/perses/perses/blob/main/docs/cli.md) CLI
to apply the project and dashboards to your Perses instance automatically:

```bash
percli login http://localhost:9090
percli apply -f examples/perses-dashboards/project.json
for d in examples/perses-dashboards/thalamus-*.json; do
  percli apply -f "$d"
done
```

Then add a Prometheus datasource to the `thalamus` project (Data
Sources tab) and open the project in the Perses UI.
- Type name and scrape interval (recommended 15s)
- Select "Proxy" in **HTTP Settings** and write the respective in-cluster Prometheus service address, depending on your installation (for the upstream Prometheus chat with default installation into `monitoring` namespace it is `http://monitoring-kube-prometheus-prometheus.monitoring.svc.cluster.local:9090`)
