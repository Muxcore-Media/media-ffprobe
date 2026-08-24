package internal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	ffprobev1 "github.com/Muxcore-Media/media-ffprobe/proto/ffprobev1"
)

const (
	chapterSourceEmbedded = "embedded"
	chapterSourceScene    = "scene"
	chapterSourceInterval = "interval"

	defaultChapterIntervalSec = 600
	minChapterIntervalSec     = 120
	minGeneratedDurationSec   = 180
	maxSceneChapters          = 48
	minSceneGapSec            = 45
)

var sceneTimeRe = regexp.MustCompile(`pts_time:([0-9.]+)`)

// GenerateIntervalChapters builds evenly spaced chapter markers for long-form content.
func GenerateIntervalChapters(durationSec, intervalSec float64) []*ffprobev1.Chapter {
	if durationSec < minGeneratedDurationSec {
		return nil
	}
	if intervalSec < minChapterIntervalSec {
		intervalSec = defaultChapterIntervalSec
	}
	var out []*ffprobev1.Chapter
	idx := int32(0)
	for start := 0.0; start < durationSec-minSceneGapSec; start += intervalSec {
		end := start + intervalSec
		if end > durationSec {
			end = durationSec
		}
		out = append(out, &ffprobev1.Chapter{
			Index:        idx,
			Title:        fmt.Sprintf("Chapter %d", idx+1),
			StartSeconds: start,
			EndSeconds:   end,
			Source:       chapterSourceInterval,
		})
		idx++
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

// DetectSceneChapterStarts uses ffmpeg scene-change detection to find cut points.
func DetectSceneChapterStarts(ctx context.Context, ffmpegBin, path string, threshold float64) ([]float64, error) {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	if threshold <= 0 {
		threshold = 0.35
	}
	filter := fmt.Sprintf("select='gt(scene,%g)',showinfo", threshold)
	cmd := exec.CommandContext(ctx, ffmpegBin,
		"-hide_banner", "-loglevel", "info",
		"-i", path,
		"-filter:v", filter,
		"-an", "-f", "null", "-",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg scene detect: %w", err)
	}
	seen := map[int]struct{}{}
	var starts []float64
	sc := bufio.NewScanner(&stderr)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "showinfo") {
			continue
		}
		m := sceneTimeRe.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		t, err := strconv.ParseFloat(m[1], 64)
		if err != nil || t < minSceneGapSec {
			continue
		}
		bucket := int(math.Round(t))
		if _, ok := seen[bucket]; ok {
			continue
		}
		seen[bucket] = struct{}{}
		starts = append(starts, t)
	}
	sort.Float64s(starts)
	return dedupeSceneStarts(starts, minSceneGapSec), nil
}

func dedupeSceneStarts(starts []float64, minGap float64) []float64 {
	if len(starts) == 0 {
		return nil
	}
	out := []float64{starts[0]}
	for _, t := range starts[1:] {
		if t-out[len(out)-1] >= minGap {
			out = append(out, t)
		}
	}
	if len(out) > maxSceneChapters {
		out = out[:maxSceneChapters]
	}
	return out
}

func sceneStartsToChapters(starts []float64, durationSec float64) []*ffprobev1.Chapter {
	if len(starts) == 0 || durationSec <= 0 {
		return nil
	}
	// Always include t=0 as first chapter when we have scene cuts.
	if starts[0] > 1 {
		starts = append([]float64{0}, starts...)
	}
	out := make([]*ffprobev1.Chapter, 0, len(starts))
	for i, start := range starts {
		end := durationSec
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		out = append(out, &ffprobev1.Chapter{
			Index:        int32(i),
			Title:        fmt.Sprintf("Scene %d", i+1),
			StartSeconds: start,
			EndSeconds:   end,
			Source:       chapterSourceScene,
		})
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

func enrichChaptersFromFile(ctx context.Context, ffmpegBin, path string, durationSec float64, embedded []*ffprobev1.Chapter) []*ffprobev1.Chapter {
	if len(embedded) > 0 {
		for _, ch := range embedded {
			if ch != nil && ch.Source == "" {
				ch.Source = chapterSourceEmbedded
			}
		}
		return embedded
	}
	if durationSec < minGeneratedDurationSec {
		return nil
	}
	if starts, err := DetectSceneChapterStarts(ctx, ffmpegBin, path, 0.35); err == nil {
		if scene := sceneStartsToChapters(starts, durationSec); len(scene) >= 2 {
			return scene
		}
	}
	return GenerateIntervalChapters(durationSec, defaultChapterIntervalSec)
}

func ffmpegBinFor(ffprobeBin string) string {
	if v := strings.TrimSpace(os.Getenv("FFMPEG_BIN")); v != "" {
		return v
	}
	if ffprobeBin != "" && ffprobeBin != "ffprobe" {
		dir := filepath.Dir(ffprobeBin)
		return filepath.Join(dir, "ffmpeg")
	}
	return "ffmpeg"
}
