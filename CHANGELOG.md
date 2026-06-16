# Changelog

## v0.1.0 (2026-06-14)

- Initial release
- ffprobe integration for media file analysis
- Stream detection: video codec, resolution, frame rate, pixel format
- HDR detection: HDR10, HDR10+, Dolby Vision, HLG
- Audio stream detection: codec, channels, language, bitrate
- Subtitle stream detection with forced/HI flags
- Quality classification with scoring (resolution + source + HDR)
- SQLite analysis cache with file modification invalidation
- gRPC API: Analyze, GetCached
