package wm

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// Fake is an in-memory WindowManagerAdapter for tests.
type Fake struct {
	mu      sync.Mutex
	env     w.ManagedEnvironment
	windows map[w.LiveWindowID]*fakeWin
	layouts map[w.WorkspaceID][][]w.LiveWindowID // columns of stacks
	focus   w.ObservedFocus
	nextID  int
	// userEvents accumulates user-origin events injected by tests.
	userEvents []UserEvent

	// --- specs §3.8 instrumentation ---
	// concurrentInFlight tracks how many mutation methods are simultaneously executing.
	// transactionContractMaxInFlight tracks the historical maximum.
	// Used by S8.A to assert single-writer ordering.
	concurrentInFlight             atomic.Int32
	transactionContractMaxInFlight atomic.Int32
	// mutationDelay is artificially injected at each mutation entry to widen the
	// race window for S8.A. Default 0.
	mutationDelay time.Duration
	// dropMutations: if true, all mutation methods become no-ops (return nil, observed
	// state unchanged). Used by S8.C to drive Verifier into ReplanExceededError.
	dropMutations atomic.Bool
}

type fakeWin struct {
	id        w.LiveWindowID
	title     string
	bundleID  string
	workspace w.WorkspaceID
	desired   *w.DesiredWindowID
	kind      w.WindowKind
}

// UserEvent is a fake-injected user event for the controller. We keep it here so the
// scenario backend can drain pending user-origin events between Steps.
type UserEvent struct {
	Kind      string // "user-reordered-columns", "user-moved-window", "user-closed-window"
	Project   *w.ProjectID
	Workspace *w.WorkspaceID
	Columns   [][]w.LiveWindowID
	Window    *w.LiveWindowID
	TargetWS  *w.WorkspaceID
}

// NewFake creates a fresh fake.
func NewFake(env w.ManagedEnvironment) *Fake {
	f := &Fake{
		env:     env,
		windows: map[w.LiveWindowID]*fakeWin{},
		layouts: map[w.WorkspaceID][][]w.LiveWindowID{},
	}
	for _, ws := range env.Workspaces.Workspaces {
		f.layouts[ws.ID] = nil
	}
	return f
}

func (f *Fake) Capabilities(ctx context.Context) (Capabilities, error) {
	return Capabilities{
		MaxVisibleColumns:             f.env.WindowManager.Layout.MaxVisibleColumns,
		MaxWindowsPerColumn:           f.env.WindowManager.Layout.MaxWindowsPerColumn,
		SupportsSummonRight:           true,
		SupportsTabbedColumn:          false,
		SupportsMoveToWorkspaceByName: true,
	}, nil
}

// Observe returns a deep snapshot.
func (f *Fake) Observe(ctx context.Context) (w.ObservedWorld, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ow := w.ObservedWorld{
		Windows:    map[w.LiveWindowID]w.ObservedWindow{},
		Workspaces: map[w.WorkspaceID]w.ObservedWorkspace{},
		Layouts:    map[w.WorkspaceID]w.ObservedLayout{},
		Focus:      f.focus,
	}
	for _, ws := range f.env.Workspaces.Workspaces {
		ow.Workspaces[ws.ID] = w.ObservedWorkspace{ID: ws.ID, Role: ws.Role}
	}
	for id, wn := range f.windows {
		var matched *w.DesiredWindowID
		if wn.desired != nil {
			d := *wn.desired
			matched = &d
		}
		ow.Windows[id] = w.ObservedWindow{
			ID:        wn.id,
			App:       w.ObservedAppRef{BundleID: wn.bundleID},
			Title:     w.ObservedTitle{Value: wn.title},
			Workspace: wn.workspace,
			Focused:   id == f.focus.Window,
			MatchedTo: matched,
			Kind:      wn.kind,
		}
	}
	for ws, cols := range f.layouts {
		ocols := make([]w.ObservedColumn, 0, len(cols))
		for _, st := range cols {
			win := append([]w.LiveWindowID(nil), st...)
			mode := w.ColumnSolo
			if len(win) > 1 {
				mode = w.ColumnStacked
			}
			ocols = append(ocols, w.ObservedColumn{Windows: win, Mode: mode})
		}
		ow.Layouts[ws] = w.ObservedLayout{Workspace: ws, Columns: ocols}
	}
	return ow, nil
}

// Spawn creates a new fake window placed at the rightmost column of the target workspace.
func (f *Fake) Spawn(ctx context.Context, r SpawnRequest) (w.LiveWindowID, error) {
	if !f.enterMutation() {
		defer f.exitMutation()
		// Drop mode: synthesize a plausible LiveWindowID without touching state.
		f.mu.Lock()
		f.nextID++
		id := w.LiveWindowID(fmt.Sprintf("dropped-%d", f.nextID))
		f.mu.Unlock()
		return id, nil
	}
	defer f.exitMutation()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := w.LiveWindowID(fmt.Sprintf("lw-%d", f.nextID))
	d := r.Desired
	wn := &fakeWin{
		id:        id,
		title:     r.Title,
		bundleID:  r.BundleID,
		workspace: r.Workspace,
		desired:   &d,
		kind:      r.Kind,
	}
	f.windows[id] = wn
	f.layouts[r.Workspace] = append(f.layouts[r.Workspace], []w.LiveWindowID{id})
	return id, nil
}

// Close removes a window from the world.
func (f *Fake) Close(ctx context.Context, id w.LiveWindowID) error {
	if !f.enterMutation() {
		f.exitMutation()
		return nil
	}
	defer f.exitMutation()
	f.mu.Lock()
	defer f.mu.Unlock()
	wn, ok := f.windows[id]
	if !ok {
		return fmt.Errorf("fake: close: unknown window %s", id)
	}
	f.removeFromLayoutLocked(wn.workspace, id)
	delete(f.windows, id)
	if f.focus.Window == id {
		f.focus.Window = ""
	}
	return nil
}

// TerminateManagedAppInstance removes the fake window associated with a managed lifecycle.
func (f *Fake) TerminateManagedAppInstance(ctx context.Context, r TerminateManagedAppInstanceRequest) error {
	if r.Desired.Project == "" {
		return fmt.Errorf("fake: terminate-managed-app-instance: missing desired identity")
	}
	return f.Close(ctx, r.LiveWindow)
}

// MoveWindowToWorkspace moves a window across workspaces, appending as a new column.
func (f *Fake) MoveWindowToWorkspace(ctx context.Context, id w.LiveWindowID, ws w.WorkspaceID) error {
	if !f.enterMutation() {
		f.exitMutation()
		return nil
	}
	defer f.exitMutation()
	f.mu.Lock()
	defer f.mu.Unlock()
	wn, ok := f.windows[id]
	if !ok {
		return fmt.Errorf("fake: move: unknown window %s", id)
	}
	f.removeFromLayoutLocked(wn.workspace, id)
	wn.workspace = ws
	f.layouts[ws] = append(f.layouts[ws], []w.LiveWindowID{id})
	return nil
}

// ReorderColumns sets the layout to exactly the given columns (must be a permutation of current).
func (f *Fake) ReorderColumns(ctx context.Context, ws w.WorkspaceID, columns [][]w.LiveWindowID) error {
	if !f.enterMutation() {
		f.exitMutation()
		return nil
	}
	defer f.exitMutation()
	f.mu.Lock()
	defer f.mu.Unlock()
	// validate permutation
	current := map[w.LiveWindowID]bool{}
	for _, c := range f.layouts[ws] {
		for _, id := range c {
			current[id] = true
		}
	}
	want := map[w.LiveWindowID]bool{}
	for _, c := range columns {
		for _, id := range c {
			want[id] = true
		}
	}
	if len(current) != len(want) {
		return fmt.Errorf("fake: reorder: columns must be a permutation of current windows")
	}
	for k := range current {
		if !want[k] {
			return fmt.Errorf("fake: reorder: missing window %s", k)
		}
	}
	cp := make([][]w.LiveWindowID, 0, len(columns))
	for _, c := range columns {
		cp = append(cp, append([]w.LiveWindowID(nil), c...))
	}
	f.layouts[ws] = cp
	return nil
}

// FocusWorkspace sets focus to a workspace (focused window becomes first in first column, or empty).
func (f *Fake) FocusWorkspace(ctx context.Context, ws w.WorkspaceID) error {
	if !f.enterMutation() {
		f.exitMutation()
		return nil
	}
	defer f.exitMutation()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.focus.Workspace = ws
	if cols := f.layouts[ws]; len(cols) > 0 && len(cols[0]) > 0 {
		f.focus.Window = cols[0][0]
	} else {
		f.focus.Window = ""
	}
	return nil
}

// FocusWindow sets focus to a specific window.
func (f *Fake) FocusWindow(ctx context.Context, id w.LiveWindowID) error {
	if !f.enterMutation() {
		f.exitMutation()
		return nil
	}
	defer f.exitMutation()
	f.mu.Lock()
	defer f.mu.Unlock()
	wn, ok := f.windows[id]
	if !ok {
		return fmt.Errorf("fake: focus: unknown window %s", id)
	}
	f.focus = w.ObservedFocus{Workspace: wn.workspace, Window: id}
	return nil
}

// --- Cockpit operations (unified design v2 — park-workspace model).
// Fake implements them as minimal no-ops or in-memory mutations so
// reducer/planner integration tests can observe the effects.

// SpawnCockpit creates an in-memory cockpit window for displayIdx.
// Idempotent: if a window with the same title already exists, returns nil.
func (f *Fake) SpawnCockpit(ctx context.Context, displayIdx int, title string) error {
	if !f.enterMutation() {
		f.exitMutation()
		return nil
	}
	defer f.exitMutation()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, wn := range f.windows {
		if wn.bundleID == "com.mitchellh.ghostty" && wn.title == title {
			return nil
		}
	}
	f.nextID++
	id := w.LiveWindowID(fmt.Sprintf("cockpit-fake-%d", f.nextID))
	f.windows[id] = &fakeWin{
		id:       id,
		title:    title,
		bundleID: "com.mitchellh.ghostty",
		kind:     w.WindowCockpit,
	}
	return nil
}

// ShowCockpitOnDisplay is a no-op for the fake; display workspace switching
// is not modelled in the in-memory fake.
func (f *Fake) ShowCockpitOnDisplay(ctx context.Context, displayID w.DisplayID, parkWsName string) error {
	if !f.enterMutation() {
		f.exitMutation()
		return nil
	}
	defer f.exitMutation()
	return nil
}

// HideCockpitOnDisplay is a no-op for the fake.
func (f *Fake) HideCockpitOnDisplay(ctx context.Context, displayID w.DisplayID, priorWsName string) error {
	if !f.enterMutation() {
		f.exitMutation()
		return nil
	}
	defer f.exitMutation()
	return nil
}

func (f *Fake) removeFromLayoutLocked(ws w.WorkspaceID, id w.LiveWindowID) {
	cols := f.layouts[ws]
	out := cols[:0]
	for _, c := range cols {
		nc := []w.LiveWindowID{}
		for _, x := range c {
			if x != id {
				nc = append(nc, x)
			}
		}
		if len(nc) > 0 {
			out = append(out, nc)
		}
	}
	f.layouts[ws] = out
}

// --- Test helpers (not part of Adapter interface) ---

// SetMutationDelay enlarges the race window for S8.A single-writer testing.
func (f *Fake) SetMutationDelay(d time.Duration) { f.mutationDelay = d }

// SetDropMutations toggles whether mutation methods silently no-op (used by S8.C).
func (f *Fake) SetDropMutations(drop bool) { f.dropMutations.Store(drop) }

// TransactionContractMaxInFlight returns the historical maximum of concurrent
// mutation invocations observed by the fake. For S8.A this MUST stay <= 1
// when controller's wmMutationLock holds.
func (f *Fake) TransactionContractMaxInFlight() int32 {
	return f.transactionContractMaxInFlight.Load()
}

// ResetTransactionContractCounters resets both in-flight and max counters.
func (f *Fake) ResetTransactionContractCounters() {
	f.concurrentInFlight.Store(0)
	f.transactionContractMaxInFlight.Store(0)
}

// enterMutation must be called at the start of every mutation method. It returns
// true if the mutation should proceed; false if it must be silently skipped
// (S8.C drop-mutations mode).
func (f *Fake) enterMutation() bool {
	cur := f.concurrentInFlight.Add(1)
	for {
		max := f.transactionContractMaxInFlight.Load()
		if cur <= max {
			break
		}
		if f.transactionContractMaxInFlight.CompareAndSwap(max, cur) {
			break
		}
	}
	if f.mutationDelay > 0 {
		time.Sleep(f.mutationDelay)
	}
	return !f.dropMutations.Load()
}

func (f *Fake) exitMutation() {
	f.concurrentInFlight.Add(-1)
}

// InjectExternalWindow places an unmanaged (external) window. Used by scenarios for §4.5.
func (f *Fake) InjectExternalWindow(id w.LiveWindowID, ws w.WorkspaceID, title, bundle string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.windows[id] = &fakeWin{id: id, title: title, bundleID: bundle, workspace: ws, kind: w.WindowExternal}
	f.layouts[ws] = append(f.layouts[ws], []w.LiveWindowID{id})
}

// SimulateUserMove emulates a user-origin cross-workspace move (specs §4.2).
func (f *Fake) SimulateUserMove(id w.LiveWindowID, target w.WorkspaceID) {
	f.mu.Lock()
	wn, ok := f.windows[id]
	if !ok {
		f.mu.Unlock()
		return
	}
	from := wn.workspace
	f.removeFromLayoutLocked(from, id)
	wn.workspace = target
	f.layouts[target] = append(f.layouts[target], []w.LiveWindowID{id})
	tw := target
	f.userEvents = append(f.userEvents, UserEvent{Kind: "user-moved-window", Window: &id, Workspace: &from, TargetWS: &tw})
	f.mu.Unlock()
}

// SimulateUserClose emulates user closing a managed window (specs §4.3).
func (f *Fake) SimulateUserClose(id w.LiveWindowID) {
	f.mu.Lock()
	wn, ok := f.windows[id]
	if !ok {
		f.mu.Unlock()
		return
	}
	ws := wn.workspace
	f.removeFromLayoutLocked(ws, id)
	delete(f.windows, id)
	f.userEvents = append(f.userEvents, UserEvent{Kind: "user-closed-window", Window: &id, Workspace: &ws})
	f.mu.Unlock()
}

// SimulateUserReorderColumns emulates same-workspace user reorder (specs §4.4).
func (f *Fake) SimulateUserReorderColumns(ws w.WorkspaceID, project w.ProjectID, columns [][]w.LiveWindowID) {
	if err := f.ReorderColumns(context.Background(), ws, columns); err != nil {
		return
	}
	f.mu.Lock()
	cp := make([][]w.LiveWindowID, 0, len(columns))
	for _, c := range columns {
		cp = append(cp, append([]w.LiveWindowID(nil), c...))
	}
	wsCopy := ws
	pj := project
	f.userEvents = append(f.userEvents, UserEvent{Kind: "user-reordered-columns", Project: &pj, Workspace: &wsCopy, Columns: cp})
	f.mu.Unlock()
}

// PeekUserEvents returns pending user-origin events without acknowledging them.
func (f *Fake) PeekUserEvents() []UserEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]UserEvent(nil), f.userEvents...)
	// Sort for determinism (stable by Kind then any pointer values projected to strings).
	sort.SliceStable(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

// AckUserEvents acknowledges the first n pending user-origin events after a
// controller transaction commits successfully.
func (f *Fake) AckUserEvents(n int) {
	if n <= 0 {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if n >= len(f.userEvents) {
		f.userEvents = nil
		return
	}
	f.userEvents = append([]UserEvent(nil), f.userEvents[n:]...)
}
