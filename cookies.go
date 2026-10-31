package aula

import (
	"net/url"
	"strings"
	"sync"
)

// cookieJar is an insertion-ordered set of cookie name/value pairs for one host.
type cookieJar struct {
	keys   []string
	values map[string]string
}

func newCookieJar() *cookieJar {
	return &cookieJar{values: make(map[string]string)}
}

func (j *cookieJar) put(name, value string) {
	if _, exists := j.values[name]; !exists {
		j.keys = append(j.keys, name)
	}
	j.values[name] = value
}

// cookieManager manages cookies per host, including support for domain-wide cookies.
type cookieManager struct {
	mu    sync.Mutex
	hosts map[string]*cookieJar
}

func newCookieManager() *cookieManager {
	return &cookieManager{hosts: make(map[string]*cookieJar)}
}

// addCookieHeader parses a single Set-Cookie header value ("name=value; ...") and
// stores the cookie for the host.
func (c *cookieManager) addCookieHeader(host, cookieHeader string) {
	host = encodeHost(host)
	equalIndex := strings.Index(cookieHeader, "=")
	if equalIndex < 0 {
		return
	}
	key := strings.TrimSpace(cookieHeader[:equalIndex])
	value := cookieHeader[equalIndex+1:]
	if semicolonIndex := strings.Index(value, ";"); semicolonIndex != -1 {
		value = value[:semicolonIndex]
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureHostEntry(host)
	c.hosts[host].put(key, strings.TrimSpace(value))
}

// addCookieHeaderArray stores an array of Set-Cookie header values for the host, and
// additionally for the cookie's domain attribute (domain-wide cookies).
func (c *cookieManager) addCookieHeaderArray(host string, cookieHeaders []string) {
	for _, cookie := range cookieHeaders {
		c.addCookieHeader(host, cookie)

		for _, part := range strings.Split(cookie, ";") {
			trimmed := strings.TrimSpace(part)
			if len(trimmed) > len("domain=") && strings.EqualFold(trimmed[:len("domain=")], "domain=") {
				domain := strings.TrimSpace(trimmed[len("domain="):])
				domain = strings.TrimPrefix(domain, ".")
				c.addCookieHeader(domain, cookie)
				break
			}
		}
	}
}

func (c *cookieManager) clearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hosts = make(map[string]*cookieJar)
}

func (c *cookieManager) hasCookie(host, name string) bool {
	host = encodeHost(host)
	c.mu.Lock()
	defer c.mu.Unlock()
	jar, ok := c.hosts[host]
	if !ok {
		return false
	}
	_, ok = jar.values[name]
	return ok
}

func (c *cookieManager) addCookieValue(host, name, value string) {
	host = encodeHost(host)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureHostEntry(host)
	c.hosts[host].put(name, value)
}

func (c *cookieManager) cookieValue(host, name string) string {
	host = encodeHost(host)
	c.mu.Lock()
	defer c.mu.Unlock()
	jar, ok := c.hosts[host]
	if !ok {
		return ""
	}
	return jar.values[name]
}

// hasEntries reports whether there are any cookies for the host or its root domain.
func (c *cookieManager) hasEntries(host string) bool {
	domainOnlyHost := encodeHost(rootDomain(host))
	host = encodeHost(host)

	c.mu.Lock()
	defer c.mu.Unlock()
	if jar, ok := c.hosts[host]; ok && len(jar.values) > 0 {
		return true
	}
	if jar, ok := c.hosts[domainOnlyHost]; ok && len(jar.values) > 0 {
		return true
	}
	return false
}

// allCookiesString returns all cookies for the host (and its root domain) as a single
// Cookie header value: "name=value; name2=value2;".
func (c *cookieManager) allCookiesString(host string) string {
	return strings.Join(c.allCookiesArray(host), "; ") + ";"
}

// allCookiesArray returns the "name=value" pairs for the host and its root domain.
// Host-specific cookies win over domain cookies with the same name.
func (c *cookieManager) allCookiesArray(host string) []string {
	domainOnlyHost := encodeHost(rootDomain(host))
	host = encodeHost(host)

	c.mu.Lock()
	defer c.mu.Unlock()

	var result []string
	for _, h := range []string{host, domainOnlyHost} {
		jar, ok := c.hosts[h]
		if !ok || len(jar.values) == 0 {
			continue
		}
		for _, key := range jar.keys {
			exists := false
			for _, existing := range result {
				if strings.HasPrefix(existing, key+"=") {
					exists = true
					break
				}
			}
			if exists {
				continue
			}
			result = append(result, key+"="+jar.values[key])
		}
	}
	return result
}

// encodeHost normalizes a host (or URL) for use as a map key: URLs are reduced to
// their host name and dots are replaced with underscores.
func encodeHost(host string) string {
	if host != "" && (strings.Contains(host, ":") || strings.Contains(host, "/")) {
		if u, err := url.Parse(host); err == nil && u.Hostname() != "" {
			host = u.Hostname()
		}
	}
	return strings.ReplaceAll(host, ".", "_")
}

// rootDomain returns the last two labels of a host name (e.g. "www.aula.dk" -> "aula.dk").
func rootDomain(host string) string {
	if host == "" {
		return ""
	}
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.Split(host, "/")[0]
	splits := strings.Split(host, ".")
	if len(splits) >= 2 {
		return splits[len(splits)-2] + "." + splits[len(splits)-1]
	}
	return host
}

func (c *cookieManager) ensureHostEntry(host string) {
	if _, ok := c.hosts[host]; !ok {
		c.hosts[host] = newCookieJar()
	}
}
