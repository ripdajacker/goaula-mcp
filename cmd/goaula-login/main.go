// Command goaula-login opens a visible Chrome window for interactive Aula login
// and stores the captured session ID (PHPSESSID) in ~/.aula/session.txt, so the
// client library and the goaula-mcp MCP server can use it.
//
// The login flow itself lives in the login package, which is shared with the
// goaula-mcp server. Two login methods are supported — the flow is identical,
// the user simply picks whichever matches their MitID setup:
//
//   - MitID app: scan the QR code shown in the browser window (or in the
//     screenshot file written by --qr-out) with the MitID app on the phone.
//   - MitID code reader: ignore the QR code and log in with username,
//     password and the physical code reader.
//
// Usage:
//
//	goaula-login                          # headed Chrome, interactive login
//	goaula-login --headless               # no window; QR screenshots written to file
//	goaula-login --browser-path /usr/bin/chromium
//	goaula-login --force                  # re-login even if the stored session is valid
//	goaula-login --ping                   # after login, ping Aula until Ctrl-C
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	aula "github.com/ripdajacker/goaula-mcp"
	"github.com/ripdajacker/goaula-mcp/login"
)

func main() {
	var (
		headless    = flag.Bool("headless", false, "run Chrome headless (QR is only available via the screenshot file)")
		browserPath = flag.String("browser-path", "", "path to the Chrome/Chromium executable (default: auto-detect)")
		sessionFile = flag.String("session-file", "", "path of the session file (default: ~/.aula/session.txt)")
		qrOut       = flag.String("qr-out", "goaula-login-qr.png", "file to write the QR screenshot to (empty disables)")
		settle      = flag.Duration("settle", 10*time.Second, "how long the session token must stay unchanged before it is saved (the secure-login step-up via the UNI-Login broker rotates the token, and may require user interaction)")
		noStepUp    = flag.Bool("skip-stepup", false, "skip the secure-login step-up (session will have the basic authentication level)")
		pingFlag    = flag.Bool("ping", false, "after login, ping the API synchronously to keep the session alive until interrupted (Ctrl-C)")
		pingInt     = flag.Duration("ping-interval", 15*time.Minute, "ping interval when --ping is set")
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

	sessionID, err := capture(login.Options{
		StartURL:    *startURL,
		Headless:    *headless,
		BrowserPath: *browserPath,
		Settle:      *settle,
		Timeout:     *timeout,
		SkipStepUp:  *noStepUp,
		QRPath:      *qrOut,
	})
	if err != nil {
		slog.Error("Login failed", "error", err)
		os.Exit(1)
	}

	if err := provider.SaveSessionID(sessionID); err != nil {
		slog.Error("Login failed", "error", fmt.Errorf("saving session: %w", err))
		os.Exit(1)
	}
	fmt.Printf("Authenticated! Session ID saved to %s\n", sessionPath)

	if *pingFlag {
		if err := pingLoop(sessionID, *pingInt, sessionPath); err != nil {
			slog.Error("Ping loop ended", "error", err)
			os.Exit(1)
		}
	}
}

// capture runs the interactive login flow, blocking until it completes.
func capture(opts login.Options) (string, error) {
	fmt.Println("Opening browser — log in to Aula using the MitID app (scan the QR code)",
		"or a physical MitID code reader.")

	session := login.NewSession(opts)
	defer session.Close()
	session.Start()

	var lastErr error
	for {
		snap := session.Poll(opts.Timeout)
		if snap.Err != nil {
			lastErr = snap.Err
		}
		switch snap.Status {
		case login.StatusAuthenticated:
			return snap.SessionID, nil
		case login.StatusBrowserClosed:
			return "", errors.New("browser window was closed before login completed")
		case login.StatusWaiting:
			if lastErr != nil {
				return "", lastErr
			}
			// Keep waiting; the QR callback handles the screenshot output.
		}
	}
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

// pingLoop keeps the session token alive with periodic pings (same pattern as
// the MCP server's ping loop) until the process is interrupted (Ctrl-C) or
// the session expires.
func pingLoop(sessionID string, interval time.Duration, sessionPath string) error {
	client := aula.NewClient(&aula.Config{Session: aula.StaticSessionProvider{ID: sessionID}})
	defer client.Close()
	if err := client.Login(); err != nil {
		return fmt.Errorf("ping login: %w", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	stop := make(chan struct{})
	go func() {
		<-sig
		close(stop)
	}()

	fmt.Printf("Pinging Aula every %s to keep the session alive — press Ctrl-C to exit.\n", interval)
	if err := aula.PingLoop(func() error {
		return client.PingAula()
	}, interval, stop); err != nil {
		return err
	}
	fmt.Println("\nExiting — session remains in", sessionPath)
	return nil
}
