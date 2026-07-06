#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

version="$(tr -d '[:space:]' < VERSION)"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z]+)*$ ]]; then
  printf 'invalid VERSION: %s\n' "$version" >&2
  exit 1
fi

dist="dist"
work="$dist/work"
pkg="playget-$version"
pkg_dir="$work/$pkg"

rm -rf "$work"
mkdir -p "$pkg_dir" "$dist"

install -m 0755 playget.py "$pkg_dir/playget.py"
install -m 0755 install.sh "$pkg_dir/install.sh"
install -m 0644 VERSION README.md device.properties googleplay.proto googleplay_pb2.py "$pkg_dir/"

tar -C "$work" -czf "$dist/$pkg.tar.gz" "$pkg"
cp "$dist/$pkg.tar.gz" "$dist/playget.tar.gz"
install -m 0755 install.sh "$dist/install.sh"

(
  cd "$dist"
  sha256sum "$pkg.tar.gz" playget.tar.gz install.sh > sha256sums.txt
)

printf 'created %s/%s.tar.gz\n' "$dist" "$pkg"
