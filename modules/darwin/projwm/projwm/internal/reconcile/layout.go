// reconcile/layout.go: niri column / stack 構造の snapshot と restore。
//
// 役割:
//   - archive (project deactivate) 直前に slot ws 上の project window 配置を
//     state.Window.Layout に save する (= snapshot)。
//   - unarchive 直後 (window spawn 完了後) に Layout に従って column / stack を
//     再構築する (= restore)。
//
// 方針:
//   - snapshot: ws を focus → QueryWindows → frame.x 昇順ソートで column 順確定。
//     y-up 座標系 (高 y = 画面上端) なので同 column 内は y 降順で stack top → bottom。
//     tabbed 検出: 同 column の全 window が同 y = tabbed mode。
//   - restore: 初回 QueryWindows で colMap (liveID → colIndex) を構築し、以後は
//     MoveColumnDirection / MoveDirection のたびに map を更新。再クエリ不要なので
//     niri の layout 伝播遅延に左右されない。
package reconcile

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/yuu-th/projwm/internal/naming"
	"github.com/yuu-th/projwm/internal/omniwm"
	"github.com/yuu-th/projwm/internal/state"
)

// columnSnapshot は ws 上の 1 column の members 列 (stack index 0 = top)。
type columnSnapshot struct {
	memberIDs []string
	tabbed    bool
}

// snapshotWorkspaceColumns は wsName 上の column 構造を x 座標ベースで確定する。
//
// 副作用: wsName に一時 focus → defer で元の ws / window に戻す。
func snapshotWorkspaceColumns(ctx context.Context, oc *omniwm.Client, wsName string, log io.Writer) ([]columnSnapshot, error) {
	if oc == nil {
		return nil, fmt.Errorf("omniwm client is nil")
	}
	origWS, _ := oc.ActiveWorkspaceName(ctx)
	origFocusedID, _ := oc.FocusedWindowID(ctx)
	defer func() {
		if origWS != "" && origWS != wsName {
			_ = oc.FocusWorkspaceByName(ctx, origWS)
		}
		if origFocusedID != "" {
			_ = oc.FocusWindow(ctx, origFocusedID)
		}
	}()

	if err := oc.FocusWorkspaceByName(ctx, wsName); err != nil {
		return nil, fmt.Errorf("focus ws %q: %w", wsName, err)
	}
	time.Sleep(150 * time.Millisecond) // wait for niri to propagate ws focus → accurate frame.x

	wins, err := oc.QueryWindows(ctx, "--workspace", wsName)
	if err != nil {
		return nil, fmt.Errorf("query windows on ws %q: %w", wsName, err)
	}
	if len(wins) == 0 {
		return nil, nil
	}

	cols := groupWindowsByColumn(wins)
	if log != nil {
		fmt.Fprintf(log, "snapshot ws=%q: %d columns\n", wsName, len(cols))
		for i, c := range cols {
			fmt.Fprintf(log, "  col[%d] tabbed=%v members=%v\n", i, c.tabbed, c.memberIDs)
		}
	}
	return cols, nil
}

// groupWindowsByColumn groups live windows by x-coordinate into columns.
// Columns are sorted left-to-right (x ascending). Within each column, windows
// are sorted top-to-bottom (y descending; y-up convention: higher y = visually top).
func groupWindowsByColumn(wins []omniwm.Window) []columnSnapshot {
	if len(wins) == 0 {
		return nil
	}

	const colTol = 5 // px tolerance for same-column x grouping

	type xGroup struct {
		x       int
		members []omniwm.Window
	}
	var groups []xGroup

	for _, w := range wins {
		placed := false
		for i := range groups {
			if absInt(groups[i].x-w.Frame.X) <= colTol {
				groups[i].members = append(groups[i].members, w)
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, xGroup{x: w.Frame.X, members: []omniwm.Window{w}})
		}
	}

	// Sort groups by x ascending (left → right = col 0, 1, 2 ...)
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].x < groups[j].x })

	cols := make([]columnSnapshot, 0, len(groups))
	for _, g := range groups {
		// Sort within column by y descending (top → bottom = stack 0, 1, 2 ...)
		sort.SliceStable(g.members, func(i, j int) bool {
			return g.members[i].Frame.Y > g.members[j].Frame.Y
		})
		tabbed := isColumnTabbedByFrames(g.members)
		ids := make([]string, len(g.members))
		for i, m := range g.members {
			ids[i] = m.ID
		}
		cols = append(cols, columnSnapshot{memberIDs: ids, tabbed: tabbed})
	}
	return cols
}

// isColumnTabbedByFrames detects tabbed mode: all members share the same y.
func isColumnTabbedByFrames(members []omniwm.Window) bool {
	if len(members) <= 1 {
		return false
	}
	firstY := members[0].Frame.Y
	for _, m := range members[1:] {
		if m.Frame.Y != firstY {
			return false
		}
	}
	return true
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// matchStateToLive は state.Window と OmniWM Window を kind ごとに照合する。
func matchStateToLive(sw state.Window, lw omniwm.Window, projectName string) bool {
	switch sw.Kind {
	case naming.KindAI, naming.KindShell:
		return lw.BundleID == naming.TerminalBundleID &&
			lw.Title == naming.GhosttyTitle(sw.Kind, sw.ID, projectName)
	case naming.KindEditor:
		return lw.BundleID == naming.ZedBundleID && lw.Title == projectName
	case naming.KindBrowser:
		return sw.LiveWindowID != "" && sw.LiveWindowID == lw.ID
	}
	return false
}

// SnapshotProjectLayout は project の slot ws 上での column / stack 配置を
// state.Window.Layout に書き込む (in-place mutate)。
//
// wsName が空 → no-op。 browser kind は touch しない。
func (r *Reconciler) SnapshotProjectLayout(ctx context.Context, projectName string, p *state.Project, wsName string, log io.Writer) error {
	if r.OmniWM == nil || wsName == "" || p == nil {
		return nil
	}
	if log == nil {
		log = io.Discard
	}
	cols, err := snapshotWorkspaceColumns(ctx, r.OmniWM, wsName, log)
	if err != nil {
		return fmt.Errorf("snapshot ws %q: %w", wsName, err)
	}

	// Second query for title/bundleID matching (frame.x not used here).
	// ws may no longer be focused (snapshotWorkspaceColumns restored orig ws),
	// but title/bundleID are focus-independent.
	wins, err := r.OmniWM.QueryWindows(ctx, "--workspace", wsName)
	if err != nil {
		return fmt.Errorf("query windows: %w", err)
	}

	newLayouts := map[int]*state.WindowLayout{}
	for colIdx, col := range cols {
		for stackIdx, liveID := range col.memberIDs {
			var liveWin *omniwm.Window
			for i := range wins {
				if wins[i].ID == liveID {
					liveWin = &wins[i]
					break
				}
			}
			if liveWin == nil {
				continue
			}
			for i := range p.Windows {
				w := &p.Windows[i]
				if matchStateToLive(*w, *liveWin, projectName) {
					l := state.WindowLayout{Column: colIdx, Stack: stackIdx, Tabbed: col.tabbed}
					newLayouts[i] = &l
					break
				}
			}
		}
	}

	for i := range p.Windows {
		w := &p.Windows[i]
		if w.Kind == naming.KindBrowser {
			continue
		}
		if l, ok := newLayouts[i]; ok {
			w.Layout = l
		} else {
			w.Layout = nil
		}
	}
	return nil
}

// liveTarget は restore で扱う 1 entry (state.Window と OmniWM live ID のペア)。
type liveTarget struct {
	sw     state.Window
	liveID string
}

// RestoreProjectLayout は spawn 完了後の windows を Layout に従って rearrange する。
//
// colMap (liveID → colIndex) を初回 QueryWindows で構築し、以後は
// MoveColumnDirection / MoveDirection のたびに更新する。 再クエリ不要なので
// niri の伝播遅延に左右されない。
func (r *Reconciler) RestoreProjectLayout(ctx context.Context, projectName string, p state.Project, wsName string, log io.Writer) error {
	if r.OmniWM == nil || wsName == "" {
		return nil
	}
	if log == nil {
		log = io.Discard
	}

	// Focus workspace first so QueryWindows returns accurate frame.x.
	origWS, _ := r.OmniWM.ActiveWorkspaceName(ctx)
	origFocused, _ := r.OmniWM.FocusedWindowID(ctx)
	defer func() {
		if origWS != "" && origWS != wsName {
			_ = r.OmniWM.FocusWorkspaceByName(ctx, origWS)
		}
		if origFocused != "" {
			_ = r.OmniWM.FocusWindow(ctx, origFocused)
		}
	}()
	if err := r.OmniWM.FocusWorkspaceByName(ctx, wsName); err != nil {
		return fmt.Errorf("focus ws: %w", err)
	}
	time.Sleep(150 * time.Millisecond)

	wins, err := r.OmniWM.QueryWindows(ctx, "--workspace", wsName)
	if err != nil {
		return fmt.Errorf("query windows: %w", err)
	}

	// Build colMap: liveID → current column index (from frame.x order).
	colMap := map[string]int{}
	for i, col := range groupWindowsByColumn(wins) {
		for _, id := range col.memberIDs {
			colMap[id] = i
		}
	}

	// Resolve state.Window → live ID.
	var targets []liveTarget
	for _, w := range p.Windows {
		if w.Layout == nil || w.Kind == naming.KindBrowser {
			continue
		}
		var liveID string
		for i := range wins {
			if matchStateToLive(w, wins[i], projectName) {
				liveID = wins[i].ID
				break
			}
		}
		if liveID == "" {
			fmt.Fprintf(log, "  layout: live window not found for %s-%d (skip)\n", w.Kind, w.ID)
			continue
		}
		targets = append(targets, liveTarget{sw: w, liveID: liveID})
	}
	if len(targets) == 0 {
		return nil
	}

	groups := map[int][]liveTarget{}
	maxCol := 0
	for _, t := range targets {
		groups[t.sw.Layout.Column] = append(groups[t.sw.Layout.Column], t)
		if t.sw.Layout.Column > maxCol {
			maxCol = t.sw.Layout.Column
		}
	}
	for c := range groups {
		grp := groups[c]
		sort.SliceStable(grp, func(i, j int) bool { return grp[i].sw.Layout.Stack < grp[j].sw.Layout.Stack })
		groups[c] = grp
	}

	// Process columns left to right.
	// For each column c: (1) move base to col c, (2) stack members via c+1 → merge left,
	// (3) toggle tabbed if needed. colMap is updated after each move (no re-query needed).
	for c := 0; c <= maxCol; c++ {
		grp := groups[c]
		if len(grp) == 0 {
			continue
		}
		base := grp[0]
		fmt.Fprintf(log, "  layout: positioning col %d base=%s+%d\n", c, base.sw.Kind, base.sw.ID)
		if err := r.moveWindowToColumn(ctx, base.liveID, c, colMap, log); err != nil {
			fmt.Fprintf(log, "  layout: WARN positioning base: %v\n", err)
		}

		for s := 1; s < len(grp); s++ {
			t := grp[s]
			fmt.Fprintf(log, "  layout: stacking col %d stack %d = %s+%d\n", c, s, t.sw.Kind, t.sw.ID)
			if err := r.moveWindowToColumn(ctx, t.liveID, c+1, colMap, log); err != nil {
				fmt.Fprintf(log, "  layout: WARN positioning stack member to c+1: %v\n", err)
				continue
			}
			if err := r.OmniWM.FocusWindow(ctx, t.liveID); err != nil {
				fmt.Fprintf(log, "  layout: WARN focus stack member for merge: %v\n", err)
				continue
			}
			time.Sleep(80 * time.Millisecond)
			_ = r.OmniWM.MoveDirection(ctx, "left")
			time.Sleep(100 * time.Millisecond)
			colMapMergeLeft(colMap, t.liveID)
		}

		if grp[0].sw.Layout.Tabbed && len(grp) > 1 {
			if err := r.OmniWM.FocusWindow(ctx, base.liveID); err != nil {
				fmt.Fprintf(log, "  layout: WARN focus base for tabbed: %v\n", err)
				continue
			}
			time.Sleep(40 * time.Millisecond)
			if err := r.OmniWM.ToggleColumnTabbed(ctx); err != nil {
				fmt.Fprintf(log, "  layout: WARN toggle tabbed: %v\n", err)
			}
			time.Sleep(40 * time.Millisecond)
		}
	}
	return nil
}

// moveWindowToColumn は windowID を target col index に move する。
// colMap を読み書きして現在位置を追跡する (QueryWindows 不要)。
func (r *Reconciler) moveWindowToColumn(ctx context.Context, windowID string, targetCol int, colMap map[string]int, log io.Writer) error {
	curCol, ok := colMap[windowID]
	if !ok {
		return fmt.Errorf("window %s not in column map", windowID)
	}
	if curCol == targetCol {
		fmt.Fprintf(log, "    move %s: already at col %d\n", windowID, targetCol)
		return nil
	}
	if err := r.OmniWM.FocusWindow(ctx, windowID); err != nil {
		return fmt.Errorf("focus: %w", err)
	}
	time.Sleep(150 * time.Millisecond)
	dir := "right"
	steps := targetCol - curCol
	if steps < 0 {
		dir = "left"
		steps = -steps
	}
	fmt.Fprintf(log, "    move %s: col %d -> %d (%d %s)\n", windowID, curCol, targetCol, steps, dir)
	for k := 0; k < steps; k++ {
		if err := r.OmniWM.MoveColumnDirection(ctx, dir); err != nil {
			return fmt.Errorf("move-column %s step %d: %w", dir, k, err)
		}
		time.Sleep(80 * time.Millisecond)
		colMapSwap(colMap, windowID, dir)
	}
	return nil
}

// colMapSwap は MoveColumnDirection 後に colMap を更新する。
// focused column (windowID) が dir 方向の隣 column と swap した結果を反映。
func colMapSwap(colMap map[string]int, windowID, dir string) {
	movedCol := colMap[windowID]
	var neighborCol int
	if dir == "left" {
		neighborCol = movedCol - 1
	} else {
		neighborCol = movedCol + 1
	}
	for id := range colMap {
		switch colMap[id] {
		case movedCol:
			colMap[id] = neighborCol
		case neighborCol:
			colMap[id] = movedCol
		}
	}
}

// FixViewerOrder は WS A (viewer workspace) の ai-view 窓を slotOrder 順に並べ直す。
//
// slotOrder: viewer title の期待順序 (例: ["ai-view-1:dotfiles","ai-view-1:projwm-jtest"])。
// viewer ws に focus → QueryWindows → frame.x 昇順で現在 column 順を確定 →
// 期待順と異なれば moveWindowToColumn で並べ直す。
func (r *Reconciler) FixViewerOrder(ctx context.Context, viewerWS string, slotOrder []string, log io.Writer) error {
	if r.OmniWM == nil || len(slotOrder) == 0 {
		return nil
	}
	if log == nil {
		log = io.Discard
	}

	// viewer ws に focus して frame.x を確定する。
	origWS, _ := r.OmniWM.ActiveWorkspaceName(ctx)
	origFocused, _ := r.OmniWM.FocusedWindowID(ctx)
	defer func() {
		if origWS != "" && origWS != viewerWS {
			_ = r.OmniWM.FocusWorkspaceByName(ctx, origWS)
		}
		if origFocused != "" {
			_ = r.OmniWM.FocusWindow(ctx, origFocused)
		}
	}()
	if err := r.OmniWM.FocusWorkspaceByName(ctx, viewerWS); err != nil {
		return fmt.Errorf("focus viewer ws %q: %w", viewerWS, err)
	}
	time.Sleep(150 * time.Millisecond)

	wins, err := r.OmniWM.QueryWindows(ctx, "--workspace", viewerWS)
	if err != nil {
		return fmt.Errorf("query viewer ws: %w", err)
	}

	// title → window ID の map を作成
	titleToID := map[string]string{}
	for _, w := range wins {
		titleToID[w.Title] = w.ID
	}

	// colMap 構築 (liveID → colIndex)
	colMap := map[string]int{}
	for i, col := range groupWindowsByColumn(wins) {
		for _, id := range col.memberIDs {
			colMap[id] = i
		}
	}

	// 期待順: slotOrder[i] が col i に来るべき
	for targetCol, title := range slotOrder {
		id, ok := titleToID[title]
		if !ok {
			fmt.Fprintf(log, "  viewer order: %q not found on ws %q (skip)\n", title, viewerWS)
			continue
		}
		fmt.Fprintf(log, "  viewer order: %q → col %d\n", title, targetCol)
		if err := r.moveWindowToColumn(ctx, id, targetCol, colMap, log); err != nil {
			fmt.Fprintf(log, "  viewer order: WARN move %q: %v\n", title, err)
		}
	}
	return nil
}

// colMapMergeLeft は MoveDirection("left") 後に colMap を更新する。
// windowID が solo column から左の column に merge した結果を反映。
// windowID の旧 column は消滅し、それより右の column index が 1 ずつ減る。
func colMapMergeLeft(colMap map[string]int, windowID string) {
	oldCol := colMap[windowID]
	colMap[windowID] = oldCol - 1
	for id := range colMap {
		if id != windowID && colMap[id] > oldCol {
			colMap[id]--
		}
	}
}
