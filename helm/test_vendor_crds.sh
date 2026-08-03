#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
fixture_root=$(mktemp -d)
trap 'rm -rf "$fixture_root"' EXIT

mkdir -p \
    "$fixture_root/bin" \
    "$fixture_root/helm/gateway-api/templates" \
    "$fixture_root/helm/gateway-api-inference-extension/templates"
cp "$repo_root/helm/Makefile" "$fixture_root/helm/Makefile"

cat > "$fixture_root/helm/gateway-api/Chart.yaml" <<'EOF'
apiVersion: v2
name: gateway-api
version: 1.5.1
EOF
cat > "$fixture_root/helm/gateway-api-inference-extension/Chart.yaml" <<'EOF'
apiVersion: v2
name: gateway-api-inference-extension
version: 1.4.0
EOF

cat > "$fixture_root/bin/curl" <<'EOF'
#!/bin/sh
set -eu
output=
while [ "$#" -gt 0 ]; do
    case "$1" in
        -o)
            output=$2
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
test -n "$output"
printf '%s\n' '# generated fixture' > "$output"
EOF
chmod +x "$fixture_root/bin/curl"

PATH="$fixture_root/bin:$PATH" make -C "$fixture_root/helm" vendor-crds \
    GATEWAY_API_VERSION=v9.9.9 \
    GATEWAY_API_INFERENCE_EXTENSION_VERSION=v8.8.8

test "$(awk '$1 == "version:" { print $2; exit }' "$fixture_root/helm/gateway-api/Chart.yaml")" = "9.9.9"
test "$(awk '$1 == "version:" { print $2; exit }' "$fixture_root/helm/gateway-api-inference-extension/Chart.yaml")" = "8.8.8"
test -s "$fixture_root/helm/gateway-api/templates/crds.yaml"
test -s "$fixture_root/helm/gateway-api-inference-extension/templates/crds.yaml"
