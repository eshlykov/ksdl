// Package config defines the configuration types shared across all ksdl packages.
// It provides the Config struct, the HTTPDoer interface for HTTP injection,
// and the Default constructor with production-ready defaults.
package config

import (
	"net/http"
	"time"
)

// Tools holds the names of the external binaries used in the download pipeline.
// Each field is the executable name resolved via PATH.
type Tools struct {
	// Nm3u8DL is the name of the N_m3u8DL-RE binary used to download HLS segments.
	Nm3u8DL string
	// Ytdlp is the name of the yt-dlp binary used to download and decrypt the video stream.
	Ytdlp string
	// Mp4dec is the name of the mp4decrypt binary used to decrypt the audio stream.
	Mp4dec string
	// Ffmpeg is the name of the ffmpeg binary used to merge video and audio into the final MP4.
	Ffmpeg string
}

// Config holds all runtime configuration for the downloader.
type Config struct {
	// BaseURL is the base URL of the Kinescope embed service, without a trailing slash.
	BaseURL string
	// HTTPClient is used for all API, playlist, and key requests.
	HTTPClient *http.Client
	// Tools contains the names of the required external binaries.
	Tools Tools
}

// Default returns a Config populated with production defaults.
func Default() Config {
	return Config{
		BaseURL:    "https://kinescope.io",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Tools: Tools{
			Nm3u8DL: "N_m3u8DL-RE",
			Ytdlp:   "yt-dlp",
			Mp4dec:  "mp4decrypt",
			Ffmpeg:  "ffmpeg",
		},
	}
}
