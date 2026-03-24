package downloader

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean name unchanged", "My Video", "My Video"},
		{"replaces colon", "Video: Title", "Video_ Title"},
		{"replaces forward slash", "path/to/video", "path_to_video"},
		{"replaces backslash", `path\to\video`, "path_to_video"},
		{"replaces null byte", "video\x00name", "video_name"},
		{"trims surrounding spaces", "  My Video  ", "My Video"},
		{"replaces angle brackets", "<title>", "_title_"},
		{"replaces double quote", `say "hello"`, "say _hello_"},
		{"replaces pipe", "a|b", "a_b"},
		{"replaces question mark", "what?", "what_"},
		{"replaces asterisk", "a*b", "a_b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindNewest(t *testing.T) {
	t.Run("returns newest file among matches", func(t *testing.T) {
		dir := t.TempDir()
		older := filepath.Join(dir, "older.m4a")
		newer := filepath.Join(dir, "newer.m4a")

		if err := os.WriteFile(older, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(newer, nil, 0600); err != nil {
			t.Fatal(err)
		}

		now := time.Now()
		if err := os.Chtimes(older, now.Add(-time.Second), now.Add(-time.Second)); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(newer, now, now); err != nil {
			t.Fatal(err)
		}

		got, err := findNewest(filepath.Join(dir, "*.m4a"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != newer {
			t.Errorf("got %q, want %q", got, newer)
		}
	})

	t.Run("returns FileNotFoundError when no files match", func(t *testing.T) {
		pattern := filepath.Join(t.TempDir(), "*.m4a")
		_, err := findNewest(pattern)

		var notFound *FileNotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("expected *FileNotFoundError, got %T: %v", err, err)
		}
		if notFound.Pattern != pattern {
			t.Errorf("Pattern = %q, want %q", notFound.Pattern, pattern)
		}
	})
}
