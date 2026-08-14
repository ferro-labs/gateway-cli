#!/usr/bin/env bash
set -euo pipefail

grep -Fxq 'module github.com/ferro-labs/gateway-cli' go.mod

if grep -RIn --include='*.go' --exclude-dir=.git \
  'github.com/ferro-labs/ai-gateway' .; then
  echo 'gateway-cli must not import AI Gateway packages' >&2
  exit 1
fi

if grep -qE '^[[:space:]]*github\.com/ferro-labs/ai-gateway([[:space:]]|$)' go.mod; then
  echo 'gateway-cli must not require the AI Gateway module' >&2
  exit 1
fi
