<!--
# SPDX-FileCopyrightText: Copyright 2026 SAP SE or an SAP affiliate company and cobaltcore-dev contributors
#
# SPDX-License-Identifier: Apache-2.0
-->

# OCM Separate Components

These mirror whatever is in helmfile.yaml.gotmpl for now.

## Building

Running from root, building blocks first, then the root:

```sh
# add the main components first
ocm add component-version \
  --constructor ocm/component-constructor.yaml \
  --working-directory . \
  --repository <target>

# and then you can add the root component that bundles the existing things
ocm add component-version \
  --constructor ocm/root-component-constructor.yaml \
  --working-directory . \
  --repository <target>
```

## Image Localization

Each component declares its container images as `ociImage` resources with an
`ociArtifact` access. The references are digest-pinned at build time. The RGDs
resolve every image through a `Resource` object with
`additionalStatusFields: oci: resource.access.toOCI()` and feed
registry, repository and digest into the HelmRelease values.

This means:

- Without copying resources, the access still points at the upstream registry,
  so `toOCI()` resolves to the upstream image, digest-pinned.
- When the component is transferred with `--copy-resources`, the access is
  rewritten to the target registry and the exact same RGD deploys the localized
  images. Nothing changes on the deployment side.

Charts without a dedicated digest value (node-feature-discovery, external-dns,
open-webui, agentgateway) get the combined `repository:tag@digest` reference
smuggled through their tag field; container runtimes resolve such references
by digest.

The following two large things are ignore for now because of size constraints:
- vllm
- gpu operator