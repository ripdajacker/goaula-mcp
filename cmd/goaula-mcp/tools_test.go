package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ripdajacker/goaula-mcp"
)

// TestSessionNotReady verifies tool calls on a session that never logged in get an
// actionable InvalidSessionError (after the one relogin attempt against the
// configured — here empty — session file).
func TestSessionNotReady(t *testing.T) {
	provider := &aula.FileSessionProvider{Path: filepath.Join(t.TempDir(), "session.txt")}
	session := newAulaSession(&aula.Config{Session: provider})

	if session.isReady() {
		t.Fatal("session should not be ready before login")
	}

	_, err := session.call(func(client *aula.Client) (any, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var invalidSession *aula.InvalidSessionError
	if !errors.As(err, &invalidSession) {
		t.Fatalf("expected InvalidSessionError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "~/.aula/session.txt") {
		t.Errorf("error should mention the session file: %v", err)
	}
}

func TestIDString(t *testing.T) {
	if got := idString("123"); got != "123" {
		t.Errorf("string: %q", got)
	}
	if got := idString(float64(169219844)); got != "169219844" {
		t.Errorf("float64: %q", got)
	}
	if got := idString(nil); got != "" {
		t.Errorf("nil: %q", got)
	}
}

func TestIntArg(t *testing.T) {
	if got := intArg(nil, 7); got != 7 {
		t.Errorf("nil: %d", got)
	}
	v := 0
	if got := intArg(&v, 7); got != 0 {
		t.Errorf("zero: %d", got)
	}
}

func TestMediaFileName(t *testing.T) {
	if got := mediaFileName("https://s3/img/image.jpg?sig=abc", "fallback"); got != "image.jpg" {
		t.Errorf("path segment: %q", got)
	}
	if got := mediaFileName("https://s3/?sig=abc", "fallback"); got != "fallback" {
		t.Errorf("no segment: %q", got)
	}
}

func TestAlbumMediaFileName(t *testing.T) {
	media := aula.AlbumMedia{ID: 42}
	if got := albumMediaFileName("https://s3/img/P123_orignal.JPG?sig=abc", media); got != "42.jpg" {
		t.Errorf("got %q, want 42.jpg", got)
	}
	if got := albumMediaFileName("https://s3/img/noext?sig=abc", media); got != "42" {
		t.Errorf("got %q, want 42", got)
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "one.jpg")
	if got := uniquePath(a); got != a {
		t.Errorf("existing-free path changed: %q", got)
	}
	if err := os.WriteFile(a, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := uniquePath(a); got != filepath.Join(dir, "one-1.jpg") {
		t.Errorf("collision: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "one-1.jpg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := uniquePath(a); got != filepath.Join(dir, "one-2.jpg") {
		t.Errorf("double collision: %q", got)
	}
}

func TestSingleDownloadTarget(t *testing.T) {
	dir, name := singleDownloadTarget("https://s3/media/123.jpg?sig=abc", "")
	if !strings.HasSuffix(dir, "aula-media") || name != "123.jpg" {
		t.Errorf("default target: dir=%q name=%q", dir, name)
	}
	dir, name = singleDownloadTarget("https://s3/media/123.jpg?sig=abc", "/tmp/pics/photo.jpg")
	if dir != "/tmp/pics/" || name != "photo.jpg" {
		t.Errorf("explicit target: dir=%q name=%q", dir, name)
	}
}

func TestReportDownloads(t *testing.T) {
	out := downloadResult{
		Saved:        []downloadedFile{{Path: "/a/b.jpg", Bytes: 10}},
		SkippedNoURL: 2,
		TotalBytes:   10,
	}
	got := reportDownloads(out)
	if !strings.Contains(got, "Downloaded 1 file(s)") || !strings.Contains(got, "skipped 2") || !strings.Contains(got, "/a/b.jpg") {
		t.Errorf("unexpected report: %s", got)
	}
}

// TestDownloadGalleryMediaSingle runs the single-URL download end to end
// against a stub download host and checks the written file.
func TestDownloadGalleryMediaSingle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("jpeg-bytes"))
	}))
	defer server.Close()

	dir := t.TempDir()
	origDirFn := defaultMediaDirFn
	defaultMediaDirFn = func() (string, error) { return dir, nil }
	defer func() { defaultMediaDirFn = origDirFn }()

	client := aula.NewClient(&aula.Config{Session: &aula.StaticSessionProvider{}})

	target := filepath.Join(dir, "picture.jpg")
	res, err := downloadGalleryMedia(client, downloadArgs{URL: &server.URL, TargetPath: target})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "picture.jpg") {
		t.Errorf("report missing path: %s", res)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "jpeg-bytes" {
		t.Errorf("file content: %q err=%v", data, err)
	}
}

// TestDownloadGalleryMediaAlbum runs the album download end to end: the Aula
// API is stubbed with one media item whose signed URL points at a stub
// download host.
func TestDownloadGalleryMediaAlbum(t *testing.T) {
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("img-" + filepath.Base(r.URL.Path)))
	}))
	defer dl.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "profiles.getProfilesByLogin") {
			w.Write([]byte(`{"data": {}, "version": 22}`))
			return
		}
		w.Write([]byte(`{"data": {"results": [
			{"id": 101, "mediaType": "image", "file": {"url": "` + dl.URL + `/P101_ORIGINAL.jpg"}}
		], "version": 22}}`))
	}))
	defer api.Close()

	dir := t.TempDir()
	origDirFn := defaultMediaDirFn
	defaultMediaDirFn = func() (string, error) { return dir, nil }
	defer func() { defaultMediaDirFn = origDirFn }()

	client := aula.NewClient(&aula.Config{Session: &aula.StaticSessionProvider{}, APIURL: api.URL + "/api/"})
	if err := client.Login(); err != nil {
		t.Fatalf("login: %v", err)
	}

	albumID := int64(7)
	res, err := downloadGalleryMedia(client, downloadArgs{AlbumID: &albumID})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "101.jpg")
	if !strings.Contains(res, want) {
		t.Errorf("report %q missing %q", res, want)
	}
	data, err := os.ReadFile(want)
	if err != nil || string(data) != "img-P101_ORIGINAL.jpg" {
		t.Errorf("file content: %q err=%v", data, err)
	}
}
