package main

import (
	"context"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestDaemonClient_ReachableRequiresAllFields(t *testing.T) {
	cases := []struct {
		name string
		gf   globalFlags
	}{
		{"empty", globalFlags{}},
		{"only-socket", globalFlags{socketPath: "/tmp/no-such"}},
		{"only-manifest", globalFlags{manifestPath: "/tmp/m"}},
		{"only-digest", globalFlags{manifestDigest: "ab"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newDaemonClient(tc.gf)
			if c.reachable() {
				t.Errorf("expected unreachable with %+v", tc.gf)
			}
		})
	}
}

func TestDaemonClient_SubmitIntentFailsWithoutSocket(t *testing.T) {
	c := newDaemonClient(globalFlags{})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := c.SubmitIntent(ctx, intent.Reconcile{})
	if err == nil {
		t.Fatal("expected error without socket path")
	}
}

func TestDaemonClient_SubmitIntentDialFails(t *testing.T) {
	c := newDaemonClient(globalFlags{
		socketPath:     "/tmp/projwm-nonexistent-" + t.Name(),
		manifestPath:   "/tmp/manifest.json",
		manifestDigest: "deadbeef",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := c.SubmitIntent(ctx, intent.SwitchProfile{To: w.ProfileID("foo")})
	if err == nil {
		t.Fatal("expected error when socket not present")
	}
}
