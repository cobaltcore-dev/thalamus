---
title: OCM Packaging of Thalamus
---

# OCM: Packaging Thalamus as a Component

_Author: [Henry Richter](https://github.com/henrichter-sap) · Co-author: [Artem Lytvynov](https://github.com/violog)_

Thalamus is currently installed through a collection of Helm charts and configuration files wired together by a local
helmfile. We need a packaging format that carries the whole installation description, including image
references, and can be transported through an OCI registry we operate.

## Technical deep dive

Together with the OCM team, and with direct help from [Gergely Brautigam](https://github.com/Skarlso), we
explored packaging the whole Thalamus installation as a set of OCM components. The prototype uses the idiomatic OCM
deployment path: a single root component references sub-components for Thalamus itself, CRDs, operators, monitoring,
and dependencies, and a ResourceGraphDefinition (RGD) orchestrates Flux `OCIRepository` and `HelmRelease` objects to
deploy them.

We packaged Thalamus and its dependencies into OCM components with Helm charts and container images as resources, then
used OCM's localization to rewrite image references to a registry we control. The RGD resolves each component resource
to an OCI artifact and creates the corresponding Flux `OCIRepository` and `HelmRelease` objects. The root
`component-constructor.yaml` declares the main component and its references:

```yaml
components:
  - name: github.com/cobaltcore-dev/thalamus
    version: 0.1.0
    provider:
      name: cobaltcore-dev
    resources:
      - name: thalamus-chart
        type: helmChart
        version: 0.1.0
        input:
          type: Helm/v1
          path: helm/thalamus
          repository: charts/thalamus:0.1.0
      - name: thalamus-rgd
        type: blob
        version: 0.1.0
        input:
          type: File/v1
          path: ocm-single-component/deploy/rgd.yaml
          mediaType: application/vnd.cncf.kro.resourcegraphdefinition.v1+yaml
      - name: operator-image
        type: ociImage
        version: 0.1.0
        access:
          type: ociArtifact
          imageReference: ghcr.io/cobaltcore-dev/thalamus:latest
    componentReferences:
      - name: thalamus-crds
        componentName: github.com/cobaltcore-dev/thalamus/thalamus-crds
        version: 0.1.0
      # ... gateway-api-crds, gpu-operator, kube-prometheus-stack, open-webui, etc.
```

## Future work

This packaging is still experimental, but it is a promising first step: it shows that Thalamus can be expressed as
a transportable OCM component with localized image references, and that the approach maps well to our sovereignty and
compliance requirements. Thanks to the OCM team and especially Gergely Brautigam for their help.
