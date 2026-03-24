// Package kinescope provides an HTTP client for the Kinescope video platform.
// It fetches embed pages to extract HLS stream URLs and retrieves CENC decryption
// keys from the Kinescope license server.
package kinescope

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ksdl/internal/config"
)

var (
	// ErrStreamURLNotFound is returned when no HLS stream URL can be extracted from the embed page.
	ErrStreamURLNotFound = errors.New("could not find stream URL in embed page")
	// ErrNoVideoStream is returned when the master playlist contains no video stream entries.
	ErrNoVideoStream = errors.New("no video stream found in master playlist")
	// ErrNoEncryptionKey is returned when the stream playlist contains no EXT-X-KEY directive.
	ErrNoEncryptionKey = errors.New("no EXT-X-KEY found in stream playlist")
)

// HTTPError is returned when an HTTP request receives a non-200 response.
type HTTPError struct {
	// StatusCode is the HTTP status code returned by the server.
	StatusCode int
	// URL is the request URL that produced the error.
	URL string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d for %s", e.StatusCode, e.URL)
}

// Client fetches data from the Kinescope API and embed pages.
type Client struct {
	cfg config.Config
}

// NewClient creates a new Client using the provided configuration.
func NewClient(cfg config.Config) *Client {
	return &Client{cfg: cfg}
}

// ExtractStreamURL fetches the embed page for videoID and pulls out the HLS master playlist URL and video title.
func (c *Client) ExtractStreamURL(ctx context.Context, videoID, referrer string) (title, streamURL string, err error) {
	embedURL, _ := url.JoinPath(c.cfg.BaseURL, videoID)
	body, err := c.fetch(ctx, embedURL, referrer)
	if err != nil {
		return "", "", err
	}

	// Title
	if m := regexp.MustCompile(`<title>(.*?)</title>`).FindSubmatch(body); m != nil {
		title = strings.TrimSpace(string(m[1]))
	} else {
		title = "video"
	}

	// Primary: playerOptions JS object
	if m := regexp.MustCompile(`(?s)var\s+playerOptions\s*=\s*(\{.*?\});\s*\n`).FindSubmatch(body); m != nil {
		jsonStr := strings.ReplaceAll(string(m[1]), `\u0026`, `&`)
		var opts struct {
			Playlist []struct {
				Sources struct {
					HLS      struct{ Src string } `json:"hls"`
					ShakaHLS struct{ Src string } `json:"shakahls"`
				} `json:"sources"`
			} `json:"playlist"`
		}
		if json.Unmarshal([]byte(jsonStr), &opts) == nil && len(opts.Playlist) > 0 {
			if src := opts.Playlist[0].Sources.HLS.Src; src != "" {
				return title, src, nil
			}
			if src := opts.Playlist[0].Sources.ShakaHLS.Src; src != "" {
				return title, src, nil
			}
		}
	}

	// Fallback: application/ld+json
	if m := regexp.MustCompile(`(?s)<script type="application/ld\+json">\s*(.*?)\s*</script>`).FindSubmatch(body); m != nil {
		var ld struct {
			ContentURL string `json:"contentUrl"`
		}
		if json.Unmarshal(m[1], &ld) == nil && ld.ContentURL != "" {
			return title, ld.ContentURL, nil
		}
	}

	return "", "", ErrStreamURLNotFound
}

// FetchDecryptionKey fetches the master playlist for videoID, selects the best stream, and returns
// the key ID and decryption key fetched from the license server.
func (c *Client) FetchDecryptionKey(ctx context.Context, videoID, masterURL string) (keyID, decryptionKey string, err error) {
	referrer, _ := url.JoinPath(c.cfg.BaseURL, videoID)
	masterPlaylist, err := c.fetch(ctx, masterURL, referrer)
	if err != nil {
		return "", "", fmt.Errorf("fetch master playlist: %w", err)
	}

	streamURL := selectBestStream(string(masterPlaylist), masterURL)
	if streamURL == "" {
		return "", "", ErrNoVideoStream
	}

	streamPlaylist, err := c.fetch(ctx, streamURL, referrer)
	if err != nil {
		return "", "", fmt.Errorf("fetch stream playlist: %w", err)
	}

	// Find the EXT-X-KEY URI
	keyMatch := regexp.MustCompile(`EXT-X-KEY:[^\n]*URI="([^"]+)"`).FindSubmatch(streamPlaylist)
	if keyMatch == nil {
		return "", "", ErrNoEncryptionKey
	}
	keyURI := string(keyMatch[1])

	// Extract key ID: it's the last path segment of the URI before the query string
	u, err := url.Parse(keyURI)
	if err != nil {
		return "", "", fmt.Errorf("parse key URI: %w", err)
	}
	parts := strings.Split(strings.TrimRight(u.Path, "/"), "/")
	keyID = parts[len(parts)-1]

	// Fetch the raw key bytes from the license server (no auth needed)
	keyBytes, err := c.fetch(ctx, keyURI, c.cfg.BaseURL)
	if err != nil {
		return "", "", fmt.Errorf("fetch key: %w", err)
	}
	decryptionKey = hex.EncodeToString(keyBytes)

	return keyID, decryptionKey, nil
}

// selectBestStream parses a master playlist and returns the absolute URL of the highest-bandwidth stream.
func selectBestStream(content, masterURL string) string {
	bandwidthRe := regexp.MustCompile(`BANDWIDTH=(\d+)`)
	lines := strings.Split(content, "\n")
	maxBandwidth := 0
	bestURL := ""

	for i, line := range lines {
		if !strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			continue
		}
		bw := 0
		if m := bandwidthRe.FindStringSubmatch(line); m != nil {
			bw, _ = strconv.Atoi(m[1])
		}
		if bw <= maxBandwidth {
			continue
		}
		if i+1 >= len(lines) {
			continue
		}
		next := strings.TrimSpace(lines[i+1])
		if next == "" || strings.HasPrefix(next, "#") {
			continue
		}
		maxBandwidth = bw
		bestURL = next
	}

	if bestURL == "" {
		return ""
	}
	base, err := url.Parse(masterURL)
	if err != nil {
		return bestURL
	}
	ref, err := url.Parse(bestURL)
	if err != nil {
		return bestURL
	}
	return base.ResolveReference(ref).String()
}

// fetch makes a GET request with the given referer and returns the response body.
func (c *Client) fetch(ctx context.Context, rawURL, referer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{StatusCode: resp.StatusCode, URL: rawURL}
	}
	return io.ReadAll(resp.Body)
}
