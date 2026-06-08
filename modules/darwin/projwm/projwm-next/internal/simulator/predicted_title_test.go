package simulator

import (
	"fmt"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// Viewer windows mirror ai-N and are titled ai-view-N (naming.viewerTitleForAI =
// "ai-view"+strip("ai"); naming/ssot_l0_identity_test: ai-view-1 ↔ ai-1). The
// simulator's predictedTitle MUST predict the same ai-view-N the executor
// actually produces, otherwise the verifier compares a wrong predicted viewer
// title against the real one and the viewer never converges. Regression for the
// 2026-06-08 off-by-one (predicted ai-view-(N+1), e.g. ai-view-2 for ai-1).
func TestPredictedTitle_ViewerMatchesAIIndex(t *testing.T) {
	for _, idx := range []int{1, 2, 3} {
		d := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowAI, Index: idx}
		got := predictedTitle(d, w.WindowViewer)
		want := fmt.Sprintf("ai-view-%d:dotfiles", idx)
		if got != want {
			t.Fatalf("predictedTitle viewer for ai-%d: got %q want %q (off-by-one regression)", idx, got, want)
		}
	}
}
