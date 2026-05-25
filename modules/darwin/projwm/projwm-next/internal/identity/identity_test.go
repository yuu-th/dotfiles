package identity

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestResolveRequiresBundleAndExactTitleForUniqueStrong(t *testing.T) {
	desired := desiredWindow()
	obs := observed("lw-1", "com.example.term", "ai-1:p1", w.WindowAI)
	res := Resolve(desired, obs)
	if res.Class != ClassUniqueStrong {
		t.Fatalf("class = %s, want unique-strong", res.Class)
	}
	if res.Live != "lw-1" {
		t.Fatalf("live = %s, want lw-1", res.Live)
	}
	if res.Confidence != 1.0 {
		t.Fatalf("confidence = %f, want 1.0", res.Confidence)
	}
	if len(res.ForbiddenEvidenceUsed) != 0 {
		t.Fatalf("forbidden evidence used: %+v", res.ForbiddenEvidenceUsed)
	}
}

func TestResolveRejectsTitleOnlyBundleMismatch(t *testing.T) {
	desired := desiredWindow()
	obs := observed("lw-1", "com.other.term", "ai-1:p1", w.WindowAI)
	res := Resolve(desired, obs)
	if res.Class != ClassMissing {
		t.Fatalf("class = %s, want missing", res.Class)
	}
	if len(res.ForbiddenEvidenceUsed) == 0 {
		t.Fatalf("expected forbidden evidence for bundle mismatch")
	}
}

func TestResolveAmbiguousStrongCandidates(t *testing.T) {
	desired := desiredWindow()
	obs := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}}
	for _, id := range []w.LiveWindowID{"lw-1", "lw-2"} {
		obs.Windows[id] = observedWindow(id, "com.example.term", "ai-1:p1", w.WindowAI)
	}
	res := Resolve(desired, obs)
	if res.Class != ClassAmbiguous {
		t.Fatalf("class = %s, want ambiguous", res.Class)
	}
	if len(res.Candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(res.Candidates))
	}
}

func TestResolveDoesNotUseBundleHintWhenControllerTitleDiffers(t *testing.T) {
	desired := desiredWindow()
	desired.MatchHints = []w.MatchHint{{Kind: w.MatchByBundleID, Pattern: "com.example.term", Confidence: w.MatchStrong}}
	obs := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		"lw-1": observedWindow("lw-1", "com.example.term", "ai-1:p1", w.WindowAI),
		"lw-2": observedWindow("lw-2", "com.example.term", "ai-1:p2", w.WindowAI),
	}}
	res := Resolve(desired, obs)
	if res.Class != ClassUniqueStrong || res.Live != "lw-1" {
		t.Fatalf("resolution = %+v, want unique strong lw-1", res)
	}
}

func TestResolvePrefixOwnedIsWeakWithoutStrongerEvidence(t *testing.T) {
	desired := desiredWindow()
	desired.TitleContract = w.TitleContract{Authority: w.TitlePrefixOwned, Prefix: "cmux:p1:", Drift: w.TitleDriftRematch}
	obs := observed("lw-1", "com.example.term", "cmux:p1:editor", w.WindowAI)
	res := Resolve(desired, obs)
	if res.Class != ClassWeak {
		t.Fatalf("class = %s, want weak", res.Class)
	}
}

func TestResolveWithExpectedWorkspaceRequiresWorkspaceEvidence(t *testing.T) {
	desired := desiredWindow()
	obs := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		"lw-1": observedWindow("lw-1", "com.example.term", "ai-1:p1", w.WindowAI),
	}}
	win := obs.Windows["lw-1"]
	win.Workspace = "Q"
	obs.Windows["lw-1"] = win

	res := ResolveWithOptions(desired, obs, ResolveOptions{ExpectedWorkspace: "Q"})
	if res.Class != ClassUniqueStrong || res.Live != "lw-1" {
		t.Fatalf("resolution = %+v, want unique-strong in expected workspace", res)
	}
	if !hasEvidence(res.Evidence, "workspace") {
		t.Fatalf("resolution missing workspace evidence: %+v", res)
	}
}

func TestResolveWithExpectedWorkspaceRejectsWrongWorkspace(t *testing.T) {
	desired := desiredWindow()
	obs := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		"lw-1": observedWindow("lw-1", "com.example.term", "ai-1:p1", w.WindowAI),
	}}
	win := obs.Windows["lw-1"]
	win.Workspace = "W"
	obs.Windows["lw-1"] = win

	res := ResolveWithOptions(desired, obs, ResolveOptions{ExpectedWorkspace: "Q"})
	if res.Class != ClassMissing {
		t.Fatalf("class = %s, want missing for wrong workspace: %+v", res.Class, res)
	}
	if !hasEvidence(res.ForbiddenEvidenceUsed, "workspace-mismatch") {
		t.Fatalf("expected workspace-mismatch forbidden evidence: %+v", res)
	}
}

func TestResolveMatchedToWithTitleDriftIsStale(t *testing.T) {
	desired := desiredWindow()
	dwid := desired.ID
	obs := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		"lw-1": observedWindow("lw-1", "com.example.term", "drifted-title", w.WindowAI),
	}}
	win := obs.Windows["lw-1"]
	win.MatchedTo = &dwid
	obs.Windows["lw-1"] = win

	res := Resolve(desired, obs)
	if res.Class != ClassStale {
		t.Fatalf("class = %s, want stale for matched-to title drift: %+v", res.Class, res)
	}
}

// Bug 2026-05-19: orphan detector saw every managed-kind live window
// as un-MatchedTo and produced spurious "manual ai window" cards.
// PopulateMatchedTo stamps strong matches onto a freshly observed
// world so downstream consumers (reducer.ReactToEvent §3.5) see a
// resolver-truthful hint.
func TestPopulateMatchedTo_StampsStrongMatchOnUnstampedWindow(t *testing.T) {
	dw := desiredWindow()
	obs := observed("lw-1", "com.example.term", "ai-1:p1", w.WindowAI)
	desiredWorld := w.DesiredWorld{
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{dw}},
		},
	}
	out := PopulateMatchedTo(desiredWorld, obs)
	got := out.Windows["lw-1"].MatchedTo
	if got == nil {
		t.Fatalf("MatchedTo not stamped on strong match")
	}
	if *got != dw.ID {
		t.Errorf("MatchedTo = %+v, want %+v", *got, dw.ID)
	}
}

// PopulateMatchedTo must never overwrite an already-set MatchedTo —
// simulator/fake adapters seed authoritative hints that we should
// preserve.
func TestPopulateMatchedTo_PreservesExistingHint(t *testing.T) {
	dw := desiredWindow()
	otherID := w.DesiredWindowID{Project: "p2", Kind: w.WindowAI, Index: 7}
	obs := observed("lw-1", "com.example.term", "ai-1:p1", w.WindowAI)
	// Pre-stamp with a different ID — Resolve would say dw matches,
	// but the existing hint must survive untouched.
	ow := obs.Windows["lw-1"]
	pre := otherID
	ow.MatchedTo = &pre
	obs.Windows["lw-1"] = ow

	desiredWorld := w.DesiredWorld{
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{dw}},
		},
	}
	out := PopulateMatchedTo(desiredWorld, obs)
	got := out.Windows["lw-1"].MatchedTo
	if got == nil || *got != otherID {
		t.Errorf("MatchedTo = %+v, want preserved %+v", got, otherID)
	}
}

// Weak-class resolution where the resolver narrows to exactly one
// candidate (e.g. prefix-owned AI window whose live title has a
// transient shell appendix) IS stamped — the orphan detector sees a
// truthful hint instead of treating the freshly-spawned project AI
// window as a "manual ai window" card. Mutation-side code (planner /
// semop / executor) still gates on ClassUniqueStrong via its own
// fresh Resolve, so a weak stamp here can't drive incorrect spawns.
func TestPopulateMatchedTo_StampsUniqueWeakResolution(t *testing.T) {
	dw := w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1},
		Kind: w.WindowAI,
		App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
		TitleContract: w.TitleContract{
			Authority: w.TitlePrefixOwned,
			Prefix:    "ai-1:p1",
			Drift:     w.TitleDriftRematch,
		},
	}
	obs := observed("lw-1", "com.mitchellh.ghostty", "ai-1:p1 ~/p1", w.WindowAI)
	desiredWorld := w.DesiredWorld{
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {ID: "p1", Windows: []w.DesiredWindow{dw}},
		},
	}
	out := PopulateMatchedTo(desiredWorld, obs)
	got := out.Windows["lw-1"].MatchedTo
	if got == nil {
		t.Fatalf("MatchedTo not stamped on unique weak resolution")
	}
	if *got != dw.ID {
		t.Errorf("MatchedTo = %+v, want %+v", *got, dw.ID)
	}
}

// Ambiguous weak-prefix match: the same live window resolves weakly
// to multiple desired windows. PopulateMatchedTo MUST refuse to
// stamp — picking the first sorted project arbitrarily would lie to
// invariant checks.
func TestPopulateMatchedTo_LeavesAmbiguousWeakPrefixUnstamped(t *testing.T) {
	mkAI := func(pid w.ProjectID) w.DesiredWindow {
		return w.DesiredWindow{
			ID:   w.DesiredWindowID{Project: pid, Kind: w.WindowAI, Index: 1},
			Kind: w.WindowAI,
			App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
			TitleContract: w.TitleContract{
				Authority: w.TitlePrefixOwned,
				Prefix:    "shared-prefix",
				Drift:     w.TitleDriftRematch,
			},
		}
	}
	obs := observed("lw-1", "com.mitchellh.ghostty", "shared-prefix live-title", w.WindowAI)
	desiredWorld := w.DesiredWorld{
		Projects: map[w.ProjectID]w.DesiredProject{
			"a":        {ID: "a", Archived: true, Windows: []w.DesiredWindow{mkAI("a")}},
			"dotfiles": {ID: "dotfiles", Windows: []w.DesiredWindow{mkAI("dotfiles")}},
		},
	}
	out := PopulateMatchedTo(desiredWorld, obs)
	if out.Windows["lw-1"].MatchedTo != nil {
		t.Errorf("ambiguous match stamped: %+v", *out.Windows["lw-1"].MatchedTo)
	}
}

func desiredWindow() w.DesiredWindow {
	return w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: "p1", Kind: w.WindowAI, Index: 1},
		Kind: w.WindowAI,
		App:  w.AppRequirement{Capability: w.CapabilityTerminal, BundleID: "com.example.term"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "ai-1:p1",
			Drift:     w.TitleDriftRepair,
		},
	}
}

func observed(id w.LiveWindowID, bundleID, title string, kind w.WindowKind) w.ObservedWorld {
	return w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		id: observedWindow(id, bundleID, title, kind),
	}}
}

func observedWindow(id w.LiveWindowID, bundleID, title string, kind w.WindowKind) w.ObservedWindow {
	return w.ObservedWindow{
		ID:    id,
		Kind:  kind,
		App:   w.ObservedAppRef{BundleID: bundleID},
		Title: w.ObservedTitle{Value: title},
	}
}

func hasEvidence(evidence []MatchEvidence, kind string) bool {
	for _, ev := range evidence {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}
