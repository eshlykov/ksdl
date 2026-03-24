package media

import (
	"errors"
	"testing"

	"github.com/eshlykov/ksdl/internal/config"
)

func TestRunner_CheckTools(t *testing.T) {
	t.Run("returns nil when all tools exist", func(t *testing.T) {
		// Use "go" as a stand-in — always present in a Go test environment.
		cfg := config.Config{Tools: config.Tools{
			Nm3u8DL: "go",
			Ytdlp:   "go",
			Mp4dec:  "go",
			Ffmpeg:  "go",
		}}
		if err := NewRunner(cfg).CheckTools(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("returns ToolNotFoundError for first missing tool", func(t *testing.T) {
		const missing = "this-binary-does-not-exist-ksdl"
		cfg := config.Config{Tools: config.Tools{
			Nm3u8DL: missing,
			Ytdlp:   "go",
			Mp4dec:  "go",
			Ffmpeg:  "go",
		}}
		err := NewRunner(cfg).CheckTools()

		var notFound *ToolNotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("expected *ToolNotFoundError, got %T: %v", err, err)
		}
		if notFound.Tool != missing {
			t.Errorf("Tool = %q, want %q", notFound.Tool, missing)
		}
	})
}
