---
title: Demo
---

# Demo

A walkthrough of the current Proof of Concept running on a Gardener-managed cluster in the SAP Cloud Infrastructure.

## Stack Deployment

The full Thalamus stack is deployed on a [Gardener](https://gardener.cloud)-managed
Kubernetes cluster via two Helm charts: `thalamus-infra` (GPU operator, monitoring,
gateway infrastructure) and `thalamus` (inference workloads, endpoint pickers, frontend).
To deploy the Thalamus stack onto your own Kubernetes cluster, head over to the [Getting Started guide](/getting-started).

![Stack deployment](./stack-deployment.gif)

## Model CRD

Inference instances in Thalamus are declared as Kubernetes resources using the
`thalamus.cloud/v1alpha1 Model` CRD. Each `Model` manifest captures the full
lifecycle of an inference workload in a single, version-controlled
object. This includes extensive configuration options, such as the inference engine, model weight location, GPU allocation,
autoscaling bounds, and scheduler assignment.

![Model CRD in action](./model-crd.gif)

::: warning Operator Under Development
The Thalamus operator, which reconciles `Model` resources into running inference
workloads, is currently under active development. Until it reaches general
availability, model instances are managed through Helm values as described in
the [Getting Started guide](/getting-started).
:::

See the [Model CRD API Reference](/reference/model-crd-api) for the full field
specification.

## Container Images in Keppel

In the PoC deployment, all container images are stored in and served from SAP's internal OCI registry called
[Keppel](https://github.com/sapcc/keppel).

![Container images pulled from Keppel](./keppel-images.gif)

::: warning Known Limitation
At the time of this recording, Keppel is experiencing issues replicating images
from `nvcr.io` (NVIDIA's container registry). This is a known issue and will be resolved in a forthcoming
update.
:::

## Accessing Thalamus

Thalamus exposes two access paths: a simple to access, browser-based chat frontend, and an OpenAI-compatible API endpoint for
programmatic access.

### Frontend — Open WebUI

Thalamus provides a chat interface which has the option to integrate with an identity provider,
allowing for direct access without any additional tooling or credentials setup.

<video src="./frontend.mov" controls controlslist="nodownload" width="100%"></video>

### API Endpoint — OpenCode

The inference gateway exposes an OpenAI-compatible API, making it a drop-in replacement for
any OpenAI SDK client. The recording below shows [OpenCode](https://opencode.ai)
configured to use the Thalamus PoC endpoint and sending a prompt to the
`gpt-oss-120b` model.

![OpenCode using the Thalamus API endpoint](./opencode.gif)
