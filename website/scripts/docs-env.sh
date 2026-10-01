#!/bin/sh
# SPDX-FileCopyrightText: Copyright 2026 SAP SE or an SAP affiliate company and cobaltcore-dev contributors
# SPDX-License-Identifier: Apache-2.0
#
# Provision the docs build environment, then run the given command.
# Defaults mirror CI (deploy-docs.yml) for the main-branch docs; every value
# can be overridden by exporting it before invoking npm. Called by the
# docs:* npm scripts so website/.vitepress/config.mts can assume a complete
# environment via a single requiredEnv helper.
set -u

DOCS_SHA="${DOCS_SHA:-$(git rev-parse HEAD 2>/dev/null || echo local)}"
export DOCS_SHA
export DOCS_REPO="${DOCS_REPO:-cobaltcore-dev/thalamus}"
export DOCS_VERSION="${DOCS_VERSION:-main}"
export CHART_VERSION="${CHART_VERSION:-0.0.0-main}"
export DOCS_BASE="${DOCS_BASE:-/}"

exec "$@"
