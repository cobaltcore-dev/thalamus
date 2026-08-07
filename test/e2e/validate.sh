#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright 2026 SAP SE or an SAP affiliate company and cobaltcore-dev contributors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

BASE_URL="http://${GATEWAY_HOST:-localhost}:${GATEWAY_PORT:-8080}"
MODEL="${MODEL:-HuggingFaceTB/SmolLM2-135M-Instruct}"
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
  if ! response=$(curl -sf "${BASE_URL}/v1/models"); then
    echo "FAIL: request to /v1/models failed" >&2
    return 1
  fi
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
  if ! response=$(curl -sf -X POST "${BASE_URL}/v1/completions" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"${MODEL}\",\"prompt\":\"The capital of France is\",\"max_tokens\":8,\"temperature\":0}"); then
    echo "FAIL: request to /v1/completions failed" >&2
    return 1
  fi
  if echo "$response" | jq -er '.choices[0].text' | grep -qi "Paris"; then
    echo "PASS: POST /v1/completions"
  else
    echo "FAIL: expected completion to mention 'Paris'" >&2
    echo "$response" >&2
    return 1
  fi
}

test_chat_completions() {
  echo "Testing POST /v1/chat/completions..."
  local response
  if ! response=$(curl -sf -X POST "${BASE_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"${MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"What is the capital of France? Answer with a single word.\"}],\"max_tokens\":8,\"temperature\":0}"); then
    echo "FAIL: request to /v1/chat/completions failed" >&2
    return 1
  fi
  if echo "$response" | jq -er '.choices[0].message.content' | grep -qi "Paris"; then
    echo "PASS: POST /v1/chat/completions"
  else
    echo "FAIL: expected chat completion to mention 'Paris'" >&2
    echo "$response" >&2
    return 1
  fi
}

wait_for_gateway
test_models_endpoint
test_completions
test_chat_completions
echo "All e2e tests passed!"
