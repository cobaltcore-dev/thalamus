# renovate: datasource=github-releases depName=kubernetes-sigs/gateway-api
GATEWAY_API_VERSION                     ?= v1.6.3
# renovate: datasource=github-releases depName=kubernetes-sigs/gateway-api-inference-extension
GATEWAY_API_INFERENCE_EXTENSION_VERSION ?= v1.6.2

.PHONY: vendor-crds update-thalamus-deps
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
	$(MAKE) update-thalamus-deps

.PHONY: update-thalamus-deps
update-thalamus-deps: ## Bump umbrella chart dependency pins and rebuild its lockfile.
	awk -v gv="$(GATEWAY_API_VERSION)" -v iv="$(GATEWAY_API_INFERENCE_EXTENSION_VERSION)" \
		'/^  - name: gateway-api-inference-extension$$/ { dep = "i" } /^  - name: gateway-api$$/ { dep = "g" } dep == "g" && /^    version:/ { sub(/^    version: .*/, "    version: " gv); dep = "" } dep == "i" && /^    version:/ { sub(/^    version: .*/, "    version: " iv); dep = "" } { print }' \
		thalamus/Chart.yaml > thalamus/Chart.yaml.tmp
	mv thalamus/Chart.yaml.tmp thalamus/Chart.yaml
	helm dependency update thalamus
	rm -rf thalamus/charts
