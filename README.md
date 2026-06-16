# Media FFprobe

[![CI](https://github.com/Muxcore-Media/media-ffprobe/actions/workflows/ci.yml/badge.svg)](https://github.com/Muxcore-Media/media-ffprobe/actions)
[![Go Version](https://img.shields.io/badge/Go-1.26-blue)](https://go.dev/)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)

**Media file analysis via ffprobe — detects codec, resolution, HDR, bitrate, and classifies quality.**

A MuxCore sidecar module that wraps ffprobe to provide structured media analysis for quality scoring, transcoding decisions, and library metadata.

---

## How It Works

```
Media file ──→ media-ffprobe (ffprobe) ──→ Structured analysis
                    │
                    ├── Codec, resolution, frame rate
                    ├── HDR type (HDR10, DV, HLG)
                    ├── Audio tracks (codec, channels, language)
                    ├── Subtitle tracks
                    └── Quality score (2160p Remux HDR = 170)
```

### Key Features

- **HDR detection** — identifies HDR10, HDR10+, Dolby Vision, and HLG from color metadata
- **Quality classification** — assigns scores based on resolution (2160p > 1080p > 720p), source (Remux > BluRay > WEB-DL), and HDR
- **Multi-stream parsing** — reports all video, audio, and subtitle tracks
- **SQLite cache** — avoids re-analysis of unchanged files; invalidates on file modification

---

## Quick Start

```bash
# Build
make build

# Run
export MUXCORE_GRPC_INSECURE=true
./media-ffprobe --muxcore-mesh-addr localhost:9090

# Analyze a file
grpcurl -d '{"file_path": "/path/to/movie.mkv"}' \
  :9480 muxcore.ffprobe.v1.AnalysisService/Analyze
```

---

## License

GPL-3.0
