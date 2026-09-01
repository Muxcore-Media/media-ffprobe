package internal

import (
	"testing"
	"time"
)

func TestSettingsProbeTunables(t *testing.T) {
	m := NewModule(Config{})
	defs := m.Settings()
	if len(defs) != 5 {
		t.Fatalf("defs=%d", len(defs))
	}
	if err := m.UpdateSetting("ffprobe_bin", "/usr/bin/ffprobe"); err != nil {
		t.Fatal(err)
	}
	if got := m.getFFprobeBin(); got != "/usr/bin/ffprobe" {
		t.Fatalf("bin=%q", got)
	}
	if err := m.UpdateSetting("probe_timeout", "45s"); err != nil {
		t.Fatal(err)
	}
	if got := m.getProbeTimeout(); got != 45*time.Second {
		t.Fatalf("timeout=%v", got)
	}
	if err := m.UpdateSetting("ffmpeg_bin", "/usr/bin/ffmpeg"); err != nil {
		t.Fatal(err)
	}
	if got := m.getFFmpegBin(); got != "/usr/bin/ffmpeg" {
		t.Fatalf("ffmpeg=%q", got)
	}
	root := t.TempDir()
	if err := m.UpdateSetting("allow_paths", root); err != nil {
		t.Fatal(err)
	}
	if got := m.getAllowPaths(); len(got) != 1 {
		t.Fatalf("allow=%v", got)
	}
	if err := m.UpdateSetting("generate_chapters", "true"); err != nil {
		t.Fatal(err)
	}
	if !m.getGenerateChapters() {
		t.Fatal("expected generate chapters true")
	}
	if err := m.UpdateSetting("probe_timeout", "nope"); err == nil {
		t.Fatal("expected error")
	}
	if err := m.UpdateSetting("unknown", "x"); err == nil {
		t.Fatal("expected error")
	}
}
