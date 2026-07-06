#!/usr/bin/env bash
set -euo pipefail

repo="${PLAYGET_REPO:-cbxss/playget}"
version="${PLAYGET_VERSION:-latest}"
install_dir="${INSTALL_DIR:-$HOME/.local/bin}"

die() {
  printf 'playget install: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

detect_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux) printf 'linux\n' ;;
    *) die "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64\n' ;;
    arm64|aarch64) printf 'arm64\n' ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
}

asset_url() {
  local asset="$1"
  if [[ -n "${PLAYGET_BINARY:-}" ]]; then
    printf '%s\n' "$PLAYGET_BINARY"
  elif [[ "$version" == "latest" ]]; then
    printf 'https://github.com/%s/releases/latest/download/%s\n' "$repo" "$asset"
  else
    local tag="$version"
    [[ "$tag" == v* ]] || tag="v$tag"
    printf 'https://github.com/%s/releases/download/%s/%s\n' "$repo" "$tag" "$asset"
  fi
}

need curl

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

os_name="${PLAYGET_OS:-$(detect_os)}"
arch="${PLAYGET_ARCH:-$(detect_arch)}"
asset="playget-${os_name}-${arch}"
url="$(asset_url "$asset")"
binary="$tmp/$asset"
printf '[*] downloading %s\n' "$url" >&2
curl -fsSL "$url" -o "$binary"

if command -v sha256sum >/dev/null 2>&1 && [[ -z "${PLAYGET_BINARY:-}" ]]; then
  sums_url="$(asset_url sha256sums.txt)"
  curl -fsSL "$sums_url" -o "$tmp/sha256sums.txt"
  grep "  ${asset}$" "$tmp/sha256sums.txt" > "$tmp/one.sum" || die "checksum missing for $asset"
  (cd "$tmp" && sha256sum -c one.sum >/dev/null)
fi

mkdir -p "$install_dir"
install -m 0755 "$binary" "$install_dir/playget"

if [[ "${PLAYGET_SKIP_VERIFY:-0}" != "1" ]]; then
  "$install_dir/playget" --tool-version >/dev/null
fi

printf 'Installed %s to %s\n' "$("$install_dir/playget" --tool-version)" "$install_dir/playget"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) printf 'Add %s to PATH if playget is not found in new shells.\n' "$install_dir" ;;
esac
