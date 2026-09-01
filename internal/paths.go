package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func splitCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseAllowPaths(env string, extra []string) []string {
	seen := make(map[string]struct{})
	var roots []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = filepath.Clean(p)
		}
		abs = filepath.Clean(abs)
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}
	for _, p := range extra {
		add(p)
	}
	for _, p := range splitCSV(env) {
		add(p)
	}
	return roots
}

func pathUnderAllowedRoot(path string, roots []string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	abs = filepath.Clean(abs)
	if len(roots) == 0 {
		return "", fmt.Errorf("path %q is not under any allowed root (configure FFPROBE_ALLOW_PATHS)", abs)
	}
	for _, root := range roots {
		root = filepath.Clean(root)
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		return abs, nil
	}
	return "", fmt.Errorf("path %q is outside allowed ffprobe roots", abs)
}

func validateMediaPath(path string, roots []string) (string, error) {
	abs, err := pathUnderAllowedRoot(path, roots)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return abs, nil
		}
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}
	if _, err := pathUnderAllowedRoot(resolved, roots); err != nil {
		return "", fmt.Errorf("symlink target %q is outside allowed ffprobe roots", resolved)
	}
	return resolved, nil
}
