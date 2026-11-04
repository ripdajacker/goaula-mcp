package aula

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDownloadMedia verifies the media HTTP download follows redirects and
// maps expired signed URLs to a "re-fetch" hint.
func TestDownloadMedia(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch {
		case r.URL.Path == "/redirect":
			// Preserve the signed query like real storage-host redirects do.
			http.Redirect(w, r, "/real?"+r.URL.RawQuery, http.StatusSeeOther)
		case r.URL.Path == "/real":
			if r.URL.Query().Get("sig") != "s3sig" {
				t.Errorf("query params lost on redirect: %q", r.URL.RawQuery)
			}
			w.Write([]byte("jpeg-bytes"))
		case r.URL.Path == "/expired":
			w.WriteHeader(http.StatusForbidden)
		case r.URL.Path == "/boom":
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	defer server.Close()

	client := NewClient(&Config{Session: &StaticSessionProvider{}})

	// Full-size URLs of gallery media are signed with query params.
	data, err := client.DownloadMedia(server.URL + "/real?sig=s3sig")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "jpeg-bytes" {
		t.Errorf("unexpected bytes: %q", data)
	}

	// Redirects are followed.
	if _, err := client.DownloadMedia(server.URL + "/redirect?sig=s3sig"); err != nil {
		t.Errorf("redirect download failed: %v", err)
	}

	// An expired signed URL gets the re-fetch hint.
	_, err = client.DownloadMedia(server.URL + "/expired")
	if err == nil || !strings.Contains(err.Error(), "re-fetch the album media") {
		t.Errorf("expected expired-URL hint, got: %v", err)
	}

	// Other statuses report and never loop.
	_, err = client.DownloadMedia(server.URL + "/boom")
	if err == nil || !strings.Contains(err.Error(), "HTTP 418") {
		t.Errorf("expected HTTP status error, got: %v", err)
	}

	// Scheme validation.
	if _, err := client.DownloadMedia("ftp://example.com/img.jpg"); err == nil {
		t.Error("expected scheme validation error")
	}
	if _, err := client.DownloadMedia(""); err == nil {
		t.Error("expected empty-URL error")
	}
}

// TestMediaForAlbum verifies the album media query and envelope parsing.
func TestMediaForAlbum(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "profiles.getProfilesByLogin") {
			w.Write([]byte(`{"data": {}, "version": 22}`))
			return
		}
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"results": [
			{"id": 42, "mediaType": "image", "file": {"url": "https://example.com/full.jpg?sig=abc"}},
			{"id": 43, "mediaType": "image", "thumbnailUrl": "https://example.com/43_t.jpg"}
		]}, "version": 22}`))
	}))
	defer server.Close()

	client := NewClient(&Config{Session: &StaticSessionProvider{}, APIURL: server.URL + "/api/"})
	if err := client.Login(); err != nil {
		t.Fatalf("login: %v", err)
	}

	media, err := client.MediaForAlbum(7, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(media) != 2 || media[0].FullSizeURL() != "https://example.com/full.jpg?sig=abc" {
		t.Fatalf("unexpected media: %+v", media)
	}
	for _, want := range []string{"method=gallery.getMedia", "albumId=7", "limit=50"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}
