package main

import (
	"testing"
	"time"
)

func TestPingInterval(t *testing.T) {
	cases := []struct {
		env    string
		want   time.Duration
		wantOK bool
	}{
		{env: "", want: 15 * time.Minute, wantOK: true},        // unset → default
		{env: "15m", want: 15 * time.Minute, wantOK: true},     // duration
		{env: "1h", want: time.Hour, wantOK: true},             // duration
		{env: "45", want: 45 * time.Minute, wantOK: true},      // minutes
		{env: "0", wantOK: false},                              // explicit off
		{env: "off", wantOK: false},                            // explicit off
		{env: "OFF", wantOK: false},                            // explicit off
		{env: "garbage", want: 15 * time.Minute, wantOK: true}, // invalid → default
	}
	for _, c := range cases {
		t.Run("env="+c.env, func(t *testing.T) {
			if c.env != "" {
				t.Setenv("GOAULA_MCP_PING_INTERVAL", c.env)
			} else {
				t.Setenv("GOAULA_MCP_PING_INTERVAL", "")
			}
			got, ok := pingInterval()
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if got != c.want {
				t.Fatalf("interval = %v, want %v", got, c.want)
			}
		})
	}
}
