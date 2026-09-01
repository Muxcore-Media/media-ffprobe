# Media FFprobe

[![CI](https://git.zem.systems/muxcore/media-ffprobe/actions/workflows/ci.yml/badge.svg)](https://git.zem.systems/muxcore/media-ffprobe/actions)
[![Go Version](https://img.shields.io/badge/Go-1.26-blue)](https://go.dev/)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)

**Media file analysis via ffprobe — detects codec, resolution, HDR, bitrate, and classifies quality.**

A MuxCore sidecar module that wraps ffprobe to provide structured media analysis for quality scoring, transcoding decisions, and library metadata.

---

## How It Works

```
Media file ──→ media-ffprobe (ffprobe) ──→ Structured analysis
                    │
                    ├── Codec, resolution, frame rate, interlace
                    ├── HDR type (HDR10, DV, HLG)
                    ├── Audio tracks (codec, channels, language, default/commentary)
                    ├── Subtitle tracks
                    ├── Quality score (2160p Remux HDR = 170)
                    └── Optional chapter generation (ffmpeg scene/interval)
```

### Key Features

- **HDR detection** — identifies HDR10, HDR10+, Dolby Vision, and HLG from color metadata
- **Quality classification** — assigns scores from filename tokens (Remux, BluRay, WEB-DL, WEBRip, HDTV) plus resolution and HDR
- **Multi-stream parsing** — largest non-cover video stream plus all audio and subtitle tracks
- **Path allowlist** — `Analyze` / `GetCached` reject paths outside configured roots (symlink escape blocked)
- **SQLite cache** — avoids re-analysis of unchanged files; `Invalidate` / `PurgeCache` RPCs for operator flush
- **gRPC API** — `Analyze`, `GetCached`, `Invalidate`, `PurgeCache` (`muxcore.ffprobe.v1.AnalysisService`)

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `FFPROBE_DB_PATH` | `/var/lib/media-ffprobe/cache.db` | SQLite analysis cache path |
| `FFPROBE_GRPC_ADDR` | `:9480` | Module analysis gRPC listen address |
| `FFPROBE_BIN` | `ffprobe` | ffprobe executable path or name |
| `FFPROBE_TIMEOUT` | `30s` | Per-file ffprobe timeout (Go duration) |
| `FFMPEG_BIN` | sibling of `FFPROBE_BIN` or `ffmpeg` | ffmpeg for optional chapter generation |
| `FFPROBE_ALLOW_PATHS` | (none) | Comma-separated media roots; required for `Analyze` / `GetCached` |
| `FFPROBE_GENERATE_CHAPTERS` | `false` | When `true`, generate scene/interval chapters when none are embedded |
| `FFPROBE_CACHE_MAX_AGE` | (none) | Prune cache entries older than this duration on startup |
| `MUXCORE_MODULE_ID` | `media-ffprobe` | Module ID registered with core |
| `MUXCORE_GRPC_ADDR` | (SDK default) | Core mesh gRPC address |
| `MUXCORE_INSECURE_DISABLE_TLS` | `false` | Disable TLS for local/dev |

Chapter generation runs a full-file ffmpeg scene pass when enabled and no embedded chapters exist. Leave disabled unless operators explicitly want generated markers — callers such as `media-scanner` already interval-fallback client-side.

---

## Quick Start

```bash
# Build
make build

# Run
export MUXCORE_INSECURE_DISABLE_TLS=true
export FFPROBE_ALLOW_PATHS=/mnt/media
./media-ffprobe --muxcore-mesh-addr localhost:9090

# Analyze a file
grpcurl -d '{"file_path": "/mnt/media/movie.mkv"}' \
  :9480 muxcore.ffprobe.v1.AnalysisService/Analyze
```

---

## License

GPL-3.0
