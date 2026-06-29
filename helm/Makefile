# renovate: datasource=github-releases depName=kubernetes-sigs/gateway-api
GATEWAY_API_VERSION                     ?= v1.5.1
# renovate: datasource=github-releases depName=kubernetes-sigs/gateway-api-inference-extension
GATEWAY_API_INFERENCE_EXTENSION_VERSION ?= v1.5.0

.PHONY: vendor-crds
vendor-crds: ## Re-download vendored upstream CRDs from their pinned release tags.
	curl -fsSL -o gateway-api/templates/crds.yaml \
		"https://github.com/kubernetes-sigs/gateway-api/releases/download/$(GATEWAY_API_VERSION)/standard-install.yaml"
	curl -fsSL -o gateway-api-inference-extension/templates/crds.yaml \
		"https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/$(GATEWAY_API_INFERENCE_EXTENSION_VERSION)/manifests.yaml"
