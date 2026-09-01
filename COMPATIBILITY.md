# Compatibility

## Core Version

| Module Version | Core Version | Status |
|----------------|-------------|--------|
| v0.1.8         | v0.5.8+     | Current |

## Contracts

| Contract | Capability | Status |
|----------|-----------|--------|
| `proto/ffprobev1` | `media.analyzer` | Current |

## Breaking Changes

This is a pre-1.0 module. Interfaces may change without notice.

## v0.1.8

- Path allowlist (`FFPROBE_ALLOW_PATHS`) with symlink escape rejection
- Filename-based source classification (Remux/BluRay/WEB-DL/WEBRip/HDTV)
- Audio default/commentary/sample_rate and video field_order/interlaced proto fields
- Optional chapter generation (`FFPROBE_GENERATE_CHAPTERS`, `FFMPEG_BIN`)
- `Invalidate` and `PurgeCache` cache RPCs; `FFPROBE_CACHE_MAX_AGE` startup prune
- Health fails when ffprobe is unavailable; stub results are not cached
