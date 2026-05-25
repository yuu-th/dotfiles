package reducer

import (
	"fmt"
	"testing"

	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Unified design v1 — Phase B reducer unit tests.

// SyncCockpitSystemWindows always creates exactly ONE cockpit SystemWindow
// for the projwm-managed monitor (requirements v2.4 §8.1), regardless of
// DisplayCount. The single entry has DisplayIdx=0, Title="projwm-cockpit-0",
// ParkWorkspace="CP1", Visibility=hidden (§8.2 "平時は隠れている").
func TestReduceIntent_SyncCockpit_FreshBuild(t *testing.T) {
	s := w.WorldState{}
	// Pass DisplayCount=3 to prove it is ignored — output must still be length 1.
	d, err := ReduceIntent(s, intent.SyncCockpitSystemWindows{DisplayCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(d.SystemWindows); got != 1 {
		t.Fatalf("SystemWindows len = %d, want 1 (requirements v2.4 §8.1: single cockpit)", got)
	}
	sw := d.SystemWindows[0]
	if sw.Kind != w.WindowCockpit {
		t.Errorf("kind = %s, want cockpit", sw.Kind)
	}
	if sw.DisplayIdx != 0 {
		t.Errorf("DisplayIdx = %d, want 0", sw.DisplayIdx)
	}
	if sw.Title != "projwm-cockpit-0" {
		t.Errorf("Title = %q, want %q", sw.Title, "projwm-cockpit-0")
	}
	if sw.ParkWorkspace != "CP1" {
		t.Errorf("ParkWorkspace = %q, want CP1", sw.ParkWorkspace)
	}
	if sw.Visibility != w.CockpitHidden {
		t.Errorf("Visibility = %s, want hidden", sw.Visibility)
	}
}

// SyncCockpitSystemWindows preserves the D0 entry's Visibility when it
// already exists (display topology changes do not reset cockpit state).
func TestReduceIntent_SyncCockpit_PreservesVisibility(t *testing.T) {
	s := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{
				// Only D0 exists — matches requirements v2.4 §8.1 (single cockpit).
				// Extra entries (D1, D2) were from old multi-cockpit design; they
				// are not present in the new desired state.
				{ID: w.SystemWindowID{Kind: w.WindowCockpit, Index: 0}, Kind: w.WindowCockpit, DisplayIdx: 0, Title: "projwm-cockpit-0", ParkWorkspace: "CP1", Visibility: w.CockpitShown},
			},
		},
	}
	// DisplayCount is ignored in v2.4 — always produces 1 entry.
	d, err := ReduceIntent(s, intent.SyncCockpitSystemWindows{DisplayCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(d.SystemWindows); got != 1 {
		t.Fatalf("len = %d, want 1 (requirements v2.4 §8.1)", got)
	}
	if d.SystemWindows[0].Visibility != w.CockpitShown {
		t.Errorf("visibility lost: got %s, want shown", d.SystemWindows[0].Visibility)
	}
}

// With requirements v2.4 §8.1, monitor plug events (DisplayCount>1) do not
// create additional cockpit entries. The existing D0 entry's Visibility is
// preserved regardless of DisplayCount.
func TestReduceIntent_SyncCockpit_MonitorPlugDoesNotAddCockpit(t *testing.T) {
	s := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{
				{ID: w.SystemWindowID{Kind: w.WindowCockpit, Index: 0}, Kind: w.WindowCockpit, DisplayIdx: 0, Title: "projwm-cockpit-0", ParkWorkspace: "CP1", Visibility: w.CockpitShown},
			},
		},
	}
	// Simulate monitor plug: DisplayCount increases to 3.
	d, err := ReduceIntent(s, intent.SyncCockpitSystemWindows{DisplayCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	// Must still be exactly 1 — other monitors get no cockpit (§8.8: monitor 接続 → 何もしない).
	if got := len(d.SystemWindows); got != 1 {
		t.Fatalf("len = %d, want 1 — new monitor must not add cockpit (requirements v2.4 §8.8)", got)
	}
	if d.SystemWindows[0].Visibility != w.CockpitShown {
		t.Errorf("visibility changed unexpectedly: got %s, want shown", d.SystemWindows[0].Visibility)
	}
}

// SetCockpitVisibility flips the single cockpit entry's Visibility.
// (Requirements v2.4 §8.1: exactly 1 cockpit; SetCockpitVisibility still
// iterates uniformly over all SystemWindows, which is correct for 1 or N.)
func TestReduceIntent_SetCockpitVisibility_Uniform(t *testing.T) {
	s := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{
				{Kind: w.WindowCockpit, DisplayIdx: 0, ParkWorkspace: "CP1", Visibility: w.CockpitHidden},
			},
		},
	}
	d, err := ReduceIntent(s, intent.SetCockpitVisibility{Visibility: w.CockpitShown})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.SystemWindows) != 1 {
		t.Fatalf("len = %d, want 1", len(d.SystemWindows))
	}
	if d.SystemWindows[0].Visibility != w.CockpitShown {
		t.Errorf("visibility = %s, want shown", d.SystemWindows[0].Visibility)
	}
}

// SSOT N-06 (2026-05-20): ToggleCockpit intent is deprecated; cockpit
// is summon-only via SetCockpitVisibility{Shown}. Former tests for
// ToggleCockpit summon-semantics deleted; equivalent behavior is now
// covered by TestReduceIntent_SetCockpitVisibility_Shown_PopulatesPrior
// and TestReduceIntent_SetCockpitVisibility_Hidden_PreservesPrior below.

// ReactToEvent (Bootstrap): emits cockpit-sync DirtyScope with display
// count encoded in Key, so the controller's applyCockpitSync can submit
// the internal intent with the right count.
func TestReactToEvent_Bootstrap_EmitsCockpitSync(t *testing.T) {
	state := w.WorldState{
		Observed: w.ObservedWorld{
			Displays: w.ObservedDisplayState{
				Displays: map[w.DisplayID]w.ObservedDisplay{
					"d:0": {ID: "d:0", Connected: true},
					"d:1": {ID: "d:1", Connected: true},
				},
			},
		},
	}
	r, err := ReactToEvent(state, event.Event{Kind: event.KindStartup})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ds := range r.DirtyScopes {
		if ds.Kind == "cockpit-sync" && ds.Key == "2" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cockpit-sync DirtyScope with count=2, got %+v", r.DirtyScopes)
	}
}

// ReactToEvent (DisplayChanged): also emits cockpit-sync.
func TestReactToEvent_DisplayChanged_EmitsCockpitSync(t *testing.T) {
	state := w.WorldState{
		Observed: w.ObservedWorld{
			Displays: w.ObservedDisplayState{
				Displays: map[w.DisplayID]w.ObservedDisplay{
					"d:0": {ID: "d:0", Connected: true},
				},
			},
		},
	}
	r, err := ReactToEvent(state, event.Event{Kind: event.KindDisplayChanged})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ds := range r.DirtyScopes {
		if ds.Kind == "cockpit-sync" && ds.Key == "1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cockpit-sync DirtyScope with count=1, got %+v", r.DirtyScopes)
	}
}

// SyncCockpitSystemWindows sets ParkWorkspace to "CP1" for the single entry
// (requirements v2.4 §8.1 / §8.3: cockpit on projwm-managed monitor only).
func TestReduceIntent_SyncCockpit_SetsParkWorkspace(t *testing.T) {
	s := w.WorldState{}
	d, err := ReduceIntent(s, intent.SyncCockpitSystemWindows{DisplayCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.SystemWindows) != 1 {
		t.Fatalf("len = %d, want 1", len(d.SystemWindows))
	}
	if d.SystemWindows[0].ParkWorkspace != "CP1" {
		t.Errorf("ParkWorkspace = %q, want CP1", d.SystemWindows[0].ParkWorkspace)
	}
}

// SyncCockpitSystemWindows preserves ParkWorkspace on survivors.
func TestReduceIntent_SyncCockpit_PreservesParkWorkspace(t *testing.T) {
	s := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{
				{ID: w.SystemWindowID{Kind: w.WindowCockpit, Index: 0}, Kind: w.WindowCockpit, DisplayIdx: 0, Title: "projwm-cockpit-0", ParkWorkspace: "CP1", Visibility: w.CockpitHidden},
			},
		},
	}
	d, err := ReduceIntent(s, intent.SyncCockpitSystemWindows{DisplayCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if d.SystemWindows[0].ParkWorkspace != "CP1" {
		t.Errorf("ParkWorkspace not preserved: got %q", d.SystemWindows[0].ParkWorkspace)
	}
}

// SetCockpitVisibility (Shown) populates PriorWorkspace from observed display.
func TestReduceIntent_SetCockpitVisibility_Shown_PopulatesPrior(t *testing.T) {
	primary := w.DisplayID("d:0")
	s := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{
				{Kind: w.WindowCockpit, DisplayIdx: 0, ParkWorkspace: "CP1", Visibility: w.CockpitHidden},
			},
		},
		Observed: w.ObservedWorld{
			Displays: w.ObservedDisplayState{
				Primary: &primary,
				Displays: map[w.DisplayID]w.ObservedDisplay{
					"d:0": {ID: "d:0", Connected: true, ActiveWorkspace: "WS1"},
				},
			},
			Focus: w.ObservedFocus{Workspace: "WS1", Window: "shell-live"},
		},
	}
	d, err := ReduceIntent(s, intent.SetCockpitVisibility{Visibility: w.CockpitShown})
	if err != nil {
		t.Fatal(err)
	}
	if d.SystemWindows[0].PriorWorkspace != "WS1" {
		t.Errorf("PriorWorkspace = %q, want WS1", d.SystemWindows[0].PriorWorkspace)
	}
	if d.SystemWindows[0].PriorWindow != "shell-live" {
		t.Errorf("PriorWindow = %q, want shell-live", d.SystemWindows[0].PriorWindow)
	}
}

// SetCockpitVisibility (Hidden) preserves PriorWorkspace when there's no
// non-park observed workspace to refresh from. Preservation is required so
// the planner's HideCockpit op still has a target to return to (req §8.2:
// after spawn switches the display to CPn, hide must switch it back).
func TestReduceIntent_SetCockpitVisibility_Hidden_PreservesPrior(t *testing.T) {
	s := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{
				{Kind: w.WindowCockpit, DisplayIdx: 0, ParkWorkspace: "CP1", Visibility: w.CockpitShown, PriorWorkspace: "WS1", PriorWindow: "shell-live"},
			},
		},
	}
	d, err := ReduceIntent(s, intent.SetCockpitVisibility{Visibility: w.CockpitHidden})
	if err != nil {
		t.Fatal(err)
	}
	if d.SystemWindows[0].Visibility != w.CockpitHidden {
		t.Errorf("Visibility = %s, want hidden", d.SystemWindows[0].Visibility)
	}
	if d.SystemWindows[0].PriorWorkspace != "WS1" {
		t.Errorf("PriorWorkspace = %q, want preserved WS1", d.SystemWindows[0].PriorWorkspace)
	}
	if d.SystemWindows[0].PriorWindow != "shell-live" {
		t.Errorf("PriorWindow = %q, want preserved shell-live", d.SystemWindows[0].PriorWindow)
	}
}

// SSOT N-06: ToggleCockpit PriorWindow population tests removed. The
// equivalent SetCockpitVisibility{Shown} coverage is upstream in
// TestReduceIntent_SetCockpitVisibility_Shown_PopulatesPrior.

// TestReduceIntent_SyncCockpit_AlwaysOneEntry_RegardlessOfDisplayCount verifies
// the core requirement v2.4 §8.1: SyncCockpitSystemWindows produces exactly one
// SystemWindow regardless of how many physical displays are connected (DisplayCount>=1).
//
// DisplayCount=0 is a special case: no physical display is available, so no cockpit
// can be placed. The reducer produces an empty slice for this edge case — this is
// intentional and matches the "cockpit is on the projwm-managed monitor" requirement
// (there is no monitor to put it on). DisplayCount=0 is separately tested in
// TestReduceIntent_SyncCockpit_FreshBuild (which passes DisplayCount=3 to verify
// the count is ignored, always producing 1) and the edge-case check below.
func TestReduceIntent_SyncCockpit_AlwaysOneEntry_RegardlessOfDisplayCount(t *testing.T) {
	// DisplayCount=0: special edge case — no display available, empty slice expected.
	// This is correct per the implementation comment in reducer.go:
	// "DisplayCount==0 means no physical display is available — produce an empty slice".
	// Requirements §8.1 says cockpit is on the projwm-managed monitor; if there is no
	// monitor, there is no cockpit.
	t.Run("DisplayCount=0_produces_empty", func(t *testing.T) {
		s := w.WorldState{}
		d, err := ReduceIntent(s, intent.SyncCockpitSystemWindows{DisplayCount: 0})
		if err != nil {
			t.Fatal(err)
		}
		if got := len(d.SystemWindows); got != 0 {
			t.Errorf("DisplayCount=0: SystemWindows len = %d, want 0 (no display, no cockpit)", got)
		}
	})

	// DisplayCount>=1: always produces exactly 1 cockpit SystemWindow on D0/CP1,
	// regardless of the specific count. This is the core v2.4 §8.1 requirement:
	// "projwm-managed モニタに 1 つだけ" (one cockpit on the projwm-managed monitor).
	for _, count := range []int{1, 2, 3, 6, 10} {
		count := count // capture range variable
		t.Run(fmt.Sprintf("DisplayCount=%d_produces_1", count), func(t *testing.T) {
			s := w.WorldState{}
			d, err := ReduceIntent(s, intent.SyncCockpitSystemWindows{DisplayCount: count})
			if err != nil {
				t.Fatalf("DisplayCount=%d: unexpected error: %v", count, err)
			}
			if got := len(d.SystemWindows); got != 1 {
				t.Errorf("DisplayCount=%d: SystemWindows len = %d, want 1 (requirements v2.4 §8.1: single cockpit on projwm-managed monitor)", count, got)
				return
			}
			sw := d.SystemWindows[0]
			if sw.DisplayIdx != 0 {
				t.Errorf("DisplayCount=%d: DisplayIdx = %d, want 0", count, sw.DisplayIdx)
			}
			if sw.Title != "projwm-cockpit-0" {
				t.Errorf("DisplayCount=%d: Title = %q, want projwm-cockpit-0", count, sw.Title)
			}
			if sw.ParkWorkspace != "CP1" {
				t.Errorf("DisplayCount=%d: ParkWorkspace = %q, want CP1", count, sw.ParkWorkspace)
			}
			if sw.Visibility != w.CockpitHidden {
				t.Errorf("DisplayCount=%d: Visibility = %s, want hidden (§8.2 平時は隠れている)", count, sw.Visibility)
			}
		})
	}
}
