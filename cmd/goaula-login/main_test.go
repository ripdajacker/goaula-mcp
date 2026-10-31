package main

import "testing"

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
