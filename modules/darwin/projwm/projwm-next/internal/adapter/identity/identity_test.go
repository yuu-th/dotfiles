package identity

import (
	"context"
	"testing"

	pid "github.com/yuu-th/projwm-next/internal/identity"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestRealResolverReturnsStructuredUniqueStrong(t *testing.T) {
	resolver := NewRealResolver()
	desired := w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: "p1", Kind: w.WindowShell, Index: 1},
		Kind: w.WindowShell,
		App:  w.AppRequirement{BundleID: "com.example.term"},
		TitleContract: w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  "shell-1:p1",
		},
	}
	observed := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		"lw-1": {
			ID:    "lw-1",
			Kind:  w.WindowShell,
			App:   w.ObservedAppRef{BundleID: "com.example.term"},
			Title: w.ObservedTitle{Value: "shell-1:p1"},
		},
	}}
	got, err := resolver.Resolve(context.Background(), desired, observed)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Class != pid.ClassUniqueStrong || got.Confidence != 1.0 {
		t.Fatalf("resolution = %+v, want unique-strong confidence 1.0", got)
	}
}
