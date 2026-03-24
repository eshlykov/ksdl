// Package media wraps the external media processing tools used in the download pipeline:
// N_m3u8DL-RE for segment downloading, yt-dlp for video decryption,
// mp4decrypt for audio decryption, and ffmpeg for muxing.
package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/eshlykov/ksdl/internal/config"
)

// ToolNotFoundError is returned when a required external tool is not found in PATH.
type ToolNotFoundError struct {
	// Tool is the name of the missing binary.
	Tool string
}

func (e *ToolNotFoundError) Error() string {
	return fmt.Sprintf("tool not found in PATH: %s", e.Tool)
}

// Runner runs external media tools.
type Runner struct {
	cfg config.Config
}

// NewRunner creates a Runner with the given configuration.
func NewRunner(cfg config.Config) *Runner {
	return &Runner{cfg: cfg}
}

// CheckTools verifies that all required external tools are present in PATH.
// It returns a ToolNotFoundError for the first missing binary it encounters.
func (r *Runner) CheckTools() error {
	for _, tool := range []string{r.cfg.Tools.Nm3u8DL, r.cfg.Tools.Ytdlp, r.cfg.Tools.Mp4dec, r.cfg.Tools.Ffmpeg} {
		if _, err := exec.LookPath(tool); err != nil {
			return &ToolNotFoundError{Tool: tool}
		}
	}
	return nil
}

// DownloadSegments runs N_m3u8DL-RE to download all HLS segments into workDir.
// The mux step at the end of N_m3u8DL-RE is expected to fail for encrypted streams;
// its exit code is intentionally ignored.
func (r *Runner) DownloadSegments(ctx context.Context, masterURL, embedURL, filename, workDir string) {
	cmd := exec.CommandContext(ctx, r.cfg.Tools.Nm3u8DL,
		"-H", "referer: "+embedURL,
		"--log-level", "INFO",
		"--del-after-done",
		"-M", "format=mp4:muxer=ffmpeg",
		"--save-name", filename,
		"--auto-select",
		masterURL,
	)
	cmd.Dir = workDir // N_m3u8DL-RE saves to current directory by default
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run() // mux will fail — ignore exit code
}

// FetchDecryptedVideo runs yt-dlp to download the best-quality video stream into workDir.
// yt-dlp handles decryption of the video track; the result is saved as _video_tmp.*.mp4.
// Errors are intentionally ignored because the audio is handled separately.
func (r *Runner) FetchDecryptedVideo(ctx context.Context, masterURL, referrer, workDir string) {
	_ = run(ctx, r.cfg.Tools.Ytdlp,
		"--referer", referrer,
		"--add-header", "Origin:"+r.cfg.BaseURL,
		"-f", "bestvideo+bestaudio",
		"--merge-output-format", "mp4",
		"-o", filepath.Join(workDir, "_video_tmp.%(ext)s"),
		masterURL,
	)
}

// DecryptAudio runs mp4decrypt to decrypt inputFile using the given CENC key,
// writing the result to outputFile.
func (r *Runner) DecryptAudio(ctx context.Context, keyID, decryptionKey, inputFile, outputFile string) error {
	if err := run(ctx, r.cfg.Tools.Mp4dec,
		"--key", keyID+":"+decryptionKey,
		inputFile,
		outputFile,
	); err != nil {
		return fmt.Errorf("mp4decrypt failed: %w", err)
	}
	return nil
}

// MergeAudioVideo runs ffmpeg to combine videoPath and audioPath into outputFile
// using stream copy (no re-encoding).
func (r *Runner) MergeAudioVideo(ctx context.Context, videoPath, audioPath, outputFile string) error {
	if err := run(ctx, r.cfg.Tools.Ffmpeg,
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-c", "copy",
		outputFile,
	); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}

// run executes a command, streaming its output, and returns any error.
func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
