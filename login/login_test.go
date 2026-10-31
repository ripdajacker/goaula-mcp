package login

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsLoggedIn(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		// Logged-in states
		{"https://www.aula.dk/portal", true},
		{"https://www.aula.dk/#/universe", true},
		// Not logged in — the pre-login front page also sets a PHPSESSID
		{"https://www.aula.dk/", false},
		{"https://www.aula.dk/login/", false},
		{"https://broker.unilogin.dk/auth/realms/broker/protocol/saml-stil?SAMLRequest=abc&RelayState=/auth/stepup.php", false},
		{"https://www.aula.dk/auth/stepup.php?stepupDone=true", false},
		{"https://www.aula.dk/auth/stepup.php", false},
		{"https://broker.unilogin.dk/some/other/page", false},
		// Unrelated sites are not "logged in"
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := isLoggedIn(c.url); got != c.want {
			t.Errorf("isLoggedIn(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestIsLoginPage(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.aula.dk/login/", true},
		{"https://www.aula.dk/", false},
		// Never reload mid-step-up, even though it is on aula.dk.
		{"https://www.aula.dk/auth/stepup.php?stepupDone=true", false},
		{"https://broker.unilogin.dk/auth/realms/broker/protocol/saml-stil", false},
	}
	for _, c := range cases {
		if got := isLoginPage(c.url); got != c.want {
			t.Errorf("isLoginPage(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestFindBrowserExplicitPath(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "my-chrome")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findBrowser(exe)
	if err != nil {
		t.Fatalf("explicit path: %v", err)
	}
	if got != exe {
		t.Fatalf("got %q, want %q", got, exe)
	}

	missing := filepath.Join(dir, "missing")
	if _, err := findBrowser(missing); err == nil {
		t.Fatal("expected error for missing explicit path")
	}
}

func TestFindBrowserAutoDetect(t *testing.T) {
	// Should find a system Chrome/Chromium if one is installed, and produce a
	// clear error otherwise — either outcome is fine as long as it doesn't panic.
	got, err := findBrowser("")
	if err != nil {
		t.Skipf("no browser on this machine: %v", err)
	}
	if got == "" {
		t.Fatal("empty path with nil error")
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("detected browser %q: %v", got, err)
	}
}
