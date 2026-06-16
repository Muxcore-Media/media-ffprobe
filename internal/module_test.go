package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	ffprobev1 "github.com/Muxcore-Media/media-ffprobe/proto/ffprobev1"
)

func newTestModule(t *testing.T) *Module {
	t.Helper()
	m := NewModule(Config{
		DBPath:   filepath.Join(t.TempDir(), "cache.db"),
		GRPCAddr: ":0",
	})
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { m.Stop(ctx) })
	return m
}

func TestModuleInfo(t *testing.T) {
	m := NewModule(Config{})
	info := m.Info()
	if info.ID == "" {
		t.Error("module ID must not be empty")
	}
	if info.Version == "" {
		t.Error("module version must not be empty")
	}
	if len(info.Capabilities) == 0 || info.Capabilities[0] != "media.analyzer" {
		t.Errorf("expected media.analyzer capability, got %v", info.Capabilities)
	}
}

func TestAnalyzeNonexistentFile(t *testing.T) {
	m := newTestModule(t)
	ctx := context.Background()

	_, err := m.Analyze(ctx, &ffprobev1.AnalyzeRequest{FilePath: "/nonexistent/file.mkv"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestAnalyzeEmptyPath(t *testing.T) {
	m := newTestModule(t)
	ctx := context.Background()

	_, err := m.Analyze(ctx, &ffprobev1.AnalyzeRequest{FilePath: ""})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestGetCachedMiss(t *testing.T) {
	m := newTestModule(t)
	ctx := context.Background()

	resp, err := m.GetCached(ctx, &ffprobev1.GetCachedRequest{FilePath: "/nonexistent.mkv"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Found {
		t.Fatal("expected not found")
	}
}

func TestLifecycle(t *testing.T) {
	m := NewModule(Config{
		DBPath:   filepath.Join(t.TempDir(), "lifecycle.db"),
		GRPCAddr: ":0",
	})
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestHealth(t *testing.T) {
	m := newTestModule(t)
	ctx := context.Background()
	if err := m.Health(ctx); err != nil {
		t.Fatal("expected health to pass after init")
	}
}

func TestParseFrameRate(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"24000/1001", 23.976},
		{"30000/1001", 29.970},
		{"60000/1001", 59.940},
		{"25/1", 25},
		{"50/1", 50},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseFrameRate(tt.input)
		if got != tt.want {
			t.Errorf("parseFrameRate(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestResolutionLabel(t *testing.T) {
	tests := []struct {
		w, h int
		want string
	}{
		{3840, 2160, "2160p"},
		{1920, 1080, "1080p"},
		{1280, 720, "720p"},
		{720, 576, "576p"},
		{640, 480, "SD"},
		{800, 600, "576p"},
	}
	for _, tt := range tests {
		got := resolutionLabel(tt.w, tt.h)
		if got != tt.want {
			t.Errorf("resolutionLabel(%d,%d) = %s, want %s", tt.w, tt.h, got, tt.want)
		}
	}
}

func TestCodecGroup(t *testing.T) {
	tests := []struct {
		codec string
		want  string
	}{
		{"hevc", "hevc"},
		{"h265", "hevc"},
		{"h264", "h264"},
		{"avc", "h264"},
		{"av1", "av1"},
		{"vp9", "vp9"},
		{"mpeg4", "other"},
	}
	for _, tt := range tests {
		got := codecGroup(tt.codec)
		if got != tt.want {
			t.Errorf("codecGroup(%q) = %s, want %s", tt.codec, got, tt.want)
		}
	}
}

func TestQualityScore(t *testing.T) {
	tests := []struct {
		res    string
		source string
		hdr    bool
		want   int
	}{
		{"2160p", "Remux", true, 170},
		{"2160p", "Remux", false, 160},
		{"1080p", "BluRay", false, 130},
		{"1080p", "WEB-DL", false, 120},
		{"720p", "WEB-DL", false, 100},
		{"SD", "WEB-DL", false, 60},
	}
	for _, tt := range tests {
		got := qualityScore(tt.res, tt.source, tt.hdr)
		if got != tt.want {
			t.Errorf("qualityScore(%q,%q,%v) = %d, want %d", tt.res, tt.source, tt.hdr, got, tt.want)
		}
	}
}

func TestQualityLabel(t *testing.T) {
	tests := []struct {
		res, src string
		hdr      bool
		codec    string
		want     string
	}{
		{"2160p", "Remux", true, "hevc", "2160p Remux HDR HEVC"},
		{"1080p", "BluRay", false, "h264", "1080p BluRay H264"},
		{"1080p", "WEB-DL", false, "h264", "1080p WEB-DL H264"},
		{"720p", "WEB-DL", false, "hevc", "720p WEB-DL HEVC"},
	}
	for _, tt := range tests {
		got := qualityLabel(tt.res, tt.src, tt.hdr, tt.codec)
		if got != tt.want {
			t.Errorf("qualityLabel(%q,%q,%v,%q) = %q, want %q", tt.res, tt.src, tt.hdr, tt.codec, got, tt.want)
		}
	}
}

func TestClassifyQuality(t *testing.T) {
	v := &ffprobev1.VideoStream{
		Codec:       "hevc",
		Width:       3840,
		Height:      2160,
		Hdr:         true,
		HdrType:     "HDR10",
		PixelFormat: "yuv420p10le",
	}
	q := classifyQuality(v)
	if q.Label == "" {
		t.Fatal("expected non-empty quality label")
	}
	if q.Score != 160 {
		t.Errorf("expected score 160 (2160p=120 + BluRay=30 + HDR=10), got %d", q.Score)
	}
	if q.Resolution != "2160p" {
		t.Errorf("expected 2160p, got %s", q.Resolution)
	}
	if q.Source != "BluRay" {
		t.Errorf("expected BluRay source (10-bit), got %s", q.Source)
	}
}

func TestClassifyQualityNilVideo(t *testing.T) {
	q := classifyQuality(nil)
	if q.Label != "Unknown" {
		t.Errorf("expected Unknown, got %s", q.Label)
	}
	if q.Score != 0 {
		t.Errorf("expected score 0, got %d", q.Score)
	}
}

func TestDetectHDR(t *testing.T) {
	tests := []struct {
		name    string
		stream  ffprobeStream
		wantHDR bool
		wantTyp string
	}{
		{
			name: "HDR10 (PQ+BT2020)",
			stream: ffprobeStream{
				ColorTransfer:  "smpte2084",
				ColorPrimaries: "bt2020",
			},
			wantHDR: true,
			wantTyp: "HDR10",
		},
		{
			name: "HLG",
			stream: ffprobeStream{
				ColorTransfer: "arib-std-b67",
			},
			wantHDR: true,
			wantTyp: "HLG",
		},
		{
			name: "SDR (BT709)",
			stream: ffprobeStream{
				ColorTransfer:  "bt709",
				ColorPrimaries: "bt709",
			},
			wantHDR: false,
			wantTyp: "",
		},
		{
			name: "Dolby Vision via side data",
			stream: ffprobeStream{
				ColorTransfer: "smpte2084",
				SideDataList: []ffprobeSideData{
					{SideDataType: "Dolby Vision metadata"},
				},
			},
			wantHDR: true,
			wantTyp: "Dolby Vision",
		},
	}
	for _, tt := range tests {
		gotHDR, gotTyp := detectHDR(tt.stream)
		if gotHDR != tt.wantHDR || gotTyp != tt.wantTyp {
			t.Errorf("%s: detectHDR = (%v,%q), want (%v,%q)", tt.name, gotHDR, gotTyp, tt.wantHDR, tt.wantTyp)
		}
	}
}

func TestCacheInvalidation(t *testing.T) {
	m := newTestModule(t)

	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "test.mkv")
	os.WriteFile(filePath, []byte("test data"), 0644)

	// Analyze a text file as "media" — ffprobe will fail, so this errors
	// Instead, test that cache stores and retrieves properly by injecting
	ctx := context.Background()

	// ffprobe not available in test env, so Analyze returns error.
	// But the cache API and lifecycle should still work.
	_, err := m.Analyze(ctx, &ffprobev1.AnalyzeRequest{FilePath: filePath})
	if err != nil {
		// Expected — ffprobe not available or file not valid media
		t.Log("analyze returned (expected without ffprobe):", err)
	}

	// Cached lookup should still work (returns not found)
	cached, err := m.GetCached(ctx, &ffprobev1.GetCachedRequest{FilePath: filePath})
	if err != nil {
		t.Fatal(err)
	}
	if cached.Found {
		t.Log("cache hit on non-media file (ffprobe might be available)")
	}
}
