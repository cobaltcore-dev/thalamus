# Thalamus Model Artifacts

> Status: no decision yet (lean: 3 vs 4 to be decided).

## Goals

- flexible sources for weights (`base` + optional `draft` for speculative)
- pre-download into PVC / node hostPath; later: schedule vllm onto node with local weights
- validation/checksums (`checksum:` TBD)
- easy abstraction; backend-agnostic (ADR-0003), `native`-only first

Drivers: D1 decoupled lifecycle (prefetch, dedup across Models, validate artifacts) / D2 CEL validation / D3 per remote maintanance/changes (CRD + code).

## Landscape

### How

|          | type              | protocol              |
|----------|-------------------|-----------------------|
| inline   | (1) inline + type | (2) inline + protocol |
| separate | (3) CRD + type    | (4) CRD + protocol    |

## Decision 1: inline vs separate

### inline

`Model` carries weights directly

Pros:

- `Model` describes what to serve and where the weights come from

Cons:

- Model grows in size
- Artifact lifecycle tied to Model
    - no prefetch
    - no deduplication

### separate

Weights move to `ModelArtifact`; `Model` keeps only `spec.artifactRef.name`.

Pros:

- pull without deployment details (`materialize: true` pulls even with `replicas: 0`)
- `ModelArtifact` owns PVC; deleting `Model` keeps cache → dedup across Models
- validation/checksums target artifact, not whole `Model`

Cons:

- second object to create/reference
- PVC ownership/GC to define (owner: artifact)

## Decision 2: type vs protocol

### type

Each source is a typed struct (`s3:`, `hf:`, ...) with its own fields.

Pros:

- easy CEL validation (e.g. enforcing credentials when using S3)

Cons:

- adding a protocol requires updating the CRD (schema/validation)

### protocol

Sources are `protocol://uri` strings.

Pros:

- add protocols without CRD change
- arbitrary mounts/env via `downloader:` (Job-like: `envFrom`, `nodeSelector`)

Cons:

- CEL validation hard/impossible

## Decision Matrix

|          | type                                                                      | protocol                                                        |
|----------|---------------------------------------------------------------------------|-----------------------------------------------------------------|
| inline   | (1) single resource, validated, but Model grows & lifecycle tied to CR(D) | (2) extensible + arbitrary mounts, no CEL, lifecycle still tied |
| separate | (3) decoupled lifecycle + CEL validation on a dedicated object            | (4) decoupled lifecycle + extensible, but no CEL                |

## Considered Options

`base` + optional `draft` are both one `source` (typed struct in 1/3, URI string in 2/4).
Shared fields: `storage:` (SC/PVC/hostPath, TBD) / `downloader:` (env/mounts/nodeSelector) / `materialize:` (pull even with `replicas: 0`) / `checksum:` (TBD). Only placement differs: inline in `Model` (1/2) vs `ModelArtifact` (3/4).

### Option 1: inline + type

```yaml
spec:
  weights:
    base:
      type: s3
      s3: { bucket: ml-artifacts/kimi-k2-7-code, region: eu-central-1 }
    draft:
      type: hf
      hf: { repoId: lightseekorg/kimi-k2.7-coder-eagle3.1-mla }
```

### Option 2: inline + protocol

```yaml
spec:
  weights:
    base: hf://moonshotai/Kimi-K2.7-Code
    draft: hf://lightseekorg/kimi-k2.7-coder-eagle3.1-mla
    storage: { storageClassName: ceph-rwx }
    downloader: { envFrom: [{ secretRef: hf-token }] }
```

### Option 3: separate + type

```yaml
spec:
  artifactRef: { name: kimi-k2-7-code }
---
apiVersion: thalamus.cloud/v1alpha1
kind: ModelArtifact
metadata: { name: kimi-k2-7-code }
spec:
  weights:
    base: { type: hf, hf: { repoId: moonshotai/Kimi-K2.7-Code } }
    draft: { type: hf, hf: { repoId: lightseekorg/kimi-k2.7-coder-eagle3.1-mla } }
  materialize: true
  storage: { storageClassName: ceph-rwx }
```

### Option 4: separate + protocol

```yaml
spec:
  artifactRef: { name: kimi-k2-7-code }
---
apiVersion: thalamus.cloud/v1alpha1
kind: ModelArtifact
metadata: { name: kimi-k2-7-code }
spec:
  weights:
    base: hf://moonshotai/Kimi-K2.7-Code
    draft: hf://lightseekorg/kimi-k2.7-coder-eagle3.1-mla
  materialize: true
  storage: { storageClassName: ceph-rwx }
  downloader: { envFrom: [{ secretRef: hf-token }] }
```
