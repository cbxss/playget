#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

mkdir -p internal/playproto
protoc \
  --go_out=internal/playproto \
  --go_opt=paths=source_relative \
  googleplay.proto
