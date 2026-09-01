package internal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	ffprobev1 "github.com/Muxcore-Media/media-ffprobe/proto/ffprobev1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newTestModule(t *testing.T) (*Module, string) {
	t.Helper()
	root := t.TempDir()
	m := NewModule(Config{
		DBPath:     filepath.Join(root, "cache.db"),
		GRPCAddr:   ":0",
		AllowPaths: []string{root},
	})
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = m.Stop(ctx) })
	return m, root
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
	for _, c := range info.Capabilities {
		if c == "metadata" {
			t.Fatal("bare metadata capability collides with metadata-tmdb; must not advertise")
		}
	}
}

func TestAnalyzeNonexistentFile(t *testing.T) {
	m, root := newTestModule(t)
	ctx := context.Background()

	_, err := m.Analyze(ctx, &ffprobev1.AnalyzeRequest{FilePath: filepath.Join(root, "missing.mkv")})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestAnalyzeEmptyPath(t *testing.T) {
	m, _ := newTestModule(t)
	ctx := context.Background()

	_, err := m.Analyze(ctx, &ffprobev1.AnalyzeRequest{FilePath: ""})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v want InvalidArgument", status.Code(err))
	}
}

func TestAnalyzePathOutsideAllowlist(t *testing.T) {
	m, _ := newTestModule(t)
	ctx := context.Background()

	_, err := m.Analyze(ctx, &ffprobev1.AnalyzeRequest{FilePath: "/etc/passwd"})
	if err == nil {
		t.Fatal("expected permission denied")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code=%v want PermissionDenied", status.Code(err))
	}
}

func TestGetCachedMiss(t *testing.T) {
	m, root := newTestModule(t)
	ctx := context.Background()

	resp, err := m.GetCached(ctx, &ffprobev1.GetCachedRequest{FilePath: filepath.Join(root, "missing.mkv")})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Found {
		t.Fatal("expected not found")
	}
}

func TestLifecycle(t *testing.T) {
	root := t.TempDir()
	m := NewModule(Config{
		DBPath:     filepath.Join(root, "lifecycle.db"),
		GRPCAddr:   ":0",
		AllowPaths: []string{root},
	})
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestHealth(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe absent; Health requires ffprobe")
	}
	m, _ := newTestModule(t)
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
		{7680, 4320, "4320p"},
		{3840, 2160, "2160p"},
		{2560, 1440, "1440p"},
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
		{"1080p", "WEBRip", false, 115},
		{"1080p", "HDTV", false, 110},
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

func TestClassifySourceFromFilename(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"Movie.2160p.BluRay.Remux.mkv", "Remux"},
		{"Movie.1080p.BluRay.x264.mkv", "BluRay"},
		{"Movie.1080p.WEB-DL.mkv", "WEB-DL"},
		{"Movie.720p.WEBRip.x264.mkv", "WEBRip"},
		{"Show.S01E01.HDTV.mkv", "HDTV"},
		{"Movie.unknown.mkv", "WEB-DL"},
	}
	for _, tt := range tests {
		got := classifySource(tt.path, &ffprobev1.VideoStream{})
		if got != tt.want {
			t.Errorf("classifySource(%q) = %q want %q", tt.path, got, tt.want)
		}
	}
}

func TestClassifyQuality(t *testing.T) {
	path := "/media/Inception.2010.2160p.BluRay.x265.mkv"
	v := &ffprobev1.VideoStream{
		Codec:  "hevc",
		Width:  3840,
		Height: 2160,
		Hdr:    true,
		HdrType: "HDR10",
	}
	q := classifyQuality(path, v)
	if q.Label == "" {
		t.Fatal("expected non-empty quality label")
	}
	if q.Score != 160 {
		t.Errorf("expected score 160, got %d", q.Score)
	}
	if q.Resolution != "2160p" {
		t.Errorf("expected 2160p, got %s", q.Resolution)
	}
	if q.Source != "BluRay" {
		t.Errorf("expected BluRay source, got %s", q.Source)
	}
}

func TestClassifyQualityNilVideo(t *testing.T) {
	q := classifyQuality("/media/x.mkv", nil)
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

func TestPickPrimaryVideoSkipsAttachedPic(t *testing.T) {
	videos := []*ffprobev1.VideoStream{
		{Width: 800, Height: 600, Codec: "mjpeg"},
		{Width: 1920, Height: 1080, Codec: "h264"},
	}
	got := pickPrimaryVideo(videos)
	if got.GetWidth() != 1920 {
		t.Fatalf("got %+v", got)
	}
}

func TestParseOutputStreams(t *testing.T) {
	m := NewModule(Config{})
	out := &ffprobeOutput{
		Format: ffprobeFormat{
			FormatName: "matroska,webm",
			Size:       "123456789",
			BitRate:    "8000000",
			Duration:   "3600.5",
		},
		Streams: []ffprobeStream{
			{
				Index: 0, CodecType: "video", CodecName: "mjpeg",
				Width: 800, Height: 600,
				Disposition: map[string]int{"attached_pic": 1},
			},
			{
				Index: 1, CodecType: "video", CodecName: "hevc", CodecLongName: "H.265",
				Width: 3840, Height: 2160, RFrameRate: "24000/1001", BitRate: "25000000",
				PixelFormat: "yuv420p10le", ColorPrimaries: "bt2020", ColorTransfer: "smpte2084",
				ColorSpace: "bt2020nc", AspectRatio: "16:9", FieldOrder: "progressive",
				SideDataList: []ffprobeSideData{{SideDataType: "Mastering display metadata"}},
			},
			{
				Index: 2, CodecType: "audio", CodecName: "aac", CodecLongName: "AAC",
				Channels: 2, ChannelLayout: "stereo", BitRate: "192000", SampleRate: "48000",
				Disposition: map[string]int{"default": 1},
				Tags: map[string]string{"language": "eng"},
			},
			{
				Index: 3, CodecType: "subtitle", CodecName: "subrip",
				Disposition: map[string]int{"forced": 1, "hearing_impaired": 0},
				Tags:        map[string]string{"language": "spa"},
			},
			{Index: 4, CodecType: "data", CodecName: "bin_data"},
		},
		Chapters: []ffprobeChapter{
			{ID: 0, StartTime: "0.000000", EndTime: "90.000000", Tags: map[string]string{"title": "Opening"}},
			{ID: 1, StartTime: "90.000000", EndTime: "600.000000", Tags: map[string]string{"title": "Act 1"}},
		},
	}
	resp := m.parseOutput(context.Background(), "/media/movie.mkv", out)
	if resp.FilePath != "/media/movie.mkv" {
		t.Fatalf("path=%q", resp.FilePath)
	}
	if resp.Video == nil || resp.Video.Codec != "hevc" || resp.Video.Width != 3840 {
		t.Fatalf("video=%+v", resp.Video)
	}
	if resp.Video.FieldOrder != "progressive" || resp.Video.Interlaced {
		t.Fatalf("field order=%q interlaced=%v", resp.Video.FieldOrder, resp.Video.Interlaced)
	}
	if len(resp.Audio) != 1 || resp.Audio[0].Language != "eng" || !resp.Audio[0].IsDefault || resp.Audio[0].SampleRate != 48000 {
		t.Fatalf("audio=%+v", resp.Audio)
	}
	if len(resp.Subtitles) != 1 || resp.Subtitles[0].Language != "spa" || !resp.Subtitles[0].Forced {
		t.Fatalf("subs=%+v", resp.Subtitles)
	}
	if resp.Quality == nil || resp.Quality.Source != "WEB-DL" {
		t.Fatalf("quality=%+v", resp.Quality)
	}
	if len(resp.Chapters) != 2 || resp.Chapters[0].GetTitle() != "Opening" {
		t.Fatalf("chapters=%+v", resp.Chapters)
	}
}

func TestParseAudioLanguageField(t *testing.T) {
	m := NewModule(Config{})
	a := m.parseAudio(ffprobeStream{
		Index: 0, CodecType: "audio", CodecName: "ac3", Language: "jpn", Channels: 6,
		Disposition: map[string]int{"comment": 1},
	})
	if a.Language != "jpn" || !a.Commentary {
		t.Fatalf("audio=%+v", a)
	}
}

func TestParseSubtitleDisposition(t *testing.T) {
	m := NewModule(Config{})
	sub := m.parseSubtitle(ffprobeStream{
		Index: 0, CodecName: "hdmv_pgs_subtitle", Language: "eng",
		Disposition: map[string]int{"hearing_impaired": 1},
	})
	if !sub.HearingImpaired || sub.Language != "eng" {
		t.Fatalf("sub=%+v", sub)
	}
}

func TestStoreAndGetCachedHit(t *testing.T) {
	m, root := newTestModule(t)
	path := filepath.Join(root, "clip.mkv")
	if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ffprobev1.AnalyzeResponse{
		FilePath:  path,
		Container: "matroska",
		Video:     &ffprobev1.VideoStream{Codec: "h264", Width: 1920, Height: 1080},
	}
	m.storeCache(path, result)

	ctx := context.Background()
	resp, err := m.GetCached(ctx, &ffprobev1.GetCachedRequest{FilePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Found || resp.Result == nil || resp.Result.Video.Codec != "h264" {
		t.Fatalf("cache miss or wrong payload: %+v", resp)
	}
	got, err := m.Analyze(ctx, &ffprobev1.AnalyzeRequest{FilePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if got.Video.Codec != "h264" {
		t.Fatalf("analyze cache hit codec=%q", got.Video.Codec)
	}
}

func TestInvalidateAndPurgeCache(t *testing.T) {
	m, root := newTestModule(t)
	path := filepath.Join(root, "clip.mkv")
	if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.storeCache(path, &ffprobev1.AnalyzeResponse{FilePath: path, Container: "matroska"})

	ctx := context.Background()
	inv, err := m.Invalidate(ctx, &ffprobev1.InvalidateRequest{FilePath: path})
	if err != nil || !inv.GetRemoved() {
		t.Fatalf("invalidate: %+v err=%v", inv, err)
	}
	cached, err := m.GetCached(ctx, &ffprobev1.GetCachedRequest{FilePath: path})
	if err != nil || cached.GetFound() {
		t.Fatalf("expected miss after invalidate: %+v err=%v", cached, err)
	}

	m.storeCache(path, &ffprobev1.AnalyzeResponse{FilePath: path})
	purged, err := m.PurgeCache(ctx, &ffprobev1.PurgeCacheRequest{})
	if err != nil || purged.GetRemovedCount() < 1 {
		t.Fatalf("purge: %+v err=%v", purged, err)
	}
}

func TestHealthUninitialized(t *testing.T) {
	m := NewModule(Config{DBPath: filepath.Join(t.TempDir(), "x.db"), GRPCAddr: ":0"})
	if err := m.Health(context.Background()); err == nil {
		t.Fatal("expected health error before Init")
	}
}

func TestHealthWithoutFFprobe(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err == nil {
		t.Skip("ffprobe present")
	}
	m, _ := newTestModule(t)
	if err := m.Health(context.Background()); err == nil {
		t.Fatal("expected health error when ffprobe missing")
	}
}
