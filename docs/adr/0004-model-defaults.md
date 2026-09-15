# 0004. Model defaults are a flat ConfigMap

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

Chosen: **Flat** + **ConfigMap.** Sharing is bimodal: either on for every `Model` (belongs in the base) or shared by one family (e.g. Qwen3 `--reasoning-parser`, written inline per `Model`). We accept Flat's partial D1 to avoid Iterative/Recursive complexity (D2). A ConfigMap keeps the base as cluster data without a redeploy (D3); we accept missing admission validation (D4) and mitigate by validating on load, failing closed to the last known good base. No versioned API is introduced, so changing the ConfigMap layout later is not a breaking change (unlike a Defaults CRD).

Three layers, precedence `base < Model < computed` (`computed` = operator-hardcoded `--port`, `kv-events-config`, ...). One ConfigMap key per component (`engine.yaml`, `epp.yaml`). Everything merges with Kubernetes strategic-merge semantics, evaluated locally in the operator (`Env` gets `patchStrategy:merge patchMergeKey:name` in the CR), except:
* `args`: concatenated `base + Model + computed`, computed last so it cannot be overridden (vLLM last-wins, no per-flag merge).
* `cache` (`VolumeSource` union): replaced when set, not merged.
* `image`: empty means inherit from base.
The base is additive-only (no single-key deletion); full opt-out via annotation applies the `Model` spec as-is. The result goes only into the engine/EPP Deployments, never back into the `Model` CR (GitOps-safe).

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
