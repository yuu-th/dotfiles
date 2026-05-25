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
