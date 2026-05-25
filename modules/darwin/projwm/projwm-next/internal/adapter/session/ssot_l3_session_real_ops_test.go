//go:build real_ops

package session

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func requireRealOps(t *testing.T) {
	t.Helper()
	if os.Getenv("PROJWM_REAL_OP_TESTS") != "1" {
		t.Skip("set PROJWM_REAL_OP_TESTS=1 to run real_ops tests")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skipf("tmux not available: %v", err)
	}
}

func TestRealOpsTmuxEnsureSession(t *testing.T) {
	requireRealOps(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := &Client{}
	name := "test-projwm-ensure-session"
	t.Cleanup(func() { _ = client.KillSession(context.Background(), name) })
	_ = client.KillSession(ctx, name)

	created, err := client.EnsureSession(ctx, name, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	if !created {
		t.Fatal("EnsureSession created=false, want true for missing session")
	}
	has, err := client.HasSession(ctx, name)
	if err != nil || !has {
		t.Fatalf("HasSession = %v, %v; want true, nil", has, err)
	}
}

func TestRealOpsTmuxEnsureSessionAlreadyExists(t *testing.T) {
	requireRealOps(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := &Client{}
	name := "test-projwm-existing-session"
	t.Cleanup(func() { _ = client.KillSession(context.Background(), name) })
	_ = client.KillSession(ctx, name)
	if _, err := client.EnsureSession(ctx, name, t.TempDir()); err != nil {
		t.Fatalf("setup EnsureSession: %v", err)
	}

	created, err := client.EnsureSession(ctx, name, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureSession existing: %v", err)
	}
	if created {
		t.Fatal("EnsureSession existing created=true, want false")
	}
}

func TestRealOpsTmuxEnsureGroupedSession(t *testing.T) {
	requireRealOps(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := &Client{}
	base := "test-projwm-group-base"
	clone := "test-projwm-group-clone"
	t.Cleanup(func() {
		_ = client.KillSession(context.Background(), clone)
		_ = client.KillSession(context.Background(), base)
	})
	_ = client.KillSession(ctx, clone)
	_ = client.KillSession(ctx, base)
	if _, err := client.EnsureSession(ctx, base, t.TempDir()); err != nil {
		t.Fatalf("setup base: %v", err)
	}
	if err := client.EnsureGroupedSession(ctx, base, clone); err != nil {
		t.Fatalf("EnsureGroupedSession: %v", err)
	}
	has, err := client.HasSession(ctx, clone)
	if err != nil || !has {
		t.Fatalf("clone HasSession = %v, %v; want true, nil", has, err)
	}
}

func TestRealOpsTmuxKillSession(t *testing.T) {
	requireRealOps(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := &Client{}
	name := "test-projwm-kill-session"
	_ = client.KillSession(ctx, name)
	if _, err := client.EnsureSession(ctx, name, t.TempDir()); err != nil {
		t.Fatalf("setup EnsureSession: %v", err)
	}
	if err := client.KillSession(ctx, name); err != nil {
		t.Fatalf("KillSession: %v", err)
	}
	has, err := client.HasSession(ctx, name)
	if err != nil {
		t.Fatalf("HasSession after kill: %v", err)
	}
	if has {
		t.Fatal("session still exists after KillSession")
	}
}

func TestRealOpsTmuxTestPrefixCleanupListable(t *testing.T) {
	requireRealOps(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := &Client{}
	sessions, err := client.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	for _, name := range sessions {
		if strings.HasPrefix(name, "test-projwm-leftover-") {
			t.Fatalf("real_ops cleanup invariant violated; leftover test session %q", name)
		}
	}
}
