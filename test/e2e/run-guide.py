#!/usr/bin/env python3
# SPDX-FileCopyrightText: Copyright 2026 SAP SE or an SAP affiliate company and cobaltcore-dev contributors
# SPDX-License-Identifier: Apache-2.0
#
#
# Block convention:
#   ```bash test            executed in GUIDE_WORKDIR, document order
#   ```sh test              same as bash
#   ```yaml test:<file>     materialized to GUIDE_WORKDIR/<file> (e.g. Helm values)
#   ```anything-else        human-only, never executed
#
#
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

import os
import re
import subprocess
import sys
from pathlib import Path

FENCE_RE = re.compile(r"^```(.*)$")
PLACEHOLDER_RE = re.compile(r"@@([A-Za-z_][A-Za-z0-9_]*)@@")
EXEC_LANGS = {"bash", "sh"}
FILE_LANGS = {"yaml", "yml"}


def fail(message: str, code: int = 1) -> None:
    print(f"ERROR: {message}", file=sys.stderr)
    raise SystemExit(code)


def require_env(name: str) -> str:
    value = os.environ.get(name)
    if value is None or value == "":
        fail(f"required environment variable {name} is not set", code=2)
    return value  # type: ignore[return-value]


GUIDE = require_env("GUIDE")
GUIDE_WORKDIR = require_env("GUIDE_WORKDIR")
REPO_ROOT = require_env("REPO_ROOT")
CHART_VERSION = require_env("CHART_VERSION")
DOCS_VERSION = require_env("DOCS_VERSION")
DRY_RUN_RAW = require_env("DRY_RUN")
if DRY_RUN_RAW not in ("true", "false"):
    fail('DRY_RUN must be "true" or "false"', code=2)
DRY_RUN = DRY_RUN_RAW == "true"


def substitute_placeholders(text: str) -> str:
    def repl(match: re.Match[str]) -> str:
        name = match.group(1)
        value = os.environ.get(name)
        if value is None or value == "":
            fail(f"placeholder @@{name}@@ has no matching environment variable set")
        return value  # type: ignore[return-value]

    return PLACEHOLDER_RE.sub(repl, text)


class Block:
    def __init__(self, index: int, lang: str, target: str, body: str) -> None:
        self.index = index
        self.lang = lang
        self.target = target
        self.body = body

    @property
    def label(self) -> str:
        suffix = f":{self.target}" if self.target else ""
        return f"block{self.index + 1} ({self.lang}{suffix})"


def extract_test_blocks(markdown: str) -> list[Block]:
    blocks: list[Block] = []
    lines = markdown.splitlines(keepends=True)
    i = 0
    while i < len(lines):
        match = FENCE_RE.match(lines[i].rstrip("\n"))
        if not match:
            i += 1
            continue
        tokens = match.group(1).split() if match.group(1) else []
        lang = tokens[0] if tokens else ""
        flags = tokens[1:]
        is_test = "test" in flags or any(f.startswith("test:") for f in flags)
        body_lines: list[str] = []
        i += 1
        while i < len(lines) and not lines[i].startswith("```"):
            body_lines.append(lines[i])
            i += 1
        i += 1  # consume closing fence (or run past EOF)
        if not is_test or not lang:
            continue
        target = ""
        for flag in flags:
            if flag.startswith("test:"):
                target = flag[len("test:") :]
        if lang in EXEC_LANGS:
            blocks.append(Block(len(blocks), lang, "", "".join(body_lines)))
        elif lang in FILE_LANGS:
            if target == "":
                fail(f"yaml test block{len(blocks) + 1} needs a target file, e.g. ```yaml test:my-cluster.yaml")
            if Path(target).name != target or target.startswith("."):
                fail(f"yaml test block target must be a bare filename, got {target!r}")
            blocks.append(Block(len(blocks), lang, target, "".join(body_lines)))
        else:
            fail(f"unsupported test block language {lang!r} (only bash/sh/yaml are executable)")
    return blocks


def run_blocks(blocks: list[Block], workdir: Path) -> None:
    shell = subprocess.Popen(
        ["bash", "--noprofile", "--norc"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        cwd=workdir,
        env=dict(os.environ, REPO_ROOT=REPO_ROOT),
    )
    assert shell.stdin is not None and shell.stdout is not None
    marker_re = re.compile(r"^__GUIDE_STEP_(\d+)_EXIT=(\d+)$")

    def send(line: str) -> None:
        assert shell.stdin is not None
        shell.stdin.write(line + "\n")
        shell.stdin.flush()

    def fail_step(label: str, message: str) -> None:
        shell.terminate()
        fail(f"{label}: {message}")

    send("set -uo pipefail")
    for index, block in enumerate(blocks):
        print(f"=== {block.label} ===")
        print(block.body, end="" if block.body.endswith("\n") else "\n")
        # Subshell with set -e: first failing command aborts the rest of the
        # block (like `bash -e script`) while the parent shell survives to
        # report the status via the marker.
        send("(")
        send("set -e")
        for line in block.body.splitlines():
            send(line)
        send(")")
        send(f"echo __GUIDE_STEP_{index}_EXIT=$?")
        while True:
            output = shell.stdout.readline()
            if output == "":
                fail_step(block.label, "shell terminated unexpectedly")
            output = output.rstrip("\n")
            marker = marker_re.match(output)
            if marker:
                status = int(marker.group(2))
                if status != 0:
                    fail_step(block.label, f"exited with status {status}")
                break
            print(f"[{block.label}] {output}")
    send("exit 0")
    shell.wait()
    if shell.returncode != 0:
        fail(f"guide shell exited with status {shell.returncode}")


def main() -> None:
    guide_path = Path(GUIDE)
    if not guide_path.is_file():
        fail(f"GUIDE {GUIDE!r} is not a file", code=2)
    repo_root = Path(REPO_ROOT)
    if not repo_root.is_dir():
        fail(f"REPO_ROOT {REPO_ROOT!r} is not a directory", code=2)
    workdir = Path(GUIDE_WORKDIR)
    workdir.mkdir(parents=True, exist_ok=True)

    markdown = substitute_placeholders(guide_path.read_text())
    blocks = extract_test_blocks(markdown)
    if not blocks:
        fail(f"no test blocks found in {GUIDE} (tag fences with e.g. ```bash test)")
    print(f"GUIDE={GUIDE} blocks={len(blocks)} dry_run={DRY_RUN}")
    for block in blocks:
        print(f"--- {block.label} ({len(block.body.splitlines())} lines) ---")

    if DRY_RUN:
        for block in blocks:
            print(f"===== {block.label} =====")
            print(block.body, end="" if block.body.endswith("\n") else "\n")
        print("DRY RUN: executed nothing")
        return

    for block in blocks:
        if block.target:
            (workdir / block.target).write_text(block.body)
            print(f"PASS: materialized {block.target}")
    run_blocks([b for b in blocks if not b.target], workdir)
    print(f"All guide checks passed for {GUIDE}!")


if __name__ == "__main__":
    main()
