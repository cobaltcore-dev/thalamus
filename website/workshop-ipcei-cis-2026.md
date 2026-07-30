---
title: IPCEI-CIS Hackathon @ SAP Innovation Center Potsdam
---

# IPCEI-CIS Hackathon @ SAP Innovation Center Potsdam

In July 2026, the Thalamus team joined a hackathon at the SAP Innovation Center in Potsdam, collaborating with
contributors from the [NeoNephos Foundation](https://neonephos.org) and [ApeiroRa](https://apeirora.eu) as part of
the broader [IPCEI-CIS](https://ipcei-cis.eu) initiative to develop open-source European cloud and AI infrastructure.

Over the course of the event we worked on three concrete topics. We built a [Naira](https://github.com/naira-project/naira)
plugin that feeds Thalamus model data into Naira's catalog and enables an MCP server to answer live questions about
running inference instances. We also tackled two
[Open Component Model (OCM)](https://ocm.software) stories, packaging Thalamus itself as an OCM component and
distributing model weights via the Hugging Face protocol through OCM.

---

## Naira Integration

[Naira](https://github.com/naira-project/naira) is an open-source AI Engineering Development Hub for cloud-native
teams. It provides a central catalog of AI assets (models, workflows, and infrastructure) populated by
Kubernetes-native plugins that collect data from the tools in your stack.

This hackathon produced a **Thalamus plugin for Naira**. The plugin continuously feeds Naira's catalog with the models
currently running in Thalamus, including their names, engine configuration, GPU allocation, and other operational details.
With that catalog populated, Naira's **MCP (Model Context Protocol) server** can expose those facts as tools that
any MCP-capable LLM client can call.

The demo ties it all together. Open WebUI is connected to a model served by Thalamus and has the Naira MCP server
configured as a tool provider. Because the Thalamus plugin keeps Naira's catalog up to date, the model can answer
precise, live questions about which models are running in the cluster and what their configuration looks like,
without any hardcoded knowledge.

### Demo

The screenshot below shows an Open WebUI chat session where the model uses the Naira MCP to retrieve live details
about Thalamus-managed inference instances.

<!-- Replace the placeholder below with the actual screenshot file once placed in website/public/ -->
<!-- e.g. ![Naira integration demo](/naira-demo.png) -->

::: info Screenshot placeholder
Place your screenshot at `website/public/naira-demo.png` and replace this block with:

```md
![Open WebUI chat via Thalamus and Naira](/naira-demo.png)
```
:::

You can also download the raw [Open WebUI chat export](/naira-chat-export.json) from the session.

<!-- Replace the placeholder above with the actual JSON file once placed in website/public/ -->
<!-- e.g. place the file at website/public/naira-chat-export.json -->

### How It Works

<!-- TODO: fill in plugin architecture, configuration, and usage -->

---

## OCM: Packaging Thalamus as a Component

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna
aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.
Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

---

## OCM: Model Weights as an OCM Component

Thalamus serves large language models, but it does not produce the model weights. Today those weights come from external
providers — chiefly [Hugging Face](https://huggingface.co). That is convenient, and it is also a problem we do not control.

Hugging Face can go down. It can be
[breached](https://openai.com/index/hugging-face-model-evaluation-security-incident/). It can apply censorship to individual
models, or restrict who is allowed to download them — SAP included. And a team that wants to run *its own* model, one that
was never on Hugging Face, has a hard time getting it into the pipeline at all. For a European sovereign-cloud offering under
the [ApeiroRa](https://apeirora.eu) and IPCEI-CIS umbrella, "our inference platform depends on a US registry we do not
operate" is not an acceptable answer. We need a registry we own and control.

[OCM (Open Component Model)](https://ocm.software) gives us a protocol built for exactly this: describing, signing, and
transporting software artifacts across registries in a verifiable, vendor-neutral way — a good fit for sovereign-cloud
requirements. Crucially, the OCM project ships a
[**model-server**](https://github.com/open-component-model/model-server): a proxy that makes OCM components stored in any
OCI registry look like a Hugging Face Hub (and Ollama, OpenAI, and MLflow) endpoint. Clients do not need to change — they
still think they are talking to Hugging Face. But the bytes now come from a registry we choose: GitHub Container Registry,
AWS ECR, Azure ACR, or a sovereign OCI registry such as [Keppel](https://github.com/sapcc/keppel).

At the hackathon we drafted an end-to-end integration of Thalamus with the OCM protocol and the model-server, proving we can
swap Hugging Face for any OCI-compatible storage. Concretely, we:

- **Packaged** a lightweight demo model ([`arnir0/Tiny-LLM`](https://huggingface.co/arnir0/Tiny-LLM), ~26 MB) as an OCM
  component;
- **Stored** it in the GitHub Container Registry (GHCR) as a signed, versioned OCM component;
- **Pulled** it from inside a Kubernetes cluster through the model-server's Hugging Face-compatible API;
- **Deployed** it into a [vLLM](https://github.com/vllm-project/vllm) engine managed by Thalamus and **served inference** —
  with no code in the inference path aware that the weights never touched Hugging Face.

The upstream work landed in [`jakobmoellerdev/model-server#1`](https://github.com/jakobmoellerdev/model-server/pull/1),
in collaboration with [Jakob Möller](https://github.com/jakobmoellerdev), who maintains the model-server.

### Technical design

The model-server is a stateless Go proxy. It reads OCM component descriptors from one or more OCI registries, builds an
in-memory index of the models it finds, and exposes them over four API surfaces simultaneously — the point being that an
unmodified Hugging Face, Ollama, OpenAI, or MLflow client can consume them. It does **no** inference and **no** proxying of
weights beyond streaming them through; it is pure model *distribution* with supply-chain traceability from OCM signatures.

<!-- Replace the placeholder below with the model-server architecture diagram once placed in website/public/ -->
<!-- e.g. ![model-server architecture](/model-server-overview.png) -->

::: info Diagram placeholder
Place the model-server overview diagram at `website/public/model-server-overview.png` and replace this block with:

```md
![model-server architecture: AI clients → model-server → OCM components in an OCI registry](/model-server-overview.png)
```
:::

**1. Packaging the model as an OCM component.** A model becomes an OCM component version whose resources are its files
(weights, config, tokenizer, model card). Model metadata is carried in the `ext.ocm.software/model-server.*` label namespace,
which is exactly what the proxy reads to reconstruct a Hugging Face-style model record:

```yaml
# real-model-constructor.yaml (excerpt)
components:
  - name: example.org/tiny-llm          # OCM names must be lowercase
    version: 1.0.0
    provider:
      name: arnir0
    labels:
      - name: ext.ocm.software/model-server.model-id
        value: "arnir0/Tiny-LLM"        # the public HF-style id clients ask for
      - name: ext.ocm.software/model-server.task
        value: "text-generation"
      - name: ext.ocm.software/model-server.library
        value: "transformers"
    resources:
      - name: modelsafetensors
        type: modelWeights
        input:
          type: file
          path: Tiny-LLM/model.safetensors
          mediaType: application/octet-stream
        labels:
          - name: ext.ocm.software/model-server.filename
            value: "model.safetensors"
          - name: ext.ocm.software/model-server.format
            value: "safetensors"
      # ... config.json (modelConfig), tokenizer files (tokenizer), README.md (modelCard)
```

**2. Publishing to GHCR.** The OCM CLI builds a local transport archive (CTF) and transfers it to the registry:

```bash
ocm add componentversions --repository "ctf::./transport-archive" \
  --constructor real-model-constructor.yaml
ocm transfer componentversions \
  "ctf::./transport-archive//example.org/tiny-llm:1.0.0" \
  ghcr.io/jakobmoellerdev/model-server/models --copy-resources
```

**3. Serving it as Hugging Face.** The server is configured to read that GHCR repository. When a client calls the HF API,
the proxy resolves the component, maps its labels and resources back into HF response shapes, and streams resource blobs on
`/{model}/resolve/{revision}/{file}`:

```yaml
# model-server.yaml (excerpt)
ocm:
  repositories:
    - name: ghcr-models
      type: OCIRegistry
      url: "ghcr.io/jakobmoellerdev/model-server/models"
      credentialsRef: ghcr-creds
  blobCache:
    path: /var/cache/model-server
    maxSizeBytes: 10737418240   # 10 GiB
apis:
  hfhub: { enabled: true }
credentials:
  ghcr-creds:
    username: ${GHCR_USERNAME}
    password: ${GHCR_TOKEN}
```

The core of the fetch path lives in `internal/ocm/client.go`. Two details there are what actually made the integration work:
proper OCI token exchange against GHCR (via the ORAS auth client), and caching so repeated metadata and blob reads do not
re-hit the registry:

```go
// ORAS auth client → real OCI bearer-token exchange for GHCR
orasClient := &auth.Client{
    Client: httpClient,
    Cache:  auth.NewCache(),
    Credential: auth.StaticCredential("ghcr.io", auth.Credential{
        Username: cred.Username,
        Password: cred.Password,
    }),
}

// On-disk blob cache, keyed by a hash of component:version:resource
func (c *Client) GetCachedResource(ctx context.Context, cv ComponentVersion, resourceName string) (io.ReadCloser, int64, error) {
    cacheFile := filepath.Join(c.cachePath, cacheKeyForResource(comp.Name, comp.Version, resourceName))
    if info, err := os.Stat(cacheFile); err == nil && info.Size() > 0 {
        // blob cache hit — serve from disk
        ...
    }
    // blob cache miss — fetch from registry, stream to a temp file, atomic-rename into cache
    ...
}
```

**4. Wiring it into Thalamus + vLLM.** From the inference side nothing exotic happens: vLLM's Hugging Face client is pointed
at the model-server via `HF_ENDPOINT`, and the model is declared as an ordinary Thalamus `Model`. vLLM then pulls
`arnir0/Tiny-LLM` — weights, tokenizer, config — straight from the OCM component, and serves it:

```yaml
# thalamus values (CPU profile)
thalamus:
  models:
    - slug: tiny-llm
      model: arnir0/Tiny-LLM
      accelerator: cpu
      replicas: 1
      extraArgs:
        - "--max-model-len=512"
        - "--dtype=float32"
      extraEnv:
        - name: VLLM_CPU_KVCACHE_SPACE
          value: "4"
```

### Challenges

The demo looks tidy in hindsight; getting there was a fight against a stack of small, sharp edges.

- **Weights served as Git LFS.** The biggest blocker. The proxy tagged model weights as LFS objects, so vLLM's HF client
  switched to the Git LFS batch protocol (`/info/lfs/objects/batch`) — which the server does not implement — and retried
  forever, roughly every ten seconds. The fix was to stop treating weight resources as LFS and serve them as plain blobs.
  This was masked early on because a fake stub model crashed vLLM at the *architecture-loading* step, before it ever tried
  to download the weights.
- **Placeholder model, fake architecture.** The first component shipped 16-byte stub weights and a `config.json` describing a
  non-existent `TinyGPTForCausalLM` architecture, which Transformers/vLLM reject outright. We replaced it with the real
  `arnir0/Tiny-LLM` (a genuine Llama-family model) — real `model.safetensors`, real tokenizer files, real config.
- **Multi-segment model IDs vs. the router.** Public model ids like `arnir0/Tiny-LLM` — and the far longer OCM component
  names — have slashes in them, which the [chi](https://github.com/go-chi/chi) router cannot match with a fixed
  `/{owner}/{model}` pattern. We switched the HF and OpenAI routes to catch-all patterns and parse the id (and the
  `resolve`/`raw`/`tree` verb and revision) out of the path in the handler.
- **OCM naming rules.** OCM component and resource identifiers must match a strict lowercase schema. `arnir0/Tiny-LLM` was
  rejected for the uppercase; resource names could not contain dots, hyphens, or underscores. So `Tiny-LLM` became the
  component `example.org/tiny-llm`, and `model.safetensors` became the resource `modelsafetensors`, while the *public* id the
  client sees is preserved separately in the `model-id` label.
- **IPv6 DNS timeouts.** On macOS the OCI dialer kept falling back to IPv6 and hanging. `internal/ocm/client.go` now forces a
  `tcp4` dialer with explicit timeouts.
- **GHCR authentication.** A plain HTTP client is not enough for GHCR — it needs a real OCI bearer-token exchange. We routed
  authenticated pulls through the ORAS `auth.Client` shown above.
- **OCM CLI churn.** The CLI flag syntax changed between OCM releases (the `ocm add componentversions` invocation differs
  across versions); the scripts had to detect and adapt to the installed CLI (pinned to `v0.42.0` in CI).
- **A patched OCM Go binding.** During the hackathon the OCI blob-caching behaviour we needed lived on an unreleased branch
  of the OCM Go bindings, pulled in via a `replace` directive. It was later bumped to the published `v0.0.48` and the
  workaround removed.
- **Exposing the SHA + revision handling.** vLLM's client wants a commit-like revision and content SHA in the HF model info.
  We added a revision handler and surfaced the resource digest as the model SHA so `revision="1.0.0"` resolves correctly.
- **Listing over plain OCI.** An OCI registry has no OCM "list all components" call (GHCR's catalog endpoint is disabled), so
  the index cannot be auto-populated by discovery alone. For the demo we relied on direct lookup by component name (and a
  local CTF archive, which *does* support listing) as the workaround.

### Further development

What we built is a working draft, not a production system. Getting an OCM-backed model registry into production for
real-world models is an epic-sized effort in its own right. The largest gaps:

1. **Stable multi-model deployment at real sizes.** Everything so far was validated with a ~26 MB toy model. Production
   models are hundreds of gigabytes, and packing/unpacking speed, memory pressure, and concurrency will surface a whole new
   class of problems that simply do not appear at 26 MB.
2. **Download speed.** Hugging Face parallelises downloads to fetch large models quickly. The model-server currently does a
   trivial single-threaded pull. We need to research what different OCI registries support (chunked/parallel/range reads) and
   implement real parallelism in the server.
3. **Full feature coverage.** Everything we intend to depend on must be tested deliberately: the HF API surface (and possibly
   others), caching, OCI integrations across registries, OCM compatibility, and versioning.
4. **Proper caching.** At 26 MB the cache barely matters; at 100+ GB it is the difference between a usable platform and an
   unusable one. The current blob cache is a starting point, not a solution.
5. **A large-model packaging workflow.** Turning an arbitrary model into a signed OCM component today is a hand-run script.
   This needs to become a repeatable, automated workflow before anyone can onboard models at scale.

Beyond that list, the strategic payoff is registry independence: because the model-server speaks plain OCI, the same setup
can point at [Keppel](https://github.com/sapcc/keppel) or any other sovereign OCI registry instead of GHCR, which is the
whole reason this matters for a European sovereign cloud. The concrete next steps and ownership are still to be discussed.
