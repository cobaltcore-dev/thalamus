# 0001. Engine defaults are a flat ConfigMap base

## Context and Problem Statement

Many `Model`s share the same configuration: the engine image and always-on args (`--enable-auto-tool-choice`, `--enable-prefix-caching`, …). They differ mostly in parsers and hardware settings (`--tensor-parallel-size`). Thalamus supplies a base of defaults so that a `Model` stays a few lines long. This ADR decides two independent questions: how the base composes with a `Model`, and where the base lives.

## Decision Drivers

* **D1: Shared, not per-`Model`.** The base is supplied once and applies to many `Model`s; it is not written into each `Model`.
* **D2: Simple to reason about.** The composed value is derivable at a glance, and the origin of any value can be determined without running the operator.
* **D3: Platform data, not code.** The base is changeable per cluster and git-versioned, without an operator build or redeploy.

## Considered Options

Two independent choices:

* **Composition:** how the base combines with the `Model`:
  * **Flat** (one base entry per serving component, one layer under the `Model`)
  * **Iterative** (an ordered stack of base entries applied in list order, last wins)
  * **Recursive** (base entries `extends:` one another, and the operator resolves the reference graph)
* **Location:** where the base lives:
  * **Operator args** (flags on the operator Deployment)
  * **Defaults CRD** (a dedicated CRD for the base)
  * **ConfigMap** (a platform-owned data object)

## Decision Outcome

Chosen: **Flat** + **ConfigMap.**

The sharing observed is bimodal. A flag is either on for every `Model`, in which case it belongs in the base, or shared by only a few `Model`s of one family (e.g. the `--reasoning-parser` value used by every Qwen3 model), in which case the arg is written inline in each `Model` of that family. The duplication is small and self-describing. With no shared middle layer to express, iterative or recursive merging adds complexity (D2) for a requirement that does not exist yet.

A ConfigMap keeps the base as git-versioned cluster data and applies changes without a redeploy (D3). Operator args fail D3, because a default change requires an operator redeploy. The structural validation a Defaults CRD would add is not yet justified by the cost of maintaining a second schema.

The base composes flat, with precedence `base < Model < computed`, from a `ConfigMap` with one key per serving component (`engine.yaml` for `spec.serving.engine`, `epp.yaml` for `spec.serving.epp`). Each base entry is merged into the `Model` spec with strategic-merge-patch. The `Model`'s args are appended after the base args, and since vLLM gives precedence to the last occurrence of an arg, no per-flag merge is needed. The base is validated on load, and on failure the operator keeps the last known good base (fail-closed). The base is additive: a `Model` can pin a value to override a base default.

### Consequences

* A `Model` stays a few lines long, because the image and always-on args come from the base, and the source of each default is visible in git.
* The base is not schema-validated, so structural errors are detected when the operator loads the base, not when the edit is made.
* The base lives in a single ConfigMap, so a change to it is a fleet-wide change.

## Pros and Cons of the Options

### Decision Matrix

🟢 meets the driver · 🟡 partially · 🔴 not met

**Composition** (D3 concerns where the base lives, not how it composes):

| Driver | Flat | Iterative | Recursive |
|---|---|---|---|
| D1 Shared | 🟢 | 🟢 | 🟢 |
| D2 Predictable | 🟢 | 🟡 | 🔴 |

**Location** (D2 is met by all three; D3 is the discriminator):

| Driver | Operator args | Defaults CRD | ConfigMap |
|---|---|---|---|
| D1 Shared | 🟢 | 🟢 | 🟢 |
| D2 Predictable | 🟢 | 🟢 | 🟢 |
| D3 Platform data | 🔴 | 🟢 | 🟢 |

### Composition

* **Flat:** one base entry per serving component, applied to every `Model`.
  * Good, because (D1) it is supplied once and shared by every `Model`.
  * Good, because (D2) the result is the base with the `Model`'s fields winning, and there is no ordering or reference to resolve.
  * Bad, because a shared partial variation (e.g. a per-accelerator overlay) must be duplicated into each `Model`; it cannot be composed from the base.
* **Iterative:** an ordered stack of base entries applied in list order, with the `Model` applied on top; no cross-references.
  * Good, because (D1) shared partial variations (per-accelerator or per-engine overlays) are written once.
  * Bad, because (D2) determining the effective value requires replaying the stack: a flag may be set, overridden, and re-set across entries.
  * Neutral, because the flexibility is only useful once the base diverges across `Model`s.
* **Recursive:** base entries reference and extend one another; the operator resolves ordering and cycles before merging the `Model`.
  * Good, because (D1) the base is written once, and per-accelerator or per-engine variants extend it without duplication.
  * Bad, because (D2) the effective base is a graph: the operator must implement and test reference resolution, and a broken reference is a new failure mode.

### Location

* **Operator args:** flags (or a JSON/YAML blob) on the operator Deployment.
  * Good, because (D1) one flag set applies to every `Model`.
  * Bad, because (D3) a default change is an operator change that requires an edit, a redeploy, and a rolling restart. The base is not inspectable as cluster data.
* **Defaults CRD:** a dedicated kind holding the base entries, with a schema.
  * Good, because (D1) one CR per serving component applies to every `Model`.
  * Good, because (D3) a CR is cluster data that can be edited without a redeploy, and its schema validates the base structure at admission.
  * Bad, because the schema must track the engine config it validates, which adds a second schema to maintain.
* **ConfigMap:** one ConfigMap in the operator namespace, rendered from Helm values and watched by the operator.
  * Good, because (D1) one map applies to every `Model`.
  * Good, because (D3) it is git-versioned via Helm, editable per cluster, and applied without a redeploy.
  * Bad, because the base is not schema-validated at admission. A malformed base is detected only when the operator loads it, not when the edit is made.
