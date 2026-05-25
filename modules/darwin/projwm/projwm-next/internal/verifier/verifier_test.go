package verifier

import (
	"strings"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestDiffDetectsDuplicateDesiredIdentityInsteadOfOverwriting(t *testing.T) {
	desired := w.DesiredWindowID{Project: "p", Kind: w.WindowShell, Index: 1}
	predicted := w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"predicted": {ID: "predicted", Kind: w.WindowShell, Workspace: "Q", MatchedTo: &desired},
		},
	}
	observed := w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"live-1": {ID: "live-1", Kind: w.WindowShell, Workspace: "Q", MatchedTo: &desired},
			"live-2": {ID: "live-2", Kind: w.WindowShell, Workspace: "Q", MatchedTo: &desired},
		},
	}

	diff := Diff(predicted, observed)
	if !containsDiff(diff, DiffExtra, "duplicate observed window") {
		t.Fatalf("expected duplicate desired identity diff, got %+v", diff.Entries)
	}
}

func TestDiffDetectsLayoutFocusDisplayAndWindowDrift(t *testing.T) {
	desired := w.DesiredWindowID{Project: "p", Kind: w.WindowShell, Index: 1}
	displayA := w.DisplayID("display-A")
	displayB := w.DisplayID("display-B")
	predicted := w.ObservedWorld{
		Displays: w.ObservedDisplayState{
			Displays: map[w.DisplayID]w.ObservedDisplay{displayA: {ID: displayA, Connected: true}},
			Primary:  &displayA,
		},
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"predicted": {
				ID:        "predicted",
				Kind:      w.WindowShell,
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "shell-1:p"},
				App:       w.ObservedAppRef{BundleID: "com.example.terminal"},
				MatchedTo: &desired,
			},
		},
		Layouts: map[w.WorkspaceID]w.ObservedLayout{
			"Q": {Workspace: "Q", Columns: []w.ObservedColumn{{Windows: []w.LiveWindowID{"predicted"}, Mode: w.ColumnSolo}}},
		},
		Focus: w.ObservedFocus{Workspace: "Q", Window: "predicted"},
	}
	observed := w.ObservedWorld{
		Displays: w.ObservedDisplayState{
			Displays: map[w.DisplayID]w.ObservedDisplay{displayB: {ID: displayB, Connected: true}},
			Primary:  &displayB,
		},
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"live": {
				ID:        "live",
				Kind:      w.WindowShell,
				Workspace: "W",
				Title:     w.ObservedTitle{Value: "unexpected"},
				App:       w.ObservedAppRef{BundleID: "com.example.terminal"},
				MatchedTo: &desired,
			},
		},
		Layouts: map[w.WorkspaceID]w.ObservedLayout{
			"Q": {Workspace: "Q"},
		},
		Focus: w.ObservedFocus{Workspace: "W", Window: "live"},
	}

	diff := Diff(predicted, observed)
	for _, want := range []string{"workspace differs", "title differs", "layout differs", "focus differs", "display state differs"} {
		if !containsDetail(diff, want) {
			t.Fatalf("expected diff containing %q, got %+v", want, diff.Entries)
		}
	}
}

func containsDiff(diff WorldDiff, class DiffClass, detail string) bool {
	for _, entry := range diff.Entries {
		if entry.Class == class && strings.Contains(entry.Detail, detail) {
			return true
		}
	}
	return false
}

func containsDetail(diff WorldDiff, detail string) bool {
	for _, entry := range diff.Entries {
		if strings.Contains(entry.Detail, detail) {
			return true
		}
	}
	return false
}
