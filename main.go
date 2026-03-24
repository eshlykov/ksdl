// Command ksdl downloads videos from Kinescope given a file of video IDs.
// It fetches each video's HLS stream, decrypts the audio, and merges everything
// into a single MP4 file using N_m3u8DL-RE, yt-dlp, mp4decrypt, and ffmpeg.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"ksdl/internal/config"
	"ksdl/internal/downloader"
	"ksdl/internal/kinescope"
	"ksdl/internal/media"
)

func setupLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{} // omit timestamp for interactive CLI use
			}
			return a
		},
	})))
}

func main() {
	var file, referrer string
	var verbose bool
	cfg := config.Default()

	cmd := &cobra.Command{
		Use:     "ksdl",
		Short:   "Kinescope Downloader",
		Long:    "ksdl downloads videos from Kinescope given a list of video IDs.",
		Example: "ksdl -f ids.txt -r https://example.com",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setupLogger(verbose)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := media.NewRunner(cfg)
			if err := runner.CheckTools(); err != nil {
				return err
			}

			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}

			var ids []string
			for id := range strings.SplitSeq(string(data), "\n") {
				if id = strings.TrimSpace(id); id != "" {
					ids = append(ids, id)
				}
			}
			if len(ids) == 0 {
				return fmt.Errorf("no IDs found in %s", file)
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			dl := downloader.New(cfg.BaseURL, kinescope.NewClient(cfg), runner)

			for i, id := range ids {
				fmt.Printf("\n\033[32m=== %s ===\033[0m\n", id)
				if err := dl.DownloadVideo(ctx, id, referrer, i+1, len(ids)); err != nil {
					if ctx.Err() != nil {
						return fmt.Errorf("interrupted")
					}
					return fmt.Errorf("failed for ID %s: %w", id, err)
				}
			}

			fmt.Println("\nAll done.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "file containing video IDs, one per line")
	cmd.Flags().StringVarP(&referrer, "referrer", "r", "", "referrer URL passed with each request")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show debug output")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("referrer")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
