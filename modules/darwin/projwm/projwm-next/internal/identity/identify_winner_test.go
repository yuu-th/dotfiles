package identity

import (
	"reflect"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §2.5 EC4 / §3.4 INV-01: IdentifyWinnerAndOrphans tiebreak policy.

func TestIdentifyWinnerAndOrphans_PicksFocusedCandidate(t *testing.T) {
	candidates := []w.LiveWindowID{"a", "b", "c"}
	winner, orphans := IdentifyWinnerAndOrphans(candidates, "b")
	if winner != "b" {
		t.Errorf("winner = %q, want b (focused)", winner)
	}
	if !reflect.DeepEqual(orphans, []w.LiveWindowID{"a", "c"}) {
		t.Errorf("orphans = %v, want [a c]", orphans)
	}
}

func TestIdentifyWinnerAndOrphans_FallsBackToSmallestWhenFocusedNotInCandidates(t *testing.T) {
	candidates := []w.LiveWindowID{"b", "c", "d"}
	winner, orphans := IdentifyWinnerAndOrphans(candidates, "elsewhere")
	if winner != "b" {
		t.Errorf("winner = %q, want b (smallest as fallback)", winner)
	}
	if !reflect.DeepEqual(orphans, []w.LiveWindowID{"c", "d"}) {
		t.Errorf("orphans = %v, want [c d]", orphans)
	}
}

func TestIdentifyWinnerAndOrphans_EmptyCandidates(t *testing.T) {
	winner, orphans := IdentifyWinnerAndOrphans(nil, "any")
	if winner != "" || len(orphans) != 0 {
		t.Errorf("empty input gave winner=%q orphans=%v", winner, orphans)
	}
}

func TestIdentifyWinnerAndOrphans_SingleCandidate(t *testing.T) {
	winner, orphans := IdentifyWinnerAndOrphans([]w.LiveWindowID{"only"}, "different")
	if winner != "only" || len(orphans) != 0 {
		t.Errorf("single candidate: winner=%q orphans=%v", winner, orphans)
	}
}

func TestResolveWithFocusTiebreak_ConvertsAmbiguousToUniqueStrong(t *testing.T) {
	desired := w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1},
		Kind: w.WindowShell,
		App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "shell-1:p1",
		},
	}
	observed := w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"live-a": {ID: "live-a", Kind: w.WindowShell, App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "shell-1:p1"}},
			"live-b": {ID: "live-b", Kind: w.WindowShell, App: w.ObservedAppRef{BundleID: "com.mitchellh.ghostty"}, Title: w.ObservedTitle{Value: "shell-1:p1"}},
		},
		Focus: w.ObservedFocus{Window: "live-b"},
	}
	res := ResolveWithFocusTiebreak(desired, observed, ResolveOptions{})
	if res.Class != ClassUniqueStrong {
		t.Fatalf("class = %s, want ClassUniqueStrong (focus-tiebreak should resolve ambiguity)", res.Class)
	}
	if res.Live != "live-b" {
		t.Errorf("Live = %q, want live-b (focused)", res.Live)
	}
	if len(res.Candidates) != 2 {
		t.Errorf("Candidates len = %d, want 2 (both still listed for orphan derivation)", len(res.Candidates))
	}
}
