# CLAUDE.md

Instructions for AI coding agents working on this repository.

## Project overview

`ksdl` is a Go CLI tool that downloads Kinescope videos. It fetches HLS streams,
decrypts audio using CENC keys from the Kinescope license server, and merges
everything into a final MP4 using four external binaries: N_m3u8DL-RE, yt-dlp,
mp4decrypt, and ffmpeg.

## Build and test

```bash
go build -o ksdl .   # build binary
go test ./...        # run all tests
make install         # build and install to $(go env GOPATH)/bin
```

## Architecture

```
main.go                         CLI entry point (cobra), wires dependencies
internal/config/                Shared Config struct and defaults
internal/kinescope/             HTTP client: embed pages, playlists, decryption keys
internal/media/                 Wrappers around the four external binaries
internal/downloader/            Orchestrates the full pipeline per video
```

Dependency graph (no cycles): `main → config, kinescope, media, downloader`

## Key conventions

**Interfaces:** Keep them minimal. The only interface in the codebase is
`config.HTTPDoer` (1 method). Follow "start with concrete types, discover
interface" — do not introduce interfaces speculatively. If in doubt, use a
concrete type and refactor when a second implementation actually appears.

**Error types:** Typed errors (`HTTPError`, `ToolNotFoundError`,
`FileNotFoundError`) are used when callers need to inspect error details via
`errors.As`. Plain `fmt.Errorf` with `%w` is used for context-wrapping. Sentinel
`var Err... = errors.New(...)` is used for well-known conditions with no extra
data.

**Go doc comments:** Field-level doc comments go on their own line *before* the
field, not inline after it. Every exported symbol must have a doc comment
starting with the symbol's name.

**Logging:** `slog` to stderr. INFO level by default, DEBUG with `--verbose`.
Progress steps use `fmt.Printf` directly (UI output, not logs).

**Testing:** Tests use `httptest.NewServer` + `srv.Client()` for HTTP — no mock
interfaces. White-box test packages (`package kinescope`, `package downloader`)
give access to unexported functions. Do not introduce interfaces just to enable
testing; use `httptest` or `os.Chtimes`-style test setup instead.

## Module

```
module github.com/eshlykov/ksdl
go 1.25
```
