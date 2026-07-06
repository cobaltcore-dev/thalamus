#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright 2026 SAP SE or an SAP affiliate company and cobaltcore-dev contributors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

BASE_URL="http://${GATEWAY_HOST:-localhost}:${GATEWAY_PORT:-8080}"
MODEL="${MODEL:-arnir0/Tiny-LLM}"
MAX_RETRIES="${MAX_RETRIES:-60}"
RETRY_INTERVAL="${RETRY_INTERVAL:-10}"

wait_for_gateway() {
  local i=0
  echo "Waiting for gateway at ${BASE_URL}..."
  while [ "$i" -lt "$MAX_RETRIES" ]; do
    if curl -sf "${BASE_URL}/v1/models" -o /dev/null; then
      echo "Gateway is ready."
      return 0
    fi
    i=$((i + 1))
    echo "  attempt ${i}/${MAX_RETRIES} — retrying in ${RETRY_INTERVAL}s"
    sleep "$RETRY_INTERVAL"
  done
  echo "ERROR: gateway did not become ready after $((MAX_RETRIES * RETRY_INTERVAL))s" >&2
  return 1
}

test_models_endpoint() {
  echo "Testing GET /v1/models..."
  local response
  response=$(curl -sf "${BASE_URL}/v1/models")
  if echo "$response" | grep -q "\"${MODEL}\""; then
    echo "PASS: GET /v1/models"
  else
    echo "FAIL: model '${MODEL}' not found in /v1/models response" >&2
    echo "$response" >&2
    return 1
  fi
}

test_completions() {
  echo "Testing POST /v1/completions..."
  local response
  response=$(curl -sf -X POST "${BASE_URL}/v1/completions" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"${MODEL}\",\"prompt\":\"Hello\",\"max_tokens\":5}")
  if echo "$response" | grep -q '"choices"'; then
    echo "PASS: POST /v1/completions"
  else
    echo "FAIL: unexpected response from POST /v1/completions" >&2
    echo "$response" >&2
    return 1
  fi
}

wait_for_gateway
test_models_endpoint
test_completions
echo "All e2e tests passed!"
