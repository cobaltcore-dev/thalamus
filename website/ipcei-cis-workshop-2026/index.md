---
title: IPCEI-CIS Hackathon @ SAP Innovation Center Potsdam
---

# IPCEI-CIS Hackathon @ SAP Innovation Center Potsdam

In July 2026, the Thalamus team joined a hackathon at the SAP Innovation Center in Potsdam, collaborating with
contributors from the [NeoNephos Foundation](https://neonephos.org) and [ApeiroRa](https://apeirora.eu) as part of
the broader [IPCEI-CIS](https://ipcei-cis.eu) initiative to develop open-source European cloud and AI infrastructure.

Over the course of the event we worked on three concrete topics. We built a [Naira](https://github.com/naira-project/naira)
plugin that feeds Thalamus model data into Naira's catalog and enables an MCP server to answer live questions about
running inference instances. We also tackled two
[Open Component Model (OCM)](https://ocm.software) stories, packaging Thalamus itself as an OCM component and
distributing model weights via the Hugging Face protocol through OCM.

---

## Naira Integration

[Naira](https://github.com/naira-project/naira) is an open-source AI Engineering Development Hub for cloud-native
teams. It provides a central catalog of AI assets (models, workflows, and infrastructure) populated by
Kubernetes-native plugins that collect data from the tools in your stack.

This hackathon produced a **Thalamus plugin for Naira**. The plugin continuously feeds Naira's catalog with the models
currently running in Thalamus, including their names, engine configuration, GPU allocation, and other operational details.
With that catalog populated, Naira's **MCP (Model Context Protocol) server** can expose those facts as tools that
any MCP-capable LLM client can call.

The demo ties it all together. Open WebUI is connected to a model served by Thalamus and has the Naira MCP server
configured as a tool provider. Because the Thalamus plugin keeps Naira's catalog up to date, the model can answer
precise, live questions about which models are running in the cluster and what their configuration looks like,
without any hardcoded knowledge.

### Demo

The screenshot below shows an Open WebUI chat session where the model uses the Naira MCP to retrieve live details
about Thalamus-managed inference instances.

<!-- Replace the placeholder below with the actual screenshot file once placed in website/public/ -->
<!-- e.g. ![Naira integration demo](/naira-demo.png) -->

::: info Screenshot placeholder
Place your screenshot at `website/public/naira-demo.png` and replace this block with:

```md
![Open WebUI chat via Thalamus and Naira](/naira-demo.png)
```
:::

You can also download the raw [Open WebUI chat export](/naira-chat-export.json) from the session.

<!-- Replace the placeholder above with the actual JSON file once placed in website/public/ -->
<!-- e.g. place the file at website/public/naira-chat-export.json -->

### How It Works

<!-- TODO: fill in plugin architecture, configuration, and usage -->

---

## OCM: Packaging Thalamus as a Component

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna
aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.
Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

---

## OCM: Model Weights as an OCM Component

Thalamus serves large language models, but it does not manage the model weights, which come from external
providers — chiefly [Hugging Face](https://huggingface.co). That is convenient, and it is also a problem we do not control.

Hugging Face is a single point of failure we do not own:

- It can go down.
- It can be [breached](https://openai.com/index/hugging-face-model-evaluation-security-incident/).
- It can apply censorship to individual models, or restrict who is allowed to download them — SAP included.
- A team that wants to run *its own* model needs to upload it to HuggingFace first, which may be complicated.

For a European sovereign-cloud offering under the [ApeiroRa](https://apeirora.eu) and IPCEI-CIS umbrella, "our inference
platform depends on a US registry we do not operate" is not an acceptable answer. We need a registry we own and control.

[OCM (Open Component Model)](https://ocm.software) provides a protocol built for exactly this: describing, signing, and
transporting software artifacts across registries in a verifiable, vendor-neutral way — a good fit for sovereign-cloud
requirements. The OCM project ships a
[**model-server**](https://github.com/open-component-model/model-server): a proxy that makes OCM components stored in any
OCI registry look like a Hugging Face Hub (and Ollama, OpenAI, and MLflow) endpoint. Clients do not need to change — they
still think they are talking to Hugging Face. But the bytes now come from a registry we choose: GitHub Container Registry,
AWS ECR, Azure ACR, or a sovereign OCI registry such as [Keppel](https://github.com/sapcc/keppel).

At the hackathon we drafted an end-to-end integration of Thalamus with the OCM protocol and the model-server, proving we can
swap Hugging Face for any OCI-compatible storage. Concretely, we:

- **Packaged** a lightweight demo model ([`arnir0/Tiny-LLM`](https://huggingface.co/arnir0/Tiny-LLM), ~26 MB) as an OCM
  component;
- **Stored** it in the GitHub Container Registry (GHCR) as a versioned OCM component;
- **Pulled** it from inside a Kubernetes cluster through the model-server's Hugging Face-compatible API;
- **Deployed** it into a [vLLM](https://github.com/vllm-project/vllm) engine managed by Thalamus and **served inference** —
  with no code in the inference path aware that the weights never touched Hugging Face.

The upstream work landed in [`jakobmoellerdev/model-server#1`](https://github.com/jakobmoellerdev/model-server/pull/1),
in collaboration with [Jakob Möller](https://github.com/jakobmoellerdev), who maintains the model-server.

... [Continue reading →](/ipcei-cis-workshop-2026/ocm-model-weights)
