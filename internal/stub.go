package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ffprobev1 "github.com/Muxcore-Media/media-ffprobe/proto/ffprobev1"
)

const stubErrorNote = "stub analysis: ffprobe not available; metadata inferred from filename"

func ffprobeAvailable(bin string) bool {
	if strings.Contains(bin, string(os.PathSeparator)) {
		info, err := os.Stat(bin)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(bin)
	return err == nil
}

func stubAnalyze(path string) (*ffprobev1.AnalyzeResponse, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	base := filepath.Base(path)
	w, h := stubDimsFromName(base)
	codec := stubCodecFromName(base)

	video := &ffprobev1.VideoStream{
		Index:           0,
		Codec:           codec,
		Width:           int32(w),
		Height:          int32(h),
		ResolutionLabel: resolutionLabel(w, h),
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	container := ext
	if container == "" {
		container = "unknown"
	}

	resp := &ffprobev1.AnalyzeResponse{
		FilePath:  path,
		SizeBytes: info.Size(),
		Container: container,
		Video:     video,
		Error:     stubErrorNote,
	}
	resp.Quality = classifyQuality(video)
	return resp, nil
}

func stubDimsFromName(name string) (width, height int) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "2160p") || strings.Contains(lower, "4320p") || strings.Contains(lower, ".4k."):
		return 3840, 2160
	case strings.Contains(lower, "1080p"):
		return 1920, 1080
	case strings.Contains(lower, "720p"):
		return 1280, 720
	case strings.Contains(lower, "576p"):
		return 720, 576
	case strings.Contains(lower, "480p"):
		return 640, 480
	default:
		return 640, 480
	}
}

func stubCodecFromName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "x265") || strings.Contains(lower, "h265") || strings.Contains(lower, "hevc"):
		return "hevc"
	case strings.Contains(lower, "av1"):
		return "av1"
	case strings.Contains(lower, "vp9"):
		return "vp9"
	case strings.Contains(lower, "x264") || strings.Contains(lower, "h264") || strings.Contains(lower, "avc"):
		return "h264"
	default:
		return "h264"
	}
}
