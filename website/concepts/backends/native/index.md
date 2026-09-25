---
title: Native Backend
---

# Native backend

The native backend is Thalamus' built-in serving backend and the default for every [`Model`](/reference/model-crd-api). The Thalamus operator reconciles each `Model` custom resource (`thalamus.cloud/v1alpha1`) into a complete per-model runtime, served behind a shared Gateway API gateway backed by [agentgateway](https://agentgateway.dev/).

![Native backend architecture](/backends-native.svg)

## Reconciliation

The operator watches `Model` resources and reconciles each one into:

- **Engine** — a vLLM `Deployment` and `Service` that serve the model.
- **Endpoint Picker (EPP)** — an llm-d deployment that selects the best engine replica for each request, scored on live signals such as KV-cache utilization, queue depth, and prefix-cache affinity.
- **InferencePool** — the Gateway API Inference Extension resource that ties the EPP to the model's engine pods.
- **Routing** — a per-model `HTTPRoute` and `AgentgatewayBackend` that attach the model to the inference gateway.

The operator also keeps a model-list `AgentgatewayPolicy` in sync, so the `/v1/models` endpoint always reflects the models that are currently `Ready` in each namespace.

## Gateway

The shared inference gateway is `agentgateway`. A body-based routing `AgentgatewayPolicy` extracts the model name from each request body, which the per-model `HTTPRoute` matches to route the request to the right model. See [Request flow](/concepts/backends/native/request-flow) for a step-by-step walkthrough.

## Artifacts and infrastructure

The `Model` deployments in Thalamus depend on additional infrastructure that needs to be provided by cluster administrators.
The container images are pulled from an OCI registry and the model weights are fetched from an object store.
The reconciled workloads run on GPU nodes prepared by the vendor GPU operators (NVIDIA, AMD, Intel), with TLS and DNS handled by cert-manager and ExternalDNS, and metrics collected by Prometheus and OpenTelemetry.
