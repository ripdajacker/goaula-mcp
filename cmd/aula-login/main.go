// Command aula-login opens a visible Chrome window for interactive Aula login
// and stores the captured session ID (PHPSESSID) in ~/.aula/session.txt, so the
// client library and the goaula-mcp MCP server can use it.
//
// Two login methods are supported — the flow is identical, the user simply
// picks whichever matches their MitID setup:
//
//   - MitID app: scan the QR code shown in the browser window (or in the
//     screenshot file written by --qr-out) with the MitID app on the phone.
//   - MitID code reader: ignore the QR code and log in with username,
//     password and the physical code reader.
//
// The login page is reloaded every few minutes while the user is still on it,
// so the QR code never goes stale. The reload is skipped once the user has
// navigated away from the login page, so a code reader login in progress is
// never interrupted.
//
// Usage:
//
//	aula-login                          # headed Chrome, interactive login
//	aula-login --headless               # no window; QR screenshots written to file
//	aula-login --browser-path /usr/bin/chromium
//	aula-login --force                  # re-login even if the stored session is valid
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	aula "github.com/ripdajacker/goaula-mcp"
)

const (
	pollInterval   = 3 * time.Second
	pageSettleTime = 3 * time.Second // time for the MitID QR to render
	qrRefreshEvery = 4 * time.Minute // Aula/MitID QR codes expire after a few minutes
)

func main() {
	var (
		headless    = flag.Bool("headless", false, "run Chrome headless (QR is only available via the screenshot file)")
		browserPath = flag.String("browser-path", "", "path to the Chrome/Chromium executable (default: auto-detect)")
		sessionFile = flag.String("session-file", "", "path of the session file (default: ~/.aula/session.txt)")
		qrOut       = flag.String("qr-out", "aula-login-qr.png", "file to write the QR screenshot to (empty disables)")
		settleFlag  = flag.Duration("settle", 10*time.Second, "how long the session token must stay unchanged before it is saved (the secure-login step-up via the UNI-Login broker rotates the token, and may require user interaction)")
		noStepUp    = flag.Bool("skip-stepup", false, "skip the secure-login step-up (session will have the basic authentication level)")
		timeout     = flag.Duration("timeout", 10*time.Minute, "how long to wait for login to complete")
		force       = flag.Bool("force", false, "re-login even if the stored session is still valid")
		startURL    = flag.String("url", "https://www.aula.dk/", "Aula URL to open")
		verbose     = flag.Bool("v", false, "verbose logging")
	)
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	sessionPath := *sessionFile
	if sessionPath == "" {
		sessionPath = aula.DefaultSessionFile()
	}
	provider := &aula.FileSessionProvider{Path: sessionPath}

	if !*force && haveValidSession(provider) {
		fmt.Println("Already authenticated — session in", sessionPath, "is valid. Use --force to re-login.")
		return
	}

	if err := run(*startURL, *headless, *browserPath, *qrOut, *settleFlag, *timeout, *noStepUp, provider); err != nil {
		slog.Error("Login failed", "error", err)
		os.Exit(1)
	}
}

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

// findBrowser returns the browser executable to use: the explicit
// --browser-path if set, otherwise the first well-known Chrome/Chromium binary
// found on PATH or in its platform-specific install location.
func findBrowser(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("--browser-path %q: %w", explicit, err)
		}
		return explicit, nil
	}
	for _, candidate := range browserCandidates() {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("no Chrome/Chromium executable found — install Chrome or Chromium, or pass --browser-path")
}

// apiV25 is the API base URL used by the step-up assertion. Like the client
// library's initial version detection, it starts at v25.
const apiV25 = "https://www.aula.dk/api/v25/"

// stepUpEnvelope mirrors the Aula API response envelope plus the profile
// fields read by the portal SPA: profiles.getProfileContext answers 448
// ("secure login required") while the session still needs the step-up, and
// once stepped up it returns the profile with isSteppedUp=true.
type stepUpEnvelope struct {
	Status struct {
		Code int `json:"code"`
	} `json:"status"`
	Data struct {
		IsSteppedUp bool `json:"isSteppedUp"`
	} `json:"data"`
}

// sessionSteppedUp asserts whether the session already has the "secure login"
// (step-up) level — without touching the browser. Whether a plain login
// yields a stepped-up token depends on the user's MitID specifics, so this is
// checked before initiating the SAML step-up. Any failure to determine the
// state conservatively reports "not stepped up".
func sessionSteppedUp(sess string) bool {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}

	// The first API response sets the Csrfp-Token cookie needed for
	// authenticated calls.
	bootstrap, err := http.NewRequest(http.MethodGet,
		apiV25+"?method=profiles.getProfilesByLogin", nil)
	if err != nil {
		return false
	}
	bootstrap.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: sess})
	resp, err := client.Do(bootstrap)
	if err != nil {
		return false
	}
	resp.Body.Close()

	var csrf string
	for _, c := range resp.Cookies() {
		if c.Name == "Csrfp-Token" {
			csrf = c.Value
		}
	}

	req, err := http.NewRequest(http.MethodGet,
		apiV25+"?method=profiles.getProfileContext", nil)
	if err != nil {
		return false
	}
	req.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: sess})
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if csrf != "" {
		req.AddCookie(&http.Cookie{Name: "Csrfp-Token", Value: csrf})
		req.Header.Set("Csrfp-Token", csrf)
	}
	resp, err = client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var env stepUpEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return false
	}
	return env.Status.Code == 0 && env.Data.IsSteppedUp
}

// haveValidSession reports whether the stored session ID can log in.
func haveValidSession(provider aula.SessionProvider) bool {
	id, err := provider.SessionID()
	if err != nil || id == "" {
		return false
	}
	client := aula.NewClient(&aula.Config{Session: aula.StaticSessionProvider{ID: id}})
	defer client.Close()
	if err := client.Login(); err != nil {
		slog.Info("Stored session is no longer valid", "error", err)
		return false
	}
	return true
}

func run(startURL string, headless bool, browserPath, qrOut string, settle, timeout time.Duration, skipStepUp bool, provider *aula.FileSessionProvider) error {
	// DefaultExecAllocatorOptions includes Headless; add VisibleWindow for a
	// headed, maximized window.
	opts := chromedp.DefaultExecAllocatorOptions[:]
	if !headless {
		opts = append(opts, chromedp.VisibleWindow)
	}
	exe, err := findBrowser(browserPath)
	if err != nil {
		return err
	}
	slog.Debug("Using browser", "executable", exe)
	opts = append(opts, chromedp.ExecPath(exe))

	// The timeout context must not be the one that launches the browser
	// (that would kill the browser on expiry), so layer them.
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	ctx, cancel := context.WithTimeout(browserCtx, timeout)
	defer cancel()

	fmt.Println("Opening browser — log in to Aula using the MitID app (scan the QR code)",
		"or a physical MitID code reader.")

	if err := chromedp.Do(ctx,
		chromedp.Navigate(startURL),
		chromedp.WaitVisible(chromedp.CSS("body")),
	); err != nil {
		return fmt.Errorf("navigating to %s: %w", startURL, err)
	}
	lastRefresh := time.Now()
	time.Sleep(pageSettleTime)
	saveScreenshot(ctx, qrOut)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Aula's "secure" login flow can rotate the PHPSESSID once more shortly
	// after the first token appears. Only save a token that has remained
	// unchanged for settle, so we don't capture a pre-rotation token.
	var (
		lastToken   string
		settleStart time.Time
		steppedUp   bool
	)

	for {
		select {
		case <-browserCtx.Done():
			// The browser was closed by the user.
			return errors.New("browser window was closed before login completed")
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("timed out after %s waiting for login", timeout)
			}
			return errors.New("browser window was closed before login completed")
		case <-ticker.C:
		}

		sess, ok, err := checkForSession(ctx)
		if err != nil {
			// The page may be mid-navigation; retry on the next tick.
			if browserCtx.Err() != nil {
				return errors.New("browser window was closed before login completed")
			}
			slog.Debug("Poll failed, retrying", "error", err)
			continue
		}
		if ok {
			if sess != lastToken {
				if lastToken == "" {
					slog.Info("Session token captured, watching briefly for the secure-login rotation",
						"window", settle)
				} else {
					slog.Info("Session token rotated, continuing to watch", "window", settle)
				}
				lastToken = sess
				settleStart = time.Now()
				continue
			}
			if time.Since(settleStart) < settle {
				continue
			}

			if err := provider.SaveSessionID(sess); err != nil {
				return fmt.Errorf("saving session: %w", err)
			}
			fmt.Printf("Authenticated! Session ID saved to %s\n", provider.Path)

			// Only do the SAML step-up if the session isn't already
			// stepped up — whether a plain login yields a privileged
			// token depends on the user's MitID specifics.
			if !skipStepUp && !steppedUp {
				steppedUp = true
				if sessionSteppedUp(sess) {
					slog.Info("Session already has the secure-login (step-up) level — skipping")
				} else {
					fmt.Println("Initiating secure-login step-up — an extra MitID approval may be required in the browser.")
					if err := chromedp.Do(ctx,
						chromedp.Navigate("https://www.aula.dk/auth/stepup.php?stepupDone=true"),
					); err != nil {
						return fmt.Errorf("initiating step-up: %w", err)
					}
					lastToken = "" // restart the settle logic for the upgraded token
					continue
				}
			}

			if err := verifySession(sess); err != nil {
				// Keep watching — the token may rotate into a valid one.
				slog.Warn("Captured session did not verify against the API; continuing to watch for a rotated token", "error", err)
				settleStart = time.Now()
				continue
			}
			return nil
		}

		// Re-load the login page if it has been open too long (stale QR).
		// Only while the user is still on the login page — never mid-MitID.
		if time.Since(lastRefresh) >= qrRefreshEvery {
			if loc, err := chromedp.Run(ctx, chromedp.Location()); err == nil && isLoginPage(loc) {
				slog.Info("Login page open for a while, reloading to refresh the QR code")
				if err := chromedp.Do(ctx, chromedp.Reload()); err == nil {
					lastRefresh = time.Now()
					time.Sleep(pageSettleTime)
					saveScreenshot(ctx, qrOut)
				}
			}
		}
	}
}

// checkForSession polls the browser for the Aula PHPSESSID cookie combined with
// a logged-in URL. It returns (sessionID, true) once the user has completed the
// login, and ("", false) while still waiting.
func checkForSession(ctx context.Context) (string, bool, error) {
	loc, err := chromedp.Run(ctx, chromedp.Location())
	if err != nil {
		return "", false, err
	}
	result, err := chromedp.Call(ctx, network.GetCookies, network.GetCookiesParams{})
	if err != nil {
		return "", false, err
	}

	var sess string
	for _, c := range result.Cookies {
		if c.Name == "PHPSESSID" {
			sess = c.Value
			break
		}
	}
	if sess == "" || !isLoggedIn(loc) {
		return "", false, nil
	}
	return sess, true, nil
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

// saveScreenshot writes a full-page screenshot of the current browser window
// (the QR code area) to qrOut. Quality 100 = PNG.
func saveScreenshot(ctx context.Context, qrOut string) {
	if qrOut == "" {
		return
	}
	buf, err := chromedp.Run(ctx, chromedp.FullScreenshot(100))
	if err != nil {
		slog.Warn("Failed to capture QR screenshot", "error", err)
		return
	}
	if err := os.WriteFile(qrOut, buf, 0o600); err != nil {
		slog.Warn("Failed to write QR screenshot", "path", qrOut, "error", err)
		return
	}
	fmt.Printf("QR screenshot written to %s — scan it with the MitID app\n", qrOut)
}

// verifySession checks that the captured session works against the API.
func verifySession(sess string) error {
	client := aula.NewClient(&aula.Config{Session: aula.StaticSessionProvider{ID: sess}})
	defer client.Close()
	return client.Login()
}
