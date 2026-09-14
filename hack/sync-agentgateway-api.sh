#!/bin/sh
# Sync the agentgateway/api pin in go.mod to the required agentgateway release commit.
set -eu

API=github.com/agentgateway/agentgateway/api
URL=https://github.com/agentgateway/agentgateway
TAG=$(awk '$1 == "github.com/agentgateway/agentgateway" { print $2; exit }' go.mod)
SHA=$(git ls-remote "$URL" "refs/tags/${TAG}" | cut -f1)
PEELED=$(git ls-remote "$URL" "refs/tags/${TAG}^{}" | cut -f1)
[ -n "$SHA" ] && [ -z "$PEELED" ] || { echo "error: tag ${TAG} not found" >&2; exit 1; }
VER=$(go list -m "${API}@${SHA}" | cut -d' ' -f2)
go mod edit -replace "${API}=${API}@${VER}"
go mod tidy
