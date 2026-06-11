<!--
# SPDX-FileCopyrightText: Copyright 2026 SAP SE or an SAP affiliate company and cobaltcore-dev contributors
#
# SPDX-License-Identifier: Apache-2.0
-->

Thalamus
========
[![Website](https://img.shields.io/badge/docs-cobaltcore--dev.github.io%2Fthalamus-blue)](https://cobaltcore-dev.github.io/thalamus/)
[![REUSE status](https://api.reuse.software/badge/github.com/cobaltcore-dev/thalamus)](https://api.reuse.software/info/github.com/cobaltcore-dev/thalamus)
<a href="https://github.com/cobaltcore-dev/thalamus"><img align="left" width="150" height="170" src="https://raw.githubusercontent.com/cobaltcore-dev/.github/main/assets/Logo_Cobalt_Core_Typo_white_background.svg"></a>

Thalamus is a vendor-neutral, Kubernetes-native inference service for sovereign LLM deployments. Weights, prompts, and context are protected and stay in the deployment perimeter.

Thalamus is built on [llm-d](https://llm-d.ai/), [vLLM](https://vllm.ai/), the [Gateway API inference extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension), and [Cortex](https://github.com/cobaltcore-dev/cortex).

Supports **Nvidia**, **AMD**, and **Intel** infrastructure.

**Full documentation** can be found on our [website](https://cobaltcore-dev.github.io/thalamus/) covering architecture, getting started, API references, and more.

Thalamus integrates natively with [CobaltCore](https://cobaltcore-dev.github.io/docs/), [Greenhouse](https://github.com/cloudoperators/greenhouse) and the broader [Apeiro Reference Architecture](https://apeirora.eu) ecosystem.

> ⚠️ **Status.** Thalamus is under active development. The value propositions and features below reflect the project vision. Not all of them are implemented today. APIs, CRDs, and chart values are subject to change.

## Value Propositions

- **Regulatory unblock.** Serves regulated and public-sector workloads that cannot use hyperscaler AI providers.
- **No vendor lock-in.** No dependency on a single hyperscaler, LLM vendor, or GPU infrastructure provider. Swap models and hardware behind a stable API.
- **IP and data protection.** Confidential computing keeps model weights and customer context inside attested GPU boundaries.
- **One stack, many targets.** The same stack runs in datacenters, sovereign cloud, and air-gapped satellite sites.

## Features

- OpenAI-compatible inference API with streaming
- Declarative model management via a Kubernetes Custom Resources including full lifecycle management
- Model-aware routing through the Gateway API Inference Extension
- KV-cache aware load balancing via the llm-d Endpoint Picker
- Autoscaling driven by time-to-first-token and queue-depth metrics
- Multi-vendor GPU support (Nvidia, AMD, Intel)
- Observability via Prometheus, OpenTelemetry, and GPU exporters. Integrates natively with [Greenhouse](https://github.com/cloudoperators).
- Ready for air-gapped satellite deployments

## Deployment Notes

`kube-prometheus-stack` in `helm/thalamus-infra` is **enabled by default**. Disable it explicitly (`kube-prometheus-stack.enabled=false`) if you run in Gardener/Greenhouse. On Gardener-onboarded clusters, the Greenhouse `kube-monitoring` plugin is used instead.

## Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/cobaltcore-dev/thalamus/issues). 

Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Security / Disclosure

If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/cobaltcore-dev/thalamus/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing

Copyright 2026 SAP SE or an SAP affiliate company and Thalamus contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/cobaltcore-dev/thalamus).
