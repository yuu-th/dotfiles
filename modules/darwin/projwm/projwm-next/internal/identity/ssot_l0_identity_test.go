package identity

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSSOTL0IdentityResolvesExactlyOneControllerOwnedWindow(t *testing.T) {
	desired := desiredWindow()
	obs := observed("live-shell", "com.example.term", "ai-1:p1", w.WindowAI)

	res := Resolve(desired, obs)
	if res.Class != ClassUniqueStrong || res.Live != "live-shell" {
		t.Fatalf("SSOT L0 identity resolution = %+v, want unique-strong live-shell", res)
	}
}

func TestSSOTL0IdentityRejectsDuplicateSpecialWindowIdentity(t *testing.T) {
	desired := desiredWindow()
	obs := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		"live-1": observedWindow("live-1", "com.example.term", "ai-1:p1", w.WindowAI),
		"live-2": observedWindow("live-2", "com.example.term", "ai-1:p1", w.WindowAI),
	}}

	res := Resolve(desired, obs)
	if res.Class != ClassAmbiguous {
		t.Fatalf("SSOT L0 duplicate identity resolution = %+v, want ambiguous", res)
	}
}
