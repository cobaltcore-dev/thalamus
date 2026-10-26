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

import argparse
import os
import sys
from collections.abc import Callable
from dataclasses import dataclass

from anthropic import Anthropic
from openai import OpenAI

PROMPT = "What is the capital of France? Answer with a single word."


@dataclass(frozen=True)
class Check:
    endpoint: str
    run: Callable[[OpenAI, Anthropic, str], None]


def require_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        print(
            f"ERROR: required environment variable {name} is not set", file=sys.stderr
        )
        sys.exit(2)
    return value


def check_models(openai_client: OpenAI, _: Anthropic, model: str) -> None:
    ids = [m.id for m in openai_client.models.list()]
    assert model in ids, f"model {model!r} not in /v1/models: {ids}"
    print("PASS: GET /v1/models")


def check_completions(openai_client: OpenAI, _: Anthropic, model: str) -> None:
    completion = openai_client.chat.completions.create(
        model=model,
        messages=[{"role": "user", "content": PROMPT}],
        max_tokens=8,
        temperature=0,
    )
    content = completion.choices[0].message.content or ""
    assert "paris" in content.lower(), (
        f"expected 'Paris' in chat completion, got: {content!r}"
    )
    print(f"PASS: POST /v1/chat/completions -> {content!r}")


def check_responses(openai_client: OpenAI, _: Anthropic, model: str) -> None:
    response = openai_client.responses.create(
        model=model,
        input=PROMPT,
    )
    content = response.output_text
    assert "paris" in content.lower(), f"expected 'Paris' in response, got: {content!r}"
    print(f"PASS: POST /v1/responses -> {content!r}")


def check_messages(_: OpenAI, anthropic_client: Anthropic, model: str) -> None:
    message = anthropic_client.messages.create(
        model=model,
        max_tokens=64,
        messages=[{"role": "user", "content": PROMPT}],
    )
    content = "".join(block.text for block in message.content if block.type == "text")
    assert "paris" in content.lower(), f"expected 'Paris' in message, got: {content!r}"
    print(f"PASS: POST /v1/messages -> {content!r}")


CHECKS: dict[str, Check] = {
    "models": Check("GET /v1/models", check_models),
    "completions": Check("POST /v1/chat/completions", check_completions),
    "responses": Check("POST /v1/responses", check_responses),
    "messages": Check("POST /v1/messages", check_messages),
}


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Validate the Thalamus inference gateway endpoints.",
        formatter_class=argparse.RawTextHelpFormatter,
        epilog=(
            "examples:\n"
            "  validate.py models\n"
            "  validate.py completions responses\n"
            "  validate.py --skip messages"
        ),
    )
    parser.add_argument(
        "targets",
        nargs="*",
        metavar="TARGET",
        choices=(*CHECKS, "all"),
        help="checks to run (default: all):\n  all: every check\n"
        + "\n".join(f"  {name}: {check.endpoint}" for name, check in CHECKS.items()),
    )
    parser.add_argument(
        "--skip",
        action="append",
        default=[],
        choices=CHECKS,
        metavar="TARGET",
        help="check to skip; may be repeated (default: none)",
    )
    return parser.parse_args(argv)


def select_checks(args: argparse.Namespace) -> list[str]:
    names = list(CHECKS)
    if not args.targets or "all" in args.targets:
        selected = names
    else:
        selected = [name for name in names if name in args.targets]

    return [name for name in selected if name not in args.skip]


def main(argv: list[str] | None = None) -> None:
    args = parse_args(argv)
    selected = select_checks(args)
    if not selected:
        print("No checks selected; nothing to do.")
        return

    model = require_env("MODEL")
    root_url = f"http://{require_env('GATEWAY_HOST')}:{require_env('GATEWAY_PORT')}"

    openai_client = OpenAI(base_url=f"{root_url}/v1", api_key="e2e", timeout=120.0)
    anthropic_client = Anthropic(base_url=root_url, api_key="e2e", timeout=120.0)

    failures = 0
    for name in selected:
        try:
            CHECKS[name].run(openai_client, anthropic_client, model)
        except Exception as exc:  # noqa: BLE001 - report every check failure
            failures += 1
            print(f"FAIL: {name} -> {exc}", file=sys.stderr)

    if failures:
        print(f"{failures} of {len(selected)} e2e checks failed", file=sys.stderr)
        sys.exit(1)

    print(f"All {len(selected)} e2e checks passed!")


if __name__ == "__main__":
    main()
