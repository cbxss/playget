#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

mkdir -p internal/playproto
protoc \
  -I proto \
  --go_out=. \
  --go_opt=module=github.com/cbxss/playget \
  googleplay.proto
