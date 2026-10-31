package aula

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

type memorySession struct {
	id    string
	saved []string
}

func (m *memorySession) SessionID() (string, error)    { return m.id, nil }
func (m *memorySession) SaveSessionID(id string) error { m.saved = append(m.saved, id); return nil }

// TestInitializeAPIUseVersionProbing verifies the connector probes API versions until
// one answers 200.
func TestInitializeAPIUseVersionProbing(t *testing.T) {
	var v22Calls, v23Calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v22/"):
			v22Calls.Add(1)
			w.WriteHeader(http.StatusGone)
		case strings.HasPrefix(r.URL.Path, "/api/v23/"):
			v23Calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data": {}, "version": 23}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	session := &memorySession{id: "sess1"}
	conn := newConnector(session)

	got, err := conn.initializeAPIUse(server.URL + "/api/")
	if err != nil {
		t.Fatal(err)
	}
	if want := server.URL + "/api/v23/"; got != want {
		t.Errorf("versioned URL = %q, want %q", got, want)
	}
	if conn.activeAPIVersion != 23 {
		t.Errorf("active version = %d, want 23", conn.activeAPIVersion)
	}
	if v22Calls.Load() != 1 || v23Calls.Load() != 1 {
		t.Errorf("probe calls: v22=%d v23=%d", v22Calls.Load(), v23Calls.Load())
	}
}

// TestInitializeAPIUseFailsAfterMaxTries verifies probing gives up after 5 versions.
func TestInitializeAPIUseFailsAfterMaxTries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	conn := newConnector(&memorySession{id: "sess1"})
	_, err := conn.initializeAPIUse(server.URL + "/api/")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !strings.Contains(apiErr.Message, "Failed to find a working API version") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCallAulaAPISendsCookiesAndCSRF verifies the request headers sent to Aula.
func TestCallAulaAPISendsCookiesAndCSRF(t *testing.T) {
	var gotCookie, gotCSRF, gotUA, gotAccept string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotCSRF = r.Header.Get("Csrfp-Token")
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"ok": true}, "version": 22}`))
	}))
	defer server.Close()

	conn := newConnector(&memorySession{id: "phpsess123"})
	if _, err := conn.initializeAPIUse(server.URL + "/api/"); err != nil {
		t.Fatal(err)
	}
	// Simulate a stored CSRF token for the test host
	u, _ := url.Parse(server.URL)
	conn.cookies.addCookieValue(u.Hostname(), "Csrfp-Token", "csrf-abc")

	data, err := conn.callAulaAPI("profiles.getProfilesByLogin", "get", nil)
	if err != nil {
		t.Fatal(err)
	}

	var parsed struct {
		Data struct {
			OK bool `json:"ok"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || !parsed.Data.OK {
		t.Fatalf("unexpected response: %v %s", err, data)
	}

	if !strings.Contains(gotCookie, "PHPSESSID=phpsess123") {
		t.Errorf("Cookie header missing PHPSESSID: %q", gotCookie)
	}
	if gotCSRF != "csrf-abc" {
		t.Errorf("Csrfp-Token header = %q", gotCSRF)
	}
	if gotUA != defaultUserAgent {
		t.Errorf("User-Agent = %q", gotUA)
	}
	if !strings.Contains(gotAccept, "application/json") {
		t.Errorf("Accept = %q", gotAccept)
	}
}

// TestPHPSESSIDRotation verifies a new PHPSESSID in Set-Cookie is persisted via the
// session provider and used for subsequent requests.
func TestPHPSESSIDRotation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("method") == "profiles.getProfilesByLogin" {
			// version probe: no cookies
			w.Write([]byte(`{"data": {}, "version": 22}`))
			return
		}
		w.Header().Set("Set-Cookie", "PHPSESSID=rotated-session; Path=/; HttpOnly")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {}, "version": 22}`))
	}))
	defer server.Close()

	session := &memorySession{id: "old-session"}
	conn := newConnector(session)
	if _, err := conn.initializeAPIUse(server.URL + "/api/"); err != nil {
		t.Fatal(err)
	}

	if _, err := conn.callAulaAPI("posts.getAllPosts", "get", nil); err != nil {
		t.Fatal(err)
	}

	if len(session.saved) == 0 || session.saved[0] != "rotated-session" {
		t.Errorf("session provider not notified of rotation: %v", session.saved)
	}
	u, _ := url.Parse(server.URL)
	if got := conn.cookies.cookieValue(u.Hostname(), "PHPSESSID"); got != "rotated-session" {
		t.Errorf("stored PHPSESSID = %q", got)
	}
}

// TestCallAulaAPI403JSON verifies a 403 with a JSON status maps to InvalidSessionError.
func TestCallAulaAPI403JSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "profiles.getProfilesByLogin") && r.URL.Path == "/api/v22/" {
			// version probe succeeds
			w.Write([]byte(`{"data": {}, "version": 22}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"status": {"code": 448, "subCode": 1, "message": "Session expired"}}`))
	}))
	defer server.Close()

	conn := newConnector(&memorySession{id: "old-session"})
	if _, err := conn.initializeAPIUse(server.URL + "/api/"); err != nil {
		t.Fatal(err)
	}

	_, err := conn.callAulaAPI("posts.getAllPosts", "get", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var invalidSession *InvalidSessionError
	if !errors.As(err, &invalidSession) {
		t.Fatalf("expected InvalidSessionError, got %T: %v", err, err)
	}
	if invalidSession.HTTPStatus != 403 {
		t.Errorf("HTTPStatus = %d", invalidSession.HTTPStatus)
	}
	if invalidSession.AulaStatusCode == nil || *invalidSession.AulaStatusCode != 448 {
		t.Errorf("AulaStatusCode = %v", invalidSession.AulaStatusCode)
	}
	if !strings.Contains(invalidSession.Message, "posts.getAllPosts") {
		t.Errorf("message = %q", invalidSession.Message)
	}
}

// TestCallAulaAPI403HTML verifies a 403 with an HTML body is reported verbatim.
func TestCallAulaAPI403HTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("method") == "profiles.getProfilesByLogin" {
			w.Write([]byte(`{"data": {}, "version": 22}`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<html><body><h2>Invalid CSRF Token</h2></body></html>`))
	}))
	defer server.Close()

	conn := newConnector(&memorySession{id: "s"})
	if _, err := conn.initializeAPIUse(server.URL + "/api/"); err != nil {
		t.Fatal(err)
	}

	_, err := conn.callAulaAPI("posts.getAllPosts", "get", nil)
	var invalidSession *InvalidSessionError
	if !errors.As(err, &invalidSession) {
		t.Fatalf("expected InvalidSessionError, got %T: %v", err, err)
	}
	if !strings.Contains(invalidSession.Message, "Invalid CSRF Token") {
		t.Errorf("message = %q", invalidSession.Message)
	}
}

// TestCallAulaAPI404 verifies a 404 maps to a method-not-found APIError.
func TestCallAulaAPI404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("method") == "profiles.getProfilesByLogin" {
			w.Write([]byte(`{"data": {}, "version": 22}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	conn := newConnector(&memorySession{id: "s"})
	if _, err := conn.initializeAPIUse(server.URL + "/api/"); err != nil {
		t.Fatal(err)
	}

	_, err := conn.callAulaAPI("bogus.method", "get", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !strings.Contains(apiErr.Message, "Method not found: bogus.method") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCallAulaAPIPost verifies POST bodies are sent as JSON.
func TestCallAulaAPIPost(t *testing.T) {
	var gotBody []byte
	var gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			gotContentType = r.Header.Get("Content-Type")
			gotBody, _ = io.ReadAll(r.Body)
		}
		w.Write([]byte(`{"data": [], "version": 22}`))
	}))
	defer server.Close()

	conn := newConnector(&memorySession{id: "s"})
	if _, err := conn.initializeAPIUse(server.URL + "/api/"); err != nil {
		t.Fatal(err)
	}

	if _, err := conn.callAulaAPI("calendar.getEventsByProfileIdsAndResourceIds", "post",
		map[string]any{"start": "2024-01-01 00:00:00.000+01:00", "instProfileIds": []int64{1}}); err != nil {
		t.Fatal(err)
	}

	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	if !strings.Contains(string(gotBody), `"instProfileIds":[1]`) {
		t.Errorf("body = %s", gotBody)
	}
}
