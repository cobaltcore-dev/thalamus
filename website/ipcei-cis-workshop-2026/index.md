---
title: IPCEI-CIS Hackathon @ SAP Innovation Center Potsdam
---

# IPCEI-CIS Hackathon @ SAP Innovation Center Potsdam

In July 2026, the Thalamus team joined a hackathon at the SAP Innovation Center in Potsdam, collaborating with
contributors from the [NeoNephos Foundation](https://neonephos.org) and [ApeiroRA](https://apeirora.eu) as part of
the broader [IPCEI-CIS](https://ipcei-cis.eu) initiative to develop open-source European cloud and AI infrastructure.

During the event, we worked on three concrete topics. We built a [Naira](https://github.com/naira-project/naira)
plugin that feeds Thalamus model data into Naira's catalog, making it possible for Naira's MCP server to answer live
questions about running inference instances. We also worked on two
[Open Component Model (OCM)](https://ocm.software) topics: packaging Thalamus itself as an OCM component and
distributing model weights through OCM using a Hugging Face-compatible API.

## Naira Integration

We built a **Thalamus plugin for Naira** that keeps Naira's AI asset catalog in sync with the models currently
running in Thalamus. With the catalog populated, Naira's **MCP (Model Context Protocol) server** lets an MCP-capable
client such as Open WebUI ask live questions about Thalamus-managed inference instances. For example, clients can ask
which models are available and what engine configuration they use.

[Read full post →](/ipcei-cis-workshop-2026/naira-integration)

## OCM: Packaging Thalamus as a Component

Thalamus today is installed through a set of Helm charts and configuration files bundles through a helmfile. Together with the OCM team, we explored packaging the whole Thalamus installation as a single OCM component. The goal is to turn Thalamus into a versioned, transportable artifact that can be pulled from a sovereign OCI registry and installed without relying on external tooling or manual setup. This work is still experimental and needs to mature before it can replace the current installation path.

[Read full post →](/ipcei-cis-workshop-2026/ocm-packaging)

## OCM: Model Weights as an OCM Component

Thalamus currently relies on external registries such as Hugging Face for model weights, which creates a dependency outside our control for a sovereign-cloud offering. We prototyped an integration with the OCM [**model-server**](https://github.com/open-component-model/model-server) to store model weights as OCM components in an OCI registry of our choice while still exposing them through a Hugging Face-compatible API. During the hackathon we packaged a lightweight demo model, stored it in GHCR, pulled it through the model-server, and served inference from it without the weights ever touching Hugging Face.

[Read full post →](/ipcei-cis-workshop-2026/ocm-model-weights)
