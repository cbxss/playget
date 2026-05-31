# playget

Download Google Play apps — including Play-only ones (e.g. Claude) and **historical versions** — locally. No emulator, no Google login, no JVM. Pure Python.

It uses anonymous credentials from [Aurora](https://gitlab.com/AuroraOSS)'s token dispenser plus Google Play's own protocol (checkin → details → purchase → delivery), reimplemented in Python.

## Requires

- **A residential IP** — Google blocks Play login from datacenter IPs.
- [`uv`](https://docs.astral.sh/uv/) (handles the `requests` + `protobuf` deps automatically).

## Usage

```sh
uv run playget.py <package> [--version <versionCode>] [--out <dir>]
```

```sh
uv run playget.py com.anthropic.claude                    # latest
uv run playget.py com.anthropic.claude --version 26020937 # a specific build
```

Downloads `base.apk` + the split APKs into `play_out/<package>/`.

## Files

| | |
|---|---|
| `playget.py` | the tool |
| `googleplay.proto` / `googleplay_pb2.py` | self-contained Play protobuf schema |
| `device.properties` | the spoofed device profile the protocol requires |

## Note

Pinned to a current Play-protocol snapshot. If Google bumps the protocol (downloads start failing), refresh the `X-DFE-*` constants and Finsky version in `playget.py` from a newer Aurora `gplayapi` release.
