# renovate: datasource=github-releases depName=kubernetes-sigs/gateway-api
GATEWAY_API_VERSION                     ?= v1.6.1
# renovate: datasource=github-releases depName=kubernetes-sigs/gateway-api-inference-extension
GATEWAY_API_INFERENCE_EXTENSION_VERSION ?= v1.5.0

.PHONY: vendor-crds
vendor-crds: ## Re-download vendored upstream CRDs from their pinned release tags.
	curl -fsSL -o gateway-api/templates/crds.yaml \
		"https://github.com/kubernetes-sigs/gateway-api/releases/download/$(GATEWAY_API_VERSION)/standard-install.yaml"
	awk -v version="$(patsubst v%,%,$(GATEWAY_API_VERSION))" \
		'/^version:/ { print "version: " version; next } { print }' \
		gateway-api/Chart.yaml > gateway-api/Chart.yaml.tmp
	mv gateway-api/Chart.yaml.tmp gateway-api/Chart.yaml
	curl -fsSL -o gateway-api-inference-extension/templates/crds.yaml \
		"https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/$(GATEWAY_API_INFERENCE_EXTENSION_VERSION)/v1-manifests.yaml"
	awk -v version="$(patsubst v%,%,$(GATEWAY_API_INFERENCE_EXTENSION_VERSION))" \
		'/^version:/ { print "version: " version; next } { print }' \
		gateway-api-inference-extension/Chart.yaml > gateway-api-inference-extension/Chart.yaml.tmp
	mv gateway-api-inference-extension/Chart.yaml.tmp gateway-api-inference-extension/Chart.yaml
