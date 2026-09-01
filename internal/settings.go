package internal

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Muxcore-Media/core/pkg/contracts"
)

func (m *Module) Settings() []contracts.SettingDef {
	return m.settingsDefs()
}

func (m *Module) UpdateSetting(key, value string) error {
	return m.updateSetting(key, value)
}

func (m *Module) settingsDefs() []contracts.SettingDef {
	return []contracts.SettingDef{
		{
			Key:         "ffprobe_bin",
			Label:       "ffprobe Binary",
			Type:        contracts.SettingTypeString,
			Value:       m.getFFprobeBin(),
			Description: "Path or name of ffprobe executable (FFPROBE_BIN)",
			Group:       "Probe",
		},
		{
			Key:         "probe_timeout",
			Label:       "Probe Timeout",
			Type:        contracts.SettingTypeString,
			Value:       m.getProbeTimeout().String(),
			Description: "Per-file analysis timeout (Go duration; FFPROBE_TIMEOUT)",
			Group:       "Probe",
		},
		{
			Key:         "ffmpeg_bin",
			Label:       "ffmpeg Binary",
			Type:        contracts.SettingTypeString,
			Value:       m.getFFmpegBin(),
			Description: "Path or name of ffmpeg executable for optional chapter generation (FFMPEG_BIN)",
			Group:       "Probe",
		},
		{
			Key:         "allow_paths",
			Label:       "Allowed Media Paths",
			Type:        contracts.SettingTypeString,
			Value:       strings.Join(m.getAllowPaths(), ","),
			Description: "Comma-separated roots; Analyze/GetCached reject paths outside these (FFPROBE_ALLOW_PATHS)",
			Group:       "Security",
		},
		{
			Key:         "generate_chapters",
			Label:       "Generate Chapters",
			Type:        contracts.SettingTypeBool,
			Value:       fmt.Sprintf("%t", m.getGenerateChapters()),
			Description: "When true and no embedded chapters exist, run ffmpeg scene/interval chapter generation",
			Group:       "Probe",
		},
	}
}

func (m *Module) updateSetting(key, value string) error {
	value = strings.TrimSpace(value)
	switch key {
	case "ffprobe_bin", "FFPROBE_BIN":
		if value == "" {
			return fmt.Errorf("ffprobe_bin must not be empty")
		}
		m.cfgMu.Lock()
		m.ffprobeBin = value
		m.cfgMu.Unlock()
		return nil
	case "probe_timeout", "FFPROBE_TIMEOUT":
		d, err := time.ParseDuration(value)
		if err != nil || d <= 0 {
			return fmt.Errorf("invalid probe_timeout %q (use Go duration like 30s)", value)
		}
		m.cfgMu.Lock()
		m.probeTimeout = d
		m.cfgMu.Unlock()
		return nil
	case "ffmpeg_bin", "FFMPEG_BIN":
		if value == "" {
			return fmt.Errorf("ffmpeg_bin must not be empty")
		}
		m.cfgMu.Lock()
		m.ffmpegBin = value
		m.cfgMu.Unlock()
		return nil
	case "allow_paths", "FFPROBE_ALLOW_PATHS":
		m.cfgMu.Lock()
		m.allowPaths = parseAllowPaths("", splitCSV(value))
		m.cfgMu.Unlock()
		return nil
	case "generate_chapters":
		m.cfgMu.Lock()
		m.generateChapters = parseBoolSetting(value)
		m.cfgMu.Unlock()
		return nil
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
}

func parseBoolSetting(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (m *Module) getFFprobeBin() string {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	if m.ffprobeBin == "" {
		return "ffprobe"
	}
	return m.ffprobeBin
}

func (m *Module) getFFmpegBin() string {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	if m.ffmpegBin != "" {
		return m.ffmpegBin
	}
	if v := strings.TrimSpace(os.Getenv("FFMPEG_BIN")); v != "" {
		return v
	}
	return ffmpegBinFor(m.getFFprobeBin())
}

func (m *Module) getProbeTimeout() time.Duration {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	if m.probeTimeout <= 0 {
		return 30 * time.Second
	}
	return m.probeTimeout
}

func (m *Module) getAllowPaths() []string {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	if len(m.allowPaths) == 0 {
		return nil
	}
	out := make([]string, len(m.allowPaths))
	copy(out, m.allowPaths)
	return out
}

func (m *Module) getGenerateChapters() bool {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.generateChapters
}
