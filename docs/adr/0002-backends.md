status: accepted

---
# Thalamus: Kubernetes-native LLM inference stack comparison

## Context and Problem Statement

Thalamus provides the sovereign AI infrastructure for the EU AI Cloud.
It needs a Kubernetes-native stack to serve LLMs in large-scale, production, sovereign, and air-gapped scenarios.
Multiple stacks and communities emerged within the open-source ecosystem.
This ADR evaluates all backend candidates to select the first backend for Thalamus.

## Decision Drivers

* D1: Features
    * D1.1: Declarative, user-facing CRD API with automated lifecycle management (standup, autoscaling, scale-to-zero, teardown) abstracted away from low-level Kubernetes primitives (StatefulSets, Deployments, etc.)
    * D1.2: Operational simplicity
    * D1.3: Multitenancy and model catalogue (optional)
* D2: Sovereignty (blocking)
    * D2.1: Support for heterogeneous hardware accelerators (AMD, Intel, NVIDIA)
    * D2.2: Engine-agnostic (no lock-in to a single inference engine)
* D3: Independence (blocking)
    * D3.1: Neutral foundation governance (CNCF or equivalent)
    * D3.2: Project and community maturity
    * D3.3: Multi-vendor contribution base

## Considered Options

* Option 1: KServe
* Option 2: llm-d
* Option 3: KAITO
* Option 4: ModelPlane
* Option 5: Nvidia Dynamo

## Decision Outcome

Chosen option: **llm-d** standalone _as the first Thalamus backend_, because the decision is driven by the blocking drivers: D2 sovereignty and D3 independence. Applying them eliminates every other option — KAITO and Nvidia Dynamo fail D2, ModelPlane and Nvidia Dynamo fail D3.1, and KServe fails D3.3. This leaves llm-d standalone as the only option with no red flag on any blocking driver.

**llm-d** carries several caveats: the translation layer must be built (D1.2), it has no model catalogue (D1.3), and it sits at the CNCF Sandbox tier (D3.1, D3.2). Its one red flag in D1.1 is non-blocking and absorbed by [ADR 0003](0003-inference-platform.md): the Model CRD abstracts away raw k8s resources and provides a simple and convenient UX.

### Consequences

* Good, because it is the only option that clears all blocking drivers (heterogeneous accelerators and multi-vendor governance), keeping Thalamus sovereign, CNCF-governed and free of single-vendor lock-in.
* Good, because building the abstraction in Thalamus allows its features and roadmap to evolve without depending on the acceptance and timing of major changes in external projects.
* Good, because it builds on and continues prior work in the Thalamus operator.
* Good, because it keeps operations simple for the initial SAP-internal use case by limiting implementation to required features and reusing SAP-internal platform services.
* Good, because it has a multi-vendor contribution base: commits over the last 6 months were distributed across Google, IBM, and Red Hat, with Intel, Neural Magic, and others also contributing.[^llmd-stats]
* Good, because it is engine-agnostic, so it fits large-scale, production, and air-gapped deployments.
* Neutral, because if Thalamus does not support additional mature backends, it requires its own community and a justification for not reusing existing solutions.
* Neutral, because it is a CNCF Sandbox project (accepted 2026) — the entry maturity tier — so its governance and community maturity are still proving out, though acceptable for a first backend.
* Neutral, because it has no native model catalogue.
* Bad, because it has no user-facing CRD and no abstracted lifecycle management, which means re-implementing or porting some of the existing solutions in a simple and maintainable way.

## Pros and Cons of the Options

### Option 1: KServe

CNCF Incubating project (since September 2025) that provides a standardized, Kubernetes-native model serving platform for both predictive and generative AI. It exposes declarative CRDs (`InferenceService` for predictive ML, `LLMInferenceService` for LLMs), supports OpenAI-compatible endpoints, streaming, multi-node/multi-GPU inference, autoscaling, scale-to-zero, and model caching.

* (D1.1) Good, because lifecycle management is fully abstracted in `LLMInferenceService`, with dual-track CRDs covering both predictive ML and LLMs.
* (D2.1, D2.2) Good, because it supports multiple engines and hardware accelerators, including AMD.
* (D3.1) Good, because CNCF Incubating status provides neutral governance and demonstrated production adoption.
* (D3.2) Good, because the overall project is mature, with a large historical contributor base (2,292 contributors from 627 organizations) built up over its predictive-ML history.
* (D1.3) Neutral, because it has no native model catalogue, though one might be translated into `LLMInferenceServiceConfig` resources by the Thalamus controller.
* (D1.2) Bad, because the LLM serving stack has comparatively high operational and debugging complexity.
* (D3.2, D3.3) Bad, because the `LLMInferenceService` path that Thalamus depends on is only ~13 months old (first commit July 2025) and ~87% Red Hat-authored (133/153 commits), with no other vendor contributing at a comparable sustained level on this path.[^kserve-llmisvc-stats]
* (D3.3) Bad, because feedback on relevant PRs and issues is limited, making merging of external contributions difficult.

### Option 2: llm-d standalone

CNCF Sandbox project (accepted 2026) led by a multi-vendor coalition, with Google, IBM, and Red Hat as the primary sustaining contributors. It is a distributed LLM inference framework that runs on top of vLLM and integrates with the Gateway API Inference Extension.

* (D2.1, D2.2) Good, because it runs on vLLM and is hardware- and engine-agnostic (NVIDIA, AMD, Intel, TPU, CPU), imposing no accelerator lock-in.
* (D3.3) Good, because it has a multi-vendor contribution base — over the last 6 months commits split roughly evenly across Google, IBM, and Red Hat (with Intel, Neural Magic, and others), so no single vendor steers it.[^llmd-stats]
* (D1.2) Neutral, because while operational complexity is manageable, llm-d's framework primitives must be translated into Thalamus' own abstractions.
* (D3.1, D3.2) Neutral, because it is a CNCF Sandbox project (accepted 2026) — the entry maturity tier.
* (D1.3) Neutral, because it has no native model catalogue.
* (D1.1) Bad, because it has no user-facing CRD and lifecycle management is not abstracted — components (router, scheduler/EPP, InferencePool) must be deployed and wired up manually.

### Option 3: KAITO

CNCF Sandbox project (accepted October 2024) originated at Microsoft. It is a Kubernetes AI Toolchain Operator that automates LLM inference, fine-tuning, and RAG deployment via Workspace CRDs, primarily on AKS and AKS enabled by Azure Arc.

* (D1.1) Good, because lifecycle management is fully abstracted in the `Workspace` CRD.
* (D1.3) Good, because it ships a preconfigured model catalogue that Thalamus could extend.
* (D2.1) Good, because it supports bring-your-own GPU nodes and AWS, so it is not tied to Azure despite its AKS origins.
* (D3.3) Good, because contributions are actively reviewed and merged, and the recent contributor base is mixed (~46% Microsoft over the last 6 months, with a substantial non-Microsoft tail).[^kaito-stats]
* (D3.1) Neutral, because governance is currently concentrated within Microsoft: both maintainers and the sole CODEOWNERS entry are Microsoft-affiliated.
* (D3.2) Neutral, because it is a CNCF Sandbox project (accepted Oct 2024) — the entry tier — though the codebase itself dates to 2023 and is actively developed.
* (D2.1, D2.2) Bad, because the controller currently supports only NVIDIA/CUDA: it hardcodes the `accelerator: nvidia` node label and `nvidia.com/gpu` capacity with no ROCm/AMD path, and runtimes are limited to vLLM and HuggingFace Transformers.

### Option 4: ModelPlane

Open-source control plane for AI inference launched by Upbound in March 2026 (currently v0.2.0). Built on CNCF-graduated Crossplane, it treats multiple Kubernetes GPU clusters across cloud, neocloud, and on-prem as one inference fleet. It is engine-agnostic (vLLM, SGLang, TensorRT-LLM) and exposes a single OpenAI-compatible gateway.

* (D1.1) Good, because lifecycle management is fully abstracted, treating multiple GPU clusters as one inference fleet.
* (D2.2) Good, because it supports multiple backends and is engine-agnostic (vLLM, SGLang, TensorRT-LLM), imposing no engine lock-in.
* (D1.2) Neutral, because it is a Crossplane-based control plane, adding Crossplane as an operational dependency while presenting a declarative API.
* (D1.3) Neutral, because it has no native model catalogue (only referenced in design docs, not implemented).
* (D3.1) Bad, because it is currently led by Upbound and has not yet been donated to a neutral foundation, so governance remains with a single company.
* (D3.2) Bad, because it is very early (first commit March 2026, currently v0.2.0) and there is no large-scale production case study publicly documented.

### Option 5: Nvidia Dynamo

Open-source distributed inference serving framework led by NVIDIA. It coordinates inference engines (TensorRT-LLM, vLLM, SGLang) into a multi-node LLM serving system with disaggregated prefill/decode, KV-aware routing, and SLA-driven GPU autoscaling. It provides Kubernetes operator/CRDs.

* (D1.1) Good, because lifecycle management is fully abstracted via the Kubernetes operator and CRDs.
* (D1.3) Neutral, because it has no native model catalogue.
* (D3.2) Neutral, because although it is actively developed and past its 1.0 release (v1.3+), it is led by a single vendor.
* (D2.1) Bad, because only NVIDIA GPUs are currently supported, although project materials describe it as "vendor-agnostic."
* (D3.1, D3.3) Bad, because governance and project steering remain with NVIDIA, without an independent foundation or multi-vendor steering.

### Decision Matrix

To ensure an objective and transparent comparison, the following table evaluates all options using the same decision
drivers.

🟢 = meets the driver fully · 🟡 = partially meets / caveats apply · 🔴 = does not meet the driver

| Decision Driver                               | KServe | llm-d standalone | KAITO | ModelPlane | Nvidia Dynamo |
|-----------------------------------------------|--------|------------------|-------|------------|---------------|
| D1.1 Lifecycle abstraction (CRD, no raw k8s)  | 🟢     | 🔴               | 🟢    | 🟢         | 🟢            |
| D1.2 Operational simplicity                   | 🔴     | 🟡               | 🟢    | 🟡         | 🟢            |
| D1.3 Model catalogue (optional)               | 🟡     | 🟡               | 🟢    | 🟡         | 🟡            |
| D2.1 Heterogeneous hardware accelerators      | 🟢     | 🟢               | 🔴    | 🟢         | 🔴            |
| D2.2 Engine-agnostic                          | 🟢     | 🟢               | 🔴    | 🟢         | 🔴            |
| D3.1 Neutral foundation governance (blocking) | 🟢     | 🟡               | 🟡    | 🔴         | 🔴            |
| D3.2 Project and community maturity           | 🟡     | 🟡               | 🟡    | 🔴         | 🟢            |
| D3.3 Multi-vendor contribution base (blocking)| 🔴     | 🟢               | 🟡    | 🔴         | 🔴            |

[^kserve-llmisvc-stats]: Contribution figures for the `LLMInferenceService` path were measured against a local clone of
[kserve/kserve](https://github.com/kserve/kserve) (`master`, HEAD `8c5e95894`, 2026-08-04), scoped to the generative-AI
source paths `cmd/llmisvc/`, `pkg/controller/v1alpha2/llmisvc/`, and `pkg/webhook/admission/llminferenceservice/`:

    ```sh
    LLMISVC=(cmd/llmisvc/ pkg/controller/v1alpha2/llmisvc/ pkg/webhook/admission/llminferenceservice/)

    # feature age (first commit) and total commit count
    git log --reverse --format='%ci %h %s' -- "${LLMISVC[@]}" | head -1   # 2025-07-03, PR #4557
    git log --oneline -- "${LLMISVC[@]}" | wc -l                          # 153

    # contributors and Red Hat share via Signed-off-by, including one public-affiliation mapping
    git shortlog -sne HEAD -- "${LLMISVC[@]}"
    git log --format='%an%x09%(trailers:key=Signed-off-by,valueonly)' -- "${LLMISVC[@]}" \
      | awk -F'\t' '$2 ~ /redhat\.com/ || $1 ~ /Majsak/ {c++} END{print c}'  # 133
    ```

    Result: 133/153 commits (~87%) are Red Hat-authored; the largest individual contribution share is 54 commits (~35%),
    and the top 5 contributors (all Red Hat-affiliated) account for ~75%. The repository-wide "2,292 contributors / 627
    organizations" figure reflects KServe's predictive-ML history and does not characterize this path.

[^llmd-stats]: Contribution figures for llm-d were measured against a local clone of
[llm-d/llm-d](https://github.com/llm-d/llm-d) (`main`, HEAD `44db58f`, 2026-07-17):

    ```sh
    # feature age (first commit) and total commit count
    git log --reverse --format='%ci %h %s' | head -1   # 2025-04-29
    git log --oneline | wc -l                           # 1150

    # contribution distribution via Signed-off-by, last 6 months (resolves *.ibm.com -> ibm.com, drops bots/noreply)
    git log --since=2026-02-05 --format='%(trailers:key=Signed-off-by,valueonly)' \
      | grep -oE '@[^ >]+' | sed 's/@//; s/.*ibm\.com/ibm.com/' \
      | grep -viE 'dependabot|ci-tag-bot|example.com|noreply|github.com' \
      | sort | uniq -c | sort -rn
    ```

    Result (last 6 months): Google 180, IBM 136, Red Hat 126, Intel 28, plus Neural Magic, DaoCloud, Solo, AMD, and others —
    no single vendor exceeds ~35%. All-time the same three vendors lead (Red Hat 265, Google 195, IBM 157 of 1,150 commits),
    indicating a sustained multi-vendor contribution base.

[^kaito-stats]: Governance and contribution figures for KAITO were measured against a local clone of
[kaito-project/kaito](https://github.com/kaito-project/kaito) (`main`, HEAD `56a7731b`, 2026-07-16):

    ```sh
    # governance: both maintainers and the sole CODEOWNERS entry are Microsoft-affiliated
    cat MAINTAINERS.md CODEOWNERS

    # recent contribution mix (last 6 months): Microsoft-affiliated vs. rest,
    # mapping known Microsoft authors who sign off with gmail addresses
    git log --since=2026-02-05 --format='%an|%ae' | awk -F'|' '
      {a=tolower($1); e=tolower($2)
       if (e ~ /microsoft\.com/ || a ~ /ishaan sehgal|heba|helayoty|fei guo|andy zhang|jerryzhuang|qinghui zhuang|bangqi|rambohe/) ms++
       else other++; tot++}
      END{printf "MS: %d/%d (%.0f%%), other: %d\n", ms, tot, 100*ms/tot, other}'
    ```

    Result: maintainer and CODEOWNERS roles are Microsoft-affiliated, but the last-6-month contribution mix is
    ~114/249 (~46%) Microsoft with substantial contributions from other organizations. The project started
    2023-07-27 (1,506 commits total).

    The NVIDIA/CUDA-only finding is based on controller code rather than documentation: node readiness hardcodes the `accelerator: nvidia`
    label and `nvidia.com/gpu` capacity (`pkg/workspace/resource/node.go`), GPU resources are always `nvidia.com/gpu`
    (`pkg/workspace/inference/preset_inferences.go`, `pkg/workspace/tuning/preset_tuning.go`), consts model only
    `nvidia.com/cuda.compute.*` (`pkg/utils/consts/consts.go`) with no ROCm path, and the only runtimes are `RuntimeNameVLLM`
    and `RuntimeNameHuggingfaceTransformers` (`api/v1beta1/labels.go`). The AMD mentions in `website/docs` concern node
    preparation only, not what the controller provisions or schedules.
