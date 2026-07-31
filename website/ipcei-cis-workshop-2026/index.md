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

---

## Naira Integration

We built a **Thalamus plugin for Naira** that keeps Naira's AI asset catalog in sync with the models currently
running in Thalamus. With the catalog populated, Naira's **MCP (Model Context Protocol) server** lets an MCP-capable
client such as Open WebUI ask live questions about Thalamus-managed inference instances. For example, clients can ask
which models are available and what engine configuration they use.

[Read full post →](/ipcei-cis-workshop-2026/naira-integration)

---

## OCM: Packaging Thalamus as a Component

We packaged the entire Thalamus installation as an OCM component, migrating our existing
Helmfile deployment 1:1. The result is a single root component with a matching ResourceGraphDefinition (RGD) that
describes the whole installation.

Values flow straight through from the root component to each HelmRelease, and OCM's localization rewrites image
references to point at our own registry, which holds both the components and their localized images. The one
exception is the GPU operator: it pulls images at runtime based on the kernel versions it discovers on each node, so
those references cannot be localized ahead of time.

This installation process is still experimental and lives on the
[`add-ocm-component`](https://github.com/henrichter-sap/thalamus/tree/add-ocm-component) branch. Thanks to the OCM
team for their help building this.

---

## OCM: Model Weights as an OCM Component

Thalamus serves large language models, but it does not manage the model weights, which come from external
providers — chiefly [Hugging Face](https://huggingface.co). That is convenient, but it also means the weights are outside our control.

Hugging Face is a single point of failure outside our control: it can go down, be
[breached](https://openai.com/index/hugging-face-model-evaluation-security-incident/), apply censorship to individual
models, or restrict users from downloading them.

For a European sovereign-cloud offering under the [ApeiroRA](https://apeirora.eu) and IPCEI-CIS umbrella, depending
on a US registry we do not operate is not acceptable. We need a registry we own and control.

[OCM (Open Component Model)](https://ocm.software) provides a protocol that meets sovereign-cloud requirements. The OCM project
ships a [**model-server**](https://github.com/open-component-model/model-server): a proxy that makes OCM components stored
in any OCI registry available through a Hugging Face-compatible API, enabling Thalamus to store models in a sovereign OCI
registry such as [Keppel](https://github.com/sapcc/keppel).

At the hackathon, we drafted an end-to-end integration of Thalamus with the OCM protocol and the model-server,
proving we can replace Hugging Face with any OCI-compatible storage.

The upstream work landed in [`jakobmoellerdev/model-server#1`](https://github.com/jakobmoellerdev/model-server/pull/1),
in collaboration with [Jakob Möller](https://github.com/jakobmoellerdev), who maintains the model-server.

[Read full post →](/ipcei-cis-workshop-2026/ocm-model-weights)
