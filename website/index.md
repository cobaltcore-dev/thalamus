---
layout: home

hero:
  name: Thalamus
  text: Sovereign AI Infrastructure
  tagline: >
    A sovereign, vendor-neutral, and Kubernetes-native inference service based on vLLM, llm-d, the Gateway API inference extension, and Cortex.
  image:
    src: https://raw.githubusercontent.com/cobaltcore-dev/thalamus/main/docs/assets/logo/Thalamus_Logo_black.svg
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
      Gardener cluster, images in Keppel, and the
      OpenAI-compatible API endpoint.
    link: /demo
    linkText: Watch the demo

  - icon: 🛠️
    title: IPCEI-CIS Hackathon
    details: >
      Highlights from our July 2026 hackathon in Potsdam: a Naira plugin for
      Thalamus and two Open Component Model integrations for packaging the
      stack and distributing model weights.
    link: /ipcei-cis-workshop-2026/
    linkText: Read more

  - icon: 🔌
    title: Backend Hackathon
    details: >
      From our November 2026 hackathon with the IronCore team: a central,
      backend-independent API with out-of-tree providers, so new inference
      backends can be added without touching the core repository.
    link: /backend-hackathon-2026/
    linkText: Read more

  - icon: 🏗️
    title: Architecture
    details: >
      Built on vLLM, llm-d, the Gateway API inference extension, Open Component Model, and Cortex.
      Learn how the components fit together.
    link: /concepts/architecture
    linkText: Architecture overview
---
