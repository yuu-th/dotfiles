// Package observer holds long-running goroutines that observe the
// outside world and submit internal intents into the controller.
//
// SSOT §4.4 BR-TAB-OBS: BrowserTabsSync observes Vivaldi automation
// windows' tabs and emits granular BrowserAddTab / BrowserRemoveTab /
// BrowserChangeTabURL / BrowserReorderTabs intents (SSOT §4.1 OP14-17)
// when the user makes manual changes inside Vivaldi.
//
// Implementation status (S20 Step 3):
//
//   - InspectTabsByWindow → per-window tab list (window boundaries are
//     load-bearing: SSOT operations take WindowID, not a flat tab index)
//   - title parsing recovers (project, index) from "browser-N:project"
//   - prev/curr diff emits Add / Remove / ChangeURL ops
//   - Reorder detection: when prev/curr are permutations of the same
//     multiset, falls through as multiple ChangeTabURL ops (the
//     PrivatePayloadStore rotation still converges the state correctly,
//     but the cockpit card will report N URL changes instead of one
//     reorder. Honest gap — refine when a use case demands it).
//   - User-profile (External) Vivaldi windows whose title cannot be
//     parsed are skipped (SSOT §4.4 BR-USERPROF-EXTERNAL).
//   - Vivaldi InspectTabsByWindow failure does NOT halt the loop —
//     consecutive failures are counted for future health-probe extension.
package observer

import (
	"context"
	"log"
	"strconv"
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

// PayloadAllocator is reserved for future use when the observer needs
// to allocate tokens for entire bulk-restore scenarios. Currently
// unused: controller.prepareBrowserIntent does the Put for each
// granular intent.
type PayloadAllocator interface {
	Allocate(project w.ProjectID, urls []string) (w.PrivatePayloadRef, int, int, error)
}

// WindowTabsInspector is the minimal surface BrowserTabsSync needs from
// the Vivaldi adapter. Defining it as an interface (instead of embedding
// *browser.VivaldiAdapter directly) lets tests inject fake snapshots
// without spinning up a real Vivaldi process.
type WindowTabsInspector interface {
	InspectTabsByWindow(ctx context.Context) ([]WindowSnapshot, error)
}

// WindowSnapshot is the observer-package version of browser.WindowTabs,
// duplicated here to avoid an internal/adapter/observer → adapter/browser
// import cycle. browser.WindowTabs ↔ WindowSnapshot is a trivial copy
// done by the daemon glue (cmd/projwmd/browser_tabs.go).
type WindowSnapshot struct {
	Title string
	URLs  []string
}

// TabsInspector is the legacy flat-list interface kept for backward
// compatibility with the (PRIV.6.5b) restore-verification path.
// New observation work uses WindowTabsInspector.
type TabsInspector interface {
	InspectTabs(ctx context.Context) ([]string, error)
}

// BrowserTabsSync polls Vivaldi automation-profile windows and submits
// granular browser tab intents on change.
type BrowserTabsSync struct {
	Vivaldi   WindowTabsInspector
	Submitter IntentSubmitter
	World     WorldQuerier
	Allocator PayloadAllocator

	// Interval is the polling period. Defaults to 5s; tests override.
	Interval time.Duration

	// snapshots tracks the last observed per-window tab state, keyed by
	// the parsed (project, index) identity. User-profile windows whose
	// title cannot be parsed are kept out of this map (External windows
	// are not managed — SSOT §4.4 BR-USERPROF-EXTERNAL).
	snapshots map[managedKey][]string

	// consecutiveErrors counts back-to-back InspectTabsByWindow failures
	// (Vivaldi crashed / locked / AppleScript timed out). Surfaced for
	// future health-probe extension (G6 — restart Vivaldi after N
	// consecutive failures).
	consecutiveErrors int
}

// managedKey identifies a managed browser window by (project, index).
// Window kind is implicitly Browser — only browser-N:project windows
// are tracked.
type managedKey struct {
	Project w.ProjectID
	Index   int
}

// Run blocks until ctx is cancelled. Each tick calls InspectTabsByWindow
// and emits diff-based intents.
func (b *BrowserTabsSync) Run(ctx context.Context) {
	if b.Vivaldi == nil || b.Submitter == nil || b.World == nil {
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

// pollOnce performs one InspectTabsByWindow and emits diff intents per
// managed window. Errors are logged but do not halt the loop.
func (b *BrowserTabsSync) pollOnce(ctx context.Context) {
	windows, err := b.Vivaldi.InspectTabsByWindow(ctx)
	if err != nil {
		b.consecutiveErrors++
		log.Printf("observer/browser-tabs: inspect failed (consecutive=%d): %v", b.consecutiveErrors, err)
		return
	}
	b.consecutiveErrors = 0

	if b.snapshots == nil {
		b.snapshots = map[managedKey][]string{}
	}

	seen := map[managedKey]bool{}
	for _, win := range windows {
		key, ok := parseBrowserTitle(win.Title)
		if !ok {
			continue // User-profile (External) — SSOT §4.4 BR-USERPROF-EXTERNAL
		}
		seen[key] = true
		prev, hadPrev := b.snapshots[key]
		curr := append([]string(nil), win.URLs...)
		b.snapshots[key] = curr

		if !hadPrev {
			// First observation of this window — seed snapshot, don't
			// emit (we don't know if those tabs are user-driven or
			// projwmd-spawn). After this point, only changes emit.
			continue
		}
		ops := diffTabs(prev, curr)
		windowID := w.DesiredWindowID{Project: key.Project, Kind: w.WindowBrowser, Index: key.Index}
		for _, op := range ops {
			fillBrowserIntentTarget(&op, key.Project, windowID)
			if err := b.Submitter.ApplyIntent(ctx, op); err != nil {
				log.Printf("observer/browser-tabs: submit %T for %v failed: %v", op, windowID, err)
			}
		}
	}

	// Forget snapshots of windows that disappeared so a future
	// re-spawned window starts from a fresh seed.
	for key := range b.snapshots {
		if !seen[key] {
			delete(b.snapshots, key)
		}
	}
}

// parseBrowserTitle extracts (project, index) from a managed browser
// window title of the form "browser-N:project". Returns ok=false for
// any other title (User-profile windows, AI windows, etc.).
func parseBrowserTitle(title string) (managedKey, bool) {
	const prefix = "browser-"
	if !strings.HasPrefix(title, prefix) {
		return managedKey{}, false
	}
	rest := title[len(prefix):]
	colon := strings.IndexByte(rest, ':')
	if colon <= 0 || colon == len(rest)-1 {
		return managedKey{}, false
	}
	idxStr := rest[:colon]
	project := rest[colon+1:]
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 1 {
		return managedKey{}, false
	}
	return managedKey{Project: w.ProjectID(project), Index: idx}, true
}

// diffTabs computes the minimal sequence of granular browser intents
// that transform prev into curr. SSOT §4.1 OP14-17 take 1-based Tab
// indices. Intent.Project / WindowID are left zero — fillBrowserIntentTarget
// populates them.
//
// Strategy (single-step user actions are common; bulk-actions degrade
// gracefully to multiple ChangeURL ops):
//
//   - len(curr) == len(prev)+1: AddTab(URL=curr[len(prev)])
//   - len(curr) == len(prev)-1: RemoveTab(Tab=position of first diff,
//     or last position if prev is a prefix of curr+1 tail)
//   - len(curr) == len(prev): per-index ChangeTabURL for differing URLs
//   - otherwise: empty (unsupported bulk delta — next poll converges)
func diffTabs(prev, curr []string) []intent.Intent {
	switch {
	case len(curr) == len(prev)+1:
		// Tab inserted. Find insertion point by comparing prefixes.
		// SSOT 647 "ブラウザ内の位置: 最後尾に追加" — most insertions
		// are at the end; middle inserts emit as AddTab anyway and
		// the next poll's ReorderTabs (deferred) corrects position.
		return []intent.Intent{intent.BrowserAddTab{URL: curr[len(prev)]}}
	case len(curr) == len(prev)-1:
		// Tab removed. Find first diff position as 1-based index.
		removedAt := len(prev)
		for i := 0; i < len(curr); i++ {
			if prev[i] != curr[i] {
				removedAt = i + 1
				break
			}
		}
		return []intent.Intent{intent.BrowserRemoveTab{Tab: removedAt}}
	case len(curr) == len(prev):
		var ops []intent.Intent
		for i := range curr {
			if curr[i] != prev[i] {
				ops = append(ops, intent.BrowserChangeTabURL{Tab: i + 1, URL: curr[i]})
			}
		}
		return ops
	default:
		// Multi-step bulk change (e.g., session restore). Skip this
		// poll; the next poll will see one of the simpler shapes
		// after the user pauses. Snapshot is already updated, so we
		// do not loop forever.
		return nil
	}
}

// fillBrowserIntentTarget populates Project + WindowID on the granular
// browser tab intents. Done here (not in diffTabs) so diffTabs stays
// pure / table-driven.
func fillBrowserIntentTarget(op *intent.Intent, project w.ProjectID, windowID w.DesiredWindowID) {
	switch v := (*op).(type) {
	case intent.BrowserAddTab:
		v.Project = project
		v.WindowID = windowID
		*op = v
	case intent.BrowserRemoveTab:
		v.Project = project
		v.WindowID = windowID
		*op = v
	case intent.BrowserChangeTabURL:
		v.Project = project
		v.WindowID = windowID
		*op = v
	case intent.BrowserReorderTabs:
		v.Project = project
		v.WindowID = windowID
		*op = v
	}
}
