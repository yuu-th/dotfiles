// Package cockpitsnap is the shared snapshot type used by the
// projwm-cockpit TUI. Lives in internal/ so both the cockpit main and
// the bubbletea TUI package can import it without circular deps.
//
// Mirrors the JSON shape returned by projwmd's QueryWorld response,
// with extra fields surfaced by the cockpit (convergence status,
// manifest digest verify, tmux/live window state).
package cockpitsnap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yuu-th/projwm-next/internal/manifest"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Snapshot is the cockpit's local mirror of WorldState.
type Snapshot struct {
	Generation      w.GenerationID                                    `json:"generation"`
	Parent          *w.GenerationID                                   `json:"parent,omitempty"`
	Epoch           w.Epoch                                           `json:"epoch"`
	ActiveProfile   w.ProfileID                                       `json:"activeProfile"`
	Profiles        map[w.ProfileID]w.DesiredProfile                  `json:"profiles"`
	Projects        map[w.ProjectID]w.DesiredProject                  `json:"projects"`
	AcceptedLayouts map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout `json:"acceptedLayouts,omitempty"`
	Workspaces      []w.WorkspaceSpec                                 `json:"workspaces"`
	Slots           []w.SlotSpec                                      `json:"slots"`
	Parked          []w.ProjectID                                     `json:"parked"`
	Archived        []w.ProjectID                                     `json:"archived"`
	ActiveCards     []w.Card                                          `json:"activeCards,omitempty"`
	PendingOrphans  []w.OrphanCandidate                               `json:"pendingOrphans,omitempty"`

	// K4.4 / K4.5: convergence status + manifest digest verification.
	ConvergenceStatus      string `json:"convergenceStatus,omitempty"`
	ManifestDigestMismatch bool   `json:"manifestDigestMismatch,omitempty"`

	// K5.1: per-window state.
	TmuxSessions map[string]bool `json:"tmuxSessions,omitempty"`
	LiveWindows  []LiveWindow    `json:"liveWindows,omitempty"`

	// Source labels how we got the snapshot. "daemon" or "store".
	Source string `json:"-"`
}

// LiveWindow is a minimal projection of ObservedWindow used by the
// cockpit to render per-window state in the slot list.
type LiveWindow struct {
	ID        w.LiveWindowID     `json:"id"`
	Workspace w.WorkspaceID      `json:"workspace,omitempty"`
	Kind      w.WindowKind       `json:"kind,omitempty"`
	MatchedTo *w.DesiredWindowID `json:"matchedTo,omitempty"`
	Focused   bool               `json:"focused,omitempty"`
}

// LoadFromStore is the daemon-down fallback path.
func LoadFromStore(ctx context.Context, storeDir, manifestPath string) (Snapshot, error) {
	if storeDir == "" {
		return Snapshot{}, fmt.Errorf("store-dir is required")
	}
	if manifestPath == "" {
		return Snapshot{}, fmt.Errorf("manifest path is required")
	}
	env, err := manifest.LoadFromFile(manifestPath)
	if err != nil {
		return Snapshot{}, err
	}
	kind, err := detectStoreKind(storeDir)
	if err != nil {
		return Snapshot{}, err
	}
	fs, err := store.OpenExistingFileStore(ctx, storeDir, kind)
	if err != nil {
		return Snapshot{}, err
	}
	gen, err := fs.LoadCurrentGeneration(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		Generation:      gen.ID,
		Parent:          gen.Parent,
		Epoch:           gen.Checkpoint.Epoch,
		ActiveProfile:   gen.Desired.ActiveProfile,
		Profiles:        gen.Desired.Profiles,
		Projects:        gen.Desired.Projects,
		AcceptedLayouts: gen.AcceptedLayouts,
		Workspaces:      env.Workspaces.Workspaces,
		Slots:           env.Workspaces.Slots,
		Parked:          parked(gen.Desired),
		Archived:        archived(gen.Desired),
		Source:          "store",
	}, nil
}

func detectStoreKind(root string) (store.StoreKind, error) {
	path := filepath.Join(root, ".store_identity.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("store identity: %w", err)
	}
	var id struct {
		StoreKind store.StoreKind `json:"storeKind"`
	}
	if err := json.Unmarshal(b, &id); err != nil {
		return "", fmt.Errorf("store identity parse: %w", err)
	}
	if id.StoreKind == "" {
		return "", fmt.Errorf("store identity: storeKind missing")
	}
	return id.StoreKind, nil
}

func parked(d w.DesiredWorld) []w.ProjectID {
	assigned := map[w.ProjectID]bool{}
	for _, prof := range d.Profiles {
		for _, pid := range prof.Assignments {
			assigned[pid] = true
		}
	}
	var ids []w.ProjectID
	for id, p := range d.Projects {
		if p.Archived {
			continue
		}
		if assigned[id] {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func archived(d w.DesiredWorld) []w.ProjectID {
	var ids []w.ProjectID
	for id, p := range d.Projects {
		if p.Archived {
			ids = append(ids, id)
		}
	}
	return ids
}

// TmuxSessionForWindow returns the conventional tmux session name for a
// DesiredWindow, matching the runtime spawn naming.
func TmuxSessionForWindow(pid w.ProjectID, dw w.DesiredWindow) string {
	switch dw.Kind {
	case w.WindowAI:
		return fmt.Sprintf("ai-%d/%s", dw.ID.Index, pid)
	case w.WindowShell:
		return fmt.Sprintf("shell-%d/%s", dw.ID.Index, pid)
	case w.WindowViewer:
		return fmt.Sprintf("ai-%d/%s_v", dw.ID.Index, pid)
	}
	return ""
}
