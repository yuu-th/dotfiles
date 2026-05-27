// Package reducer computes new DesiredWorld from (WorldState, Intent) and Reaction from (WorldState, Event).
// Reducer is a pure function. design.md §13. specs §5.
package reducer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// ReduceIntent applies a user intent to DesiredWorld. Pure.
// Returns a deep-copied new DesiredWorld; never mutates the input.
func ReduceIntent(state w.WorldState, in intent.Intent) (w.DesiredWorld, error) {
	d := cloneDesired(state.Desired)
	switch v := in.(type) {
	case intent.SwitchProfile:
		if _, ok := d.Profiles[v.To]; !ok {
			return d, fmt.Errorf("reducer: switch-profile to unknown profile %q", v.To)
		}
		d.ActiveProfile = v.To
	case intent.ArchiveProject:
		p, ok := d.Projects[v.Project]
		if !ok {
			return d, fmt.Errorf("reducer: archive-project unknown %q", v.Project)
		}
		p.Archived = true
		d.Projects[v.Project] = p
		// Remove from any profile assignments.
		for pid, prof := range d.Profiles {
			newAssign := map[w.SlotID]w.ProjectID{}
			for slot, proj := range prof.Assignments {
				if proj != v.Project {
					newAssign[slot] = proj
				}
			}
			prof.Assignments = newAssign
			d.Profiles[pid] = prof
		}
	case intent.UnarchiveProject:
		// SSOT §4.5: unarchive returns the project to park state. The
		// reducer only clears the Archived flag; slot assignment is a
		// separate explicit step via AssignProject.
		p, ok := d.Projects[v.Project]
		if !ok {
			return d, fmt.Errorf("reducer: unarchive-project unknown %q", v.Project)
		}
		p.Archived = false
		d.Projects[v.Project] = p
	case intent.AssignProject:
		if _, ok := d.Projects[v.Project]; !ok {
			return d, fmt.Errorf("reducer: assign: unknown project %q", v.Project)
		}
		prof, ok := d.Profiles[d.ActiveProfile]
		if !ok {
			return d, fmt.Errorf("reducer: assign: no active profile")
		}
		if prof.Assignments == nil {
			prof.Assignments = map[w.SlotID]w.ProjectID{}
		}
		prof.Assignments[v.Slot] = v.Project
		d.Profiles[d.ActiveProfile] = prof
	case intent.UnassignSlot:
		prof, ok := d.Profiles[d.ActiveProfile]
		if !ok {
			return d, fmt.Errorf("reducer: unassign: no active profile")
		}
		newAssign := map[w.SlotID]w.ProjectID{}
		for slot, proj := range prof.Assignments {
			if slot != v.Slot {
				newAssign[slot] = proj
			}
		}
		prof.Assignments = newAssign
		d.Profiles[d.ActiveProfile] = prof
	case intent.Reconcile:
		// no DesiredWorld change.
	case intent.ValidateEnvironment:
		// no DesiredWorld change.

	// ---------------------------------------------------------------------
	// v2.3 / design v3 additions.
	// ---------------------------------------------------------------------

	case intent.CreateProject:
		if v.ID == "" {
			return d, fmt.Errorf("reducer: create-project: empty project ID")
		}
		if _, exists := d.Projects[v.ID]; exists {
			return d, fmt.Errorf("reducer: create-project: %q already exists", v.ID)
		}
		wins := append([]w.DesiredWindow(nil), v.Windows...)
		if len(wins) == 0 {
			wins = DefaultProjectWindows(v.ID, "claude")
		}
		// Each window's project ID must match v.ID. If a caller passes a
		// window with a mismatched DesiredWindowID.Project (e.g. copy-paste),
		// rewrite it to keep invariants.
		for i := range wins {
			wins[i].ID.Project = v.ID
		}
		d.Projects[v.ID] = w.DesiredProject{
			ID:      v.ID,
			Root:    v.Path,
			Windows: wins,
			Layouts: map[w.WorkspaceID]w.DesiredLayout{},
		}

	case intent.DeleteProject:
		if _, exists := d.Projects[v.ID]; !exists {
			return d, fmt.Errorf("reducer: delete-project: unknown %q", v.ID)
		}
		delete(d.Projects, v.ID)
		// Drop slot assignments referencing this project, in every profile.
		for pid, prof := range d.Profiles {
			newAssign := map[w.SlotID]w.ProjectID{}
			for slot, proj := range prof.Assignments {
				if proj != v.ID {
					newAssign[slot] = proj
				}
			}
			prof.Assignments = newAssign
			d.Profiles[pid] = prof
		}
		if d.AcceptedLayouts != nil {
			delete(d.AcceptedLayouts, v.ID)
		}

	case intent.AddWindow:
		pr, ok := d.Projects[v.Project]
		if !ok {
			return d, fmt.Errorf("reducer: add-window: unknown project %q", v.Project)
		}
		idx := v.Index
		if idx <= 0 {
			idx = nextWindowIndex(pr.Windows, v.WindowKind)
		}
		if windowExists(pr.Windows, v.WindowKind, idx) {
			return d, fmt.Errorf("reducer: add-window: %s-%d already exists in project %s", v.WindowKind, idx, v.Project)
		}
		newWin := defaultWindowForKind(v.Project, v.WindowKind, idx, v.AIName)
		pr.Windows = append(pr.Windows, newWin)
		d.Projects[v.Project] = pr

	case intent.RemoveWindow:
		pr, ok := d.Projects[v.Project]
		if !ok {
			return d, fmt.Errorf("reducer: remove-window: unknown project %q", v.Project)
		}
		idx := -1
		for i, win := range pr.Windows {
			if win.ID == v.WindowID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return d, fmt.Errorf("reducer: remove-window: %v not in project %s", v.WindowID, v.Project)
		}
		pr.Windows = append(pr.Windows[:idx], pr.Windows[idx+1:]...)
		d.Projects[v.Project] = pr

	case intent.CreateProfile:
		if v.ID == "" {
			return d, fmt.Errorf("reducer: create-profile: empty profile ID")
		}
		if _, exists := d.Profiles[v.ID]; exists {
			return d, fmt.Errorf("reducer: create-profile: %q already exists", v.ID)
		}
		policy := v.InactivePolicy
		if policy == "" {
			policy = w.InactivePolicyRemove
		}
		d.Profiles[v.ID] = w.DesiredProfile{
			ID:             v.ID,
			Description:    v.Description,
			InactivePolicy: policy,
			Assignments:    map[w.SlotID]w.ProjectID{},
		}

	case intent.DeleteProfile:
		if _, exists := d.Profiles[v.ID]; !exists {
			return d, fmt.Errorf("reducer: delete-profile: unknown %q", v.ID)
		}
		if v.ID == d.ActiveProfile {
			return d, fmt.Errorf("reducer: delete-profile: cannot delete active profile %q", v.ID)
		}
		delete(d.Profiles, v.ID)

	case intent.RenameProfile:
		if v.Old == v.New {
			return d, fmt.Errorf("reducer: rename-profile: source and target equal")
		}
		prof, exists := d.Profiles[v.Old]
		if !exists {
			return d, fmt.Errorf("reducer: rename-profile: unknown %q", v.Old)
		}
		if _, taken := d.Profiles[v.New]; taken {
			return d, fmt.Errorf("reducer: rename-profile: target %q already exists", v.New)
		}
		prof.ID = v.New
		delete(d.Profiles, v.Old)
		d.Profiles[v.New] = prof
		if d.ActiveProfile == v.Old {
			d.ActiveProfile = v.New
		}

	case intent.AdoptOrphanWindow:
		// Adoption appends a DesiredWindow under the target project so the
		// next reconcile rematches the existing live window. The live ID
		// itself is not stored in DesiredWorld; identity resolver matches
		// via title-prefix / bundle-id like any other window.
		pr, ok := d.Projects[v.AsProject]
		if !ok {
			return d, fmt.Errorf("reducer: adopt-orphan: unknown project %q", v.AsProject)
		}
		idx := nextWindowIndex(pr.Windows, v.AsWindowKind)
		newWin := defaultWindowForKind(v.AsProject, v.AsWindowKind, idx, "")
		pr.Windows = append(pr.Windows, newWin)
		d.Projects[v.AsProject] = pr

	case intent.DismissOrphanWindow:
		// No DesiredWorld mutation; the controller's card subsystem
		// translates this into an AX-close operation in Phase 4.
		_ = v

	case intent.AutoSyncLayout:
		// Internal intent emitted by the controller after Tier 2 detection.
		if d.AcceptedLayouts == nil {
			d.AcceptedLayouts = map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout{}
		}
		if d.AcceptedLayouts[v.Project] == nil {
			d.AcceptedLayouts[v.Project] = map[w.WorkspaceID]w.DesiredLayout{}
		}
		d.AcceptedLayouts[v.Project][v.Workspace] = w.DesiredLayout{
			Workspace: v.Workspace,
			Columns:   append([]w.DesiredColumn(nil), v.Columns...),
			Source:    w.LayoutAuthorityAcceptedManual,
		}

	case intent.ReconstructFromObserved:
		// SSOT §3.5 case B/D / INV-10: re-register managed windows observed
		// at startup whose identity is absent from DesiredWorld.
		reconstructFromObserved(&d, state)

	case intent.DismissCard, intent.DismissAllCards:
		// Pure ControllerMeta mutation — applied at the controller level
		// after this returns. Reducer leaves DesiredWorld untouched.
		_ = v

	case intent.SetCockpitVisibility:
		// Flip Visibility uniformly on every cockpit SystemWindow.
		// "uniform" matches requirements §8.2 全モニタ同期 show/hide.
		//
		// PriorWorkspace/PriorWindow policy (SSOT §5.4 Proposal mode + §7.5
		// HideCockpitOnDisplay): refresh from observed ONLY when the display
		// is currently on a non-park workspace (= we can see the user's
		// natural workspace + focused window). When observed already equals
		// ParkWorkspace, preserve existing PriorWorkspace/PriorWindow so the
		// hide path still has a focus restoration target.
		for i := range d.SystemWindows {
			if d.SystemWindows[i].Kind != w.WindowCockpit {
				continue
			}
			refreshed := priorWorkspaceForParkWs(d.SystemWindows[i].ParkWorkspace, d.SystemWindows[i].DisplayIdx, state.Observed)
			if refreshed != "" && refreshed != d.SystemWindows[i].ParkWorkspace {
				d.SystemWindows[i].PriorWorkspace = refreshed
				if win := state.Observed.Focus.Window; win != "" {
					d.SystemWindows[i].PriorWindow = win
				}
			}
			d.SystemWindows[i].Visibility = v.Visibility
		}

	case intent.SyncCockpitSystemWindows:
		// Requirements v2.4 §8.1: always produce exactly ONE cockpit SystemWindow
		// on the projwm-managed monitor (workspace A / Q-P monitor), regardless
		// of how many physical displays are connected. DisplayCount>1 is treated
		// as 1; CP2-CP6 are abolished.
		//
		// Exception: DisplayCount==0 means no physical display is available —
		// produce an empty slice (cannot spawn a cockpit with no display).
		// This matches the old behaviour for DisplayCount=0 and keeps
		// external-event no-mutation invariants intact in environments without
		// displays (e.g. the fake backend in S8.E tests).
		//
		// Preserves Visibility/PriorWorkspace on the surviving D0 entry if it
		// already exists; initialises to hidden (§8.2 "平時は隠れている") if not.
		if v.DisplayCount == 0 {
			d.SystemWindows = nil
			break
		}

		// SSOT §7.3 naming convention: cockpit title is
		// "projwm-cockpit-<display>" with the bare display index (no "D"
		// prefix). cockpit lives on the projwm-managed display only,
		// indexed 0, parked on CP1 (SSOT §2.4 / INV-06).
		const cockpitDisplayIdx = 0
		const cockpitTitle = "projwm-cockpit-0"
		const cockpitParkWs = w.WorkspaceID("CP1")

		// Look for an existing D0 entry to preserve.
		var existing w.SystemWindow
		haveExisting := false
		for _, sw := range d.SystemWindows {
			if sw.Kind == w.WindowCockpit && sw.DisplayIdx == cockpitDisplayIdx {
				existing = sw
				haveExisting = true
				break
			}
		}

		var entry w.SystemWindow
		if !haveExisting {
			prior := priorWorkspaceForParkWs(cockpitParkWs, cockpitDisplayIdx, state.Observed)
			entry = w.SystemWindow{
				ID:             w.SystemWindowID{Kind: w.WindowCockpit, Index: cockpitDisplayIdx},
				Kind:           w.WindowCockpit,
				DisplayIdx:     cockpitDisplayIdx,
				Title:          cockpitTitle,
				ParkWorkspace:  cockpitParkWs,
				Visibility:     w.CockpitHidden,
				PriorWorkspace: prior,
			}
		} else {
			entry = existing
			// Ensure ParkWorkspace is always CP1 (migration from older multi-CP state).
			entry.ParkWorkspace = cockpitParkWs
			// Ensure Title is canonical.
			entry.Title = cockpitTitle
			// Refresh PriorWorkspace from observed if the display is not already on CP1.
			refreshed := priorWorkspaceForParkWs(cockpitParkWs, cockpitDisplayIdx, state.Observed)
			if refreshed != "" {
				entry.PriorWorkspace = refreshed
			}
		}
		d.SystemWindows = []w.SystemWindow{entry}

	case intent.ShowScratchShell:
		// SSOT §4.1 OP11: scratch shell を表示。冪等。
		// scratch SystemWindow (kind=WindowScratch, Index=0) を必ず単一存在
		// させ Visibility=Shown に設定。表示直前の focused window を PriorWindow
		// に capture することで Hide で復帰できる (Hidden→Shown のときだけ capture、
		// 既に Shown のときは上書きしないことで scratch 自身を prior にしない)。
		idx := -1
		for i := range d.SystemWindows {
			if d.SystemWindows[i].Kind == w.WindowScratch {
				idx = i
				break
			}
		}
		wasShown := false
		if idx < 0 {
			// 初期化: Hidden 状態で append し、下の遷移ロジックに任せる。
			d.SystemWindows = append(d.SystemWindows, w.SystemWindow{
				ID:         w.SystemWindowID{Kind: w.WindowScratch, Index: 0},
				Kind:       w.WindowScratch,
				DisplayIdx: 0,
				Title:      "projwm-scratch-shell",
				Visibility: w.CockpitHidden,
			})
			idx = len(d.SystemWindows) - 1
		} else {
			wasShown = d.SystemWindows[idx].Visibility == w.CockpitShown
		}
		if !wasShown {
			// Hidden→Shown 遷移のときのみ PriorWindow を取り直す。
			// 既に Shown のときは scratch 自身が focus を持っている可能性が
			// あるので上書きしない。
			if win := state.Observed.Focus.Window; win != "" {
				d.SystemWindows[idx].PriorWindow = win
			}
			d.SystemWindows[idx].Visibility = w.CockpitShown
		}

	case intent.SwitchProject:
		// SSOT §4.1 OP04: target slot の workspace に切替。reducer は
		// DesiredWorld を変えない — planner が commandKey から target
		// workspace を resolve して focus-workspace op を emit する
		// (omniwm の per-workspace MRU が focus 復帰を担当)。
		_ = v

	case intent.CycleSlotWindow:
		// SSOT §4.1 OP05: 同じ slot 内の (kind) window に focus 切替。
		// workspace は変えない (planner が focus-window のみ emit)。
		_ = v

	case intent.BrowserAddTab:
		// SSOT §4.1 OP14 + §4.4 BR-PRIV-NOSTORE: browser window に URL タブを追加。
		// reducer は純粋関数なので I/O を持たない: URL → opaque token への
		// 変換 (PrivatePayloadStore.Put) は controller.prepareBrowserIntent で
		// 先行実施され、v.URL は controller 経由なら token、CLI / test が
		// payloadStore=nil で呼ぶ場合は literal URL 文字列が入る。reducer は
		// どちらでも同じ動作 (URLPayloadRefs に append) — SSOT §6.4 に従い
		// DesiredWorld への書き込みは reducer 専権、payload 実体は store 側。
		if err := mutateBrowserSession(&d, v.Project, v.WindowID, func(b *w.DesiredBrowserSession) error {
			b.URLPayloadRefs = append(b.URLPayloadRefs, w.PrivatePayloadRef(v.URL))
			b.URLCount = len(b.URLPayloadRefs)
			return nil
		}); err != nil {
			return d, err
		}

	case intent.BrowserRemoveTab:
		// SSOT §4.1 OP15: tab index (1-based) を削除。最後の tab 削除時は
		// browser window 自体を close (この方針は planner / observer 側で
		// fire — reducer は URLPayloadRefs と URLCount のみ更新)。
		if err := mutateBrowserSession(&d, v.Project, v.WindowID, func(b *w.DesiredBrowserSession) error {
			if v.Tab < 1 || v.Tab > len(b.URLPayloadRefs) {
				return fmt.Errorf("reducer: browser-remove-tab: tab %d out of range (1..%d)", v.Tab, len(b.URLPayloadRefs))
			}
			b.URLPayloadRefs = append(b.URLPayloadRefs[:v.Tab-1], b.URLPayloadRefs[v.Tab:]...)
			b.URLCount = len(b.URLPayloadRefs)
			return nil
		}); err != nil {
			return d, err
		}

	case intent.BrowserChangeTabURL:
		// SSOT §4.1 OP16: tab index の URL を URL に差し替え。
		if err := mutateBrowserSession(&d, v.Project, v.WindowID, func(b *w.DesiredBrowserSession) error {
			if v.Tab < 1 || v.Tab > len(b.URLPayloadRefs) {
				return fmt.Errorf("reducer: browser-change-tab-url: tab %d out of range (1..%d)", v.Tab, len(b.URLPayloadRefs))
			}
			b.URLPayloadRefs[v.Tab-1] = w.PrivatePayloadRef(v.URL)
			return nil
		}); err != nil {
			return d, err
		}

	case intent.BrowserReorderTabs:
		// SSOT §4.1 OP17: tab を From → To に移動 (1-based)。
		if err := mutateBrowserSession(&d, v.Project, v.WindowID, func(b *w.DesiredBrowserSession) error {
			n := len(b.URLPayloadRefs)
			if v.From < 1 || v.From > n || v.To < 1 || v.To > n {
				return fmt.Errorf("reducer: browser-reorder-tabs: from=%d to=%d out of range (1..%d)", v.From, v.To, n)
			}
			if v.From == v.To {
				return nil
			}
			ref := b.URLPayloadRefs[v.From-1]
			b.URLPayloadRefs = append(b.URLPayloadRefs[:v.From-1], b.URLPayloadRefs[v.From:]...)
			// Re-insert at To-1 (after the From removal, indices shift)
			insertAt := v.To - 1
			if v.To > v.From {
				// 削除で 1 つ前にずれているので調整不要 (v.To-1 そのまま)
				_ = insertAt
			}
			b.URLPayloadRefs = append(b.URLPayloadRefs[:insertAt], append([]w.PrivatePayloadRef{ref}, b.URLPayloadRefs[insertAt:]...)...)
			return nil
		}); err != nil {
			return d, err
		}

	case intent.SummonShell, intent.SummonEditor, intent.SummonBrowser:
		// SSOT §4.1 OP01-03: slot 内の (shell/editor/browser) を summon。
		// reducer は DesiredWorld を変えない — observed.Focus を見て cycle
		// する transient な focus 操作なので planner 側で resolve する
		// (commandKey="intent:summon-shell:<slot>" 等で dispatch)。
		_ = v

	case intent.SummonViewer:
		// SSOT §4.1 OP06: viewer に jump。reducer は DesiredWorld を変えない
		// — 「どの viewer を focus するか」は observed.Focus に依存する
		// transient な決定なので planner 側で resolve する (commandKey=
		// "intent:summon-viewer" の branch でハンドル)。
		// 直前にフォーカスしていた window の identity を見て: AI なら同じ
		// (project, index) の viewer、それ以外なら slot order の最初の viewer。
		_ = v

	case intent.HideScratchShell:
		// SSOT §4.1 OP11: scratch を隠す。Visibility=Hidden。PriorWindow は
		// 残しておく (planner が hide op の Target.LiveWindow として消費する)。
		for i := range d.SystemWindows {
			if d.SystemWindows[i].Kind == w.WindowScratch {
				d.SystemWindows[i].Visibility = w.CockpitHidden
				break
			}
		}

	default:
		return d, fmt.Errorf("reducer: unknown intent %T", in)
	}
	return d, nil
}

// mutateBrowserSession locates the target browser window inside DesiredWorld
// and applies the mutator to its DesiredBrowserSession (allocated on demand).
// Errors out if the project / window cannot be located or kind ≠ browser.
func mutateBrowserSession(d *w.DesiredWorld, project w.ProjectID, windowID w.DesiredWindowID, mutate func(*w.DesiredBrowserSession) error) error {
	pr, ok := d.Projects[project]
	if !ok {
		return fmt.Errorf("reducer: browser-tab: unknown project %q", project)
	}
	idx := -1
	for i, win := range pr.Windows {
		if win.ID == windowID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("reducer: browser-tab: window %v not in project %s", windowID, project)
	}
	if pr.Windows[idx].Kind != w.WindowBrowser {
		return fmt.Errorf("reducer: browser-tab: window %v is %s, not browser", windowID, pr.Windows[idx].Kind)
	}
	if pr.Windows[idx].Browser == nil {
		pr.Windows[idx].Browser = &w.DesiredBrowserSession{}
	}
	if err := mutate(pr.Windows[idx].Browser); err != nil {
		return err
	}
	d.Projects[project] = pr
	return nil
}

// nextWindowIndex returns the next unused 1-based ordinal for kind in wins.
func nextWindowIndex(wins []w.DesiredWindow, kind w.WindowKind) int {
	max := 0
	for _, dw := range wins {
		if dw.Kind != kind {
			continue
		}
		if dw.ID.Index > max {
			max = dw.ID.Index
		}
	}
	return max + 1
}

func windowExists(wins []w.DesiredWindow, kind w.WindowKind, idx int) bool {
	for _, dw := range wins {
		if dw.Kind == kind && dw.ID.Index == idx {
			return true
		}
	}
	return false
}

// parseManagedTitle parses a projwm controller-owned Ghostty title of the form
// "<kind>-<id>:<project>" (SSOT §7.3) into its identity components. Returns
// ok=false for titles that do not match the managed convention (e.g. a Zed
// basename title or an arbitrary user window). Viewer titles
// ("ai-view-<id>:<project>") parse to Kind=viewer.
func parseManagedTitle(title string) (w.WindowKind, int, w.ProjectID, bool) {
	colon := strings.IndexByte(title, ':')
	if colon <= 0 {
		return "", 0, "", false
	}
	prefix, project := title[:colon], title[colon+1:]
	if project == "" {
		return "", 0, "", false
	}
	var kindStr, idxStr string
	if strings.HasPrefix(prefix, "ai-view-") {
		kindStr, idxStr = "ai-view", strings.TrimPrefix(prefix, "ai-view-")
	} else {
		dash := strings.LastIndexByte(prefix, '-')
		if dash <= 0 || dash == len(prefix)-1 {
			return "", 0, "", false
		}
		kindStr, idxStr = prefix[:dash], prefix[dash+1:]
	}
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 1 {
		return "", 0, "", false
	}
	switch kindStr {
	case "ai":
		return w.WindowAI, idx, w.ProjectID(project), true
	case "shell":
		return w.WindowShell, idx, w.ProjectID(project), true
	case "browser":
		return w.WindowBrowser, idx, w.ProjectID(project), true
	case "ai-view":
		return w.WindowViewer, idx, w.ProjectID(project), true
	default:
		return "", 0, "", false
	}
}

// reconstructFromObserved implements SSOT §3.5 case B/D / INV-10: re-register
// managed windows from the observed world when their identity is absent from
// DesiredWorld (state lost / corrupted with no backup, or an orphan with a
// parseable title at startup). For each observed window whose controller-owned
// title parses to a managed identity not already desired, the project + window
// are recreated and the project is assigned to the slot whose workspace the
// window currently occupies (if that slot is free in the active profile).
// Viewer windows are skipped — they are derived mirrors of AI windows, so the
// AI window drives reconstruction. Unparseable titles are left for the
// orphan-card path (PromoteOrphans).
func reconstructFromObserved(d *w.DesiredWorld, state w.WorldState) {
	for _, ow := range state.Observed.Windows {
		kind, idx, pid, ok := parseManagedTitle(ow.Title.Value)
		if !ok || kind == w.WindowViewer {
			continue
		}
		pr, exists := d.Projects[pid]
		if exists && windowExists(pr.Windows, kind, idx) {
			continue // identity already desired — nothing to reconstruct
		}
		if !exists {
			pr = w.DesiredProject{ID: pid}
		}
		pr.Windows = append(pr.Windows, defaultWindowForKind(pid, kind, idx, ""))
		if d.Projects == nil {
			d.Projects = map[w.ProjectID]w.DesiredProject{}
		}
		d.Projects[pid] = pr

		// Assign to the slot whose workspace this window occupies, in the
		// active profile, when that slot is currently unassigned.
		prof, ok := d.Profiles[d.ActiveProfile]
		if !ok {
			continue
		}
		for _, slot := range state.Environment.Workspaces.Slots {
			if slot.Workspace != ow.Workspace {
				continue
			}
			if prof.Assignments == nil {
				prof.Assignments = map[w.SlotID]w.ProjectID{}
			}
			if _, taken := prof.Assignments[slot.ID]; !taken {
				prof.Assignments[slot.ID] = pid
				d.Profiles[d.ActiveProfile] = prof
			}
			break
		}
	}
}

// DefaultProjectWindows returns the canonical (ai-1 + shell-1 + editor-1)
// initial DesiredWindow set, used when intent.CreateProject is submitted
// without explicit Windows.
func DefaultProjectWindows(pid w.ProjectID, aiName string) []w.DesiredWindow {
	if aiName == "" {
		aiName = "claude"
	}
	return []w.DesiredWindow{
		defaultWindowForKind(pid, w.WindowAI, 1, aiName),
		defaultWindowForKind(pid, w.WindowShell, 1, ""),
		defaultWindowForKind(pid, w.WindowEditor, 1, ""),
	}
}

// defaultWindowForKind constructs a DesiredWindow with sensible
// TitleContract defaults for the given kind. Wrappers and the planner
// rely on the resulting MatchHints / TitleContract.Prefix to match live
// windows back to desired state.
func defaultWindowForKind(pid w.ProjectID, kind w.WindowKind, idx int, aiName string) w.DesiredWindow {
	dw := w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: pid, Kind: kind, Index: idx},
		Kind: kind,
	}
	switch kind {
	case w.WindowAI:
		// SSOT §7.3: AI title is "ai-<id>:<project>" — the AI name
		// (claude/copilot/etc.) is metadata on the spawn, not part of
		// the identity title. SSOT §4.4 ai: aiName is persisted on the
		// DesiredAISession so spawn-time tmux send-keys can route the
		// right launch command.
		dw.TitleContract = w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  fmt.Sprintf("ai-%d:%s", idx, pid),
			Drift:     w.TitleDriftRepair,
		}
		dw.App = w.AppRequirement{BundleID: "com.mitchellh.ghostty"}
		if aiName != "" {
			dw.AI = &w.DesiredAISession{Name: aiName}
		}
	case w.WindowShell:
		dw.TitleContract = w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  fmt.Sprintf("shell-%d:%s", idx, pid),
			Drift:     w.TitleDriftRepair,
		}
		dw.App = w.AppRequirement{BundleID: "com.mitchellh.ghostty"}
	case w.WindowEditor:
		// Zed sets its window title to filepath.Base(cwd) (see
		// naming.ZedTitle). Without Expected, identity.Resolve cannot
		// disambiguate a Zed window for project "dotfiles" from any
		// other Zed window on the desktop — it returns ClassMissing
		// and the planner re-spawns forever, with each new window
		// landing on whichever workspace omniwm happened to focus.
		// Setting Expected=<project id> (= the basename Zed uses)
		// gives the resolver a strong disambiguator.
		dw.TitleContract = w.TitleContract{
			Authority: w.TitleAppOwned,
			Expected:  string(pid),
			Drift:     w.TitleDriftObserveOnly,
		}
		dw.App = w.AppRequirement{BundleID: "dev.zed.Zed"}
	case w.WindowBrowser:
		dw.TitleContract = w.TitleContract{
			Authority: w.TitleAppOwned,
			Drift:     w.TitleDriftObserveOnly,
		}
		dw.App = w.AppRequirement{BundleID: "com.vivaldi.Vivaldi"}
		dw.Browser = &w.DesiredBrowserSession{
			PrivacyMode: w.BrowserSnapshotPrivateContent,
			RestoreURLs: true,
		}
	case w.WindowViewer:
		// SSOT §7.3: viewer title is "ai-view-<id>:<project>" — no AI name.
		// Viewer mirrors its source AI session, so it inherits AIName for
		// downstream code that wants to display "what AI is being mirrored".
		dw.TitleContract = w.TitleContract{
			Authority: w.TitleControllerOwned,
			Expected:  fmt.Sprintf("ai-view-%d:%s", idx, pid),
			Drift:     w.TitleDriftRepair,
		}
		dw.App = w.AppRequirement{BundleID: "com.mitchellh.ghostty"}
		if aiName != "" {
			dw.AI = &w.DesiredAISession{Name: aiName}
		}
	}
	return dw
}

// ReactToEvent translates an external event into DirtyScopes / ObserveScopes /
// Lifecycle / NewCards / UserCloseRecords. MUST NOT modify DesiredWorld.
// design.md §13, specs §2-D, §2-E, §4. SSOT N-12: no longer emits
// ManualLayoutCandidate (Tier 2 auto-overwrite path via AutoSyncLayout).
func ReactToEvent(state w.WorldState, ev event.Event) (event.Reaction, error) {
	// Stale-epoch events are dropped (specs §2-F).
	if ev.Epoch != 0 && ev.Epoch < state.Meta.Epoch {
		return event.Reaction{Discard: true}, nil
	}
	r := event.Reaction{}
	switch ev.Kind {
	case event.KindStartup:
		r.Lifecycle = w.LifecycleBootstrap
		// Trigger cockpit SystemWindows construction in controller's
		// post-step (unified design v1 §4.1). The Key encodes the
		// display count observed at this instant so the internal
		// intent the controller submits is deterministic.
		r.DirtyScopes = append(r.DirtyScopes, w.DirtyScope{
			Kind: "cockpit-sync",
			Key:  fmt.Sprintf("%d", len(state.Observed.Displays.Displays)),
		})
	case event.KindWake:
		r.Lifecycle = w.LifecycleWakeRecovery
		// Wake: cockpit may have lost its display attachments; resync.
		r.DirtyScopes = append(r.DirtyScopes, w.DirtyScope{
			Kind: "cockpit-sync",
			Key:  fmt.Sprintf("%d", len(state.Observed.Displays.Displays)),
		})
	case event.KindDisplayChanged:
		r.Lifecycle = w.LifecycleDisplayReconfigure
		// Display topology changed: resync SystemWindows length.
		r.DirtyScopes = append(r.DirtyScopes, w.DirtyScope{
			Kind: "cockpit-sync",
			Key:  fmt.Sprintf("%d", len(state.Observed.Displays.Displays)),
		})
		// v2.7 §8.3.1: Display change may also alter which workspace is
		// active on the cockpit's display; sync Visibility accordingly.
		if vis, drift := detectCockpitVisibilityDrift(state); drift {
			r.DirtyScopes = append(r.DirtyScopes, w.DirtyScope{
				Kind: "cockpit-visibility-sync",
				Key:  string(vis),
			})
		}
	case event.KindSafetyTimer:
		r.Lifecycle = w.LifecycleFullReconcile
	case event.KindWindowsChanged:
		// missing/extra windows → dirty scope on global; controller will replan.
		r.DirtyScopes = []w.DirtyScope{{Kind: "global"}}
		// v2.7 §8.3.1: observed→desired Visibility 双方向同期。
		// active workspace の変化 (e.g., ユーザの space+1/q による
		// workspace 切替) を Visibility に反映し、planner の
		// 「Visibility=Shown だが observedActive≠CP1」誤反応を防ぐ。
		if vis, drift := detectCockpitVisibilityDrift(state); drift {
			r.DirtyScopes = append(r.DirtyScopes, w.DirtyScope{
				Kind: "cockpit-visibility-sync",
				Key:  string(vis),
			})
		}
		// Tier 1 detection (requirements §3.1 / design v3 §3.5):
		// observed windows that landed on a managed workspace, classify
		// as a managed kind, and have no MatchedTo are candidates for a
		// [NEW] card. The controller's PromoteOrphans() runs the 5-second
		// grace evaluation; we just enqueue here.
		now := nowNano()
		for liveID, ow := range state.Observed.Windows {
			if ow.MatchedTo != nil {
				continue
			}
			if !isManagedWorkspaceForEnv(ow.Workspace, state.Environment) {
				continue
			}
			if !isManagedKind(ow.Kind) {
				continue
			}
			if orphanAlreadyTracked(state.Meta.PendingOrphans, liveID) {
				continue
			}
			// §3.6 / §3.1 dedup: if a [NEW] card for this live window already
			// exists in ActiveCards, do not re-enqueue as a pending orphan.
			// This prevents the promote→discard→re-enqueue→re-promote cycle
			// that causes card spam when a window stays unmatched indefinitely.
			if orphanCardAlreadyActive(state.Meta.ActiveCards, liveID) {
				continue
			}
			r.OrphanAdds = append(r.OrphanAdds, w.OrphanCandidate{
				LiveID:     liveID,
				Kind:       ow.Kind,
				Workspace:  ow.Workspace,
				BundleID:   ow.App.BundleID,
				Title:      ow.Title.Value,
				DetectedAt: now,
			})
		}
	case event.KindLayoutChanged:
		// Tier 2 (requirements §3 / design v3 §3.5): emit a layout-sync
		// DirtyScope so the controller can auto-submit AutoSyncLayout for
		// each managed workspace that has a project assigned in the active
		// profile. We populate Project / Workspace via DirtyScope.Key as
		// "<projectID>|<workspaceID>".
		if ev.Data.Workspace != nil {
			if proj := identifyProjectForWorkspace(state.Desired, *ev.Data.Workspace, state.Environment); proj != "" {
				r.DirtyScopes = append(r.DirtyScopes, w.DirtyScope{
					Kind: "layout-sync",
					Key:  string(proj) + "|" + string(*ev.Data.Workspace),
				})
				return r, nil
			}
		}
		// Otherwise observe-only (existing semantics).
		r.ObserveScopes = []w.WorldScope{{Kind: "global"}}
	case event.KindUserClosedWindow:
		r.DirtyScopes = []w.DirtyScope{{Kind: "global"}}
		// Tier 4 [CLOSED] card. design v3 §3.5. Planner already drives the
		// respawn; the card is so the cockpit can surface what happened.
		//
		// T4.4 60s rate limit: if the same DesiredWindow has been closed
		// twice in the past 60 seconds, we emit a warning-flavored card
		// and tag the controller scope so respawn is suppressed.
		if ev.Data.Window != nil && ev.Data.Workspace != nil {
			var dwidStr string
			closes := userCloseCountFor(state.Meta.UserCloseHistory, state.Observed, *ev.Data.Window, &dwidStr)
			label := "managed window closed by user"
			limited := false
			if closes >= 2 {
				label = "managed window closed twice in 60s — respawn suppressed"
				limited = true
			}
			card := w.Card{
				Type:    w.CardTypeClosed,
				Subject: label,
				Context: map[string]string{
					"window":      string(*ev.Data.Window),
					"workspace":   string(*ev.Data.Workspace),
					"desired":     dwidStr,
					"closes-60s":  fmt.Sprintf("%d", closes+1),
					"rateLimited": fmt.Sprintf("%t", limited),
				},
				Actions: []w.CardAction{
					{Key: "Enter", Label: "keep restored"},
					{Key: "k", Label: "keep removed"},
					{Key: "u", Label: "undo (retry respawn)"},
					{Key: "Esc", Label: "dismiss"},
				},
			}
			r.NewCards = append(r.NewCards, card)
			// Annotate the dirty scope so the controller can short-circuit
			// respawn for this DesiredWindow.
			if limited {
				r.DirtyScopes = append(r.DirtyScopes, w.DirtyScope{
					Kind: "user-close-suppress",
					Key:  dwidStr,
				})
			}
			// Record the close-event timestamp so subsequent closes can
			// see the rate.
			r.UserCloseRecords = append(r.UserCloseRecords, event.UserCloseRecord{
				Window:    *ev.Data.Window,
				DesiredID: dwidStrToDesired(dwidStr),
				At:        nowNano(),
			})
		}
	case event.KindUserMovedWindow:
		// Cross-workspace user move: NOT a manual-layout candidate (specs §4.2).
		// Reducer emits DirtyScope so controller restores placement.
		r.DirtyScopes = []w.DirtyScope{{Kind: "global"}}
		// Tier 4 [MOVED] card so the cockpit can surface what happened.
		// Card creation is best-effort here; planner already drives the
		// revert via existing managed-window contracts.
		if ev.Data.Window != nil && ev.Data.Workspace != nil && ev.Data.TargetWS != nil {
			card := w.Card{
				Type:    w.CardTypeMoved,
				Subject: "managed window moved",
				Context: map[string]string{
					"window": string(*ev.Data.Window),
					"from":   string(*ev.Data.Workspace),
					"to":     string(*ev.Data.TargetWS),
				},
				Actions: []w.CardAction{
					{Key: "Enter", Label: "acknowledge revert"},
					{Key: "Esc", Label: "dismiss"},
				},
			}
			r.NewCards = append(r.NewCards, card)
		}
	case event.KindFocusChanged:
		r.ObserveScopes = []w.WorldScope{{Kind: "global"}}
	case event.KindUserReorderedColumns:
		// SSOT N-12 (2026-05-20) / §6.3 Level 3 auto-overwrite: same-workspace
		// user reorder now emits a layout-sync DirtyScope so the controller
		// dispatches AutoSyncLayout. The legacy ManualLayoutCandidate
		// machinery is removed; the controller queries observed columns via
		// observedColumnsForProject when consuming the scope.
		if ev.Data.Project != nil && ev.Data.Workspace != nil {
			r.DirtyScopes = append(r.DirtyScopes, w.DirtyScope{
				Kind: "layout-sync",
				Key:  string(*ev.Data.Project) + "|" + string(*ev.Data.Workspace),
			})
		}
	case event.KindLegacyAgentDetected:
		r.DirtyScopes = []w.DirtyScope{{Kind: "global"}}
	}
	return r, nil
}

// nowNano is overridable in tests for deterministic OrphanCandidate.DetectedAt.
var nowNano = func() int64 { return time.Now().UnixNano() }

// userCloseCountFor returns the number of recorded user-close events
// for the DesiredWindow corresponding to liveID in the last 60 seconds,
// and writes the DesiredWindowID stringification into outID. Returns
// 0 with empty outID if liveID is not currently matched.
func userCloseCountFor(history map[w.DesiredWindowID][]int64, obs w.ObservedWorld, liveID w.LiveWindowID, outID *string) int {
	ow, ok := obs.Windows[liveID]
	if !ok || ow.MatchedTo == nil {
		return 0
	}
	*outID = desiredToStr(*ow.MatchedTo)
	cutoff := nowNano() - int64(60*time.Second)
	count := 0
	for _, ts := range history[*ow.MatchedTo] {
		if ts >= cutoff {
			count++
		}
	}
	return count
}

func desiredToStr(id w.DesiredWindowID) string {
	return string(id.Project) + "|" + string(id.Kind) + "|" + fmt.Sprintf("%d", id.Index)
}

func dwidStrToDesired(s string) w.DesiredWindowID {
	parts := []string{"", "", ""}
	cur := 0
	for i := 0; i < len(s) && cur < 3; i++ {
		if s[i] == '|' {
			cur++
			continue
		}
		parts[cur] += string(s[i])
	}
	idx := 0
	fmt.Sscanf(parts[2], "%d", &idx)
	return w.DesiredWindowID{
		Project: w.ProjectID(parts[0]),
		Kind:    w.WindowKind(parts[1]),
		Index:   idx,
	}
}

// isManagedWorkspaceForEnv returns true if ws is one of the manifest's
// project / viewer workspaces (requirements §2).
func isManagedWorkspaceForEnv(ws w.WorkspaceID, env w.ManagedEnvironment) bool {
	if ws == env.Workspaces.Viewer {
		return true
	}
	for _, sl := range env.Workspaces.Slots {
		if sl.Workspace == ws {
			return true
		}
	}
	return false
}

// isManagedKind reports whether kind classifies as a managed window
// (Tier 1 candidate). External / Viewer are excluded.
func isManagedKind(k w.WindowKind) bool {
	switch k {
	case w.WindowAI, w.WindowShell, w.WindowEditor, w.WindowBrowser:
		return true
	}
	return false
}

// orphanAlreadyTracked returns true if liveID is in pending.
func orphanAlreadyTracked(pending []w.OrphanCandidate, liveID w.LiveWindowID) bool {
	for _, oc := range pending {
		if oc.LiveID == liveID {
			return true
		}
	}
	return false
}

// orphanCardAlreadyActive returns true if there is already a [NEW] card in
// activeCards whose Context["live"] matches liveID. This prevents the
// promote→discard→re-enqueue→re-promote spam cycle (requirements §3.1 /
// §3.6 / §10.4 dedup).
func orphanCardAlreadyActive(activeCards []w.Card, liveID w.LiveWindowID) bool {
	for _, c := range activeCards {
		if c.Type != w.CardTypeNew {
			continue
		}
		if w.LiveWindowID(c.Context["live"]) == liveID {
			return true
		}
	}
	return false
}

// identifyProjectForWorkspace returns the project assigned to the slot
// whose workspace == ws under the active profile. Empty if none.
func identifyProjectForWorkspace(d w.DesiredWorld, ws w.WorkspaceID, env w.ManagedEnvironment) w.ProjectID {
	prof, ok := d.Profiles[d.ActiveProfile]
	if !ok {
		return ""
	}
	for _, sl := range env.Workspaces.Slots {
		if sl.Workspace != ws {
			continue
		}
		if pid, ok := prof.Assignments[sl.ID]; ok {
			return pid
		}
	}
	return ""
}

// priorWorkspaceForParkWs returns the currently active workspace for the
// display that owns the given park workspace (e.g. "CP2"). This is saved as
// PriorWorkspace when showing a cockpit so the hide operation can restore it.
//
// displayIdx is the SystemWindow.DisplayIdx, used as a fallback when
// WorkspaceToDisplay is not populated (e.g. fake adapter in tests).
//
// Uses ObservedDisplayState.WorkspaceToDisplay (populated by sigwm.Observe
// from live window placement) as the primary source, replacing the old
// alphabetical-sort approach which broke when display IDs were non-contiguous.
// detectCockpitVisibilityDrift compares the observed active workspace of
// the cockpit's display against DesiredWorld.SystemWindows[cockpit].Visibility.
// If the display is on ParkWorkspace, the target visibility is Shown; if
// it is on any other workspace, the target is Hidden. Returns the target
// visibility and true if it differs from the current Visibility, else
// ("", false).
//
// Realises requirements v2.7 §8.3.1 — bidirectional sync between active
// workspace and Visibility. Without this sync the planner emits ShowCockpit
// ops whenever the user manually switches away from CP1, fighting the
// user's intent (= "勝手な CP1 戻り" loop).
func detectCockpitVisibilityDrift(state w.WorldState) (w.CockpitVisibility, bool) {
	for _, sw := range state.Desired.SystemWindows {
		if sw.Kind != w.WindowCockpit {
			continue
		}
		var displayID w.DisplayID
		if state.Observed.Displays.WorkspaceToDisplay != nil {
			if id, ok := state.Observed.Displays.WorkspaceToDisplay[sw.ParkWorkspace]; ok {
				displayID = id
			}
		}
		if displayID == "" {
			sorted := sortedDisplayIDsFromObs(state.Observed)
			if sw.DisplayIdx >= 0 && sw.DisplayIdx < len(sorted) {
				displayID = sorted[sw.DisplayIdx]
			}
		}
		if displayID == "" {
			continue
		}
		d, ok := state.Observed.Displays.Displays[displayID]
		if !ok {
			continue
		}
		var target w.CockpitVisibility
		if d.ActiveWorkspace == sw.ParkWorkspace {
			target = w.CockpitShown
		} else {
			target = w.CockpitHidden
		}
		if target == sw.Visibility {
			return "", false
		}
		return target, true
	}
	return "", false
}

func priorWorkspaceForParkWs(parkWs w.WorkspaceID, displayIdx int, obs w.ObservedWorld) w.WorkspaceID {
	if parkWs == "" {
		return ""
	}
	// Find the display that owns this park workspace.
	var displayID w.DisplayID
	found := false
	if obs.Displays.WorkspaceToDisplay != nil {
		if id, ok := obs.Displays.WorkspaceToDisplay[parkWs]; ok {
			displayID = id
			found = true
		}
	}
	if !found {
		// Fallback: use sorted display index (works for fake adapter in tests).
		sorted := sortedDisplayIDsFromObs(obs)
		if displayIdx >= 0 && displayIdx < len(sorted) {
			displayID = sorted[displayIdx]
			found = true
		}
	}
	if !found {
		return ""
	}
	if d, ok := obs.Displays.Displays[displayID]; ok {
		// Defensive: if the display is *already* on its park workspace
		// (e.g., daemon restart inheriting cockpit-visible state from a
		// previous run), the active workspace IS the park workspace —
		// returning it as PriorWorkspace would make the hide op a no-op
		// loop. Return empty instead; the next SetCockpitVisibility /
		// Toggle call when the user is on a non-park workspace will
		// refresh it properly.
		if d.ActiveWorkspace == parkWs {
			return ""
		}
		return d.ActiveWorkspace
	}
	return ""
}

// sortedDisplayIDsFromObs returns display IDs in a stable sort order
// (primary/main first, then by ID string). Used as a fallback in
// priorWorkspaceForParkWs when WorkspaceToDisplay is not populated.
func sortedDisplayIDsFromObs(obs w.ObservedWorld) []w.DisplayID {
	ids := make([]w.DisplayID, 0, len(obs.Displays.Displays))
	for id := range obs.Displays.Displays {
		ids = append(ids, id)
	}
	// sort: primary first, then by ID string
	n := len(ids)
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			iPrimary := obs.Displays.Primary != nil && *obs.Displays.Primary == ids[i]
			jPrimary := obs.Displays.Primary != nil && *obs.Displays.Primary == ids[j]
			swap := false
			if !iPrimary && jPrimary {
				swap = true
			} else if iPrimary == jPrimary && ids[i] > ids[j] {
				swap = true
			}
			if swap {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}

// cloneDesired makes a deep copy used by ReduceIntent.
func cloneDesired(d w.DesiredWorld) w.DesiredWorld {
	out := w.DesiredWorld{
		ActiveProfile:   d.ActiveProfile,
		Profiles:        make(map[w.ProfileID]w.DesiredProfile, len(d.Profiles)),
		Projects:        make(map[w.ProjectID]w.DesiredProject, len(d.Projects)),
		FocusPolicy:     cloneFocusPolicy(d.FocusPolicy),
		AcceptedLayouts: cloneAccepted(d.AcceptedLayouts),
		SystemWindows:   append([]w.SystemWindow(nil), d.SystemWindows...),
	}
	for k, v := range d.Profiles {
		assign := make(map[w.SlotID]w.ProjectID, len(v.Assignments))
		for s, p := range v.Assignments {
			assign[s] = p
		}
		v.Assignments = assign
		out.Profiles[k] = v
	}
	for k, v := range d.Projects {
		v.Windows = append([]w.DesiredWindow(nil), v.Windows...)
		layouts := make(map[w.WorkspaceID]w.DesiredLayout, len(v.Layouts))
		for ws, l := range v.Layouts {
			l.Columns = append([]w.DesiredColumn(nil), l.Columns...)
			layouts[ws] = l
		}
		v.Layouts = layouts
		out.Projects[k] = v
	}
	return out
}

func cloneFocusPolicy(f w.FocusPolicySet) w.FocusPolicySet {
	out := w.FocusPolicySet{FinalFocus: make(map[string]w.WorkspaceID, len(f.FinalFocus))}
	for k, v := range f.FinalFocus {
		out.FinalFocus[k] = v
	}
	return out
}

func cloneAccepted(in map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout) map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout {
	out := make(map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout, len(in))
	for p, m := range in {
		mm := make(map[w.WorkspaceID]w.DesiredLayout, len(m))
		for ws, l := range m {
			l.Columns = append([]w.DesiredColumn(nil), l.Columns...)
			mm[ws] = l
		}
		out[p] = mm
	}
	return out
}
