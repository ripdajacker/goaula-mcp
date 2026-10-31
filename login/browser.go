package login

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// browserCandidates lists well-known Chrome/Chromium executables, probed in
// order. The first one found is used.
func browserCandidates() []string {
	candidates := []string{
		"chromium",
		"chromium-browser",
		"google-chrome-stable",
		"google-chrome",
		"chrome",
	}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		)
	case "windows":
		candidates = append(candidates,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			os.ExpandEnv(`${LOCALAPPDATA}\Google\Chrome\Application\chrome.exe`),
			`C:\Program Files\Chromium\Application\chrome.exe`,
		)
	case "linux":
		// Flatpak installs expose launcher scripts in their export
		// directories, which are often not on PATH.
		for _, dir := range flatpakExportDirs() {
			for _, app := range []string{"org.chromium.Chromium", "com.google.Chrome"} {
				candidates = append(candidates, filepath.Join(dir, app))
			}
		}
	}
	return candidates
}

// flatpakExportDirs returns the directories where Flatpak exposes application
// launchers (system-wide and per-user).
func flatpakExportDirs() []string {
	dirs := []string{"/var/lib/flatpak/exports/bin"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "flatpak", "exports", "bin"))
	}
	return dirs
}

// findBrowser returns the browser executable to use: the explicit path if set,
// otherwise the first well-known Chrome/Chromium binary found on PATH or in
// its platform-specific install location.
func findBrowser(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("browser path %q: %w", explicit, err)
		}
		return explicit, nil
	}
	for _, candidate := range browserCandidates() {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("no Chrome/Chromium executable found — install Chrome or Chromium, or pass a browser path")
}

// isLoginPage reports whether the URL is the Aula login page — the only page
// it is safe to auto-reload for a fresh QR code. UNI-Login and step-up URLs
// must never be reloaded (and note "unilogin" contains "login").
func isLoginPage(loc string) bool {
	lower := strings.ToLower(loc)
	if strings.Contains(lower, "unilogin") ||
		strings.Contains(lower, "stepup") ||
		strings.Contains(lower, "saml") {
		return false
	}
	return strings.Contains(lower, "login")
}

// isLoggedIn reports whether login is fully complete. The "secure" login
// (step-up) flow routes through the UNI-Login SAML broker and the Aula
// step-up endpoint, where an intermediate PHPSESSID may exist — that stage
// must not count as logged in. A plain aula.dk/ URL also does not count:
// the pre-login front page sets a PHPSESSID too.
func isLoggedIn(loc string) bool {
	lower := strings.ToLower(loc)
	if strings.Contains(lower, "unilogin") ||
		strings.Contains(lower, "stepup") ||
		strings.Contains(lower, "saml") {
		return false
	}
	return strings.Contains(loc, "/portal") || strings.Contains(loc, "#/")
}
