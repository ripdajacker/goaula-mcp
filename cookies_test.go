package aula

import (
	"reflect"
	"testing"
)

func TestCookieManagerAddAndGet(t *testing.T) {
	c := newCookieManager()
	c.addCookieHeader("www.aula.dk", "PHPSESSID=abc123; Path=/; HttpOnly")
	c.addCookieHeader("www.aula.dk", "Csrfp-Token=xyz; Path=/")

	if !c.hasCookie("www.aula.dk", "PHPSESSID") {
		t.Error("expected PHPSESSID cookie")
	}
	if got := c.cookieValue("www.aula.dk", "Csrfp-Token"); got != "xyz" {
		t.Errorf("Csrfp-Token = %q, want %q", got, "xyz")
	}
	if c.hasCookie("other.dk", "PHPSESSID") {
		t.Error("did not expect PHPSESSID for other.dk")
	}
}

func TestCookieManagerDomainCookies(t *testing.T) {
	c := newCookieManager()
	c.addCookieHeaderArray("www.aula.dk", []string{"Shared=1; Domain=.aula.dk; Path=/"})

	if !c.hasEntries("www.aula.dk") {
		t.Error("expected entries for www.aula.dk")
	}
	// The domain cookie should be visible to a sibling subdomain too
	got := c.allCookiesArray("api.aula.dk")
	if len(got) != 1 || got[0] != "Shared=1" {
		t.Errorf("allCookiesArray(api.aula.dk) = %v, want [Shared=1]", got)
	}
}

func TestCookieManagerHostCookieWins(t *testing.T) {
	c := newCookieManager()
	// A host-specific cookie wins over a domain cookie with the same name.
	c.addCookieValue("www.aula.dk", "Token", "host")
	c.addCookieValue("aula.dk", "Token", "domain")

	got := c.allCookiesString("www.aula.dk")
	want := "Token=host;"
	if got != want {
		t.Errorf("allCookiesString = %q, want %q", got, want)
	}
}

func TestCookieManagerInsertionOrder(t *testing.T) {
	c := newCookieManager()
	c.addCookieValue("www.aula.dk", "A", "1")
	c.addCookieValue("www.aula.dk", "B", "2")
	c.addCookieValue("www.aula.dk", "A", "3") // update keeps position

	want := []string{"A=3", "B=2"}
	if got := c.allCookiesArray("www.aula.dk"); !reflect.DeepEqual(got, want) {
		t.Errorf("allCookiesArray = %v, want %v", got, want)
	}
}

func TestEncodeHost(t *testing.T) {
	for in, want := range map[string]string{
		"www.aula.dk":          "www_aula_dk",
		"https://www.aula.dk/": "www_aula_dk",
		"aula.dk":              "aula_dk",
	} {
		if got := encodeHost(in); got != want {
			t.Errorf("encodeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRootDomain(t *testing.T) {
	for in, want := range map[string]string{
		"www.aula.dk":           "aula.dk",
		"aula.dk":               "aula.dk",
		"https://sub.aula.dk/x": "aula.dk",
		"localhost":             "localhost",
	} {
		if got := rootDomain(in); got != want {
			t.Errorf("rootDomain(%q) = %q, want %q", in, got, want)
		}
	}
}
