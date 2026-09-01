package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathUnderAllowedRoot(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "movie.mkv")
	roots := []string{root}

	got, err := pathUnderAllowedRoot(child, roots)
	if err != nil {
		t.Fatal(err)
	}
	if got != child {
		t.Fatalf("got %q want %q", got, child)
	}

	outside := filepath.Join(os.TempDir(), "outside.mkv")
	if _, err := pathUnderAllowedRoot(outside, roots); err == nil {
		t.Fatal("expected error for outside path")
	}
}

func TestValidateMediaPathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.mkv")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.mkv")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlink not permitted")
	}
	if _, err := validateMediaPath(link, []string{root}); err == nil {
		t.Fatal("expected symlink escape rejection")
	}
}

func TestParseAllowPaths(t *testing.T) {
	root := t.TempDir()
	got := parseAllowPaths(root+","+root, nil)
	if len(got) != 1 {
		t.Fatalf("dedupe failed: %v", got)
	}
}
