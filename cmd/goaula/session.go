package main

// Session and authentication commands: login (the interactive MitID flow,
// shared with the MCP server via the login package), session status, and the
// keep-alive ping.

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

// cmdLogin mirrors goaula-login: browser-based MitID login capturing the
// session token into ~/.aula/session.txt.
func cmdLogin(args []string) int {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	headless := fs.Bool("headless", false, "run Chrome headless (QR is only available via the screenshot file)")
	browserPath := fs.String("browser-path", "", "path of the Chrome/Chromium executable (default: auto-detect)")
	sessionFile := fs.String("session-file", "", "path of the session file (default: ~/.aula/session.txt)")
	qrOut := fs.String("qr-out", "goaula-login-qr.png", "file to write the QR screenshot to (empty disables)")
	settle := fs.Duration("settle", 10*time.Second, "how long the session token must stay unchanged before it is saved (the secure-login step-up rotates the token)")
	noStepUp := fs.Bool("skip-stepup", false, "skip the secure-login step-up (session will have the basic authentication level)")
	timeout := fs.Duration("timeout", 10*time.Minute, "how long to wait for login to complete")
	force := fs.Bool("force", false, "re-login even if the stored session is still valid")
	startURL := fs.String("url", "https://www.aula.dk/", "Aula URL to open")
	verbose := fs.Bool("v", false, "verbose logging")
	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if *verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	sessionPath := *sessionFile
	if sessionPath == "" {
		sessionPath = aula.DefaultSessionFile()
	}
	provider := &aula.FileSessionProvider{Path: sessionPath}

	if !*force && haveValidSession(provider) {
		fmt.Println("Already authenticated — session in", sessionPath, "is valid. Use --force to re-login.")
		return exitOK
	}

	sessionID, err := captureLogin(login.Options{
		StartURL:    *startURL,
		Headless:    *headless,
		BrowserPath: *browserPath,
		Settle:      *settle,
		Timeout:     *timeout,
		SkipStepUp:  *noStepUp,
		QRPath:      *qrOut,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "goaula: login failed: %v\n", err)
		return exitFail
	}

	if err := provider.SaveSessionID(sessionID); err != nil {
		fmt.Fprintf(os.Stderr, "goaula: login failed: saving session: %v\n", err)
		return exitFail
	}
	fmt.Printf("Authenticated! Session ID saved to %s\n", sessionPath)
	return exitOK
}

// haveValidSession reports whether the stored session ID can log in.
func haveValidSession(provider aula.SessionProvider) bool {
	id, err := provider.SessionID()
	if err != nil || id == "" {
		return false
	}
	client := aula.NewClient(&aula.Config{Session: provider})
	defer client.Close()
	return client.Login() == nil
}

// captureLogin runs the interactive login flow, blocking until it completes.
func captureLogin(opts login.Options) (string, error) {
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
			if snap.SteppedUp {
				fmt.Println("Secure login (step-up) level achieved.")
			}
			return snap.SessionID, nil
		case login.StatusBrowserClosed:
			return "", errors.New("browser window was closed before login completed")
		case login.StatusWaiting:
			if lastErr != nil {
				return "", lastErr
			}
			// Keep waiting; the QR screenshot goes to the file.
		}
	}
}

// cmdSession dispatches session status/ping.
func cmdSession(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `Usage: goaula session <status|ping>`)
		return exitUsage
	}
	switch args[0] {
	case "status":
		return cmdSessionStatus(args[1:])
	case "ping":
		return cmdSessionPing(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "goaula: unknown session subcommand %q\n", args[0])
		return exitUsage
	}
}

// cmdSessionStatus checks the session without touching the browser. Exit 0
// when authenticated (freshly verified), 1 otherwise; --json prints
// {"authenticated": true|false, ...} either way.
func cmdSessionStatus(args []string) int {
	fs := flag.NewFlagSet("session status", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	client := aula.NewClient(&aula.Config{Session: aula.NewFileSessionProvider()})
	defer client.Close()
	if err := client.Login(); err != nil {
		reason := err.Error()
		if *jsonOut {
			_ = jsonOutPrint(map[string]any{"authenticated": false, "reason": reason})
		} else {
			fmt.Fprintf(os.Stderr, "goaula: not authenticated: %v\n", reason)
			fmt.Fprintln(os.Stderr, "       Run 'goaula login' first.")
		}
		return exitFail
	}
	if *jsonOut {
		_ = jsonOutPrint(map[string]any{"authenticated": true})
	} else {
		fmt.Println("Authenticated.")
	}
	return exitOK
}

// cmdSessionPing pings once (exit 0/1) or keep-alives in a loop with --watch.
// The watch loop is for cron and background scripts: it exits 1 when the
// session expires, 0 on Ctrl-C/termination.
func cmdSessionPing(args []string) int {
	fs := flag.NewFlagSet("session ping", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output machine-readable JSON")
	watch := fs.Bool("watch", false, "keep pinging until interrupted or the session expires")
	interval := fs.Duration("interval", 15*time.Minute, "ping interval with --watch")
	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	client := aula.NewClient(&aula.Config{Session: aula.NewFileSessionProvider()})
	defer client.Close()
	// Negotiate the API version once so the pings do not warn; this also
	// validates the session like 'session status'.
	if err := client.Login(); err != nil {
		if !*watch {
			return pingResult(false, err, *jsonOut)
		}
		fmt.Fprintf(os.Stderr, "goaula: ping loop not started — login failed: %v\n", err)
		return exitFail
	}

	if !*watch {
		if err := client.PingAula(); err != nil {
			return pingResult(false, err, *jsonOut)
		}
		return pingResult(true, nil, *jsonOut)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	stop := make(chan struct{})
	go func() {
		<-sig
		close(stop)
	}()

	err := aula.PingLoop(func() error {
		perr := client.PingAula()
		if perr == nil {
			if *jsonOut {
				_ = jsonOutPrint(map[string]any{"authenticated": true, "time": time.Now().UTC().Format(time.RFC3339)})
			} else {
				fmt.Println("ping ok", time.Now().Format(time.Kitchen))
			}
		}
		return perr
	}, *interval, stop)
	if err == nil {
		return exitOK // interrupted
	}
	return pingResult(false, err, *jsonOut)
}

func pingResult(ok bool, err error, jsonOut bool) int {
	if jsonOut {
		out := map[string]any{"authenticated": ok}
		if err != nil {
			out["reason"] = err.Error()
		}
		_ = jsonOutPrint(out)
	} else if !ok {
		fmt.Fprintf(os.Stderr, "goaula: ping failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "       Run 'goaula login' or check ~/.aula/session.txt.")
	}
	if ok {
		return exitOK
	}
	return exitFail
}
