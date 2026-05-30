// Package invariant implements the 13 acceptance invariants from specs.md §2.
// Pure functions; no side effects. Each Check returns nil on pass or a Violation.
package invariant

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yuu-th/projwm-next/internal/identity"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Violation is a structured failure for a single invariant.
type Violation struct {
	ID      int
	Name    string
	Message string
}

func (v Violation) Error() string {
	return fmt.Sprintf("invariant %d (%s): %s", v.ID, v.Name, v.Message)
}

// CheckOptions controls Check.
type CheckOptions struct {
	// FinalFocusCommandKey is the command key of the just-committed transaction (for invariant 10).
	// Empty disables invariant 10.
	FinalFocusCommandKey string
}

// CheckAll runs all 13 invariants. Returns the slice of all Violations.
func CheckAll(state w.WorldState, opts CheckOptions) []Violation {
	var vs []Violation
	if v := Check1Manifest(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check2ActiveProfile(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check3SlotAssignment(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check4ActiveDesiredPresent(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check5ArchivedAbsent(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check6InactivePolicy(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check7ViewerSet(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check8ViewerOrder(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check9LayoutSemantics(state); v != nil {
		vs = append(vs, *v)
	}
	if opts.FinalFocusCommandKey != "" {
		if v := Check10FinalFocus(state, opts.FinalFocusCommandKey); v != nil {
			vs = append(vs, *v)
		}
	}
	if v := Check11WorkspaceRoleSegregation(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check12TitleDrift(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check13NoUnprocessedDirty(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check14DuplicateWindow(state); v != nil {
		vs = append(vs, *v)
	}
	if v := Check15CockpitOnParkWorkspace(state); v != nil {
		vs = append(vs, *v)
	}
	return vs
}

// Check15CockpitOnParkWorkspace realises SSOT §3.4 INV-06: 各 cockpit
// SystemWindow に対し、対応する observed live window は常に SystemWindow
// 自身の ParkWorkspace 上に存在する。違反時 [INVARIANT] card を出し、
// transaction loop は MoveCockpitToParkWorkspace op で修復する。
//
// observed の cockpit live window は title (controller-owned) で識別する。
func Check15CockpitOnParkWorkspace(state w.WorldState) *Violation {
	for _, sw := range state.Desired.SystemWindows {
		if sw.Kind != w.WindowCockpit {
			continue
		}
		if sw.ParkWorkspace == "" {
			continue
		}
		// observed.Windows から sw.Title と一致する cockpit ghostty を探す。
		for _, ow := range state.Observed.Windows {
			if ow.Kind != w.WindowCockpit {
				continue
			}
			if ow.Title.Value != sw.Title {
				continue
			}
			if ow.Workspace != sw.ParkWorkspace {
				return &Violation{ID: 15, Name: "cockpit-park-workspace",
					Message: fmt.Sprintf("cockpit %q is on workspace %q but should be on park workspace %q (INV-06)", sw.Title, ow.Workspace, sw.ParkWorkspace)}
			}
		}
	}
	return nil
}

// Check14DuplicateWindow realises SSOT §2.5 EC4 / §3.4 INV-01: the world should
// have at most ONE observed window per (project, kind, index). If multiple are
// observed (typically: omniwm app-rule re-fire spawned an extra, or user
// manually opened a second Ghostty with the same title), this is reported as
// an invariant violation so the controller emits a [INVARIANT] card. The
// planner separately uses focus-tiebreak to pick a winner and continue
// converging — this invariant is the user-visible notification surface.
func Check14DuplicateWindow(state w.WorldState) *Violation {
	// Group observed windows by MatchedTo + observed.Kind. Both keys are
	// required because viewers carry MatchedTo pointing to the source AI
	// identity (kind=ai) per convention but are themselves kind=viewer;
	// they are NOT duplicates of the AI. Genuine duplicates have identical
	// MatchedTo AND matching observed.Kind.
	type groupKey struct {
		Desired w.DesiredWindowID
		Kind    w.WindowKind
	}
	groups := map[groupKey][]w.LiveWindowID{}
	for _, ow := range state.Observed.Windows {
		if ow.MatchedTo == nil {
			continue
		}
		// Only count windows whose observed.Kind matches the desired Kind
		// (filters out viewer-pairing artifacts above).
		if ow.Kind != ow.MatchedTo.Kind {
			continue
		}
		// SSOT §6.9.1 ATTR-B4: when provenance owns a window for this identity,
		// any same-group live id that is NOT the provenance window is the user's
		// External window (single-process apps collide on title). Exclude it
		// from the duplicate set so we never flag the user's window. With no
		// provenance entry, every candidate stays in the set (guard: genuine
		// managed duplicates still fire).
		if provLive, ok := state.Meta.WindowProvenance[*ow.MatchedTo]; ok && provLive != ow.ID {
			continue
		}
		k := groupKey{Desired: *ow.MatchedTo, Kind: ow.Kind}
		groups[k] = append(groups[k], ow.ID)
	}
	for k, ids := range groups {
		if len(ids) <= 1 {
			continue
		}
		// Only flag for active (non-archived) projects.
		pr, ok := state.Desired.Projects[k.Desired.Project]
		if !ok || pr.Archived {
			continue
		}
		return &Violation{ID: 14, Name: "duplicate-window",
			Message: fmt.Sprintf("desired window %s/%s/%d has %d observed candidates: %v (INV-01 — orphan all but the most-recently-focused)", k.Desired.Project, k.Desired.Kind, k.Desired.Index, len(ids), ids)}
	}
	return nil
}

func Check1Manifest(state w.WorldState) *Violation {
	if state.Environment.SchemaVersion == 0 {
		return &Violation{ID: 1, Name: "manifest", Message: "manifest schema version unset"}
	}
	return nil
}

func Check2ActiveProfile(state w.WorldState) *Violation {
	if _, ok := state.Desired.Profiles[state.Desired.ActiveProfile]; !ok {
		return &Violation{ID: 2, Name: "active-profile", Message: fmt.Sprintf("ActiveProfile %q is not in DesiredWorld", state.Desired.ActiveProfile)}
	}
	return nil
}

func Check3SlotAssignment(state w.WorldState) *Violation {
	prof, ok := state.Desired.Profiles[state.Desired.ActiveProfile]
	if !ok {
		return nil
	}
	for sid := range prof.Assignments {
		if _, ok := state.Environment.SlotByID(sid); !ok {
			return &Violation{ID: 3, Name: "slot-assignment", Message: fmt.Sprintf("slot %q in profile not present in environment", sid)}
		}
	}
	return nil
}

func Check4ActiveDesiredPresent(state w.WorldState) *Violation {
	prof, ok := state.Desired.Profiles[state.Desired.ActiveProfile]
	if !ok {
		return nil
	}
	for _, sid := range sortedSlotKeys(prof.Assignments) {
		pid := prof.Assignments[sid]
		pr, ok := state.Desired.Projects[pid]
		if !ok || pr.Archived {
			continue
		}
		slot, hasSlot := state.Environment.SlotByID(sid)
		for _, dw := range pr.Windows {
			opts := identity.ResolveOptions{Provenance: state.Meta.WindowProvenance}
			if hasSlot {
				opts.ExpectedWorkspace = slot.Workspace
			}
			res := identity.ResolveWithOptions(dw, state.Observed, opts)
			if res.Class != identity.ClassUniqueStrong {
				return &Violation{ID: 4, Name: "active-desired-present",
					Message: fmt.Sprintf("desired window %s/%s/%d resolves as %s (need unique-strong)", dw.ID.Project, dw.ID.Kind, dw.ID.Index, res.Class)}
			}
		}
	}
	return nil
}

func Check5ArchivedAbsent(state w.WorldState) *Violation {
	for _, pid := range sortedProjectKeys(state.Desired.Projects) {
		pr := state.Desired.Projects[pid]
		if !pr.Archived {
			continue
		}
		for _, ow := range state.Observed.Windows {
			if ow.MatchedTo != nil && ow.MatchedTo.Project == pid {
				return &Violation{ID: 5, Name: "archived-absent",
					Message: fmt.Sprintf("archived project %q still has live window %s", pid, ow.ID)}
			}
		}
	}
	return nil
}

func Check6InactivePolicy(state w.WorldState) *Violation {
	prof, ok := state.Desired.Profiles[state.Desired.ActiveProfile]
	if !ok {
		return nil
	}
	active := map[w.ProjectID]bool{}
	for _, p := range prof.Assignments {
		active[p] = true
	}
	for _, pid := range sortedProjectKeys(state.Desired.Projects) {
		pr := state.Desired.Projects[pid]
		if pr.Archived || active[pid] {
			continue
		}
		// inactive project: enforce policy from active profile
		policy := prof.InactivePolicy
		if policy == "" {
			policy = w.InactivePolicyRemove
		}
		if policy == w.InactivePolicyRemove {
			for _, ow := range state.Observed.Windows {
				if ow.MatchedTo != nil && ow.MatchedTo.Project == pid {
					return &Violation{ID: 6, Name: "inactive-policy",
						Message: fmt.Sprintf("inactive project %q (policy=Remove) still has live window %s", pid, ow.ID)}
				}
			}
		}
	}
	return nil
}

func Check7ViewerSet(state w.WorldState) *Violation {
	viewerWS := state.Environment.Workspaces.Viewer
	if viewerWS == "" {
		return nil
	}
	prof, ok := state.Desired.Profiles[state.Desired.ActiveProfile]
	if !ok {
		return nil
	}
	want := map[w.DesiredWindowID]bool{}
	titles := map[w.DesiredWindowID]string{}
	for _, sid := range sortedSlotKeys(prof.Assignments) {
		pid := prof.Assignments[sid]
		pr, ok := state.Desired.Projects[pid]
		if !ok || pr.Archived {
			continue
		}
		for _, dw := range pr.Windows {
			if dw.ID.Kind == w.WindowAI {
				want[dw.ID] = true
				titles[dw.ID] = viewerTitleForAI(dw.TitleContract.Expected)
			}
		}
	}
	got := map[w.DesiredWindowID]bool{}
	wantList := make([]w.DesiredWindowID, 0, len(want))
	for d := range want {
		wantList = append(wantList, d)
	}
	for _, ow := range state.Observed.Windows {
		if ow.Kind != w.WindowViewer || ow.Workspace != viewerWS {
			continue
		}
		matched, ok := matchViewerDesired(ow, wantList, titles)
		if !ok {
			return &Violation{ID: 7, Name: "viewer-set", Message: fmt.Sprintf("viewer %s has no active AI match", ow.ID)}
		}
		got[matched] = true
	}
	for d := range want {
		if !got[d] {
			return &Violation{ID: 7, Name: "viewer-set", Message: fmt.Sprintf("viewer for AI window %s/%d missing", d.Project, d.Index)}
		}
	}
	for d := range got {
		if !want[d] {
			return &Violation{ID: 7, Name: "viewer-set", Message: fmt.Sprintf("stale viewer for %s/%d", d.Project, d.Index)}
		}
	}
	return nil
}

func Check8ViewerOrder(state w.WorldState) *Violation {
	viewerWS := state.Environment.Workspaces.Viewer
	if viewerWS == "" {
		return nil
	}
	prof, ok := state.Desired.Profiles[state.Desired.ActiveProfile]
	if !ok {
		return nil
	}
	expected := []w.DesiredWindowID{}
	titles := map[w.DesiredWindowID]string{}
	for _, sid := range state.Environment.SlotOrder() {
		pid, ok := prof.Assignments[sid]
		if !ok {
			continue
		}
		pr, ok := state.Desired.Projects[pid]
		if !ok || pr.Archived {
			continue
		}
		ais := []w.DesiredWindow{}
		for _, dw := range pr.Windows {
			if dw.ID.Kind == w.WindowAI {
				ais = append(ais, dw)
			}
		}
		sort.Slice(ais, func(i, j int) bool { return ais[i].ID.Index < ais[j].ID.Index })
		for _, a := range ais {
			expected = append(expected, a.ID)
			titles[a.ID] = viewerTitleForAI(a.TitleContract.Expected)
		}
	}
	layout := state.Observed.Layouts[viewerWS]
	got := []w.DesiredWindowID{}
	for _, c := range layout.Columns {
		for _, lid := range c.Windows {
			ow := state.Observed.Windows[lid]
			if matched, ok := matchViewerDesired(ow, expected, titles); ok {
				got = append(got, matched)
			}
		}
	}
	if len(got) != len(expected) {
		return &Violation{ID: 8, Name: "viewer-order", Message: fmt.Sprintf("viewer count %d != expected %d", len(got), len(expected))}
	}
	for i := range expected {
		if got[i] != expected[i] {
			return &Violation{ID: 8, Name: "viewer-order", Message: fmt.Sprintf("viewer at pos %d is %s/%d, want %s/%d", i, got[i].Project, got[i].Index, expected[i].Project, expected[i].Index)}
		}
	}
	return nil
}

func matchViewerDesired(ow w.ObservedWindow, desired []w.DesiredWindowID, titles map[w.DesiredWindowID]string) (w.DesiredWindowID, bool) {
	if ow.MatchedTo != nil {
		return *ow.MatchedTo, true
	}
	for _, d := range desired {
		if ow.Title.Value == titles[d] {
			return d, true
		}
	}
	return w.DesiredWindowID{}, false
}

// Check9LayoutSemantics verifies INV-12 viewer/project layout alignment.
// SSOT N-12: the `AllowManualLayoutCandidates` short-circuit was removed.
// AcceptedLayouts (populated by AutoSyncLayout) is the sole authority.
func Check9LayoutSemantics(state w.WorldState) *Violation {
	prof, ok := state.Desired.Profiles[state.Desired.ActiveProfile]
	if !ok {
		return nil
	}
	for _, sid := range state.Environment.SlotOrder() {
		pid, ok := prof.Assignments[sid]
		if !ok {
			continue
		}
		pr, ok := state.Desired.Projects[pid]
		if !ok || pr.Archived {
			continue
		}
		slot, _ := state.Environment.SlotByID(sid)
		ws := slot.Workspace
		want := desiredColumns(pr, ws, state.Desired.AcceptedLayouts)
		obs := state.Observed.Layouts[ws]
		if !semanticEq(obs.Columns, want, pr, ws, state.Observed, state.Meta.WindowProvenance) {
			return &Violation{ID: 9, Name: "layout-semantics",
				Message: fmt.Sprintf("project %q layout on %q does not match desired", pid, ws)}
		}
	}
	return nil
}

func Check10FinalFocus(state w.WorldState, command string) *Violation {
	want, ok := state.Desired.FocusPolicy.FinalFocus[command]
	if !ok || want == "" {
		return nil
	}
	if state.Observed.Focus.Workspace != want {
		return &Violation{ID: 10, Name: "final-focus",
			Message: fmt.Sprintf("focus on %q, want %q (command %q)", state.Observed.Focus.Workspace, want, command)}
	}
	return nil
}

func Check11WorkspaceRoleSegregation(state w.WorldState) *Violation {
	for _, ow := range state.Observed.Windows {
		if ow.MatchedTo == nil {
			continue
		}
		ws, ok := state.Environment.WorkspaceByID(ow.Workspace)
		if !ok {
			continue
		}
		// managed project window must be on WorkspaceProject (or, for viewers, WorkspaceViewer).
		if ow.Kind == w.WindowViewer {
			if ws.Role != w.WorkspaceViewer {
				return &Violation{ID: 11, Name: "role-segregation", Message: fmt.Sprintf("viewer %s on non-viewer workspace %q", ow.ID, ow.Workspace)}
			}
			continue
		}
		if ws.Role != w.WorkspaceProject {
			return &Violation{ID: 11, Name: "role-segregation", Message: fmt.Sprintf("managed window %s on workspace %q with role %q (need WorkspaceProject)", ow.ID, ow.Workspace, ws.Role)}
		}
	}
	return nil
}

func Check12TitleDrift(state w.WorldState) *Violation {
	for _, ow := range state.Observed.Windows {
		if ow.MatchedTo == nil {
			continue
		}
		pr, ok := state.Desired.Projects[ow.MatchedTo.Project]
		if !ok {
			continue
		}
		var dw *w.DesiredWindow
		for i := range pr.Windows {
			if pr.Windows[i].ID == *ow.MatchedTo {
				dw = &pr.Windows[i]
				break
			}
		}
		if dw == nil {
			continue
		}
		c := dw.TitleContract
		if c.Authority != w.TitleControllerOwned {
			continue
		}
		if c.Drift == w.TitleDriftRepair {
			expected := c.Expected
			if ow.Kind == w.WindowViewer {
				expected = viewerTitleForAI(expected)
			}
			if ow.Title.Value != expected {
				return &Violation{ID: 12, Name: "title-drift", Message: fmt.Sprintf("window %s title=%q, expected %q", ow.ID, ow.Title.Value, expected)}
			}
		}
	}
	return nil
}

func viewerTitleForAI(aiTitle string) string {
	if strings.HasPrefix(aiTitle, "ai-") {
		return "ai-view-" + strings.TrimPrefix(aiTitle, "ai-")
	}
	return "ai-view-" + aiTitle
}

func Check13NoUnprocessedDirty(state w.WorldState) *Violation {
	if len(state.Meta.DirtyScopes) > 0 {
		return &Violation{ID: 13, Name: "no-unprocessed-dirty", Message: fmt.Sprintf("%d unprocessed DirtyScopes remain", len(state.Meta.DirtyScopes))}
	}
	return nil
}

// helpers

func sortedSlotKeys(m map[w.SlotID]w.ProjectID) []w.SlotID {
	out := make([]w.SlotID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sortedProjectKeys(m map[w.ProjectID]w.DesiredProject) []w.ProjectID {
	out := make([]w.ProjectID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func desiredColumns(pr w.DesiredProject, ws w.WorkspaceID, accepted map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout) []w.DesiredColumn {
	if accepted != nil {
		if m, ok := accepted[pr.ID]; ok {
			if l, ok2 := m[ws]; ok2 {
				return l.Columns
			}
		}
	}
	if l, ok := pr.Layouts[ws]; ok {
		return l.Columns
	}
	all := append([]w.DesiredWindow(nil), pr.Windows...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].ID.Kind != all[j].ID.Kind {
			return all[i].ID.Kind < all[j].ID.Kind
		}
		return all[i].ID.Index < all[j].ID.Index
	})
	out := make([]w.DesiredColumn, 0, len(all))
	for _, dw := range all {
		out = append(out, w.DesiredColumn{Windows: []w.DesiredWindowID{dw.ID}, Mode: w.ColumnSolo})
	}
	return out
}

func semanticEq(obs []w.ObservedColumn, want []w.DesiredColumn, pr w.DesiredProject, ws w.WorkspaceID, world w.ObservedWorld, provenance map[w.DesiredWindowID]w.LiveWindowID) bool {
	// Resolve desired columns to their live window IDs (the managed set).
	wantLiveCols := make([][]w.LiveWindowID, 0, len(want))
	managed := map[w.LiveWindowID]bool{}
	for _, col := range want {
		live := make([]w.LiveWindowID, 0, len(col.Windows))
		for _, dwid := range col.Windows {
			dw := findDesiredWindow(pr, dwid)
			if dw == nil {
				return false
			}
			res := identity.ResolveWithOptions(*dw, world, identity.ResolveOptions{ExpectedWorkspace: ws, Provenance: provenance})
			if res.Class != identity.ClassUniqueStrong {
				return false
			}
			live = append(live, res.Live)
			managed[res.Live] = true
		}
		wantLiveCols = append(wantLiveCols, live)
	}
	// SSOT §6.3 L3 / §4.3: external (unmanaged) windows that have drifted into
	// the slot are NOT part of DesiredLayout.Columns. Filter them out of the
	// observed columns (dropping any column that becomes empty) before
	// comparing, so a drifted external window does not raise a false
	// layout-semantics violation. Mirrors planner.managedObservedColumns /
	// wm.managedOrderSettled.
	filtered := make([][]w.LiveWindowID, 0, len(obs))
	for _, col := range obs {
		kept := make([]w.LiveWindowID, 0, len(col.Windows))
		for _, id := range col.Windows {
			if managed[id] {
				kept = append(kept, id)
			}
		}
		if len(kept) > 0 {
			filtered = append(filtered, kept)
		}
	}
	if len(filtered) != len(wantLiveCols) {
		return false
	}
	for i := range filtered {
		if len(filtered[i]) != len(wantLiveCols[i]) {
			return false
		}
		seen := map[w.LiveWindowID]int{}
		for _, id := range filtered[i] {
			seen[id]++
		}
		for _, id := range wantLiveCols[i] {
			seen[id]--
		}
		for _, n := range seen {
			if n != 0 {
				return false
			}
		}
	}
	return true
}

func findDesiredWindow(pr w.DesiredProject, id w.DesiredWindowID) *w.DesiredWindow {
	for i := range pr.Windows {
		if pr.Windows[i].ID == id {
			return &pr.Windows[i]
		}
	}
	return nil
}
