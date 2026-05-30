package invariant

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// L1 invariant tests for SSOT §6.9.1 attribution behavior table (ATTR-*),
// specifically ATTR-B4 folded onto INV-01 / Check14DuplicateWindow.
//
// Connection (SSOT §6.9.1 "既存列挙との接続"):
//   ATTR-B4（唯一性）は INV-01（§3.4 / Check14）を provenance-aware 化したもの。
//
// The contract: when the user has opened same-title Zed windows that collide
// with our managed editor (single-process app — title is ambiguous), only the
// window in state.Meta.WindowProvenance is OURS. The colliding non-provenance
// windows are External (the user's) and must NOT be reported as INV-01
// duplicates-to-close. Check14 must become provenance-aware.
//
// Status note (honest, pre-implementation):
//   - The B4 case below is TRUE RED today: Check14 groups purely by
//     MatchedTo+Kind and ignores WindowProvenance, so it flags the user's
//     window as a duplicate. The assertion (no violation) fails until the
//     impl consults provenance.
//   - The guard case is GREEN today and must stay green: with NO provenance
//     and two genuine managed duplicates, Check14 still fires (no regression).

// attrEditorDesiredID is the managed Zed editor identity these tests collide on.
func attrEditorDesiredID() w.DesiredWindowID {
	return w.DesiredWindowID{Project: "p1", Kind: w.WindowEditor, Index: 1}
}

// attrZedObserved builds an observed dev.zed.Zed editor window matched to the
// editor identity. All same-title windows share MatchedTo (title is ambiguous
// for a single-process app), so Check14's naive MatchedTo+Kind grouping sees
// them as the same group.
func attrZedObserved(id w.LiveWindowID) w.ObservedWindow {
	did := attrEditorDesiredID()
	return w.ObservedWindow{
		ID:        id,
		Kind:      w.WindowEditor,
		App:       w.ObservedAppRef{BundleID: "dev.zed.Zed"},
		Title:     w.ObservedTitle{Value: "p1"},
		MatchedTo: &did,
	}
}

// ATTR-B4 (RED): a desired Zed editor plus TWO observed same-title dev.zed.Zed
// windows, both currently matched to the editor identity. WindowProvenance maps
// the identity to exactly ONE of them ("zed-ours"). The other ("zed-user") is
// the user's External window. Check14 must NOT flag a duplicate-to-close:
// provenance establishes which window is uniquely ours; the non-provenance
// window is not an INV-01 duplicate (user-window protection, §6.9 INV-01).
//
// RED today: Check14DuplicateWindow groups by MatchedTo+Kind and ignores
// state.Meta.WindowProvenance, so it sees 2 candidates and fires.
func TestZedAttr_B4_ProvenanceAwareCheck14DoesNotFlagUserWindow(t *testing.T) {
	did := attrEditorDesiredID()
	state := w.WorldState{
		Desired: w.DesiredWorld{
			Projects: map[w.ProjectID]w.DesiredProject{
				"p1": {ID: "p1"},
			},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"zed-ours": attrZedObserved("zed-ours"),
				"zed-user": attrZedObserved("zed-user"),
			},
		},
		Meta: w.ControllerMeta{
			// We own exactly one of the colliding windows; the other is the user's.
			WindowProvenance: map[w.DesiredWindowID]w.LiveWindowID{
				did: "zed-ours",
			},
		},
	}
	if v := Check14DuplicateWindow(state); v != nil {
		t.Fatalf("ATTR-B4: Check14 flagged a duplicate-to-close despite provenance owning exactly one window (zed-ours); the non-provenance window zed-user is the user's External window and must not be an INV-01 duplicate: %s", v.Message)
	}
}

// ATTR-B4 guard (GREEN, regression fence): with NO provenance entry for the
// identity and two genuine managed same-title duplicates (e.g. omniwm app-rule
// re-fire spawned an extra), Check14 must STILL fire. Making Check14
// provenance-aware must not suppress real duplicates when provenance is absent.
func TestZedAttr_B4_Check14StillFiresWithoutProvenance(t *testing.T) {
	state := w.WorldState{
		Desired: w.DesiredWorld{
			Projects: map[w.ProjectID]w.DesiredProject{
				"p1": {ID: "p1"},
			},
		},
		Observed: w.ObservedWorld{
			Windows: map[w.LiveWindowID]w.ObservedWindow{
				"zed-a": attrZedObserved("zed-a"),
				"zed-b": attrZedObserved("zed-b"),
			},
		},
		// No WindowProvenance: neither window is owned, both are candidate
		// duplicates of the same managed identity.
		Meta: w.ControllerMeta{},
	}
	v := Check14DuplicateWindow(state)
	if v == nil {
		t.Fatal("ATTR-B4 guard: Check14 must still fire for two genuine managed duplicates when there is no provenance (no regression)")
	}
	if v.ID != 14 {
		t.Errorf("ATTR-B4 guard: violation ID = %d, want 14", v.ID)
	}
}
