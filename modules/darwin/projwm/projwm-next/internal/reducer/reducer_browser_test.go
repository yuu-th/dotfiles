package reducer

import (
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §4.1 OP14-17 browser tab CRUD の reducer 状態遷移を verify する。
//
// HONEST GAP (S14 第一段階): URL 本文は本来 PrivatePayloadStore に格納し、
// DesiredWorld には opaque ref を残す設計 (SSOT 650) だが、現状は
// URLPayloadRefs に URL を literal 格納する一時実装。S20 で controller-
// level の Put/Forget に置き換えて proper opaque ref 化する。

func browserBaseState() w.WorldState {
	browserID := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}
	return w.WorldState{
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles": {ID: "dotfiles", Windows: []w.DesiredWindow{
					{ID: browserID, Kind: w.WindowBrowser, Browser: &w.DesiredBrowserSession{
						URLPayloadRefs: []w.PrivatePayloadRef{"https://a.example", "https://b.example"},
						URLCount:       2,
					}},
				}},
			},
		},
	}
}

func TestReduceIntent_BrowserAddTab_AppendsURL(t *testing.T) {
	s := browserBaseState()
	bid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}
	d, err := ReduceIntent(s, intent.BrowserAddTab{
		Project: "dotfiles", WindowID: bid, URL: "https://c.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	pr := d.Projects["dotfiles"]
	if len(pr.Windows[0].Browser.URLPayloadRefs) != 3 {
		t.Fatalf("URLPayloadRefs len = %d, want 3", len(pr.Windows[0].Browser.URLPayloadRefs))
	}
	if pr.Windows[0].Browser.URLPayloadRefs[2] != "https://c.example" {
		t.Errorf("appended ref = %q, want https://c.example", pr.Windows[0].Browser.URLPayloadRefs[2])
	}
	if pr.Windows[0].Browser.URLCount != 3 {
		t.Errorf("URLCount = %d, want 3", pr.Windows[0].Browser.URLCount)
	}
}

func TestReduceIntent_BrowserRemoveTab_RemovesAtIndex(t *testing.T) {
	s := browserBaseState()
	bid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}
	d, err := ReduceIntent(s, intent.BrowserRemoveTab{
		Project: "dotfiles", WindowID: bid, Tab: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	refs := d.Projects["dotfiles"].Windows[0].Browser.URLPayloadRefs
	if len(refs) != 1 || refs[0] != "https://b.example" {
		t.Errorf("after remove tab=1, refs = %+v, want [https://b.example]", refs)
	}
	if d.Projects["dotfiles"].Windows[0].Browser.URLCount != 1 {
		t.Errorf("URLCount = %d, want 1", d.Projects["dotfiles"].Windows[0].Browser.URLCount)
	}
}

func TestReduceIntent_BrowserRemoveTab_OutOfRangeErrors(t *testing.T) {
	s := browserBaseState()
	bid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}
	_, err := ReduceIntent(s, intent.BrowserRemoveTab{
		Project: "dotfiles", WindowID: bid, Tab: 99,
	})
	if err == nil {
		t.Fatal("expected error for out-of-range tab, got nil")
	}
}

func TestReduceIntent_BrowserChangeTabURL_Replaces(t *testing.T) {
	s := browserBaseState()
	bid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}
	d, err := ReduceIntent(s, intent.BrowserChangeTabURL{
		Project: "dotfiles", WindowID: bid, Tab: 2, URL: "https://changed.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	refs := d.Projects["dotfiles"].Windows[0].Browser.URLPayloadRefs
	if refs[1] != "https://changed.example" {
		t.Errorf("tab 2 = %q, want changed", refs[1])
	}
	if d.Projects["dotfiles"].Windows[0].Browser.URLCount != 2 {
		t.Errorf("URLCount changed unexpectedly: %d, want 2 (preserved)", d.Projects["dotfiles"].Windows[0].Browser.URLCount)
	}
}

func TestReduceIntent_BrowserReorderTabs_MovesFromTo(t *testing.T) {
	s := browserBaseState()
	bid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}
	d, err := ReduceIntent(s, intent.BrowserReorderTabs{
		Project: "dotfiles", WindowID: bid, From: 1, To: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	refs := d.Projects["dotfiles"].Windows[0].Browser.URLPayloadRefs
	if len(refs) != 2 || refs[0] != "https://b.example" || refs[1] != "https://a.example" {
		t.Errorf("after reorder from=1 to=2, refs = %+v, want [b, a]", refs)
	}
}

func TestReduceIntent_BrowserReorderTabs_SameFromToIsNoop(t *testing.T) {
	s := browserBaseState()
	bid := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 1}
	d, err := ReduceIntent(s, intent.BrowserReorderTabs{
		Project: "dotfiles", WindowID: bid, From: 1, To: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	refs := d.Projects["dotfiles"].Windows[0].Browser.URLPayloadRefs
	if len(refs) != 2 || refs[0] != "https://a.example" || refs[1] != "https://b.example" {
		t.Errorf("after no-op reorder, refs = %+v, want unchanged", refs)
	}
}

func TestReduceIntent_BrowserOps_RejectNonBrowserWindow(t *testing.T) {
	shellID := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowShell, Index: 1}
	state := w.WorldState{
		Desired: w.DesiredWorld{
			ActiveProfile: "work",
			Profiles: map[w.ProfileID]w.DesiredProfile{
				"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{}},
			},
			Projects: map[w.ProjectID]w.DesiredProject{
				"dotfiles": {ID: "dotfiles", Windows: []w.DesiredWindow{
					{ID: shellID, Kind: w.WindowShell},
				}},
			},
		},
	}
	_, err := ReduceIntent(state, intent.BrowserAddTab{
		Project: "dotfiles", WindowID: shellID, URL: "https://x",
	})
	if err == nil {
		t.Fatal("expected error when target window is not a browser, got nil")
	}
}

func TestReduceIntent_BrowserOps_RejectUnknownWindow(t *testing.T) {
	s := browserBaseState()
	unknown := w.DesiredWindowID{Project: "dotfiles", Kind: w.WindowBrowser, Index: 99}
	_, err := ReduceIntent(s, intent.BrowserRemoveTab{
		Project: "dotfiles", WindowID: unknown, Tab: 1,
	})
	if err == nil {
		t.Fatal("expected error for unknown window id, got nil")
	}
}
