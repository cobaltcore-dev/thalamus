# Thalamus Model Serving Spec

> Status: PodSpec, no mutating or validating webhook (Option 1). Defaults (container name/image/pull policy) are resolved by the operator at reconcile. Semantic pod errors surface in `status.conditions` at engine instantiation.

## Goals

- users author `engine`/`epp` as plain Kubernetes podspec (no re-implemented fields)
- minimal specs work: `epp: {container.image: ghcr.io/llm-d/llm-d-router-endpoint-picker:v0.9.0}` is valid, defaults filled by the platform
- invalid specs do not silently produce broken model deployments
- aim for low complexity; only introduce webhooks if they provide substantial value

Drivers: D1 minimal user-facing API / D2 validation fidelity / D3 operational simplicity.

## Landscape

Three independent axes:

| axis             | options                                        |
|------------------|------------------------------------------------|
| spec type        | (1) `PodSpec` / (2) `PodTemplateSpec`          |
| mutating webhook | (3) yes / (4) no                                |
| validating webhook | (5) none / (6) custom / (7) dry-run / (8) k8s import |

## Decision 1: PodSpec vs PodTemplateSpec

### PodSpec

`spec.serving.engine` is `corev1.PodSpec`; pod metadata (labels, service account) stays operator-owned.

Pros:

- operator keeps pod labels (Service selector, `thalamus.cloud/model`) and SA; no label-merge or selector-conflict logic
- smaller surface; nothing for the user to get wrong in metadata

Cons:

- users cannot set pod labels/annotations (e.g. service-mesh or scraping annotations)

### PodTemplateSpec

`spec.serving.engine` is `corev1.PodTemplateSpec` incl. `metadata`.

Pros:

- full pod control (labels/annotations)
- same shape as `Deployment.spec.template`

Cons:

- operator must merge user labels with required ones
- more surface to validate

## Decision 2: Defaults via mutating webhook

### yes

Defaulter fills container `name`, `image`, `imagePullPolicy` at `kubectl apply`; the stored object carries the effective spec.

Pros:

- `kubectl get model -o yaml` is self-describing
- defaults pinned at apply time (platform upgrade does not silently swap images of existing Models)

Cons:

- webhook down + `failurePolicy: Fail` = no Model can be applied in the namespace
- webhook down + `failurePolicy: Ignore` = operator still needs a resolve step as backstop → two code paths for the same defaults
- needs TLS bootstrap (self-signed on operator start) and RBAC for webhook configs

### no

Defaults are a pure function the operator applies at reconcile; stored object is exactly what the user wrote.

Pros:

- no admission machinery, no availability coupling, no cert ordering
- single source of truth; defaults may track the platform version

Cons:

- stored object is not self-describing (effective spec = user input + operator defaults)
- validation at apply time can only see what the user wrote, not the effective spec after defaulting

## Decision 3: Strategy for validating webhook

### none

Reconcile-time validation → `status.conditions`; final authority is the apiserver when the engine/epp Deployment is created (full pod validation there, for free).

Pros:

- one code path; no half-safety with holes
- nothing extra to operate

Cons:

- `kubectl apply` succeeds; the error shows seconds later in Model status/events

### custom rules only

Manually implement checks that cannot be expressed by kube-builder annotations.

Pros:

- apply-time feedback for thalamus-specific business rules

Cons:

- partial: schema-level and deep pod semantics issues (invalid `imagePullPolicy` enum, ports, volumes) pass through

### dry-run round-trip (untested)

Webhook POSTs a scratch pod with `dryRun=All` and converts the 422 into a denial

Pros:

- the cluster's actual validation (always in sync, no version coupling)

Cons:

- extra API call per admission

### k8s import

Call `k8svalidation.ValidatePodSpec` from `k8s.io/kubernetes`.

Pros:

- exact apiserver podspec semantics, maintained by Kubernetes

Cons:

- ~25 `replace` directives, all version-locked to one k8s release
- validates against the compiled k8s version, not the cluster's
- still excludes separate admission plugins (LimitRange, PodSecurity, ResourceQuota)
- discouraged by upstream [(kubernetes/kubernetes#79384)](https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-505627280)

In practice: Volcano runs exactly this for every `VcJob` task template (see Notes).

## Decision Matrix

| mutating          | validating        | apply-time feedback            | ops cost |
|-------------------|-------------------|--------------------------------|----------|
| no                | none              | schema only; rest in status    | lowest   |
| yes               | none              | schema + visible defaults      | medium   |
| yes               | dry-run           | schema + full pod semantics    | medium   |
| yes               | k8s import        | everything except plugins      | highest  |

## Notes

Prior art:

- **Kueue**: `PodSet.Template` is a full `corev1.PodTemplateSpec` (<https://github.com/kubernetes-sigs/kueue/blob/main/apis/kueue/v1beta2/workload_types.go#L558>); validating webhook is thin and domain-specific (<https://github.com/kubernetes-sigs/kueue/blob/main/pkg/webhooks/workload_webhook.go#L169>); k8s validation funcs are *copied*, not imported ("extracted from …", <https://github.com/kubernetes-sigs/kueue/blob/main/pkg/webhooks/workload_webhook.go#L273>); `k8s.io/kubernetes` is imported only for scheduler simulation (<https://github.com/kubernetes-sigs/kueue/blob/main/go.mod>); podspec is pre-validated because it is copied from an already-admitted Job.
- **KServe**: `LLMInferenceService` embeds `*corev1.PodSpec` in 4 places (<https://github.com/kserve/kserve/blob/main/pkg/apis/serving/v1alpha2/llm_inference_service_types.go#L187>); no `k8s.io/kubernetes` import at all; validating webhook enforces domain rules like blocked env vars (<https://github.com/kserve/kserve/blob/main/pkg/apis/serving/v1beta1/inference_service_validation.go#L318>, registered in <https://github.com/kserve/kserve/blob/main/cmd/manager/main.go>); deep pod errors surface in `status.conditions` at reconcile, not at apply.
- **KServe CRD**: `container.name` is `required` with no default — users of the podspec escape hatch must write it (<https://github.com/kserve/kserve/blob/main/config/crd/full/llmisvc/serving.kserve.io_llminferenceservices.yaml>).
- **NVIDIA Dynamo**: podspec is an *override*, not the source of truth — `DynamoComponentDeployment.spec.extraPodSpec` is `*corev1.PodSpec` inlined + a separate `mainContainer` (<https://github.com/ai-dynamo/dynamo/blob/main/deploy/operator/api/v1alpha1/common.go#L209>); a custom `MarshalJSON` works around the CRD rejecting `containers: null`. Validation follows a documented "structural validation" convention: typed validators mirror the Go API type tree and compose `field.ErrorList` via exact `field.Path` — i.e. custom rules only, k8s style (<https://github.com/ai-dynamo/dynamo/blob/main/deploy/operator/internal/webhook/validation/STRUCTURAL_VALIDATION.md>).
- **Volcano**: `VcJob.spec.tasks[].template` is a full `corev1.PodTemplateSpec` (<https://github.com/volcano-sh/volcano/blob/main/staging/src/volcano.sh/apis/pkg/apis/batch/v1alpha1/job.go#L260>); the admission webhook is the one production example of the **k8s import** option — it converts to the core internal type and calls `k8scorevalid.ValidatePodTemplate` with feature-gate-aware `PodValidationOptions` (<https://github.com/volcano-sh/volcano/blob/main/pkg/webhooks/admission/jobs/validate/admit_job.go#L329>), backed by `k8s.io/kubernetes v1.36.1` + ~30 `replace` directives (<https://github.com/volcano-sh/volcano/blob/main/go.mod>).
- **KAITO**: no podspec embedding at all — `InferenceSet`/`Workspace` are fully structured (model/hardware/replicas; the only `corev1` type referenced is `ConfigMap`, <https://github.com/kaito-project/kaito/blob/main/api/v1alpha1/inferenceset_types.go>); no `k8s.io/kubernetes` import; knative-style CRD validation webhooks per resource (<https://github.com/kaito-project/kaito/blob/main/pkg/workspace/webhooks/webhooks.go>).
- **k8s import**: `ValidatePodSpec`/`ValidatePodTemplate` live in <https://github.com/kubernetes/kubernetes/blob/master/pkg/apis/core/validation/validation.go>; import problem discussed in <https://github.com/kubernetes/kubernetes/issues/79384>.
