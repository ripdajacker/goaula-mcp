package aula

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestAlbums verifies the album listing makes one getAlbums call (and no
// per-album getMedia calls), with the media fields left empty.
func TestAlbums(t *testing.T) {
	var mediaCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "gallery.getMedia") {
			mediaCalls++
		}
		if strings.Contains(r.URL.RawQuery, "profiles.getProfilesByLogin") {
			w.Write([]byte(`{"data": {}, "version": 22}`))
			return
		}
		w.Write([]byte(`{"data": [
			{"id": 7, "title": "Sommerfest", "creationDate": "2025-07-01T10:00:00+02:00"},
			{"id": 8, "title": "Skolestart", "creationDate": "2025-08-01T10:00:00+02:00"}
		], "version": 22}`))
	}))
	defer server.Close()

	client := NewClient(&Config{Session: &StaticSessionProvider{}, APIURL: server.URL + "/api/"})
	if err := client.Login(); err != nil {
		t.Fatalf("login: %v", err)
	}

	albums, err := client.Albums(10, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if mediaCalls != 0 {
		t.Errorf("gallery.getMedia called %d times; Albums must not fetch media", mediaCalls)
	}
	if len(albums) != 2 || albums[0].ID != 8 || albums[1].ID != 7 {
		t.Errorf("unexpected albums: %+v", albums)
	}
	for i := range albums {
		if len(albums[i].Media) != 0 {
			t.Errorf("album %d should have no media", albums[i].ID)
		}
	}
}

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

// TestCalendarEventsChunking emulates Aula's measured 50-day span cap: any
// request whose start..end window exceeds it answers HTTP 403 with Aula code
// 403. A year-long request must transparently page in chunks, cover every
// day, and dedupe the boundary-overlap event.
func TestCalendarEventsChunking(t *testing.T) {
	layout := "2006-01-02 15:04:05.000Z07:00"
	calendarCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "profiles.getProfilesByLogin") {
			w.Write([]byte(`{"data": {}, "version": 22}`))
			return
		}
		if strings.Contains(r.URL.RawQuery, "calendar.getEvents") {
			calendarCalls++
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
			}
			var req struct {
				Start string `json:"start"`
				End   string `json:"end"`
			}
			_ = json.Unmarshal(body, &req)
			s, sErr := time.Parse(layout, req.Start)
			e, eErr := time.Parse(layout, req.End)
			if sErr != nil || eErr != nil || e.Before(s) {
				t.Errorf("bad calendar window: start=%q end=%q", req.Start, req.End)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if window := int(e.Sub(s).Hours() / 24); window > 50 {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"status": {"code": 403, "subCode": 0, "message": ""}}`))
				return
			}
			// One unique event per day inside the window, plus a boundary
			// event at the exact (overlapping) end timestamp. IDs are
			// absolute-date based, so overlapping days share IDs — like the
			// real API's stable event IDs.
			var events []CalendarEvent
			for d := 0; d <= int(e.Sub(s).Hours()/24); d++ {
				day := s.AddDate(0, 0, d)
				events = append(events, CalendarEvent{
					ID:            int64(day.Year()*1000 + day.YearDay()),
					StartDateTime: day.UTC().Format(time.RFC3339),
					Title:         "event " + day.UTC().Format("2006-01-02"),
				})
			}
			boundary := CalendarEvent{ID: 987654, StartDateTime: e.UTC().Format(time.RFC3339), Title: "boundary"}
			events = append(events, boundary)
			raw, _ := MarshalJSON(events)
			w.Write([]byte(`{"data": ` + string(raw) + `, "version": 22}`))
		}
	}))
	defer server.Close()

	client := NewClient(&Config{Session: &StaticSessionProvider{}, APIURL: server.URL + "/api/"})
	if err := client.Login(); err != nil {
		t.Fatal(err)
	}

	events, err := client.CalendarEvents(100, 100, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calendarCalls < 3 {
		t.Errorf("expected >=3 chunked calls, got %d", calendarCalls)
	}

	seen := map[int64]int{}
	for _, e := range events {
		seen[e.ID]++
		if seen[e.ID] > 1 {
			t.Errorf("duplicate event id %d", e.ID)
		}
	}
	// Every day of the 201-day window plus the boundary event.
	if got := len(events); got < 202 {
		t.Errorf("expected >=202 unique events, got %d", got)
	}
}
