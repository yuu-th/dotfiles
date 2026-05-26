package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
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
		return emitStatusJSON(snap, gf, stdout)
	}
	renderHuman(snap, stdout)
	return nil
}

// convergenceFromCheckpoint maps the store-recorded DirtyScopes to the
// SSOT §5.6 convergence vocabulary. REPLAN_FAILED is a daemon-runtime
// signal not derivable from the store alone — we surface the
// store-level interpretation honestly: empty scopes ⇒ CONVERGED;
// otherwise CONVERGING. The daemon-aware status path may overwrite
// this when daemon contact is enabled.
func convergenceFromCheckpoint(dirtyScopes []w.DirtyScope) string {
	if len(dirtyScopes) == 0 {
		return "CONVERGED"
	}
	return "CONVERGING"
}

// manifestDigestCheck compares the manifest file at gf.manifestPath
// against gf.manifestDigest. SSOT §5.6 item #9.
func manifestDigestCheck(gf globalFlags) string {
	if gf.manifestPath == "" || gf.manifestDigest == "" {
		return "UNCHECKED"
	}
	data, err := os.ReadFile(gf.manifestPath)
	if err != nil {
		return "UNCHECKED"
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) == gf.manifestDigest {
		return "OK"
	}
	return "MISMATCH"
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

	// Convergence is the store-derived convergence status (SSOT §5.6
	// item #8). CONVERGED when no DirtyScopes are pending; CONVERGING
	// when the controller has recorded outstanding work. REPLAN_FAILED
	// requires daemon runtime signal and is not derivable from the
	// store alone — when this command runs daemon-free, the value is
	// reported honestly as the store-level state.
	Convergence string `json:"convergence,omitempty"`

	// ManifestDigest reports the manifest digest comparison (SSOT §5.6
	// item #9). "OK" when the manifest at --managed-environment hashes
	// to the digest in --manifest-digest; "MISMATCH" otherwise;
	// "UNCHECKED" when either argument was omitted.
	ManifestDigest string `json:"manifestDigest,omitempty"`
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

func emitStatusJSON(snap WorldSnapshot, gf globalFlags, out io.Writer) error {
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
		Convergence:     convergenceFromCheckpoint(snap.Checkpoint.DirtyScopes),
		ManifestDigest:  manifestDigestCheck(gf),
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
