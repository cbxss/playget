# playget

Download Google Play apps — including Play-only ones (e.g. Claude) and **historical versions** — locally. No emulator, no Google login, no JVM. Pure Python.

It uses anonymous credentials from [Aurora](https://gitlab.com/AuroraOSS)'s token dispenser plus Google Play's own protocol (checkin → details → purchase → delivery), reimplemented in Python.

## Requires

- **A residential IP** — Google blocks Play login from datacenter IPs.
- [`uv`](https://docs.astral.sh/uv/) (handles the `requests` + `protobuf` deps automatically).

## Install

```sh
curl -fsSL https://github.com/cbxss/playget/releases/latest/download/install.sh | bash
```

The installer downloads the latest release tarball, installs the tool under
`~/.local/share/playget`, and writes a `playget` wrapper to `~/.local/bin`. If
`uv` is missing, it installs `uv` with Astral's official installer first.

## Usage

```sh
playget <package> [--version <versionCode>] [--out <dir>]
```

```sh
playget com.anthropic.claude                    # latest
playget com.anthropic.claude --version 26020937 # a specific build
```

Downloads `base.apk` + the split APKs into `play_out/<package>/`.

By default, `--profile auto` starts from `device.properties` and retries with temporary
feature overlays when Play reports the app is unavailable for the uploaded device
configuration. Successful package/profile combinations are cached under
`~/.cache/playget/profile-cache.json`. Use `--no-cache` to ignore that cache, or
`--extra-feature <android.feature>` to advertise a feature for one run without editing
`device.properties`.

For source checkouts, `uv run playget.py ...` still works.

## Release

Versions live in `VERSION` and releases are tagged as `vX.Y.Z`. To cut a release:

```sh
scripts/release.sh 0.1.1
```

Pushing the tag runs the GitHub Actions release workflow and uploads `install.sh`,
`playget.tar.gz`, `playget-<version>.tar.gz`, and `sha256sums.txt`.

## Tests

```sh
scripts/smoke.sh
PLAYGET_LIVE=1 scripts/smoke.sh
```

The default smoke test is offline and includes a Go/Python oracle comparison for
device config output. `PLAYGET_LIVE=1` adds live Play checks for checkin/device
config upload, Withings base-profile unavailability, Withings runtime-overlay
delivery metadata, and IATA base-profile delivery metadata.

## Go port

The durable end state is a Go CLI at `cmd/playget`. Until it reaches live
protocol parity, the Python implementation remains the release artifact and
oracle. Go parity work starts with protobuf/device-config equivalence and grows
toward replacing the Python downloader.

## Files

| | |
|---|---|
| `playget.py` | the tool |
| `VERSION` | release version |
| `cmd/playget` | native Go port in progress |
| `googleplay.proto` / `googleplay_pb2.py` | self-contained Play protobuf schema |
| `device.properties` | the spoofed device profile the protocol requires |

## Note

Pinned to a current Play-protocol snapshot. If Google bumps the protocol (downloads start failing), refresh the `X-DFE-*` constants and Finsky version in `playget.py` from a newer Aurora `gplayapi` release.
