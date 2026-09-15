# 0001. Model defaults are a flat ConfigMap

## Context and Problem Statement

Many `Model`s share the same configuration: the engine image and always-on args (`--enable-auto-tool-choice`, `--enable-prefix-caching`, …). They differ mostly in parsers and hardware settings (`--tensor-parallel-size`). Thalamus supplies a base of defaults so that a `Model` stays a few lines long. This ADR decides two independent questions: how the base composes with a `Model`, and where the base lives.

## Decision Drivers

* **D1: Sharing expressiveness.** Base values have an expressible application scope: all `Model`s, or a subset such as an engine or accelerator family. A value shared by a subset is declared once, not restated in each member `Model`.
* **D2: Predictable outcomes.** The composed value is derivable at a glance, and the origin of any value can be determined without running the operator.
* **D3: Per-cluster mutability.** The base is changeable per cluster without an operator build or redeploy.
* **D4: Early error detection.** A malformed base is rejected at edit time (admission) or load time, not discovered later from operator logs.
* **D5: Low maintenance surface.** Carrying the base must not add definitions that must stay in sync with the engine config (e.g. a second schema).

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

### Composition

* **Flat:** one base entry per serving component, applied to every `Model`.
  * (D2) Good, because the result is the base with the `Model`'s fields winning, and there is no ordering or reference to resolve.
  * (D1) Partial, because it is supplied once and shared by every `Model`, but a shared partial variation (e.g. a per-accelerator overlay) must be duplicated into each `Model`.
* **Iterative:** an ordered stack of base entries applied in list order, with the `Model` applied on top; no cross-references.
  * (D1) Good, because shared partial variations (per-accelerator or per-engine overlays) are written once.
  * (D2) Partial, because determining the effective value requires replaying the stack: a flag may be set, overridden, and re-set across entries.
* **Recursive:** base entries reference and extend one another; the operator resolves ordering and cycles before merging the `Model`.
  * (D1) Good, because the base is written once, and per-accelerator or per-engine variants extend it without duplication.
  * (D2) Bad, because the effective base is a graph: the operator must implement and test reference resolution, and a broken reference is a new failure mode.

### Location

* **Operator args:** flags (or a JSON/YAML blob) on the operator Deployment.
  * (D1) Good, because one flag set applies to every `Model`.
  * (D5) Good, because args are trivial to add/parse.
  * (D3) Bad, because a default change is an operator change that requires an edit, a redeploy, and a rolling restart. The base is not inspectable as cluster data.
  * (D4) Bad, because there is no validation path at edit time: invalid arguments surface only as a startup crashloop or are silently ignored.
* **Defaults CRD:** a dedicated kind holding the base entries, with a schema.
  * (D1) Good, because one CR per serving component applies to every `Model`.
  * (D3) Good, because a CR is cluster data that can be edited without a redeploy, and its schema validates the base structure at admission.
  * (D4) Partial, because Kubernetes structural schema validation rejects malformed bases at admission (shape only; unknown engine args still surface at apply time).
  * (D5) Bad, because the schema must track the engine config it validates, which adds a second schema to maintain.
* **ConfigMap:** one ConfigMap in the operator namespace, rendered from Helm values and watched by the operator.
  * (D1) Good, because one map applies to every `Model`.
  * (D3) Good, because it is editable per cluster and applied without a redeploy.
  * (D5) Good, because a new key is an operator-only change; there is no schema to keep in sync.
  * (D4) Bad, because the base is not schema-validated at admission. A malformed base is detected only when the operator loads it, not when the edit is made.

## Decision Outcome

Chosen: **Flat** + **ConfigMap.** The sharing observed is bimodal: a flag is either on for every `Model`, in which case it belongs in the base, or shared by only a few `Model`s of one family (e.g. the `--reasoning-parser` value used by every Qwen3 model), in which case the arg is written inline in each `Model` of that family. The duplication is small and self-describing, and with no shared middle layer to express, we accept Flat's partial D1 rather than pay Iterative or Recursive merging's complexity (D2) for a requirement that does not exist yet. For location, a ConfigMap keeps the base as cluster data and applies changes without a redeploy (D3); operator args fail D3, because a default change requires an operator redeploy. The ConfigMap concedes D4: a malformed base is not rejected at edit time. We mitigate by validating on load and failing closed to the last known good base, so a bad edit cannot break serving; we accept that residual risk because the Defaults CRD's admission-time validation costs a second schema to maintain (D5), and the two locations differ in retraction cost: the ConfigMap's layout is an operator-internal contract, so extending it, or later replacing it with a CRD, changes no API that `Model`s target; a CRD installed today is a versioned API surface users may write against, and if the base turns out to need only a single entry, removing or breaking it later is a breaking change that requires a deprecation cycle.

The base composes flat, with precedence `base < Model < computed`, from a `ConfigMap` with one key per serving component (`engine.yaml` for `spec.serving.engine`, `epp.yaml` for `spec.serving.epp`). Each base entry is merged with the `Model` spec using strategic-merge-patch, and the result is applied only to the corresponding engine and EPP Deployments. The base defaults are never reflected in the deployed `Model` CR, to avoid conflicting with GitOps reconcilers such as Flux or Argo CD. The `Model`'s args are appended after the base args, and since vLLM gives precedence to the last occurrence of an arg, no per-flag merge is needed. The base is validated on load, and on failure the operator keeps the last known good base (fail-closed). The base is additive: a `Model` can pin a value to override a base default, or opt out of the base entirely via an annotation, in which case no base entry is merged and its spec is applied as-is.

### Consequences

* A `Model` stays a few lines long, because the image and always-on args come from the base.
* Args shared by a family of Model are written into each member, which creates some repetition.
* The base is not schema-validated, so structural errors are detected when the operator loads the base, not when the edit is made.
* The base lives in a single ConfigMap, so a change to it is a fleet-wide change.

## Pros and Cons of the Options

### Decision Matrix

🟢 meets the driver · 🟡 partially · 🔴 not met

**Composition** (D3, D4, and D5 concern where the base lives, not how it composes):

| Driver | Flat | Iterative | Recursive |
|---|---|---|---|
| D1 Expressive | 🟡 | 🟢 | 🟢 |
| D2 Predictable | 🟢 | 🟡 | 🔴 |

**Location** (D1 and D2 are met by all three; D3, D4, and D5 discriminate):

| Driver | Operator args | Defaults CRD | ConfigMap |
|---|---|---|---|
| D3 Mutable | 🔴 | 🟢 | 🟢 |
| D4 Early detection | 🔴 | 🟡 (shape only) | 🔴 |
| D5 Maintenance | 🟢 | 🔴 | 🟢 |
