#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

uv run --with 'requests>=2.31' --with 'protobuf>=5' python -m py_compile playget.py googleplay_pb2.py
uv run --with 'requests>=2.31' --with 'protobuf>=5' python -m unittest discover -s tests
go test ./...
scripts/package-release.sh >/dev/null
