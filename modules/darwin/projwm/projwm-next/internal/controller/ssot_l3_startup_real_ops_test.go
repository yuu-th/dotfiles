//go:build real_ops

package controller

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/session"
	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func requireSSOTStartupRealOps(t *testing.T) {
	t.Helper()
	if os.Getenv("PROJWM_REAL_OP_TESTS") != "1" {
		t.Skip("set PROJWM_REAL_OP_TESTS=1 to run SSOT startup recovery real_ops tests")
	}
	for _, bin := range []string{"omniwmctl", "tmux", "ghostty"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available: %v", bin, err)
		}
	}
}

func TestStartupNormal(t *testing.T) {
	requireSSOTStartupRealOps(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	env := startupEnv()
	projectRoot := t.TempDir()
	desired := startupDesired(projectRoot, true)
	adapter := newStartupRealAdapter(env)
	live := startupSpawnShell(t, ctx, adapter.SigWM, projectRoot, "shell-1:projwm", "projwm-next-startup-normal/projwm")
	t.Cleanup(func() {
		startupCleanupShell(t, adapter.SigWM, live, "shell-1:projwm", "projwm-next-startup-normal/projwm")
	})
	st, err := store.OpenFileStore(ctx, t.TempDir(), store.StoreKindTest, desired)
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	gen, err := st.LoadCurrentGeneration(ctx)
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	ctrl := NewFromGeneration(env, gen, adapter, st)
	ctrl.RuntimeValidator = startupRuntimeValidator{}

	result, err := ctrl.ApplyEvent(ctx, event.Event{Kind: event.KindStartup, Source: event.SourceSystem})
	if err != nil {
		t.Fatalf("startup normal: %v", err)
	}
	if result.Trace.TotalOperations != 0 || result.Trace.MutationOperations != 0 {
		t.Fatalf("startup normal must do no work when store and observed state match: %+v", result.Trace)
	}
}

func TestStartupMissingWindow(t *testing.T) {
	requireSSOTStartupRealOps(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	env := startupEnv()
	projectRoot := t.TempDir()
	desired := startupDesired(projectRoot, true)
	st, err := store.OpenFileStore(ctx, t.TempDir(), store.StoreKindTest, desired)
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	gen, err := st.LoadCurrentGeneration(ctx)
	if err != nil {
		t.Fatalf("LoadCurrentGeneration: %v", err)
	}
	adapter := newStartupRealAdapter(env)
	startupCleanupTitle(t, adapter.SigWM, "shell-1:projwm", "projwm-next-startup-missing/projwm")
	ctrl := NewFromGeneration(env, gen, adapter, st)
	ctrl.RuntimeValidator = startupRuntimeValidator{}

	if _, err := ctrl.ApplyEvent(ctx, event.Event{Kind: event.KindStartup, Source: event.SourceSystem}); err != nil {
		t.Fatalf("startup missing-window reconcile: %v", err)
	}
	obs, err := adapter.Observe(ctx)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if startupCountTitle(obs, "shell-1:projwm") != 1 {
		t.Fatalf("startup missing-window must spawn the desired window, observed=%+v", obs.Windows)
	}
	for id, win := range obs.Windows {
		if win.Title.Value == "shell-1:projwm" {
			startupCleanupShell(t, adapter.SigWM, id, "shell-1:projwm", "projwm-next-startup-missing/projwm")
		}
	}
}

func TestStartupOrphanWindow(t *testing.T) {
	requireSSOTStartupRealOps(t)
	t.Run("parseable title is re-registered", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		env := startupEnv()
		projectRoot := t.TempDir()
		desired := startupEmptyDesired()
		adapter := newStartupRealAdapter(env)
		live := startupSpawnShell(t, ctx, adapter.SigWM, projectRoot, "shell-1:unknown", "projwm-next-startup-orphan/unknown")
		t.Cleanup(func() {
			startupCleanupShell(t, adapter.SigWM, live, "shell-1:unknown", "projwm-next-startup-orphan/unknown")
		})
		st := store.NewMemoryStore(desired)
		ctrl := New(env, desired, adapter, st)
		ctrl.RuntimeValidator = startupRuntimeValidator{}

		if _, err := ctrl.ApplyEvent(ctx, event.Event{Kind: event.KindStartup, Source: event.SourceSystem}); err != nil {
			t.Fatalf("startup orphan-window transaction: %v", err)
		}
		startupAssertReconstructedShell(t, ctrl.State().Desired, "unknown", "shell-1:unknown")
	})
	t.Run("unparseable title is surfaced as orphan", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		env := startupEnv()
		projectRoot := t.TempDir()
		desired := startupEmptyDesired()
		adapter := newStartupRealAdapter(env)
		title := "random-window"
		tmuxSession := "projwm-next-startup-orphan/random-window"
		live := startupSpawnShell(t, ctx, adapter.SigWM, projectRoot, title, tmuxSession)
		t.Cleanup(func() {
			startupCleanupShell(t, adapter.SigWM, live, title, tmuxSession)
		})
		st := store.NewMemoryStore(desired)
		ctrl := New(env, desired, adapter, st)
		ctrl.RuntimeValidator = startupRuntimeValidator{}

		if _, err := ctrl.ApplyEvent(ctx, event.Event{Kind: event.KindStartup, Source: event.SourceSystem}); err != nil {
			t.Fatalf("startup orphan-window transaction: %v", err)
		}
		startupAssertOrphanSurfaced(t, ctrl.State().Meta, title)
	})
}

func TestStartupStateCorrupted(t *testing.T) {
	requireSSOTStartupRealOps(t)
	t.Run("recovers from CURRENT.bak", func(t *testing.T) {
		ctx := context.Background()
		root := t.TempDir()
		desired := startupDesired(t.TempDir(), false)
		if _, err := store.OpenFileStore(ctx, root, store.StoreKindTest, desired); err != nil {
			t.Fatalf("OpenFileStore bootstrap: %v", err)
		}
		current := filepath.Join(root, "CURRENT")
		good, err := os.ReadFile(current)
		if err != nil {
			t.Fatalf("read CURRENT: %v", err)
		}
		if err := os.WriteFile(current+".bak", good, 0o644); err != nil {
			t.Fatalf("write CURRENT.bak: %v", err)
		}
		if err := os.WriteFile(current, []byte("not-a-generation\n"), 0o644); err != nil {
			t.Fatalf("corrupt CURRENT: %v", err)
		}
		st, err := store.OpenExistingFileStore(ctx, root, store.StoreKindTest)
		if err != nil {
			t.Fatalf("startup must recover corrupted CURRENT from CURRENT.bak: %v", err)
		}
		gen, err := st.LoadCurrentGeneration(ctx)
		if err != nil {
			t.Fatalf("LoadCurrentGeneration after CURRENT.bak recovery: %v", err)
		}
		if gen.Desired.ActiveProfile != desired.ActiveProfile {
			t.Fatalf("recovered desired active profile = %q, want %q", gen.Desired.ActiveProfile, desired.ActiveProfile)
		}
	})
	t.Run("without backup reconstructs from live windows", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		env := startupEnv()
		projectRoot := t.TempDir()
		desired := startupEmptyDesired()
		adapter := newStartupRealAdapter(env)
		title := "shell-1:projwm"
		tmuxSession := "projwm-next-startup-reconstruct/projwm"
		live := startupSpawnShell(t, ctx, adapter.SigWM, projectRoot, title, tmuxSession)
		t.Cleanup(func() {
			startupCleanupShell(t, adapter.SigWM, live, title, tmuxSession)
		})
		st := store.NewMemoryStore(desired)
		ctrl := New(env, desired, adapter, st)
		ctrl.RuntimeValidator = startupRuntimeValidator{}

		if _, err := ctrl.ApplyEvent(ctx, event.Event{Kind: event.KindStartup, Source: event.SourceSystem}); err != nil {
			t.Fatalf("startup corrupted-without-backup transaction: %v", err)
		}
		startupAssertReconstructedShell(t, ctrl.State().Desired, "projwm", title)
	})
}

func startupEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		WindowManager: w.WindowManagerEnvironment{Backend: "omniwm", Layout: w.LayoutTuning{MaxVisibleColumns: 2, MaxWindowsPerColumn: 4}},
		Workspaces: w.WorkspaceEnvironment{
			Workspaces: []w.WorkspaceSpec{{ID: "8", RawName: "8", DisplayName: "8", Role: w.WorkspaceProject}},
			Slots:      []w.SlotSpec{{ID: "Q", Workspace: "8", Order: 1}},
		},
		Apps: w.AppEnvironment{ManagedApps: []w.ManagedAppPolicy{{
			Capability: w.CapabilityTerminal,
			BundleID:   "com.mitchellh.ghostty",
		}}},
	}
}

func startupDesired(projectRoot string, withWindow bool) w.DesiredWorld {
	project := w.DesiredProject{ID: "projwm", Root: projectRoot, Layouts: map[w.WorkspaceID]w.DesiredLayout{}}
	if withWindow {
		id := w.DesiredWindowID{Project: "projwm", Kind: w.WindowShell, Index: 1}
		project.Windows = []w.DesiredWindow{{
			ID:   id,
			Kind: w.WindowShell,
			App:  w.AppRequirement{BundleID: "com.mitchellh.ghostty"},
			TitleContract: w.TitleContract{
				Authority: w.TitleControllerOwned,
				Expected:  "shell-1:projwm",
				Drift:     w.TitleDriftRepair,
			},
		}}
		project.Layouts["8"] = w.DesiredLayout{
			Workspace: "8",
			Columns:   []w.DesiredColumn{{Windows: []w.DesiredWindowID{id}, Mode: w.ColumnSolo}},
			Source:    w.LayoutAuthorityImported,
		}
	}
	return w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", Assignments: map[w.SlotID]w.ProjectID{"Q": "projwm"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{"projwm": project},
	}
}

func startupEmptyDesired() w.DesiredWorld {
	return w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
}

type startupRuntimeValidator struct{}

func (startupRuntimeValidator) ValidateEnvironment(ctx context.Context, env w.ManagedEnvironment) ([]store.RuntimeValidationReport, bool, error) {
	return nil, false, nil
}

type startupRealAdapter struct {
	*wm.SigWM
}

func newStartupRealAdapter(env w.ManagedEnvironment) startupRealAdapter {
	sw := wm.NewSigWM(env, nil, nil)
	sw.Tmux = &session.Client{}
	return startupRealAdapter{SigWM: sw}
}

func (a startupRealAdapter) Observe(ctx context.Context) (w.ObservedWorld, error) {
	obs, err := a.SigWM.Observe(ctx)
	if err != nil {
		return obs, err
	}
	// Startup recovery B1-B3 is about project windows. Remove display
	// topology so cockpit-sync does not turn this unit operation test into
	// a display/cockpit integration scenario.
	obs.Displays = w.ObservedDisplayState{}
	return obs, nil
}

func startupSpawnShell(t *testing.T, ctx context.Context, sw *wm.SigWM, projectRoot, title, tmuxSession string) w.LiveWindowID {
	t.Helper()
	startupCleanupTitle(t, sw, title, tmuxSession)
	live, err := sw.Spawn(ctx, wm.SpawnRequest{
		Workspace:   "8",
		Kind:        w.WindowShell,
		Desired:     w.DesiredWindowID{Project: "projwm", Kind: w.WindowShell, Index: 1},
		Title:       title,
		BundleID:    "com.mitchellh.ghostty",
		ProjectPath: projectRoot,
		TmuxSession: tmuxSession,
	})
	if err != nil {
		t.Fatalf("spawn startup shell %q: %v", title, err)
	}
	if live == "" {
		t.Fatalf("spawn startup shell %q returned empty live id", title)
	}
	return live
}

func startupCleanupTitle(t *testing.T, sw *wm.SigWM, title, tmuxSession string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = (&session.Client{}).KillSession(ctx, tmuxSession)
	obs, err := sw.Observe(ctx)
	if err != nil {
		return
	}
	for id, win := range obs.Windows {
		if win.Title.Value == title {
			startupCleanupShell(t, sw, id, title, tmuxSession)
		}
	}
}

func startupCleanupShell(t *testing.T, sw *wm.SigWM, live w.LiveWindowID, title, tmuxSession string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = sw.TerminateManagedAppInstance(ctx, wm.TerminateManagedAppInstanceRequest{
		LiveWindow: live,
		Desired:    w.DesiredWindowID{Project: "projwm", Kind: w.WindowShell, Index: 1},
		Kind:       w.WindowShell,
		Title:      title,
		BundleID:   "com.mitchellh.ghostty",
	})
	_ = (&session.Client{}).KillSession(ctx, tmuxSession)
}

func startupCountTitle(obs w.ObservedWorld, title string) int {
	count := 0
	for _, win := range obs.Windows {
		if win.Title.Value == title {
			count++
		}
	}
	return count
}

func startupAssertReconstructedShell(t *testing.T, desired w.DesiredWorld, projectID string, title string) {
	t.Helper()
	project, ok := desired.Projects[w.ProjectID(projectID)]
	if !ok {
		t.Fatalf("startup live reconstruction did not recreate project %q in DesiredWorld: %+v", projectID, desired.Projects)
	}
	if len(project.Windows) != 1 {
		t.Fatalf("reconstructed project %q windows = %+v, want exactly one shell window", projectID, project.Windows)
	}
	win := project.Windows[0]
	if win.ID.Project != w.ProjectID(projectID) || win.ID.Kind != w.WindowShell || win.ID.Index != 1 || win.Kind != w.WindowShell {
		t.Fatalf("reconstructed window identity = %+v kind=%q, want shell-1:%s", win.ID, win.Kind, projectID)
	}
	if win.TitleContract.Authority != w.TitleControllerOwned || win.TitleContract.Expected != title {
		t.Fatalf("reconstructed title contract = %+v, want controller-owned %q", win.TitleContract, title)
	}
	profile, ok := desired.Profiles[desired.ActiveProfile]
	if !ok {
		t.Fatalf("reconstructed DesiredWorld active profile %q is missing", desired.ActiveProfile)
	}
	if got := profile.Assignments["Q"]; got != w.ProjectID(projectID) {
		t.Fatalf("reconstructed slot Q assignment = %q, want %q", got, projectID)
	}
}

func startupAssertOrphanSurfaced(t *testing.T, meta w.ControllerMeta, title string) {
	t.Helper()
	for _, orphan := range meta.PendingOrphans {
		if orphan.Title == title {
			return
		}
	}
	for _, card := range meta.ActiveCards {
		if card.Subject == title || card.Context["title"] == title || card.Context["windowTitle"] == title {
			return
		}
	}
	t.Fatalf("startup unparseable live window %q was not surfaced as orphan: meta=%+v", title, meta)
}
