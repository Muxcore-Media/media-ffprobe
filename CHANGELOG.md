# Changelog

## [Unreleased]

### Security
- `Analyze` refuses `file_path` values that do not resolve inside `FFPROBE_ALLOWED_ROOTS` (empty allow-list fails closed, including symlink escapes). `GetCached` misses closed for those paths.

## [0.1.13] - 2026-10-05


### Security
- gRPC server and peer dials use mesh TLS (meshtls, sdk/go/module v0.6.5) unless the dev insecure flag is set (ADR-0016/0017).

## [0.1.12] - 2026-10-05

### Changed
- Built on core v0.6.14 / sdk/go/module v0.6.4: unregisters on shutdown and re-registers after core restarts (ADR-0022).

## [0.1.11] - 2026-10-05


### Changed
- Reported version comes from muxcore.json (ADR-0021); built on core v0.6.12 / sdk/go/module v0.6.3 (mesh enrollment, ADR-0017).

## [0.1.9] - 2026-10-05

### Changed
- CI runs on GitHub-hosted runners from the umbrella template; retired-origin workflows removed.
- Dependencies resolve from published GitHub tags (no filesystem `replace`); requires core v0.6.0.

## [0.1.9] — 2026-09-08

### Added
- Probe container chapters via `ffprobe -show_chapters` and return them on `AnalyzeResponse.chapters`.

## [0.1.8] — 2026-08-10

### Added

- Advertise `settings` capability so admin-ui discovers SettingsProvider without ListAll probing.

## [0.1.7] — 2026-08-10

### Added
- SettingsProvider mesh (`RegisterSettings`) for `ffprobe_bin` and `probe_timeout`.

### Changed
- Pin `core/sdk/go/module` to **v0.5.2**.


## [0.1.6] — 2026-08-10

### Fixed
- Sync Info()/muxcore.json version to **0.1.6**.


## [0.1.5] — 2026-08-10

### Fixed
- Module `Info().Version` aligned to **0.1.5** (was 0.1.3).

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
