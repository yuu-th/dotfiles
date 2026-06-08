package world

// DirtyScope. design.md §13.
type DirtyScope struct {
	Kind  string // "project", "workspace", "viewer", "global"
	Key   string // project ID / workspace ID / etc
}

// WorldScope is for ObserveScope etc. design.md §13.
type WorldScope = DirtyScope

// ControllerMeta. design.md §8.
type ControllerMeta struct {
	Epoch         Epoch
	Transaction   *TransactionID
	PendingEvents []EventID
	DirtyScopes   []DirtyScope
	// ActiveCards holds in-memory cockpit proposal cards. Not persisted.
	// Re-observed each daemon startup. requirements §10, design v3 §3.9.
	ActiveCards []Card
	// PendingOrphans tracks unmatched live windows in managed workspaces
	// that are within their 5-second grace period before becoming [NEW]
	// cards. design v3 §3.5 / §3.6.
	PendingOrphans []OrphanCandidate
	// UserCloseHistory tracks user-initiated close events for the 60s
	// rate-limiter (requirements T4.4). Key is DesiredWindowID; value is
	// the list of unix-nano timestamps when ReactToEvent observed a
	// user-close for that window. Older entries are pruned by the
	// controller's grace evaluator.
	UserCloseHistory map[DesiredWindowID][]int64
	// SummonFocusAnchor freezes the focused window as observed at the START
	// of the current transaction. summon/jump/cycle (§4.1 OP01-05) decide
	// "am I already on a window of this (project,kind) → cycle to the next
	// one" — a STATEFUL decision relative to where the user was when they
	// pressed the key. The converge loop replans against live observation,
	// and each replan would otherwise re-read the focus WE just set and
	// cycle again, alternating forever across 2+ candidate windows (never
	// reaching 0 ops → max-replans fail). The planner's cycle decision reads
	// this frozen anchor instead of the live focus; the "do I still need to
	// emit a focus op" decision keeps using live focus. Empty outside a
	// transaction (planner unit tests fall back to Observed.Focus).
	SummonFocusAnchor LiveWindowID
	// WindowProvenance records the live window ID that projwm spawned/adopted
	// for each desired identity (SSOT §6.9 / §6.9.1). It is the primary
	// attribution signal for single-process apps (Zed) where title is ambiguous
	// and process attribution is impossible. It is a VALIDATED CACHE: every
	// observe cycle re-checks the live ID is present with the expected
	// bundle/title, dropping stale entries (window-ID reuse / silent close). A
	// window not in this map is never claimed by provenance — user windows with
	// colliding titles stay External. Persisted with the store.
	WindowProvenance map[DesiredWindowID]LiveWindowID
	// ConvergedLayoutHandles is the SSOT §4.3 / N-15 recovery-gate. For each
	// managed workspace it records the SET of managed live window IDs observed
	// at the last converged commit (where the column layout matched the
	// desired / AcceptedLayouts). The Tier-2 observe-accept path
	// (autoAcceptObservedReorders) treats an observed same-set-but-different-
	// order layout as a USER reorder (→ accept) ONLY when the ws's current
	// managed handle-set is UNCHANGED since that converged commit. A changed
	// handle-set means recovery or a structural change — OmniWM restart re-mints
	// every handle with a fresh instance-UUID (ow_<base64(<instanceUUID>:…)>),
	// and window add/close changes membership — so the layout is restored/placed
	// by the planner, NOT adopted. This is how "定常=accept / 復旧=restore" is
	// distinguished without an OmniWM intent channel. Not persisted: rebuilt on
	// the first converged commit each session (empty → autoAccept skips until
	// projwm has enforced the saved layout, the safe default).
	ConvergedLayoutHandles map[WorkspaceID][]LiveWindowID
}

// WorldState is the controller's view. design.md §8.
type WorldState struct {
	Environment ManagedEnvironment
	Desired     DesiredWorld
	Observed    ObservedWorld
	Predicted   *PredictedWorld
	Meta        ControllerMeta
}

// SlotOrder returns slot IDs ordered by their declared environment order (deterministic).
func (e *ManagedEnvironment) SlotOrder() []SlotID {
	out := make([]SlotID, 0, len(e.Workspaces.Slots))
	// stable sort by Order then ID
	cp := make([]SlotSpec, len(e.Workspaces.Slots))
	copy(cp, e.Workspaces.Slots)
	for i := 0; i < len(cp); i++ {
		for j := i + 1; j < len(cp); j++ {
			if cp[j].Order < cp[i].Order || (cp[j].Order == cp[i].Order && cp[j].ID < cp[i].ID) {
				cp[i], cp[j] = cp[j], cp[i]
			}
		}
	}
	for _, s := range cp {
		out = append(out, s.ID)
	}
	return out
}

// SlotByID looks up a slot spec.
func (e *ManagedEnvironment) SlotByID(id SlotID) (SlotSpec, bool) {
	for _, s := range e.Workspaces.Slots {
		if s.ID == id {
			return s, true
		}
	}
	return SlotSpec{}, false
}

// WorkspaceByID looks up a workspace spec.
func (e *ManagedEnvironment) WorkspaceByID(id WorkspaceID) (WorkspaceSpec, bool) {
	for _, w := range e.Workspaces.Workspaces {
		if w.ID == id {
			return w, true
		}
	}
	return WorkspaceSpec{}, false
}

// WorkspaceRole returns the role for a workspace ID, "" if unknown.
func (e *ManagedEnvironment) WorkspaceRole(id WorkspaceID) WorkspaceRole {
	if w, ok := e.WorkspaceByID(id); ok {
		return w.Role
	}
	return ""
}

// SlotForWorkspace returns the slot whose workspace matches id, if any.
func (e *ManagedEnvironment) SlotForWorkspace(id WorkspaceID) (SlotSpec, bool) {
	for _, s := range e.Workspaces.Slots {
		if s.Workspace == id {
			return s, true
		}
	}
	return SlotSpec{}, false
}

// ActiveProfile returns the desired profile for the active profile id.
func (d *DesiredWorld) ActiveProfileObj() (DesiredProfile, bool) {
	p, ok := d.Profiles[d.ActiveProfile]
	return p, ok
}

// ProjectAssignedSlot returns the slot assigned to the given project in the active profile.
func (d *DesiredWorld) ProjectAssignedSlot(p ProjectID) (SlotID, bool) {
	prof, ok := d.ActiveProfileObj()
	if !ok {
		return "", false
	}
	for slot, pid := range prof.Assignments {
		if pid == p {
			return slot, true
		}
	}
	return "", false
}

// IsProjectActive: project is "active" iff non-archived AND assigned to a slot in active profile.
func (d *DesiredWorld) IsProjectActive(p ProjectID) bool {
	pr, ok := d.Projects[p]
	if !ok || pr.Archived {
		return false
	}
	_, assigned := d.ProjectAssignedSlot(p)
	return assigned
}
