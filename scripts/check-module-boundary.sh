#!/usr/bin/env bash
set -euo pipefail

if ! grep -Fxq 'module github.com/ferro-labs/gateway-cli' go.mod; then
  echo 'go.mod must declare module github.com/ferro-labs/gateway-cli' >&2
  exit 1
fi

# go list walks the real import graph (covers aliased and blank imports, and
# can't be tripped by a doc comment or string literal naming the module the
# way a text grep can) instead of grepping source text. -tags integration
# pulls itest/ into the graph too.
# Two statements, not one pipeline: `|| true` is needed for grep's "no match"
# exit 1, and on a pipeline it would swallow a go list failure too — passing
# the check because the scan never ran, which is the hole this rewrite closes.
deps="$(go list -tags integration -deps ./...)"
offenders="$(printf '%s\n' "$deps" | grep '^github\.com/ferro-labs/ai-gateway' || true)"
if [ -n "$offenders" ]; then
  echo "$offenders" >&2
  echo 'gateway-cli must not import AI Gateway packages' >&2
  exit 1
fi

if grep -qE '(^|[[:space:]>])github\.com/ferro-labs/ai-gateway([[:space:]/]|$)' go.mod; then
  echo 'gateway-cli must not reference the AI Gateway module in go.mod' >&2
  exit 1
fi
