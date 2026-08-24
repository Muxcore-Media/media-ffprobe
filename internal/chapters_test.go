package internal

import (
	"testing"
)

func TestGenerateIntervalChapters(t *testing.T) {
	if got := GenerateIntervalChapters(100, 600); got != nil {
		t.Fatalf("short duration should not generate: %v", got)
	}
	ch := GenerateIntervalChapters(3600, 600)
	if len(ch) != 6 {
		t.Fatalf("want 6 chapters, got %d", len(ch))
	}
	if ch[0].Source != chapterSourceInterval || ch[0].StartSeconds != 0 {
		t.Fatalf("first chapter: %+v", ch[0])
	}
	if ch[5].EndSeconds != 3600 {
		t.Fatalf("last end: %v", ch[5].EndSeconds)
	}
}

func TestDedupeSceneStarts(t *testing.T) {
	starts := dedupeSceneStarts([]float64{0, 10, 11, 50, 51, 120}, 30)
	if len(starts) != 3 || starts[1] != 50 {
		t.Fatalf("got %v", starts)
	}
}

func TestSceneStartsToChapters(t *testing.T) {
	ch := sceneStartsToChapters([]float64{120, 600}, 1200)
	if len(ch) != 3 {
		t.Fatalf("want 3 with leading zero, got %d", len(ch))
	}
	if ch[0].Title != "Scene 1" || ch[0].Source != chapterSourceScene {
		t.Fatalf("first: %+v", ch[0])
	}
	if ch[2].EndSeconds != 1200 {
		t.Fatalf("last end %v", ch[2].EndSeconds)
	}
}
