package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// cmdStatus implements `projwm status [--json]`.
//
// Reads the current generation from the local store and renders a
// human-readable or JSON summary. Daemon contact is not required —
// status remains useful when projwmd is stopped.
func cmdStatus(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	snap, err := loadSnapshotWithTimeout(gf, 5*time.Second)
	if err != nil {
		return err
	}
	if *asJSON {
		return emitStatusJSON(snap, stdout)
	}
	renderHuman(snap, stdout)
	return nil
}

// statusJSON is the public schema for `projwm status --json`.
// It MUST stay backward-compatible — only add fields, never rename or
// remove. Field tags are lowerCamel to match other projwm-next JSON.
type statusJSON struct {
	Generation       w.GenerationID                                    `json:"generation"`
	ParentGeneration *w.GenerationID                                   `json:"parent,omitempty"`
	Epoch            w.Epoch                                           `json:"epoch"`
	ActiveProfile    w.ProfileID                                       `json:"activeProfile"`
	Profiles         map[w.ProfileID]profileJSON                       `json:"profiles"`
	Projects         map[w.ProjectID]projectJSON                       `json:"projects"`
	AcceptedLayouts  map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout `json:"acceptedLayouts,omitempty"`
	Workspaces       []workspaceJSON                                   `json:"workspaces"`
	Slots            []slotJSON                                        `json:"slots"`
	Parked           []w.ProjectID                                     `json:"parked"`
	Archived         []w.ProjectID                                     `json:"archived"`
}

type profileJSON struct {
	Description    string                       `json:"description,omitempty"`
	InactivePolicy w.InactivePolicy             `json:"inactivePolicy"`
	Assignments    map[w.SlotID]w.ProjectID     `json:"assignments"`
}

type projectJSON struct {
	Archived bool                                  `json:"archived"`
	Windows  []w.DesiredWindow                     `json:"windows,omitempty"`
	Layouts  map[w.WorkspaceID]w.DesiredLayout     `json:"layouts,omitempty"`
}

type workspaceJSON struct {
	ID   w.WorkspaceID   `json:"id"`
	Role w.WorkspaceRole `json:"role"`
}

type slotJSON struct {
	ID        w.SlotID      `json:"id"`
	Workspace w.WorkspaceID `json:"workspace"`
	Order     int           `json:"order"`
}

func emitStatusJSON(snap WorldSnapshot, out io.Writer) error {
	resp := statusJSON{
		Generation:      snap.Generation,
		ParentGeneration: snap.ParentGeneration,
		Epoch:           snap.Checkpoint.Epoch,
		ActiveProfile:   snap.Desired.ActiveProfile,
		Profiles:        map[w.ProfileID]profileJSON{},
		Projects:        map[w.ProjectID]projectJSON{},
		AcceptedLayouts: snap.AcceptedLayouts,
		Parked:          snap.parkedProjects(),
		Archived:        snap.archivedProjects(),
	}
	// Alias field-tagged backref: statusJSON.Parent is the marshal name,
	// statusJSON.ParentGeneration is the internal name; renaming below.
	// (Inline struct alias trick keeps the json tag spelled "parent".)
	for id, p := range snap.Desired.Profiles {
		resp.Profiles[id] = profileJSON{
			Description:    p.Description,
			InactivePolicy: p.InactivePolicy,
			Assignments:    p.Assignments,
		}
	}
	for id, p := range snap.Desired.Projects {
		resp.Projects[id] = projectJSON{
			Archived: p.Archived,
			Windows:  p.Windows,
			Layouts:  p.Layouts,
		}
	}
	for _, ws := range snap.Environment.Workspaces.Workspaces {
		resp.Workspaces = append(resp.Workspaces, workspaceJSON{ID: ws.ID, Role: ws.Role})
	}
	for _, sl := range snap.Environment.Workspaces.Slots {
		resp.Slots = append(resp.Slots, slotJSON{ID: sl.ID, Workspace: sl.Workspace, Order: sl.Order})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		return fmt.Errorf("encode status json: %w", err)
	}
	return nil
}
