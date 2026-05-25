package verifier

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSSOTL0VerifierClassifiesObservedDrift(t *testing.T) {
	desired := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	predicted := w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"predicted": {
				ID:        "predicted",
				Kind:      w.WindowShell,
				Workspace: "Q",
				Title:     w.ObservedTitle{Value: "shell-1:dotfiles"},
				MatchedTo: &desired,
			},
		},
		Focus: w.ObservedFocus{Workspace: "Q", Window: "predicted"},
	}
	observed := w.ObservedWorld{
		Windows: map[w.LiveWindowID]w.ObservedWindow{
			"live": {
				ID:        "live",
				Kind:      w.WindowShell,
				Workspace: "9",
				Title:     w.ObservedTitle{Value: "shell-1:dotfiles"},
				MatchedTo: &desired,
			},
		},
		Focus: w.ObservedFocus{Workspace: "9", Window: "live"},
	}

	diff := Diff(predicted, observed)
	if diff.Empty() {
		t.Fatal("SSOT L0 verifier must classify predicted/observed drift")
	}
	if !containsDetail(diff, "workspace differs") {
		t.Fatalf("SSOT L0 verifier drift entries = %+v, want workspace drift", diff.Entries)
	}
}
