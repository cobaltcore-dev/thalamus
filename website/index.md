---
layout: home

hero:
  name: Thalamus
  text: Kubernetes-Native LLM Inference
  tagline: >
    A vendor-neutral, Kubernetes-native inference service based on llm-d,
    the Gateway API inference extension, and Cortex.
  image:
    src: https://raw.githubusercontent.com/cobaltcore-dev/.github/main/assets/Logo_Cobalt_Core_Typo_white_background.svg
    alt: CobaltCore
  actions:
    - theme: brand
      text: Get Started
      link: /getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/cobaltcore-dev/thalamus

features:
  - icon: 🎬
    title: Demo
    details: >
      See the current Proof of Concept in action: stack deployment on a
      Gardener cluster, images in Keppel, Elektra navigation, and the
      OpenAI-compatible API endpoint.
    link: /demo
    linkText: Watch the demo

  - icon: 🧠
    title: Model CRD
    details: >
      Describe any LLM as a Kubernetes resource. One YAML manifest covers
      engine, weights, GPU allocation, autoscaling, and access policy.
    link: /reference/model-crd-api
    linkText: Model CRD reference

  - icon: 🏗️
    title: Architecture
    details: >
      Built on llm-d, the Gateway API inference extension, and Cortex.
      Learn how the components fit together.
    link: /concepts/architecture
    linkText: Architecture overview
---
