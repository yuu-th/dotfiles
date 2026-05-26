package controller

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §5.4 cards 6 種: NEW / CLOSED / MOVED / INVARIANT / MANIFEST /
// OMNIWM-RECOVERY. The first five are emitted via reducer / planner /
// invariant paths; the sixth (OMNIWM-RECOVERY) surfaces self-heal ladder
// actions that are taken outside the transaction loop (omniwm restart,
// rule re-deploy, managed app relaunch) and therefore needs a dedicated
// Controller surface.

func TestEmitOmniwmRecoveryCard_AppendsActiveCard(t *testing.T) {
	c := minimalControllerEnv(t)
	c.EmitOmniwmRecoveryCard("Lv1", "redeploy rules", "rule count 3 below floor 5")

	cards := c.state.Meta.ActiveCards
	if len(cards) != 1 {
		t.Fatalf("expected 1 active card, got %d", len(cards))
	}
	got := cards[0]
	if got.Type != w.CardTypeOmniwmRecovery {
		t.Errorf("card type = %q, want %q", got.Type, w.CardTypeOmniwmRecovery)
	}
	if got.Subject == "" || got.Context["level"] != "Lv1" || got.Context["action"] != "redeploy rules" {
		t.Errorf("card content insufficient: %+v", got)
	}
	if got.Context["detail"] != "rule count 3 below floor 5" {
		t.Errorf("card detail = %q, want full message", got.Context["detail"])
	}
}

func TestEmitOmniwmRecoveryCard_OmitsEmptyDetail(t *testing.T) {
	c := minimalControllerEnv(t)
	c.EmitOmniwmRecoveryCard("Lv3", "restart omniwm", "")
	cards := c.state.Meta.ActiveCards
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	if _, hasDetail := cards[0].Context["detail"]; hasDetail {
		t.Errorf("card context should omit empty detail field, got: %+v", cards[0].Context)
	}
}

func TestEmitOmniwmRecoveryCard_MultipleLadderStepsAccumulate(t *testing.T) {
	c := minimalControllerEnv(t)
	c.EmitOmniwmRecoveryCard("Lv1", "redeploy rules", "")
	c.EmitOmniwmRecoveryCard("Lv2", "relaunch managed app", "Ghostty")
	c.EmitOmniwmRecoveryCard("Lv3", "restart omniwm", "")
	cards := c.state.Meta.ActiveCards
	if len(cards) != 3 {
		t.Fatalf("expected 3 distinct ladder-step cards, got %d", len(cards))
	}
	wantLevels := []string{"Lv1", "Lv2", "Lv3"}
	for i, want := range wantLevels {
		if cards[i].Context["level"] != want {
			t.Errorf("card[%d].level = %q, want %q", i, cards[i].Context["level"], want)
		}
	}
}
