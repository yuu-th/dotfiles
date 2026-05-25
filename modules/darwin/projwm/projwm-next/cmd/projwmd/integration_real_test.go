//go:build integration

// integration_real_test.go — env-gated smoke test for the real omniwmctl
// backend. Read-only: queries `omniwmctl query workspaces --json` and asserts
// at least one workspace is returned. Does NOT spawn / move / focus anything.
//
// Skipped unless PROJWM_NEXT_REAL_GUI=1 is set, so CI runs without a GUI host.
//
// Run: PROJWM_NEXT_REAL_GUI=1 go test -tags integration ./cmd/projwmd
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestRealOmniwmctlReadOnlySmoke(t *testing.T) {
	if os.Getenv("PROJWM_NEXT_REAL_GUI") != "1" {
		t.Skip("PROJWM_NEXT_REAL_GUI != 1; skipping real-GUI smoke test")
	}
	if _, err := exec.LookPath("omniwmctl"); err != nil {
		t.Skipf("omniwmctl not on PATH: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "omniwmctl", "query", "workspaces", "--format", "json").Output()
	if err != nil {
		t.Fatalf("omniwmctl query workspaces failed: %v", err)
	}
	var resp struct {
		OK     bool `json:"ok"`
		Result struct {
			Payload struct {
				Workspaces []struct {
					ID      string `json:"id"`
					RawName string `json:"rawName"`
					Number  int    `json:"number"`
				} `json:"workspaces"`
			} `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("decode: %v (raw=%s)", err, string(out))
	}
	if !resp.OK {
		t.Fatalf("omniwmctl returned ok=false; raw=%s", string(out))
	}
	if len(resp.Result.Payload.Workspaces) == 0 {
		t.Fatalf("expected >= 1 workspace; raw=%s", string(out))
	}
	t.Logf("omniwm workspaces: %d", len(resp.Result.Payload.Workspaces))
}
