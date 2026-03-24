// Package downloader orchestrates the full Kinescope video download pipeline.
// It coordinates the kinescope and media packages to fetch stream metadata,
// download and decrypt video and audio tracks, and merge them into a final MP4.
package downloader

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/eshlykov/ksdl/internal/kinescope"
	"github.com/eshlykov/ksdl/internal/media"
)

const (
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
)

// FileNotFoundError is returned when no file matching a glob pattern is found.
type FileNotFoundError struct {
	// Pattern is the glob expression that matched no files.
	Pattern string
}

func (e *FileNotFoundError) Error() string {
	return fmt.Sprintf("no file matching %q found", e.Pattern)
}

// Downloader orchestrates the full video download pipeline.
type Downloader struct {
	baseURL string
	client  *kinescope.Client
	runner  *media.Runner
}

// New creates a Downloader with the given dependencies.
func New(baseURL string, client *kinescope.Client, runner *media.Runner) *Downloader {
	return &Downloader{baseURL: baseURL, client: client, runner: runner}
}

// DownloadVideo runs the full pipeline for a single video ID: fetch metadata,
// download segments and decrypted video, decrypt audio, and merge into a final MP4.
// The output file is named after the video title and saved in the current directory.
// If the output file already exists it is skipped. idx and total are used only for
// progress display.
func (d *Downloader) DownloadVideo(ctx context.Context, id, referrer string, idx, total int) error {
	padWidth := len(strconv.Itoa(total))
	s := func(msg string) { logStep(msg, idx, total, padWidth) }

	s("Fetching embed page and extracting stream URL")
	title, masterURL, err := d.client.ExtractStreamURL(ctx, id, referrer)
	if err != nil {
		return fmt.Errorf("extract stream URL: %w", err)
	}
	slog.Info("video found", "title", title)
	slog.Debug("master playlist", "url", masterURL)

	filename := sanitizeFilename(title)
	outputFile := filename + ".mp4"
	if _, err := os.Stat(outputFile); err == nil {
		slog.Info("skipping, already exists", "file", outputFile)
		return nil
	}

	s("Fetching encryption key")
	keyID, decryptionKey, err := d.client.FetchDecryptionKey(ctx, id, masterURL)
	if err != nil {
		return fmt.Errorf("fetch decryption key: %w", err)
	}
	slog.Debug("encryption key", "id", keyID, "value", decryptionKey)

	workDir, err := os.MkdirTemp(".", "kinescope-*")
	if err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	slog.Debug("work directory", "path", workDir)

	embedURL, _ := url.JoinPath(d.baseURL, id)

	s("Downloading segments with N_m3u8DL-RE (mux failure at the end is expected)")
	d.runner.DownloadSegments(ctx, masterURL, embedURL, filename, workDir)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	encryptedAudio, err := findNewest(filepath.Join(workDir, "*.und.m4a"))
	if err != nil {
		return fmt.Errorf("find encrypted audio file — check N_m3u8DL-RE output above: %w", err)
	}
	slog.Debug("encrypted audio", "path", encryptedAudio)

	s("Downloading decrypted video with yt-dlp")
	d.runner.FetchDecryptedVideo(ctx, masterURL, referrer, workDir)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	decryptedVideo, err := findNewest(filepath.Join(workDir, "_video_tmp.*.mp4"))
	if err != nil {
		return fmt.Errorf("find yt-dlp video output — check yt-dlp output above: %w", err)
	}
	slog.Debug("decrypted video", "path", decryptedVideo)

	s("Decrypting audio with mp4decrypt")
	decryptedAudio := filepath.Join(workDir, "_audio_dec.m4a")
	if err := d.runner.DecryptAudio(ctx, keyID, decryptionKey, encryptedAudio, decryptedAudio); err != nil {
		return err
	}

	s("Merging video and audio with ffmpeg")
	if err := d.runner.MergeAudioVideo(ctx, decryptedVideo, decryptedAudio, outputFile); err != nil {
		return err
	}

	slog.Info("done", "file", outputFile)
	return nil
}

// findNewest returns the most recently modified file matching the glob pattern.
func findNewest(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", &FileNotFoundError{Pattern: pattern}
	}
	newest := matches[0]
	newestTime := int64(0)
	for _, f := range matches {
		if info, err := os.Stat(f); err == nil {
			if t := info.ModTime().Unix(); t > newestTime {
				newestTime = t
				newest = f
			}
		}
	}
	return newest, nil
}

// sanitizeFilename removes characters that are invalid in filenames.
func sanitizeFilename(s string) string {
	re := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	return strings.TrimSpace(re.ReplaceAllString(s, "_"))
}

func logStep(msg string, idx, total, padWidth int) {
	fmt.Printf("\n%s[%*d/%d] %s%s\n", colorGreen, padWidth, idx, total, msg, colorReset)
}
