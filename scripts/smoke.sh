#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

go test ./...
scripts/package-release.sh >/dev/null
