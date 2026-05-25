package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
	"time"

	"github.com/yuu-th/projwm-next/internal/controller"
)

// manifestWatchdog state machine: emit one [MANIFEST] card per drift,
// reset when digest returns to the boot value.
type manifestWatchdogState struct {
	mu       sync.Mutex
	notified bool
}

// runManifestWatchdog re-hashes the manifest on disk every 60s and
// invokes ctrl.EmitManifestMismatchCard when it diverges from the
// digest the daemon booted with. Suppresses repeat cards while the
// drift is unresolved.
//
// Exit when ctx is done.
func runManifestWatchdog(ctx context.Context, ctrl *controller.Controller, manifestPath, expectedDigest string) {
	if manifestPath == "" || expectedDigest == "" {
		return
	}
	state := &manifestWatchdogState{}
	t := time.NewTicker(manifestWatchdogInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			checkManifestDigestOnce(ctrl, manifestPath, expectedDigest, state)
		}
	}
}

// manifestWatchdogInterval is overridable in tests.
var manifestWatchdogInterval = 60 * time.Second

// checkManifestDigestOnce performs one read-and-compare cycle. Exposed
// so tests drive a single iteration without a goroutine.
func checkManifestDigestOnce(ctrl *controller.Controller, manifestPath, expectedDigest string, state *manifestWatchdogState) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// File transiently unavailable — treat as drift only after the
		// next successful read shows a different digest.
		return
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	state.mu.Lock()
	defer state.mu.Unlock()
	if got == expectedDigest {
		// Digest matches; reset so a future drift fires a fresh card.
		state.notified = false
		return
	}
	if state.notified {
		return
	}
	state.notified = true
	ctrl.EmitManifestMismatchCard(expectedDigest, got)
}
