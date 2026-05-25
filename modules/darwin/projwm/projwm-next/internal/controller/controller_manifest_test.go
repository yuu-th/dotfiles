package controller

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// E2.1: a manifest digest mismatch surfaces as a [MANIFEST] card with
// expected / observed context entries so the cockpit can render a
// validation-report prompt.
func TestEmitManifestMismatchCard(t *testing.T) {
	c := minimalControllerEnv(t)
	c.EmitManifestMismatchCard("expected-digest-abc", "observed-digest-xyz")
	cards := c.State().Meta.ActiveCards
	if len(cards) != 1 {
		t.Fatalf("expected 1 manifest card, got %d", len(cards))
	}
	card := cards[0]
	if card.Type != w.CardTypeManifest {
		t.Errorf("type = %s, want MANIFEST", card.Type)
	}
	if card.Context["expected"] != "expected-digest-abc" {
		t.Errorf("expected = %q", card.Context["expected"])
	}
	if card.Context["observed"] != "observed-digest-xyz" {
		t.Errorf("observed = %q", card.Context["observed"])
	}
	if len(card.Actions) < 2 {
		t.Errorf("expected at least 2 actions, got %d", len(card.Actions))
	}
}

// E2.3: a real invariant violation during converge surfaces as an
// [INVARIANT] card before the no-commit trace is recorded. We pin the
// shape so future refactors don't accidentally drop the card emit.
//
// We invoke appendActiveCards directly with the same shape the
// converge loop produces.
func TestAppendActiveCards_InvariantShape(t *testing.T) {
	c := minimalControllerEnv(t)
	c.appendActiveCards([]w.Card{{
		Type:    w.CardTypeInvariant,
		Subject: "invariant violation",
		Context: map[string]string{"detail": "INV.3: ..."},
		Actions: []w.CardAction{
			{Key: "Enter", Label: "show details"},
			{Key: "Esc", Label: "dismiss"},
		},
	}})
	cards := c.State().Meta.ActiveCards
	if len(cards) != 1 {
		t.Fatalf("expected 1 invariant card, got %d", len(cards))
	}
	if cards[0].Type != w.CardTypeInvariant {
		t.Errorf("type = %s, want INVARIANT", cards[0].Type)
	}
	if cards[0].Context["detail"] == "" {
		t.Errorf("detail field empty")
	}
}
