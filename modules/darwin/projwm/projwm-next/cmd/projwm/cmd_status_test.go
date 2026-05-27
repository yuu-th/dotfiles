package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// newProjectStore creates an isolated FileStore + manifest pair seeded
// with a one-profile DesiredWorld so cmd_status tests can exercise the
// human and JSON renderers.
func newProjectStore(t *testing.T) (gf globalFlags, storeDir string) {
	t.Helper()
	tmp := t.TempDir()
	storeDir = filepath.Join(tmp, "store")
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {
				ID:             "work",
				Description:    "primary profile",
				InactivePolicy: w.InactivePolicyRemove,
				Assignments: map[w.SlotID]w.ProjectID{
					"Q": "dotfiles",
				},
			},
			"misc": {
				ID:             "misc",
				InactivePolicy: w.InactivePolicyKeep,
				Assignments:    map[w.SlotID]w.ProjectID{},
			},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {ID: "dotfiles"},
			"spike-x":  {ID: "spike-x"},                  // parked
			"old":      {ID: "old", Archived: true},
		},
	}
	if _, err := store.OpenFileStore(context.Background(), storeDir, store.StoreKindTest, desired); err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	manifest := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifest, minimalManifestJSON(t), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return globalFlags{storeDir: storeDir, manifestPath: manifest}, storeDir
}

func TestCmdStatus_Human(t *testing.T) {
	gf, _ := newProjectStore(t)
	var stdout, stderr bytes.Buffer
	if err := cmdStatus(gf, nil, &stdout, &stderr); err != nil {
		t.Fatalf("cmdStatus: %v (stderr=%s)", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Active:", "work", "Q (workspace=Q)",
		"dotfiles", "Parked projects", "spike-x",
		"Archived projects", "old",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestStatusConvergenceVocabularyReflectsState is an owner test for SSOT §3.1
// / §5.6 #8 (previously §10.9 GAP-02 convergence half): the user-visible
// convergence string is the store-level signal that distinguishes the
// "正常稼働" state (no outstanding work → CONVERGED) from the "復旧中" /
// pending-work state (DirtyScopes recorded → CONVERGING). This is what the
// user sees in `projwm status` / cockpit topbar to tell those states apart.
func TestStatusConvergenceVocabularyReflectsState(t *testing.T) {
	if got := convergenceFromCheckpoint(nil); got != "CONVERGED" {
		t.Errorf("§5.6 #8: no outstanding work must read CONVERGED, got %q", got)
	}
	if got := convergenceFromCheckpoint([]w.DirtyScope{}); got != "CONVERGED" {
		t.Errorf("§5.6 #8: empty scopes must read CONVERGED, got %q", got)
	}
	if got := convergenceFromCheckpoint([]w.DirtyScope{{Kind: "global"}}); got != "CONVERGING" {
		t.Errorf("§5.6 #8/§3.1 復旧中: outstanding DirtyScopes must read CONVERGING (user sees pending recovery), got %q", got)
	}
	if got := convergenceFromCheckpoint([]w.DirtyScope{{Kind: "layout-sync", Key: "dotfiles|Q"}, {Kind: "global"}}); got != "CONVERGING" {
		t.Errorf("§5.6 #8: multiple outstanding scopes must read CONVERGING, got %q", got)
	}
}

func TestCmdStatus_JSON(t *testing.T) {
	gf, _ := newProjectStore(t)
	var stdout, stderr bytes.Buffer
	if err := cmdStatus(gf, []string{"--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("cmdStatus --json: %v (stderr=%s)", err, stderr.String())
	}
	var resp statusJSON
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout.String())
	}
	if resp.ActiveProfile != "work" {
		t.Errorf("ActiveProfile = %q", resp.ActiveProfile)
	}
	if _, ok := resp.Profiles["work"]; !ok {
		t.Errorf("missing work profile in JSON")
	}
	if _, ok := resp.Projects["old"]; !ok {
		t.Errorf("missing archived project in JSON")
	}
	if !containsProject(resp.Parked, "spike-x") {
		t.Errorf("Parked should contain spike-x, got %v", resp.Parked)
	}
	if !containsProject(resp.Archived, "old") {
		t.Errorf("Archived should contain old, got %v", resp.Archived)
	}

	// SSOT §5.6 status #8: convergence MUST appear (store-derived).
	// Empty DirtyScopes in the test fixture → CONVERGED.
	if resp.Convergence != "CONVERGED" {
		t.Errorf("SSOT §5.6 #8: Convergence = %q, want CONVERGED for clean checkpoint", resp.Convergence)
	}
	// SSOT §5.6 status #9: manifest digest. Fixture leaves digest
	// unconfigured → UNCHECKED honest value.
	if resp.ManifestDigest == "" {
		t.Errorf("SSOT §5.6 #9: ManifestDigest must be populated (got empty), want UNCHECKED/OK/MISMATCH")
	}
}

func containsProject(in []w.ProjectID, want w.ProjectID) bool {
	for _, p := range in {
		if p == want {
			return true
		}
	}
	return false
}

func TestCmdProfileList(t *testing.T) {
	gf, _ := newProjectStore(t)
	var stdout, stderr bytes.Buffer
	if err := cmdProfile(gf, []string{"list"}, &stdout, &stderr); err != nil {
		t.Fatalf("cmdProfile list: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "* work") {
		t.Errorf("active profile marker missing\n%s", out)
	}
	if !strings.Contains(out, "  misc") {
		t.Errorf("inactive profile missing\n%s", out)
	}
}

func TestCmdProfileShow_Default(t *testing.T) {
	gf, _ := newProjectStore(t)
	var stdout, stderr bytes.Buffer
	if err := cmdProfile(gf, []string{"show"}, &stdout, &stderr); err != nil {
		t.Fatalf("cmdProfile show: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Profile: work (active)") {
		t.Errorf("expected active marker\n%s", out)
	}
}

func TestCmdProfileShow_Unknown(t *testing.T) {
	gf, _ := newProjectStore(t)
	var stdout, stderr bytes.Buffer
	err := cmdProfile(gf, []string{"show", "no-such"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got: %v", err)
	}
}

func TestCmdArchiveList(t *testing.T) {
	gf, _ := newProjectStore(t)
	var stdout, stderr bytes.Buffer
	if err := cmdArchive(gf, []string{"list"}, &stdout, &stderr); err != nil {
		t.Fatalf("cmdArchive list: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "old" {
		t.Errorf("expected 'old', got %q", got)
	}
}

func TestCmdArchive_RequiresArg(t *testing.T) {
	gf, _ := newProjectStore(t)
	var stdout, stderr bytes.Buffer
	err := cmdArchive(gf, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected usage error")
	}
}

func TestResolveJumpTarget(t *testing.T) {
	gf, _ := newProjectStore(t)
	snap, err := loadSnapshotFromStore(context.Background(), gf)
	if err != nil {
		t.Fatalf("loadSnapshot: %v", err)
	}
	cases := []struct {
		in       string
		wantWS   w.WorkspaceID
		wantKind string
	}{
		{"Q", "Q", "slot"},
		{"work", "Q", "profile"},  // first slot of work profile is Q
		{"dotfiles", "Q", "project"},
		{"A", "A", "workspace"},
		{"no-such", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			ws, kind, ok := resolveJumpTarget(snap, tc.in)
			if tc.wantWS == "" {
				if ok {
					t.Errorf("expected no resolution, got ws=%s kind=%s", ws, kind)
				}
				return
			}
			if !ok {
				t.Fatalf("expected resolution for %s", tc.in)
			}
			if ws != tc.wantWS || kind != tc.wantKind {
				t.Errorf("got ws=%s kind=%s, want ws=%s kind=%s", ws, kind, tc.wantWS, tc.wantKind)
			}
		})
	}
}

func TestParseWindowSpec(t *testing.T) {
	cases := []struct {
		in        string
		wantKind  w.WindowKind
		wantIndex int
		wantErr   bool
	}{
		{"ai-1", w.WindowAI, 1, false},
		{"shell-3", w.WindowShell, 3, false},
		{"editor-1", w.WindowEditor, 1, false},
		{"browser-2", w.WindowBrowser, 2, false},
		{"ai", "", 0, true},
		{"foo-1", "", 0, true},
		{"shell-0", "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			k, idx, err := parseWindowSpec(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr {
				if k != tc.wantKind || idx != tc.wantIndex {
					t.Errorf("got %s/%d want %s/%d", k, idx, tc.wantKind, tc.wantIndex)
				}
			}
		})
	}
}

func TestCmdTrace_RequiresStoreDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cmdTrace(globalFlags{}, []string{"--last"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "store-dir") {
		t.Errorf("expected store-dir error, got %v", err)
	}
}

func TestCmdTrace_NoTracesYet(t *testing.T) {
	gf, _ := newProjectStore(t)
	var stdout, stderr bytes.Buffer
	err := cmdTrace(gf, []string{"--last"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for empty traces dir")
	}
	if !strings.Contains(err.Error(), "no traces") {
		t.Errorf("expected no-traces error, got: %v", err)
	}
}

func TestCmdDoctor_RunsAllChecks(t *testing.T) {
	gf, _ := newProjectStore(t)
	var stdout, stderr bytes.Buffer
	// doctor will produce WARN/FAIL rows for missing daemon etc., but should
	// not panic, and exit-code-wise should error iff any FAIL row appears.
	_ = cmdDoctor(gf, nil, &stdout, &stderr)
	out := stdout.String()
	checks := allDoctorChecks()
	if want := len(checks); strings.Count(out, "\n") < want {
		t.Errorf("expected at least %d doctor lines, got %d:\n%s", want, strings.Count(out, "\n"), out)
	}
	for _, level := range []string{"[PASS]", "[WARN]", "[FAIL]"} {
		_ = level // just here for documentation
	}
}
