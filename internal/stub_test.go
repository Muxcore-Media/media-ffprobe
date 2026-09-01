package internal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	ffprobev1 "github.com/Muxcore-Media/media-ffprobe/proto/ffprobev1"
)

func TestAnalyzeStubWhenFFprobeAbsent(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err == nil {
		t.Skip("ffprobe present on PATH; stub path not exercised")
	}

	m, root := newTestModule(t)
	ctx := context.Background()

	filePath := filepath.Join(root, "Fight.Club.1999.1080p.BluRay.x264.mkv")
	if err := os.WriteFile(filePath, []byte("fixture-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := m.Analyze(ctx, &ffprobev1.AnalyzeRequest{FilePath: filePath})
	if err != nil {
		t.Fatalf("Analyze stub: %v", err)
	}
	if resp.GetVideo() == nil {
		t.Fatal("expected stub video stream")
	}
	if resp.GetVideo().GetWidth() != 1920 || resp.GetVideo().GetHeight() != 1080 {
		t.Fatalf("dims: got %dx%d want 1920x1080", resp.GetVideo().GetWidth(), resp.GetVideo().GetHeight())
	}
	if resp.GetQuality() == nil || resp.GetQuality().GetResolution() != "1080p" {
		t.Fatalf("quality resolution: %+v", resp.GetQuality())
	}
	if resp.GetQuality().GetSource() != "BluRay" {
		t.Fatalf("quality source: %+v", resp.GetQuality())
	}
	if resp.GetSizeBytes() != 13 {
		t.Fatalf("size: got %d want 13", resp.GetSizeBytes())
	}
	if resp.GetError() == "" {
		t.Error("expected stub error note")
	}

	cached, err := m.GetCached(ctx, &ffprobev1.GetCachedRequest{FilePath: filePath})
	if err != nil {
		t.Fatal(err)
	}
	if cached.GetFound() {
		t.Fatal("stub results must not be cached")
	}
}

func TestStubDimsAndCodec(t *testing.T) {
	w, h := stubDimsFromName("movie.2160p.remux.mkv")
	if w != 3840 || h != 2160 {
		t.Fatalf("2160 dims: %dx%d", w, h)
	}
	if got := stubCodecFromName("movie.x265.mkv"); got != "hevc" {
		t.Fatalf("codec: got %q want hevc", got)
	}
}

func TestAnalyzeWithHostFFmpegFixture(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe absent; stub path covered by TestAnalyzeStubWhenFFprobeAbsent")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg absent; cannot synthesize Analyze fixture")
	}

	m, root := newTestModule(t)
	tmp := t.TempDir()
	out := filepath.Join(tmp, "fixture-1080p.mkv")
	cmd := exec.Command(ffmpeg, "-y", "-f", "lavfi", "-i", "testsrc=size=1920x1080:rate=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", out)
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg fixture generate failed (ok offline): %v\n%s", err, outBytes)
	}

	allowed := filepath.Join(root, "allowed.mkv")
	if err := os.Link(out, allowed); err != nil {
		if err2 := copyFile(out, allowed); err2 != nil {
			allowed = out
			m.cfgMu.Lock()
			m.allowPaths = parseAllowPaths("", []string{tmp})
			m.cfgMu.Unlock()
		}
	}

	resp, err := m.Analyze(context.Background(), &ffprobev1.AnalyzeRequest{FilePath: allowed})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if resp.GetVideo() == nil || resp.GetVideo().GetWidth() != 1920 {
		t.Fatalf("video: %+v", resp.GetVideo())
	}
	if resp.GetQuality() == nil || resp.GetQuality().GetResolution() != "1080p" {
		t.Fatalf("quality: %+v", resp.GetQuality())
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
