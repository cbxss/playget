#!/usr/bin/env bash
set -euo pipefail

repo="${PLAYGET_REPO:-cbxss/playget}"
version="${PLAYGET_VERSION:-latest}"
install_dir="${INSTALL_DIR:-$HOME/.local/bin}"
data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
app_dir="${PLAYGET_APP_DIR:-$data_home/playget}"

die() {
  printf 'playget install: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

ensure_uv() {
  if command -v uv >/dev/null 2>&1; then
    return
  fi
  if [[ "${PLAYGET_INSTALL_UV:-1}" != "1" ]]; then
    die "uv is not installed; set PLAYGET_INSTALL_UV=1 or install uv first"
  fi
  need curl
  printf '[*] installing uv\n' >&2
  curl -LsSf https://astral.sh/uv/install.sh | sh
  export PATH="$HOME/.local/bin:$HOME/.cargo/bin:$PATH"
  command -v uv >/dev/null 2>&1 || die "uv install completed but uv is not on PATH"
}

tarball_url() {
  if [[ -n "${PLAYGET_TARBALL:-}" ]]; then
    printf '%s\n' "$PLAYGET_TARBALL"
  elif [[ "$version" == "latest" ]]; then
    printf 'https://github.com/%s/releases/latest/download/playget.tar.gz\n' "$repo"
  else
    local tag="$version"
    [[ "$tag" == v* ]] || tag="v$tag"
    printf 'https://github.com/%s/releases/download/%s/playget.tar.gz\n' "$repo" "$tag"
  fi
}

need curl
need tar
ensure_uv

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

url="$(tarball_url)"
printf '[*] downloading %s\n' "$url" >&2
curl -fsSL "$url" -o "$tmp/playget.tar.gz"
tar -xzf "$tmp/playget.tar.gz" -C "$tmp"

src="$(find "$tmp" -maxdepth 1 -type d -name 'playget-*' | sort | head -n 1)"
[[ -n "$src" ]] || die "release archive did not contain a playget-* directory"

rm -rf "$app_dir"
mkdir -p "$app_dir" "$install_dir"
cp -R "$src"/. "$app_dir"/
chmod +x "$app_dir/playget.py"

uv_bin="$(command -v uv)"
app_dir_literal="$(printf '%q' "$app_dir")"
uv_bin_literal="$(printf '%q' "$uv_bin")"
wrapper="$install_dir/playget"
cat > "$wrapper" <<EOF
#!/usr/bin/env bash
set -euo pipefail
APP_DIR=$app_dir_literal
UV_BIN=$uv_bin_literal
if [[ -x "\$UV_BIN" ]]; then
  exec "\$UV_BIN" run --script "\$APP_DIR/playget.py" "\$@"
fi
exec uv run --script "\$APP_DIR/playget.py" "\$@"
EOF
chmod +x "$wrapper"

if [[ "${PLAYGET_SKIP_VERIFY:-0}" != "1" ]]; then
  "$wrapper" --tool-version >/dev/null
fi

printf 'Installed %s to %s\n' "$("$wrapper" --tool-version)" "$wrapper"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) printf 'Add %s to PATH if playget is not found in new shells.\n' "$install_dir" ;;
esac
