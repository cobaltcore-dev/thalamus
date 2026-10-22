---
title: Request flow
---

# Request flow

This is how a chat completion request flows through the [native backend](/concepts/backends/native/).

![Request flow](/backends-native-request-flow.svg)

1. Client sends `POST /v1/chat/completions` with `{"model": "zai-org/GLM-5.3"}` to the API endpoint (`api.thalamus.../v1`).
2. The request reaches the API listener of the shared inference gateway. An `AgentgatewayPolicy` extracts the model name from the JSON body and sets it as the `X-Gateway-Base-Model-Name` header. Other policies, for example authentication, can apply here too (see [Gateway Integrations](/gateway-integrations)).
3. The per-model `HTTPRoute` matches that header and the path, and forwards to the `AgentgatewayBackend`, which references the model's `InferencePool`.
4. The gateway uses the `InferencePool` to discover candidate engine pods and to find the corresponding Endpoint-Picker (EPP).
5. The EPP scores model replicas using real-time signals (e.g. KV-cache utilization, queue depth, ...) and returns the chosen engine deployment.
6. vLLM generates the response and streams it back through the same path.

Requests to `/v1/models` get answered directly via a model-list `AgentgatewayPolicy` that the operator keeps in sync with the `Ready` models in the namespace. Only model replicas that are ready to respond to requests are exposed to users.
