---
title: Migrating to v0.2.0
---

# Migrating from v0.1.0 to v0.2.0

v0.2.0 is a breaking release: the two v0.1.0 charts (`thalamus-crds` +
`thalamus`) consolidate into a single `thalamus` chart, the `Gateway` is no
longer created by the chart, and the `Model` CRD
is refactored.

## What changed

- **One chart instead of two.** v0.1.0 shipped two charts: `thalamus-crds`
  (the Thalamus, Gateway API, inference extension, and agentgateway CRDs) and
  `thalamus` (operator + data plane). v0.2.0 is a single self-contained
  `thalamus` chart that bundles all CRDs.
- **The Gateway is now user-provided.** In v0.1.0 the chart rendered the
  `Gateway` and `AgentgatewayParameters`; since v0.2.0 you create and own it.
- **PodSpec-based serving.** `spec.serving.engine` and `spec.serving.router`
  are now `corev1.PodSpec`s with named containers `engine` and `router`.
- **Old EPP is now the router.** `spec.serving.epp` becomes
  `spec.serving.router`.
- **Scheduling moved into the pod spec.** Old `spec.scheduling.nodeSelector`
  moves to `spec.serving.engine.nodeSelector` etc. (tolerations, volumes, and
  other pod-level fields are now supported).
- **`weights.hf.tokenSecret` is now optional**.
- **`open-webui` is no longer bundled as a chart dependency.** Install it as a
  separate release if you use it.

## Migration steps

Follow the steps below to upgrade your Thalamus deployment from `v0.1.0` to
`v0.2.0` while ensuring model availability. Alternatively, you can tear down
Thalamus altogether for a simpler upgrade using `helm uninstall thalamus`, but
this will terminate your currently running models.

### 1. Backup your models

```bash
kubectl get models --all-namespaces -o yaml > models-backup.yaml
```

### 2. Consolidate to the single chart and upgrade the stack

Upgrade the `thalamus` release with
`--take-ownership` so the new chart can adopt the CRDs that the
`thalamus-crds` release previously owned. Scale the
operator to zero in the same step. This applies the new CRD schema and the
new operator image in one shot, without the operator touching any old-schema
objects:

```bash
kubectl -n thalamus annotate \
  gateways.gateway.networking.k8s.io/inference-gateway \
  helm.sh/resource-policy=keep --overwrite
helm upgrade --install thalamus oci://ghcr.io/cobaltcore-dev/charts/thalamus \
  --namespace thalamus --create-namespace --wait \
  --version v0.2.0 \
  --set operator.replicas=0 \
  --take-ownership

The upgrade re-parents every CRD to the `thalamus` release. Do **not** run
`helm uninstall thalamus-crds`. Uninstall deletes the resources listed in
that release's manifest, including the CRDs you just adopted. The old release
no longer owns anything, so just forget its Helm record (run this only after
the upgrade above, which re-parents the CRDs):

```bash
kubectl delete secret --namespace thalamus -l owner=helm,name=thalamus-crds
```

> If your cluster already provides any of the bundled CRDs, disable them,
> e.g. `--set gateway-api.enabled=false`.

### 3. Recreate the Gateway

v0.1.0 created the `Gateway` and `AgentgatewayParameters` from the chart, so on
upgrade Helm **deletes them**. v0.2.0 does not manage the Gateway, but
users are expected to create their own, e.g. by applying the example manifest:

```bash
kubectl apply -f https://raw.githubusercontent.com/cobaltcore-dev/thalamus/v0.2.0/examples/gateway.yaml
kubectl wait gateway/inference-gateway \
  --namespace thalamus --for=condition=Programmed=True --timeout=120s
```

### 4. Rewrite your Model resources in the new schema

Every setting under `spec.serving` is restructured into the PodSpec shape.
You need to manually adjust the configuration and create a `models-v0.2.0.yaml` file,
then apply it to the cluster:

```bash
# edit the manifests into the new shape, then:
kubectl apply -f models-v0.2.0.yaml
```

For example, translating a model to the v0.2.0 shape looks like this:

```diff
@@ spec:
   backend: native
   replicas: 1
-  scheduling:
-    nodeSelector:
-      disktype: ssd
   serving:
     engine:
-      image: docker.io/vllm/vllm-openai:v0.26.0
+      containers:
+        - name: engine
+          image: docker.io/vllm/vllm-openai:v0.26.0
           args:
             - --disable-uvicorn-access-log
             - --gpu-memory-utilization=0.95
             - --tensor-parallel-size=2
             - --default-chat-template-kwargs={"enable_thinking":true}
             - --reasoning-parser=qwen3
             - --speculative-config={"method":"mtp","num_speculative_tokens":2}
             - --enable-auto-tool-choice
             - --tool-call-parser=qwen3_coder
           env:
             - name: NCCL_IB_DISABLE
               value: "1"
           resources:
             requests:
               nvidia.com/gpu: 2
             limits:
               nvidia.com/gpu: 2
+      nodeSelector:
+        disktype: ssd
-    epp:
+    router:
+      containers:
+        - name: router
           image: ghcr.io/llm-d/llm-d-router-endpoint-picker:v0.9.0
           resources:
             requests:
               cpu: 100m
               memory: 256Mi
             limits:
               cpu: 200m
               memory: 512Mi
   weights:
     type: hf
     hf:
       repoId: Qwen/Qwen3.6-27B
       tokenSecret:
         name: hf-token
         key: HF_TOKEN
```

Notes:

- Each `containers` entry needs `name` and `image`; additional containers of
  other names are added as sidecars.
- `tokenSecret` is now optional; keep it if you still want to authenticate to
  Hugging Face.
- Because the same model name/namespace/kind is kept, the operator reconciles
  into the same object. Its engine/router workloads are recreated under the new
  shape when it comes back up.

### 5. Scale the operator back up

```bash
helm upgrade --install thalamus oci://ghcr.io/cobaltcore-dev/charts/thalamus \
  --namespace thalamus --create-namespace --wait \
  --version v0.2.0 \
  --set operator.replicas=1
```

The operator should now pick up the new model specs and perform
an in-place rolling update of the engine and router deployments.

### 6. Verify

```bash
kubectl get models --namespace thalamus
kubectl wait model/<model-name> --namespace thalamus --for=condition=Ready --timeout=600s
kubectl logs deploy/thalamus-operator --namespace thalamus  # no reconciliation errors
```
