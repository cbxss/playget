#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

version="${1:-}"
version="${version#v}"

if [[ -z "$version" || ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z]+)*$ ]]; then
  printf 'usage: scripts/release.sh X.Y.Z\n' >&2
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  printf 'working tree has uncommitted changes; commit or stash them first\n' >&2
  exit 1
fi

current="$(tr -d '[:space:]' < VERSION)"
if [[ "$current" != "$version" ]]; then
  printf '%s\n' "$version" > VERSION
  git add VERSION
  git commit -m "Release v$version"
fi

tag="v$version"
if git rev-parse "$tag" >/dev/null 2>&1; then
  printf 'tag already exists: %s\n' "$tag" >&2
  exit 1
fi

scripts/package-release.sh >/dev/null
git tag -a "$tag" -m "playget $tag"
git push origin HEAD
git push origin "$tag"
printf 'pushed %s; GitHub Actions will publish the release\n' "$tag"
