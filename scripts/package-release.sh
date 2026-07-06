#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

version="$(tr -d '[:space:]' < VERSION)"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z]+)*$ ]]; then
  printf 'invalid VERSION: %s\n' "$version" >&2
  exit 1
fi

dist="dist"

rm -rf "$dist"
mkdir -p "$dist"

build_one() {
  local os_name="$1"
  local arch="$2"
  local out="$dist/playget-${os_name}-${arch}"
  CGO_ENABLED=0 GOOS="$os_name" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$out" .
}

build_one linux amd64
build_one linux arm64
install -m 0755 install.sh "$dist/install.sh"

(
  cd "$dist"
  sha256sum playget-* install.sh > sha256sums.txt
)

printf 'created release binaries in %s\n' "$dist"
