package internal

import (
	"fmt"
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
	default:
		return fmt.Errorf("unknown setting %q", key)
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

func (m *Module) getProbeTimeout() time.Duration {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	if m.probeTimeout <= 0 {
		return 30 * time.Second
	}
	return m.probeTimeout
}
