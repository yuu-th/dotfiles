package migration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestDesiredWorldFromLegacyStateWhitelistsDurableFields(t *testing.T) {
	const legacy = `{
  "active_profile": "work",
  "profiles": {
    "work": {
      "description": "main",
      "assignments": {"Q": "dotfiles"}
    }
  },
  "projects": {
    "dotfiles": {
      "cwd": "/Users/yuta/dev/dotfiles",
      "archived": false,
      "windows": [
        {"id": 1, "kind": "editor", "layout": {"column": 0, "stack": 0}},
        {"id": 1, "kind": "ai", "ai": "claude", "layout": {"column": 1, "stack": 0}},
        {"id": 2, "kind": "shell", "live_window_id": "live-unsafe", "layout": {"column": 1, "stack": 1}},
        {"id": 1, "kind": "browser", "saved_urls": ["https://secret.example"], "layout": {"column": 2, "stack": 0}}
      ]
    }
  }
}`
	desired, report, err := DesiredWorldFromLegacyState([]byte(legacy))
	if err != nil {
		t.Fatalf("DesiredWorldFromLegacyState: %v", err)
	}
	if desired.ActiveProfile != "work" {
		t.Fatalf("ActiveProfile = %q", desired.ActiveProfile)
	}
	if got := desired.Profiles["work"].Assignments["Q"]; got != "dotfiles" {
		t.Fatalf("assignment Q = %q", got)
	}
	project := desired.Projects["dotfiles"]
	if project.Root != "/Users/yuta/dev/dotfiles" {
		t.Fatalf("project root = %q", project.Root)
	}
	if len(project.Windows) != 4 {
		t.Fatalf("windows = %d, want 4", len(project.Windows))
	}
	if project.Windows[0].Kind != w.WindowEditor || project.Windows[0].ID.Index != 1 {
		t.Fatalf("first window = %+v", project.Windows[0])
	}
	if report.BrowserURLRecords != 1 {
		t.Fatalf("BrowserURLRecords = %d, want 1", report.BrowserURLRecords)
	}
	b, err := json.Marshal(desired)
	if err != nil {
		t.Fatalf("marshal desired: %v", err)
	}
	if stringContains(string(b), "secret.example") || stringContains(string(b), "live-unsafe") {
		t.Fatalf("desired world leaked quarantined legacy data: %s", string(b))
	}
	layout := desired.AcceptedLayouts["dotfiles"]["Q"]
	if len(layout.Columns) != 3 {
		t.Fatalf("layout columns = %d, want 3", len(layout.Columns))
	}
	if layout.Columns[1].Mode != w.ColumnStacked {
		t.Fatalf("middle column mode = %q, want stacked", layout.Columns[1].Mode)
	}
}

func TestDesiredWorldFromLegacyStateRejectsUnknownProjectAssignment(t *testing.T) {
	const legacy = `{
  "active_profile": "work",
  "profiles": {"work": {"assignments": {"Q": "missing"}}},
  "projects": {}
}`
	if _, _, err := DesiredWorldFromLegacyState([]byte(legacy)); err == nil {
		t.Fatal("expected unknown project assignment error")
	}
}

func TestMigrateLegacySavedURLsToPrivatePayloadStoreDoesNotLeakReport(t *testing.T) {
	const canary = "SHOULD_NOT_APPEAR"
	legacy := `{
  "projects": {
    "dotfiles": {
      "cwd": "/Users/yuta/dev/dotfiles",
      "windows": [
        {"id": 1, "kind": "browser", "saved_urls": ["https://browser-fixture.test/` + canary + `", "javascript:alert('x')", "not a url"]}
      ]
    }
  }
}`
	root := t.TempDir()
	privateStore, err := browser.NewFilePrivatePayloadStore(root)
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	report, err := MigrateLegacySavedURLsToPrivatePayloadStore(context.Background(), []byte(legacy), privateStore)
	if err != nil {
		t.Fatalf("MigrateLegacySavedURLsToPrivatePayloadStore: %v", err)
	}
	if report.Discovered != 3 || report.MigratedToPrivatePayload != 1 || report.SkippedInvalid != 2 || report.CommittedRawURLs != 0 || !report.PrivatePayloadRedacted {
		t.Fatalf("report = %+v", report)
	}
	rawReport, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if stringContains(string(rawReport), canary) || stringContains(string(rawReport), "browser-fixture.test") {
		t.Fatalf("migration report leaked private browser payload: %s", rawReport)
	}
	privateFiles, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read private store: %v", err)
	}
	if len(privateFiles) != 1 {
		t.Fatalf("private store files = %d, want 1", len(privateFiles))
	}
	privatePayload, err := os.ReadFile(filepath.Join(root, privateFiles[0].Name()))
	if err != nil {
		t.Fatalf("read private payload: %v", err)
	}
	if !stringContains(string(privatePayload), canary) {
		t.Fatalf("private payload store did not receive canary URL: %s", privatePayload)
	}
}

func TestDesiredWorldFromLegacyStateWithPrivatePayloadStorePersistsOnlyOpaqueRefs(t *testing.T) {
	const canary = "SHOULD_NOT_APPEAR"
	projectRoot := t.TempDir()
	legacy := `{
  "active_profile": "work",
  "profiles": {"work": {"assignments": {"Q": "dotfiles"}}},
  "projects": {
    "dotfiles": {
      "cwd": "` + projectRoot + `",
      "windows": [
        {"id": 1, "kind": "browser", "saved_urls": ["https://browser-fixture.test/` + canary + `"]}
      ]
    }
  }
}`
	privateStore, err := browser.NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	desired, _, privateReport, err := DesiredWorldFromLegacyStateWithPrivatePayloadStore(context.Background(), []byte(legacy), privateStore)
	if err != nil {
		t.Fatalf("DesiredWorldFromLegacyStateWithPrivatePayloadStore: %v", err)
	}
	if privateReport.Discovered != 1 || privateReport.MigratedToPrivatePayload != 1 || privateReport.CommittedRawURLs != 0 {
		t.Fatalf("private report = %+v", privateReport)
	}
	window := desired.Projects["dotfiles"].Windows[0]
	if window.Browser == nil {
		t.Fatal("browser session missing private payload ref")
	}
	if window.Browser.PrivacyMode != w.BrowserSnapshotPrivateContent || window.Browser.RestoreURLs {
		t.Fatalf("browser session policy = %+v", window.Browser)
	}
	if len(window.Browser.URLPayloadRefs) != 1 || string(window.Browser.URLPayloadRefs[0]) == "" {
		t.Fatalf("payload refs = %+v", window.Browser.URLPayloadRefs)
	}
	rawDesired, err := json.Marshal(desired)
	if err != nil {
		t.Fatalf("marshal desired: %v", err)
	}
	if stringContains(string(rawDesired), canary) || stringContains(string(rawDesired), "browser-fixture.test") {
		t.Fatalf("desired world leaked raw URL: %s", rawDesired)
	}
	payload, err := privateStore.Get(context.Background(), string(window.Browser.URLPayloadRefs[0]))
	if err != nil {
		t.Fatalf("Get private payload: %v", err)
	}
	if len(payload.URLs) != 1 || !stringContains(payload.URLs[0], canary) {
		t.Fatalf("private payload = %+v", payload)
	}
}

func TestDesiredWorldFromLegacyStateWithPrivatePayloadStoreDropsBrowserWithoutPayload(t *testing.T) {
	projectRoot := t.TempDir()
	legacy := `{
  "active_profile": "work",
  "profiles": {"work": {"assignments": {"Q": "dotfiles"}}},
  "projects": {
    "dotfiles": {
      "cwd": "` + projectRoot + `",
      "windows": [
        {"id": 1, "kind": "editor", "layout": {"column": 0, "stack": 0}},
        {"id": 1, "kind": "browser", "saved_urls": ["vivaldi://settings"], "layout": {"column": 1, "stack": 0}}
      ]
    }
  }
}`
	privateStore, err := browser.NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	desired, _, privateReport, err := DesiredWorldFromLegacyStateWithPrivatePayloadStore(context.Background(), []byte(legacy), privateStore)
	if err != nil {
		t.Fatalf("DesiredWorldFromLegacyStateWithPrivatePayloadStore: %v", err)
	}
	if privateReport.Discovered != 1 || privateReport.SkippedInvalid != 1 || privateReport.DroppedBrowserWindowsWithoutPayload != 1 {
		t.Fatalf("private report = %+v", privateReport)
	}
	project := desired.Projects["dotfiles"]
	if len(project.Windows) != 1 || project.Windows[0].Kind != w.WindowEditor {
		t.Fatalf("windows after prune = %+v", project.Windows)
	}
	for _, layout := range []w.DesiredLayout{project.Layouts["Q"], desired.AcceptedLayouts["dotfiles"]["Q"]} {
		for _, column := range layout.Columns {
			for _, id := range column.Windows {
				if id.Kind == w.WindowBrowser {
					t.Fatalf("layout retained dropped browser id: %+v", layout)
				}
			}
		}
	}
}

func TestDesiredWorldFromLegacyStateWithPrivatePayloadStoreArchivesMissingProjectRoots(t *testing.T) {
	legacy := `{
  "active_profile": "work",
  "profiles": {"work": {"assignments": {"Q": "missing-root"}}},
  "projects": {
    "missing-root": {
      "cwd": "/path/that/does/not/exist/projwm-next-test",
      "windows": [
        {"id": 1, "kind": "editor", "layout": {"column": 0, "stack": 0}}
      ]
    }
  }
}`
	privateStore, err := browser.NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	desired, report, _, err := DesiredWorldFromLegacyStateWithPrivatePayloadStore(context.Background(), []byte(legacy), privateStore)
	if err != nil {
		t.Fatalf("DesiredWorldFromLegacyStateWithPrivatePayloadStore: %v", err)
	}
	if report.MissingProjectRoots != 1 {
		t.Fatalf("MissingProjectRoots = %d, want 1", report.MissingProjectRoots)
	}
	if !desired.Projects["missing-root"].Archived {
		t.Fatalf("missing-root project was not archived: %+v", desired.Projects["missing-root"])
	}
	if len(desired.Profiles["work"].Assignments) != 0 {
		t.Fatalf("missing-root assignment was not removed: %+v", desired.Profiles["work"].Assignments)
	}
	if _, ok := desired.AcceptedLayouts["missing-root"]; ok {
		t.Fatalf("accepted layout retained missing-root project: %+v", desired.AcceptedLayouts["missing-root"])
	}
	rawReport, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if stringContains(string(rawReport), "/path/that/does/not/exist") {
		t.Fatalf("report leaked missing project root: %s", rawReport)
	}
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
