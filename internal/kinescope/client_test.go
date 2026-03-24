package kinescope

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ksdl/internal/config"
)

func TestSelectBestStream(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		masterURL string
		want      string
	}{
		{
			name:      "picks highest bandwidth",
			content:   "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\nlow.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=5000\nhigh.m3u8\n",
			masterURL: "https://example.com/master.m3u8",
			want:      "https://example.com/high.m3u8",
		},
		{
			name:      "resolves relative URL against master",
			content:   "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\nstream.m3u8\n",
			masterURL: "https://example.com/path/master.m3u8",
			want:      "https://example.com/path/stream.m3u8",
		},
		{
			name:      "keeps absolute URL unchanged",
			content:   "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\nhttps://cdn.example.com/stream.m3u8\n",
			masterURL: "https://example.com/master.m3u8",
			want:      "https://cdn.example.com/stream.m3u8",
		},
		{
			name:      "returns empty string for empty playlist",
			content:   "#EXTM3U\n",
			masterURL: "https://example.com/master.m3u8",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectBestStream(tt.content, tt.masterURL)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClient_ExtractStreamURL(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantTitle string
		wantURL   string
		wantErr   error
	}{
		{
			name:      "playerOptions hls source",
			html:      "<title>My Video</title>\n<script>\nvar playerOptions = {\"playlist\":[{\"sources\":{\"hls\":{\"src\":\"https://cdn.example.com/master.m3u8\"}}}]};\n</script>",
			wantTitle: "My Video",
			wantURL:   "https://cdn.example.com/master.m3u8",
		},
		{
			name:      "playerOptions shakahls source",
			html:      "<title>My Video</title>\n<script>\nvar playerOptions = {\"playlist\":[{\"sources\":{\"shakahls\":{\"src\":\"https://cdn.example.com/shaka.m3u8\"}}}]};\n</script>",
			wantTitle: "My Video",
			wantURL:   "https://cdn.example.com/shaka.m3u8",
		},
		{
			name:      "ld+json fallback",
			html:      "<title>Fallback Video</title>\n<script type=\"application/ld+json\">\n{\"contentUrl\":\"https://cdn.example.com/content.m3u8\"}\n</script>",
			wantTitle: "Fallback Video",
			wantURL:   "https://cdn.example.com/content.m3u8",
		},
		{
			name:    "ErrStreamURLNotFound when no URL in page",
			html:    "<title>No Stream</title>",
			wantErr: ErrStreamURLNotFound,
		},
		{
			name:      "defaults title to 'video' when no title tag",
			html:      "<script>\nvar playerOptions = {\"playlist\":[{\"sources\":{\"hls\":{\"src\":\"https://cdn.example.com/master.m3u8\"}}}]};\n</script>",
			wantTitle: "video",
			wantURL:   "https://cdn.example.com/master.m3u8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.html)
			}))
			defer srv.Close()

			cfg := config.Config{BaseURL: srv.URL, HTTPClient: srv.Client()}
			title, streamURL, err := NewClient(cfg).ExtractStreamURL(context.Background(), "vid", "")

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if streamURL != tt.wantURL {
				t.Errorf("streamURL = %q, want %q", streamURL, tt.wantURL)
			}
		})
	}
}

func TestClient_ExtractStreamURL_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	cfg := config.Config{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, _, err := NewClient(cfg).ExtractStreamURL(context.Background(), "vid", "")

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want %d", httpErr.StatusCode, http.StatusForbidden)
	}
}

func TestClient_FetchDecryptionKey(t *testing.T) {
	keyBytes := []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c}
	const wantKeyID = "abc123"
	wantKey := hex.EncodeToString(keyBytes)

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/master.m3u8":
			fmt.Fprintf(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=5000000\n%s/stream.m3u8\n", srvURL)
		case "/stream.m3u8":
			fmt.Fprintf(w, "#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"%s/key/%s\"\n#EXTINF:10,\nseg.ts\n", srvURL, wantKeyID)
		case "/key/" + wantKeyID:
			w.Write(keyBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	cfg := config.Config{BaseURL: srv.URL, HTTPClient: srv.Client()}
	keyID, key, err := NewClient(cfg).FetchDecryptionKey(context.Background(), "vid", srv.URL+"/master.m3u8")
	if err != nil {
		t.Fatalf("FetchDecryptionKey() error: %v", err)
	}
	if keyID != wantKeyID {
		t.Errorf("keyID = %q, want %q", keyID, wantKeyID)
	}
	if key != wantKey {
		t.Errorf("key = %q, want %q", key, wantKey)
	}
}

func TestClient_FetchDecryptionKey_Errors(t *testing.T) {
	t.Run("ErrNoVideoStream when master playlist is empty", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "#EXTM3U\n")
		}))
		defer srv.Close()

		cfg := config.Config{BaseURL: srv.URL, HTTPClient: srv.Client()}
		_, _, err := NewClient(cfg).FetchDecryptionKey(context.Background(), "vid", srv.URL+"/master.m3u8")
		if !errors.Is(err, ErrNoVideoStream) {
			t.Errorf("error = %v, want ErrNoVideoStream", err)
		}
	})

	t.Run("ErrNoEncryptionKey when stream playlist has no EXT-X-KEY", func(t *testing.T) {
		var srvURL string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/master.m3u8":
				fmt.Fprintf(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=5000000\n%s/stream.m3u8\n", srvURL)
			case "/stream.m3u8":
				fmt.Fprint(w, "#EXTM3U\n#EXTINF:10,\nseg.ts\n")
			}
		}))
		defer srv.Close()
		srvURL = srv.URL

		cfg := config.Config{BaseURL: srv.URL, HTTPClient: srv.Client()}
		_, _, err := NewClient(cfg).FetchDecryptionKey(context.Background(), "vid", srv.URL+"/master.m3u8")
		if !errors.Is(err, ErrNoEncryptionKey) {
			t.Errorf("error = %v, want ErrNoEncryptionKey", err)
		}
	})
}
