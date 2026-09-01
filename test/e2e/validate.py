#!/usr/bin/env python3
# SPDX-FileCopyrightText: Copyright 2026 SAP SE or an SAP affiliate company and cobaltcore-dev contributors
# SPDX-License-Identifier: Apache-2.0

# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "anthropic>=0.49",
#   "openai>=2,<3",
# ]
# ///

import os
import sys

from anthropic import Anthropic
from openai import OpenAI


def require_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        print(f"ERROR: required environment variable {name} is not set", file=sys.stderr)
        sys.exit(2)
    return value


MODEL = require_env("MODEL")
ROOT_URL = f"http://{require_env('GATEWAY_HOST')}:{require_env('GATEWAY_PORT')}"
PROMPT = "What is the capital of France? Answer with a single word."


def main() -> None:
    openai_client = OpenAI(base_url=f"{ROOT_URL}/v1", api_key="e2e", timeout=120.0)
    anthropic_client = Anthropic(base_url=ROOT_URL, api_key="e2e", timeout=120.0)

    ids = [model.id for model in openai_client.models.list()]
    assert MODEL in ids, f"model {MODEL!r} not in /v1/models: {ids}"
    print("PASS: GET /v1/models")

    completion = openai_client.chat.completions.create(
        model=MODEL,
        messages=[{"role": "user", "content": PROMPT}],
        max_tokens=8,
        temperature=0,
    )
    content = completion.choices[0].message.content or ""
    assert "paris" in content.lower(), f"expected 'Paris' in chat completion, got: {content!r}"
    print(f"PASS: POST /v1/chat/completions -> {content!r}")

    response = openai_client.responses.create(
        model=MODEL,
        input=PROMPT,
    )
    content = response.output_text
    assert "paris" in content.lower(), f"expected 'Paris' in response, got: {content!r}"
    print(f"PASS: POST /v1/responses -> {content!r}")

    message = anthropic_client.messages.create(
        model=MODEL,
        max_tokens=64,
        messages=[{"role": "user", "content": PROMPT}],
    )
    content = "".join(block.text for block in message.content if block.type == "text")
    assert "paris" in content.lower(), f"expected 'Paris' in message, got: {content!r}"
    print(f"PASS: POST /v1/messages -> {content!r}")

    token_count = anthropic_client.messages.count_tokens(
        model=MODEL,
        messages=[{"role": "user", "content": PROMPT}],
    )
    assert token_count.input_tokens > 0, (
        f"expected positive input_tokens, got {token_count.input_tokens!r}"
    )
    print(f"PASS: POST /v1/messages/count_tokens -> {token_count.input_tokens} tokens")

    print("All e2e tests passed!")


if __name__ == "__main__":
    main()
