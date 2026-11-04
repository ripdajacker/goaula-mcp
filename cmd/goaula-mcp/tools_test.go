package main

import (
	"errors"
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
