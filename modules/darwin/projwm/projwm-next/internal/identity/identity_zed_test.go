package identity

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// Pin: app-owned title with Expected value must disambiguate
// multi-window single-PID apps (the real Zed case).
//
// Before the fix, identity.Resolve would put BOTH Zed windows
// (title="dotfiles" and title="projwm-jtest") into `strong` because
// the only MatchHint was bundle-id (strong) and the resolver didn't
// inspect TitleContract.Expected when Authority == app-owned. Result:
// len(strong) > 1 → ClassAmbiguous → planner refused all mutation
// (this was the live bug observed on a production daemon).
func TestResolve_AppOwnedTitleDisambiguates(t *testing.T) {
	dotfilesEditor := w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowEditor, Index: 1},
		Kind: w.WindowEditor,
		App:  w.AppRequirement{BundleID: "dev.zed.Zed"},
		TitleContract: w.TitleContract{
			Authority: w.TitleAppOwned,
			Expected:  "dotfiles",
			Drift:     w.TitleDriftRematch,
		},
		MatchHints: []w.MatchHint{
			{Kind: w.MatchByBundleID, Pattern: "dev.zed.Zed", Confidence: w.MatchStrong},
		},
	}
	observed := w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"zed-dotfiles":     {ID: "zed-dotfiles", Kind: w.WindowEditor, App: w.ObservedAppRef{BundleID: "dev.zed.Zed"}, Title: w.ObservedTitle{Value: "dotfiles"}},
			"zed-projwm-jtest": {ID: "zed-projwm-jtest", Kind: w.WindowEditor, App: w.ObservedAppRef{BundleID: "dev.zed.Zed"}, Title: w.ObservedTitle{Value: "projwm-jtest"}},
		},
	}
	res := Resolve(dotfilesEditor, observed)
	if res.Class != ClassUniqueStrong {
		t.Fatalf("class = %s, want UniqueStrong (got candidates %v)", res.Class, res.Candidates)
	}
	if res.Live != "zed-dotfiles" {
		t.Errorf("Live = %s, want zed-dotfiles", res.Live)
	}
}

// Sentinel: app-owned title with mismatched Expected still does not
// pull in unrelated windows (so a window whose title doesn't match
// Expected stays out of `strong`).
func TestResolve_AppOwnedTitleMismatchKeepsOut(t *testing.T) {
	desired := w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowEditor, Index: 1},
		Kind: w.WindowEditor,
		App:  w.AppRequirement{BundleID: "dev.zed.Zed"},
		TitleContract: w.TitleContract{
			Authority: w.TitleAppOwned,
			Expected:  "dotfiles",
			Drift:     w.TitleDriftRematch,
		},
		MatchHints: []w.MatchHint{
			{Kind: w.MatchByBundleID, Pattern: "dev.zed.Zed", Confidence: w.MatchStrong},
		},
	}
	observed := w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"zed-something-else": {ID: "zed-something-else", Kind: w.WindowEditor, App: w.ObservedAppRef{BundleID: "dev.zed.Zed"}, Title: w.ObservedTitle{Value: "other"}},
		},
	}
	res := Resolve(desired, observed)
	if res.Class != ClassMissing {
		t.Errorf("class = %s, want Missing (no title match)", res.Class)
	}
}

// Sentinel: app-owned with EMPTY Expected falls back to old behavior
// (bundle-id strong) — this is the existing test surface guaranteeing
// no regression for projects that never set Expected.
func TestResolve_AppOwnedNoExpected_BundleIDOnly(t *testing.T) {
	desired := w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: "p", Kind: w.WindowEditor, Index: 1},
		Kind: w.WindowEditor,
		App:  w.AppRequirement{BundleID: "dev.zed.Zed"},
		TitleContract: w.TitleContract{
			Authority: w.TitleAppOwned,
			// no Expected
			Drift: w.TitleDriftObserveOnly,
		},
		MatchHints: []w.MatchHint{
			{Kind: w.MatchByBundleID, Pattern: "dev.zed.Zed", Confidence: w.MatchStrong},
		},
	}
	observed := w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"a": {ID: "a", Kind: w.WindowEditor, App: w.ObservedAppRef{BundleID: "dev.zed.Zed"}, Title: w.ObservedTitle{Value: "x"}},
		},
	}
	res := Resolve(desired, observed)
	if res.Class != ClassUniqueStrong {
		t.Errorf("class = %s, want UniqueStrong", res.Class)
	}
}
