# playget

Download Google Play apps — including Play-only ones (e.g. Claude) and **historical versions** — locally. No emulator, no Google login, no JVM.

It uses anonymous credentials from [Aurora](https://gitlab.com/AuroraOSS)'s token dispenser plus Google Play's own protocol (checkin -> details -> purchase -> delivery), implemented as a native Go CLI.

## Requires

- **A residential IP** — Google blocks Play login from datacenter IPs.
- Linux on `amd64` or `arm64` for the release installer.

## Install

```sh
curl -fsSL https://github.com/cbxss/playget/releases/latest/download/install.sh | bash
```

The installer detects your Linux architecture, downloads the latest native Go
binary from GitHub Releases, verifies its checksum when `sha256sum` is
available, and installs it to `~/.local/bin/playget`.

## Usage

```sh
playget <package> [--version <versionCode>] [--out <dir>]
```

```sh
playget com.anthropic.claude                    # latest
playget com.anthropic.claude --version 26020937 # a specific build
```

Downloads `base.apk` + the split APKs into `play_out/<package>/`.

By default, `--profile auto` starts from the embedded device profile and retries with temporary
feature overlays when Play reports the app is unavailable for the uploaded device
configuration. Successful package/profile combinations are cached under
`~/.cache/playget/profile-cache.json`. Use `--no-cache` to ignore that cache, or
`--extra-feature <android.feature>` to advertise a feature for one run without editing
the embedded defaults.

## Release

Versions live in `VERSION` and releases are tagged as `vX.Y.Z`. To cut a release:

```sh
scripts/release.sh 0.2.1
```

Pushing the tag runs the GitHub Actions release workflow and uploads `install.sh`,
native Go binaries for Linux on amd64/arm64, and `sha256sums.txt`.

## Tests

```sh
scripts/smoke.sh
PLAYGET_LIVE=1 scripts/smoke.sh
scripts/clean.sh
```

The default smoke test is offline. `PLAYGET_LIVE=1` adds live Play checks for
checkin/device config upload, Withings base-profile unavailability, Withings
runtime-overlay delivery metadata, and IATA base-profile delivery metadata.
`scripts/clean.sh` removes local generated outputs like `dist/`, `play_out/`,
and APK reverse-engineering scratch directories.

## Files

| | |
|---|---|
| `cmd/playget` | the CLI |
| `VERSION` | release version |
| `googleplay.proto` / `internal/playproto` | self-contained Play protobuf schema |
| `internal/assets/device.properties` | embedded spoofed device profile |

## Note

Pinned to a current Play-protocol snapshot. If Google bumps the protocol (downloads start failing), refresh the `X-DFE-*` constants and Finsky version in `internal/play` from a newer Aurora `gplayapi` release.
