# renovate: datasource=github-releases depName=kubernetes-sigs/gateway-api
GATEWAY_API_VERSION                     ?= v1.6.2
# renovate: datasource=github-releases depName=kubernetes-sigs/gateway-api-inference-extension
GATEWAY_API_INFERENCE_EXTENSION_VERSION ?= v1.6.2
# renovate: datasource=github-releases depName=helm-unittest/helm-unittest
HELM_UNITTEST_VERSION                   ?= v1.2.1

.PHONY: deps
deps: ## Build the helm dependencies of the thalamus chart.
	helm dependency build thalamus

.PHONY: lint
lint: ## Lint the thalamus chart.
	helm lint thalamus

.PHONY: unittest
unittest: ## Run Helm unit tests for the thalamus chart.
	# TODO: use OCI installation and add --verify=false for Helm4
	@set -e; \
	if ! helm plugin list | grep -q "^unittest"; then \
		helm plugin install https://github.com/helm-unittest/helm-unittest.git --version $(HELM_UNITTEST_VERSION); \
	fi
	helm unittest helm/thalamus

.PHONY: vendor-crds
vendor-crds: ## Re-download vendored upstream CRDs from their pinned release tags.
	curl -fsSL -o gateway-api/templates/crds.yaml \
		"https://github.com/kubernetes-sigs/gateway-api/releases/download/$(GATEWAY_API_VERSION)/standard-install.yaml"
	awk -v version="$(GATEWAY_API_VERSION)" \
		'/^version:/ { print "version: " version; next } { print }' \
		gateway-api/Chart.yaml > gateway-api/Chart.yaml.tmp
	mv gateway-api/Chart.yaml.tmp gateway-api/Chart.yaml
	curl -fsSL -o gateway-api-inference-extension/templates/crds.yaml \
		"https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/$(GATEWAY_API_INFERENCE_EXTENSION_VERSION)/v1-manifests.yaml"
	awk -v version="$(GATEWAY_API_INFERENCE_EXTENSION_VERSION)" \
		'/^version:/ { print "version: " version; next } { print }' \
		gateway-api-inference-extension/Chart.yaml > gateway-api-inference-extension/Chart.yaml.tmp
	mv gateway-api-inference-extension/Chart.yaml.tmp gateway-api-inference-extension/Chart.yaml
