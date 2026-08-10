# Changelog

## v0.1.3 (2026-08-10)

- Drop bare `metadata` capability (collides with metadata-tmdb discovery); keep `media.analyzer`
- Expand unit tests: parseOutput / audio+subtitle helpers, storeCache→GetCached/Analyze hit, Health before Init
- golangci-lint config: explicit `version: "1"`

## v0.1.2

- Initial release
- ffprobe integration for media file analysis
- Stream detection: video codec, resolution, frame rate, pixel format
- HDR detection: HDR10, HDR10+, Dolby Vision, HLG
- Audio stream detection: codec, channels, language, bitrate
- Subtitle stream detection with forced/HI flags
- Quality classification with scoring (resolution + source + HDR)
- SQLite analysis cache with file modification invalidation
- gRPC API: Analyze, GetCached
