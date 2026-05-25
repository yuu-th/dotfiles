package reducer

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.1 OP11: scratch shell の reducer state 遷移を verify する。

// TestReduceIntent_ShowScratchShell_FreshBuild — DesiredWorld に scratch
// SystemWindow がまだ無いとき、ShowScratchShell intent で初期化される。
func TestReduceIntent_ShowScratchShell_FreshBuild(t *testing.T) {
	s := w.WorldState{
		Observed: w.ObservedWorld{
			Focus: w.ObservedFocus{Window: "shell-1-omni"},
		},
	}
	d, err := ReduceIntent(s, intent.ShowScratchShell{})
	if err != nil {
		t.Fatal(err)
	}
	var scratch *w.SystemWindow
	for i := range d.SystemWindows {
		if d.SystemWindows[i].Kind == w.WindowScratch {
			scratch = &d.SystemWindows[i]
			break
		}
	}
	if scratch == nil {
		t.Fatal("scratch SystemWindow was not created")
	}
	if scratch.Title != "projwm-scratch-shell" {
		t.Errorf("Title = %q, want projwm-scratch-shell", scratch.Title)
	}
	if scratch.Visibility != w.CockpitShown {
		t.Errorf("Visibility = %s, want shown", scratch.Visibility)
	}
	if scratch.PriorWindow != "shell-1-omni" {
		t.Errorf("PriorWindow = %q, want shell-1-omni", scratch.PriorWindow)
	}
}

// TestReduceIntent_ShowScratchShell_PreservesPriorWindowOnReshow —
// scratch が既に Shown のとき、再度 ShowScratchShell を打っても PriorWindow
// は上書きされない (scratch 自身を prior にしてしまうのを防ぐ)。
func TestReduceIntent_ShowScratchShell_PreservesPriorWindowOnReshow(t *testing.T) {
	s := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{
				{
					ID:          w.SystemWindowID{Kind: w.WindowScratch, Index: 0},
					Kind:        w.WindowScratch,
					Title:       "projwm-scratch-shell",
					Visibility:  w.CockpitShown,
					PriorWindow: "shell-1-omni",
				},
			},
		},
		Observed: w.ObservedWorld{
			// Focus は scratch 自身に当たっている (再 show なので)
			Focus: w.ObservedFocus{Window: "scratch-omni"},
		},
	}
	d, err := ReduceIntent(s, intent.ShowScratchShell{})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.SystemWindows) != 1 {
		t.Fatalf("SystemWindows len = %d, want 1 (no duplication)", len(d.SystemWindows))
	}
	if d.SystemWindows[0].PriorWindow != "shell-1-omni" {
		t.Errorf("PriorWindow overwritten on re-show: got %q, want shell-1-omni", d.SystemWindows[0].PriorWindow)
	}
	if d.SystemWindows[0].Visibility != w.CockpitShown {
		t.Errorf("Visibility = %s, want shown", d.SystemWindows[0].Visibility)
	}
}

// TestReduceIntent_HideScratchShell_TogglesVisibility — Visibility が
// Hidden になるが PriorWindow は保持 (planner が hide op の Target に使う)。
func TestReduceIntent_HideScratchShell_TogglesVisibility(t *testing.T) {
	s := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{
				{
					ID:          w.SystemWindowID{Kind: w.WindowScratch, Index: 0},
					Kind:        w.WindowScratch,
					Title:       "projwm-scratch-shell",
					Visibility:  w.CockpitShown,
					PriorWindow: "shell-1-omni",
				},
			},
		},
	}
	d, err := ReduceIntent(s, intent.HideScratchShell{})
	if err != nil {
		t.Fatal(err)
	}
	scratch := d.SystemWindows[0]
	if scratch.Visibility != w.CockpitHidden {
		t.Errorf("Visibility = %s, want hidden", scratch.Visibility)
	}
	if scratch.PriorWindow != "shell-1-omni" {
		t.Errorf("PriorWindow lost: got %q, want shell-1-omni (planner needs it for hide op target)", scratch.PriorWindow)
	}
}

// TestReduceIntent_HideScratchShell_NoEntryIsNoop — scratch SystemWindow
// が存在しないときに HideScratchShell が来てもエラーにならず、新規作成も
// しない。
func TestReduceIntent_HideScratchShell_NoEntryIsNoop(t *testing.T) {
	s := w.WorldState{}
	d, err := ReduceIntent(s, intent.HideScratchShell{})
	if err != nil {
		t.Fatalf("HideScratchShell with no scratch entry should be no-op, got error: %v", err)
	}
	for _, sw := range d.SystemWindows {
		if sw.Kind == w.WindowScratch {
			t.Fatalf("HideScratchShell created a scratch entry where none existed: %+v", sw)
		}
	}
}

// TestReduceIntent_ShowScratchShell_HiddenToShownCapturesPrior —
// Visibility=Hidden 状態から再度 ShowScratchShell が来たとき、
// PriorWindow を observed.Focus から更新する (用途: 手動で hide した後、
// 別の window を focus してから再度 show したら新しい prior が記録される)。
func TestReduceIntent_ShowScratchShell_HiddenToShownCapturesPrior(t *testing.T) {
	s := w.WorldState{
		Desired: w.DesiredWorld{
			SystemWindows: []w.SystemWindow{
				{
					ID:          w.SystemWindowID{Kind: w.WindowScratch, Index: 0},
					Kind:        w.WindowScratch,
					Title:       "projwm-scratch-shell",
					Visibility:  w.CockpitHidden,
					PriorWindow: "old-prior",
				},
			},
		},
		Observed: w.ObservedWorld{
			Focus: w.ObservedFocus{Window: "new-prior-omni"},
		},
	}
	d, err := ReduceIntent(s, intent.ShowScratchShell{})
	if err != nil {
		t.Fatal(err)
	}
	if d.SystemWindows[0].PriorWindow != "new-prior-omni" {
		t.Errorf("PriorWindow = %q, want new-prior-omni (hidden→shown should refresh)", d.SystemWindows[0].PriorWindow)
	}
	if d.SystemWindows[0].Visibility != w.CockpitShown {
		t.Errorf("Visibility = %s, want shown", d.SystemWindows[0].Visibility)
	}
}
