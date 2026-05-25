// Package observer holds long-running goroutines that observe the
// outside world and submit internal intents into the controller.
//
// === STATUS: browser_tabs OBSERVER IS DISABLED (2026-05-24) ===
//
// SSOT §4.4 browser BR-TAB-OBS requires the observer to auto-emit
// granular BrowserAddTab / BrowserRemoveTab / BrowserChangeTabURL /
// BrowserReorderTabs intents (SSOT §4.1 OP14-17) by diffing the current
// tab snapshot against the previous one.
//
// As of 2026-05-24 the legacy SyncBrowserTabs catch-all intent has been
// removed (SSOT N-12 / deprecated intent purge) and the granular intents
// have NOT yet been wired into a diff producer. Until slices S14
// (browser tab CRUD) and S20 (observer sidecar) land, this struct only
// tracks the snapshot hash for change detection — pollOnce emits
// nothing. Result: user-driven tab edits (add / remove / reorder /
// URL change) inside the managed Vivaldi profile are NOT observed by
// projwmd, and DesiredBrowserSession remains out of sync with the
// live tab set until the proper observer is rebuilt.
//
// This file is intentionally kept (rather than deleted) so the daemon's
// wiring in cmd/projwmd/browser_tabs.go and the IntentSubmitter contract
// remain stable while S14/S20 reattach the emission body.
package observer

import (
	"context"
	"strings"
	"time"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// IntentSubmitter is the controller-facing interface used by the
// observer to submit internal intents.
type IntentSubmitter interface {
	ApplyIntent(ctx context.Context, in intent.Intent) error
}

// WorldQuerier returns the currently active profile's assignments so we
// can map "Vivaldi automation profile URLs" → which project's
// DesiredBrowserSession should receive them.
type WorldQuerier interface {
	ActiveProject() (w.ProjectID, bool)
}

// PayloadAllocator wraps PrivatePayloadStore so the observer can rotate
// tokens when URLs change.
type PayloadAllocator interface {
	Allocate(project w.ProjectID, urls []string) (w.PrivatePayloadRef, int, int, error)
}

// TabsInspector is the minimal surface BrowserTabsSync needs from the
// Vivaldi adapter. Defining it as an interface (instead of embedding
// *browser.VivaldiAdapter directly) lets tests inject fake URL lists
// without spinning up a real Vivaldi process.
type TabsInspector interface {
	InspectTabs(ctx context.Context) ([]string, error)
}

// BrowserTabsSync polls Vivaldi automation-profile tab URLs and submits
// SyncBrowserTabs intents on change. Cheap O(1) polling at 5s intervals
// after a debounce — design v3 T6 confirmed 188ms/call.
type BrowserTabsSync struct {
	Vivaldi   TabsInspector
	Submitter IntentSubmitter
	World     WorldQuerier
	Allocator PayloadAllocator

	// Interval is the polling period. Defaults to 5s; tests override.
	Interval time.Duration

	lastSnapshot string
}

// Run blocks until ctx is cancelled. Each tick calls InspectTabs, hashes
// the URL list (order-preserving), and submits SyncBrowserTabs only when
// the hash changes.
func (b *BrowserTabsSync) Run(ctx context.Context) {
	if b.Vivaldi == nil || b.Submitter == nil || b.World == nil || b.Allocator == nil {
		return
	}
	interval := b.Interval
	if interval == 0 {
		interval = 5 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.pollOnce(ctx)
		}
	}
}

// pollOnce performs one InspectTabs + snapshot update. Intent emission is
// disabled until S14/S20 rebuild this against the granular SSOT browser
// tab intents.
func (b *BrowserTabsSync) pollOnce(ctx context.Context) {
	urls, err := b.Vivaldi.InspectTabs(ctx)
	if err != nil {
		return
	}
	snap := canonicalize(urls)
	if snap == b.lastSnapshot {
		return
	}
	b.lastSnapshot = snap
	// SSOT §4.4 BR-TAB-OBS: emit BrowserAddTab/RemoveTab/ChangeTabURL/
	// ReorderTabs intents derived from diff(prev, urls). Pending S14/S20.
	_ = intent.Intent(nil)
	_ = w.ProjectID("")
}

// canonicalize joins URLs with NUL into a single string usable as a
// change-detection hash. Order-preserving (Vivaldi tab order is
// load-bearing semantic data).
func canonicalize(urls []string) string {
	cp := append([]string(nil), urls...)
	return strings.Join(cp, "\x00")
}
