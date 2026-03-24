# ksdl

[![CI](https://github.com/eshlykov/ksdl/actions/workflows/ci.yml/badge.svg)](https://github.com/eshlykov/ksdl/actions/workflows/ci.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/eshlykov/ksdl)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/github/license/eshlykov/ksdl)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/eshlykov/ksdl)](https://goreportcard.com/report/github.com/eshlykov/ksdl)
[![codecov](https://codecov.io/gh/eshlykov/ksdl/branch/main/graph/badge.svg)](https://codecov.io/gh/eshlykov/ksdl)

A command-line tool for downloading videos from [Kinescope](https://kinescope.io). Given a list of video IDs, it downloads each one, decrypts the audio track, and produces a merged MP4 file.

## How it works

Kinescope streams use HLS with SAMPLE-AES encryption. The video and audio tracks are encrypted separately and require different tools to handle:

1. **N_m3u8DL-RE** downloads all HLS segments. The final mux step fails intentionally — the encrypted audio cannot be muxed yet.
2. **yt-dlp** downloads and decrypts the video track independently.
3. **mp4decrypt** decrypts the audio track using the CENC key fetched from the Kinescope license server.
4. **ffmpeg** merges the decrypted video and audio into the final MP4.

For a detailed walkthrough of the manual process that `ksdl` automates, see [docs/HOWTO.md](docs/HOWTO.md).

## Prerequisites

The following external tools must be available in your `PATH`:

| Tool | Install |
|------|---------|
| [N_m3u8DL-RE](https://github.com/nilaoda/N_m3u8DL-RE) | Download binary from releases |
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | `brew install yt-dlp` |
| [mp4decrypt](https://www.bento4.com) | `brew install bento4` |
| [ffmpeg](https://ffmpeg.org) | `brew install ffmpeg` |

Go 1.25 or later is required to build from source.

## Installation

```bash
git clone https://github.com/eshlykov/ksdl.git
cd ksdl
```

Build and install the binary to `$(go env GOPATH)/bin` in one step (requires all prerequisites to already be installed):

```bash
make install
```

Or build only, without installing:

```bash
make build
# or equivalently:
go build -o ksdl .
```

## Usage

```
ksdl -f <id-file> -r <referrer>
```

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--file` | `-f` | Yes | Path to a file containing video IDs, one per line |
| `--referrer` | `-r` | Yes | URL of the page that embeds the video |
| `--verbose` | `-v` | No | Show debug output (URLs, key values, temp paths) |

**Example:**

```bash
ksdl -f ids.txt -r https://example.com/course/lesson-1
```

Each video is saved as `<video title>.mp4` in the current directory. If a file with that name already exists, the video is skipped.

## Finding the video ID and referrer

### Video ID

1. Open the page that contains the embedded video in Chrome.
2. Press **Cmd+Option+I** (macOS) to open DevTools.
3. Go to the **Network** tab.
4. Play the video, or reload the page if it does not start automatically.
5. Filter requests by typing `kinescope.io` in the search box.
6. Look for a request with:
   - **Method:** `GET`
   - **Status:** `200`
   - **Type:** `document`
   - A path that looks like `/AbCdEfGhIjKlMnO` (a short alphanumeric string)

The full URL will look like:

```
https://kinescope.io/q4WzE3saqokSWORJuTxKiE
```

The video ID is the path segment after the slash: `q4WzE3saqokSWORJuTxKiE`.

### Referrer

The referrer is the URL of the **page that embeds the video** — what is shown in your browser's address bar when you watch the video, not the `kinescope.io` URL. Copy it directly from the address bar.

The referrer must be provided so that Kinescope's servers accept the stream and key requests. Without the correct referrer, requests will be rejected.

## Batch processing

Create a plain text file with one video ID per line:

```
q4WzE3saqokSWORJuTxKiE
YKn2FECq2kirVwZQce4LHh
EfHIJhNsyhKpmJyWSZGPn0
```

Pass it with `-f`. `ksdl` processes each ID in sequence and prints step-by-step progress:

```
=== q4WzE3saqokSWORJuTxKiE ===

[1/3] Fetching embed page and extracting stream URL
[1/3] Fetching encryption key
[1/3] Downloading segments with N_m3u8DL-RE (mux failure at the end is expected)
...

=== YKn2FECq2kirVwZQce4LHh ===
...

All done.
```

Press **Ctrl+C** at any time to stop cleanly after the current step finishes.

## Development

Run the test suite:

```bash
go test ./...
```

### Package overview

| Package | Responsibility |
|---------|---------------|
| `internal/config` | Shared configuration types and defaults |
| `internal/kinescope` | HTTP client: fetches embed pages, playlists, and decryption keys |
| `internal/media` | Wrappers around N_m3u8DL-RE, yt-dlp, mp4decrypt, and ffmpeg |
| `internal/downloader` | Orchestrates the full pipeline for a single video |
