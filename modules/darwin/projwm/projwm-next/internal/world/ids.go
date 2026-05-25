// Package world owns the WorldState type and its constituents. See design.md §3-§8.
package world

// Identity primitives. design.md §4.
type (
	ProfileID     string
	ProjectID     string
	SlotID        string
	WorkspaceID   string
	DisplayID     string
	LiveWindowID  string
	TransactionID string
	Epoch         uint64
	OperationID   string
	EventID       string
	GenerationID  string
	StoreVersion  string
	PlanID        string
)

// DesiredWindowID is the controller-stable handle for a project window. design.md §4.
// Index is a stable ordinal within a project, NOT a display order.
type DesiredWindowID struct {
	Project ProjectID
	Kind    WindowKind
	Index   int
}
