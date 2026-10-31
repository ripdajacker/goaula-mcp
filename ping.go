package aula

import (
	"fmt"
	"time"
)

// MaxPingFailures is the number of consecutive failed pings before PingLoop
// reports the session as expired.
const MaxPingFailures = 3

// PingLoop calls ping on a ticker until stop is closed or the session expires
// (MaxPingFailures consecutive failures). It runs synchronously on the calling
// goroutine and returns nil when stop is closed, or an error when the session
// has expired. The goaula-mcp and goaula-login commands both use it to keep a
// session token alive.
func PingLoop(ping func() error, interval time.Duration, stop <-chan struct{}) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-stop:
			return nil
		case <-ticker.C:
		}

		if err := ping(); err != nil {
			failures++
			if failures >= MaxPingFailures {
				return fmt.Errorf("session expired after %d consecutive ping failures: %w", failures, err)
			}
			continue
		}
		failures = 0
	}
}
