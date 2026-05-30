package identity

import (
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// L0 tests for SSOT §6.9.1 attribution behavior table (ATTR-*).
//
// These exercise the identity resolver's PROVENANCE precedence: for a
// single-process app (Zed) the title is ambiguous (the user can open the same
// project), so the desired identity must resolve to the live window projwm
// actually spawned/adopted (opts.Provenance), and a colliding non-provenance
// window must stay External.
//
// Status note (honest, pre-implementation):
//   - ATTR-A2 / B1 / B4 / C1 are TRUE RED: today two same-title Zed windows
//     resolve to ClassAmbiguous; provenance must make them unique.
//   - ATTR-A3 / A4 are GUARD tests: with provenance unimplemented they pass
//     trivially (no window / bundle mismatch already yields Missing); they
//     protect the implementation from trusting a stale/reused ID blindly.

func attrZedDesired(project string, idx int) w.DesiredWindow {
	return w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: w.ProjectID(project), Kind: w.WindowEditor, Index: idx},
		Kind: w.WindowEditor,
		App:  w.AppRequirement{BundleID: "dev.zed.Zed"},
		TitleContract: w.TitleContract{
			Authority: w.TitleAppOwned,
			Expected:  project, // Zed title = basename(cwd) == project
			Drift:     w.TitleDriftRematch,
		},
		MatchHints: []w.MatchHint{
			{Kind: w.MatchByBundleID, Pattern: "dev.zed.Zed", Confidence: w.MatchStrong},
		},
	}
}

func attrZedWindow(id w.LiveWindowID, title string) w.ObservedWindow {
	return w.ObservedWindow{
		ID:    id,
		Kind:  w.WindowEditor,
		App:   w.ObservedAppRef{BundleID: "dev.zed.Zed"},
		Title: w.ObservedTitle{Value: title},
	}
}

// ATTR-A2 / B1: provenance precedence. Two Zed windows share title "dotfiles"
// (ours + a user window). Provenance points at ours → must resolve uniquely to
// ours; the user's window is External (not the resolution).
func TestZedAttr_A2_ProvenancePrecedenceOverCollidingTitle(t *testing.T) {
	desired := attrZedDesired("dotfiles", 1)
	observed := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		"zed-ours": attrZedWindow("zed-ours", "dotfiles"),
		"zed-user": attrZedWindow("zed-user", "dotfiles"),
	}}
	opts := ResolveOptions{Provenance: map[w.DesiredWindowID]w.LiveWindowID{
		desired.ID: "zed-ours",
	}}
	res := ResolveWithOptions(desired, observed, opts)
	if res.Class != ClassUniqueStrong || res.Live != "zed-ours" {
		t.Fatalf("ATTR-A2: class=%s live=%s, want UniqueStrong zed-ours (provenance must beat colliding title; zed-user must stay External). candidates=%v",
			res.Class, res.Live, res.Candidates)
	}
}

// ATTR-A3: provenance live ID is no longer observed → entry must be treated as
// stale (not reported as Live); resolution falls through. GUARD (green until the
// resolver starts trusting provenance — then this stops it trusting a dead ID).
func TestZedAttr_A3_StaleProvenanceIDNotReported(t *testing.T) {
	desired := attrZedDesired("dotfiles", 1)
	observed := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{}}
	opts := ResolveOptions{Provenance: map[w.DesiredWindowID]w.LiveWindowID{
		desired.ID: "zed-gone",
	}}
	res := ResolveWithOptions(desired, observed, opts)
	if res.Live == "zed-gone" {
		t.Fatalf("ATTR-A3: resolver reported a provenance ID (zed-gone) that is not observed; must validate presence first")
	}
	if res.Class != ClassMissing {
		t.Fatalf("ATTR-A3: class=%s, want Missing (no live window → respawn)", res.Class)
	}
}

// ATTR-A4: provenance ID is present but the window behind it is now a DIFFERENT
// app (window-ID reuse) → must reject the provenance match (bundle mismatch) and
// not claim it. GUARD against blind-ID trust.
func TestZedAttr_A4_ProvenanceIDReusedByDifferentApp(t *testing.T) {
	desired := attrZedDesired("dotfiles", 1)
	observed := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		// Same ID our provenance points at, but it is now a Safari window.
		"zed-reused": {ID: "zed-reused", Kind: w.WindowExternal, App: w.ObservedAppRef{BundleID: "com.apple.Safari"}, Title: w.ObservedTitle{Value: "dotfiles"}},
	}}
	opts := ResolveOptions{Provenance: map[w.DesiredWindowID]w.LiveWindowID{
		desired.ID: "zed-reused",
	}}
	res := ResolveWithOptions(desired, observed, opts)
	if res.Live == "zed-reused" {
		t.Fatalf("ATTR-A4: resolver claimed a reused ID whose window is com.apple.Safari; must validate bundle before trusting provenance")
	}
}

// ATTR-B4: three windows share the title; provenance owns one. Resolution must
// be that one; the other two are External (NOT INV-01 duplicates to close).
func TestZedAttr_B4_ProvenanceUniqueAmongThreeCollidingTitles(t *testing.T) {
	desired := attrZedDesired("dotfiles", 1)
	observed := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		"zed-ours":  attrZedWindow("zed-ours", "dotfiles"),
		"zed-userA": attrZedWindow("zed-userA", "dotfiles"),
		"zed-userB": attrZedWindow("zed-userB", "dotfiles"),
	}}
	opts := ResolveOptions{Provenance: map[w.DesiredWindowID]w.LiveWindowID{
		desired.ID: "zed-ours",
	}}
	res := ResolveWithOptions(desired, observed, opts)
	if res.Class != ClassUniqueStrong || res.Live != "zed-ours" {
		t.Fatalf("ATTR-B4: class=%s live=%s, want UniqueStrong zed-ours among 3 colliding titles", res.Class, res.Live)
	}
}

// ATTR-C1 / C2(provenance part): two editors of the SAME project share the same
// title (basename). Provenance gives each its distinct live ID, so editor-1 and
// editor-2 resolve to different windows. (Recovery-time title ambiguity when
// provenance is lost is an accepted limitation — doc only, not tested.)
func TestZedAttr_C1_ProvenanceDistinguishesMultipleEditors(t *testing.T) {
	ed1 := attrZedDesired("dotfiles", 1)
	ed2 := attrZedDesired("dotfiles", 2)
	observed := w.ObservedWorld{Windows: map[w.LiveWindowID]w.ObservedWindow{
		"zed-ed1": attrZedWindow("zed-ed1", "dotfiles"),
		"zed-ed2": attrZedWindow("zed-ed2", "dotfiles"),
	}}
	prov := map[w.DesiredWindowID]w.LiveWindowID{
		ed1.ID: "zed-ed1",
		ed2.ID: "zed-ed2",
	}
	r1 := ResolveWithOptions(ed1, observed, ResolveOptions{Provenance: prov})
	r2 := ResolveWithOptions(ed2, observed, ResolveOptions{Provenance: prov})
	if r1.Class != ClassUniqueStrong || r1.Live != "zed-ed1" {
		t.Fatalf("ATTR-C1: editor-1 resolved to class=%s live=%s, want zed-ed1", r1.Class, r1.Live)
	}
	if r2.Class != ClassUniqueStrong || r2.Live != "zed-ed2" {
		t.Fatalf("ATTR-C1: editor-2 resolved to class=%s live=%s, want zed-ed2", r2.Class, r2.Live)
	}
}
