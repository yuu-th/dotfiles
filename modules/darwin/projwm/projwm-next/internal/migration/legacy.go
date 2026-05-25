// Package migration converts explicitly supplied legacy/admin inputs into a
// DesiredWorld seed for the first PersistentStore generation.
package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	w "github.com/yuu-th/projwm-next/internal/world"
)

type LegacyReport struct {
	Profiles            int      `json:"profiles"`
	Projects            int      `json:"projects"`
	Windows             int      `json:"windows"`
	BrowserURLRecords   int      `json:"browserUrlRecords"`
	MissingProjectRoots int      `json:"missingProjectRoots"`
	QuarantinedFields   []string `json:"quarantinedFields,omitempty"`
}

type LegacyPrivatePayloadReport struct {
	Discovered                          int  `json:"discovered"`
	MigratedToPrivatePayload            int  `json:"migratedToPrivatePayload"`
	SkippedInvalid                      int  `json:"skippedInvalid"`
	DroppedBrowserWindowsWithoutPayload int  `json:"droppedBrowserWindowsWithoutPayload"`
	CommittedRawURLs                    int  `json:"committedRawUrls"`
	PrivatePayloadRedacted              bool `json:"privatePayloadRedacted"`
}

type legacyState struct {
	ActiveProfile string                   `json:"active_profile"`
	Profiles      map[string]legacyProfile `json:"profiles"`
	Projects      map[string]legacyProject `json:"projects"`
}

type legacyProfile struct {
	Description string            `json:"description,omitempty"`
	Assignments map[string]string `json:"assignments"`
}

type legacyProject struct {
	CWD      string         `json:"cwd"`
	Archived bool           `json:"archived"`
	Windows  []legacyWindow `json:"windows"`
}

type legacyWindow struct {
	ID             int           `json:"id"`
	Kind           string        `json:"kind"`
	AI             string        `json:"ai,omitempty"`
	BrowserProfile string        `json:"browser_profile,omitempty"`
	SavedURLs      []string      `json:"saved_urls,omitempty"`
	LiveWindowID   string        `json:"live_window_id,omitempty"`
	Layout         *legacyLayout `json:"layout,omitempty"`
}

type legacyLayout struct {
	Column int  `json:"column"`
	Stack  int  `json:"stack"`
	Tabbed bool `json:"tabbed,omitempty"`
}

func DesiredWorldFromLegacyStateFile(path string) (w.DesiredWorld, LegacyReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return w.DesiredWorld{}, LegacyReport{}, fmt.Errorf("migration: read legacy state %s: %w", path, err)
	}
	return DesiredWorldFromLegacyState(data)
}

func DesiredWorldFromLegacyState(data []byte) (w.DesiredWorld, LegacyReport, error) {
	var legacy legacyState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return w.DesiredWorld{}, LegacyReport{}, fmt.Errorf("migration: parse legacy state: %w", err)
	}
	if legacy.Profiles == nil {
		legacy.Profiles = map[string]legacyProfile{}
	}
	if legacy.Projects == nil {
		legacy.Projects = map[string]legacyProject{}
	}
	if legacy.ActiveProfile == "" && len(legacy.Profiles) > 0 {
		return w.DesiredWorld{}, LegacyReport{}, fmt.Errorf("migration: legacy active_profile is required when profiles exist")
	}
	if legacy.ActiveProfile != "" {
		if _, ok := legacy.Profiles[legacy.ActiveProfile]; !ok {
			return w.DesiredWorld{}, LegacyReport{}, fmt.Errorf("migration: active_profile %q not present in profiles", legacy.ActiveProfile)
		}
	}

	report := LegacyReport{
		Profiles: len(legacy.Profiles),
		Projects: len(legacy.Projects),
	}
	desired := w.DesiredWorld{
		ActiveProfile:   w.ProfileID(legacy.ActiveProfile),
		Profiles:        map[w.ProfileID]w.DesiredProfile{},
		Projects:        map[w.ProjectID]w.DesiredProject{},
		AcceptedLayouts: map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout{},
	}

	profileNames := sortedKeys(legacy.Profiles)
	for _, name := range profileNames {
		prof := legacy.Profiles[name]
		assignments := map[w.SlotID]w.ProjectID{}
		for _, slot := range sortedKeys(prof.Assignments) {
			project := prof.Assignments[slot]
			if _, ok := legacy.Projects[project]; !ok {
				return w.DesiredWorld{}, LegacyReport{}, fmt.Errorf("migration: profile %q slot %q assigns unknown project %q", name, slot, project)
			}
			assignments[w.SlotID(slot)] = w.ProjectID(project)
		}
		desired.Profiles[w.ProfileID(name)] = w.DesiredProfile{
			ID:             w.ProfileID(name),
			Description:    prof.Description,
			Assignments:    assignments,
			InactivePolicy: w.InactivePolicyRemove,
		}
	}

	projectNames := sortedKeys(legacy.Projects)
	for _, name := range projectNames {
		project := legacy.Projects[name]
		dp := w.DesiredProject{
			ID:       w.ProjectID(name),
			Root:     project.CWD,
			Archived: project.Archived,
			Windows:  make([]w.DesiredWindow, 0, len(project.Windows)),
			Layouts:  map[w.WorkspaceID]w.DesiredLayout{},
		}
		for _, legacyWindow := range project.Windows {
			dw, err := migrateWindow(name, project.CWD, legacyWindow)
			if err != nil {
				return w.DesiredWorld{}, LegacyReport{}, err
			}
			if legacyWindow.LiveWindowID != "" {
				report.QuarantinedFields = append(report.QuarantinedFields, fmt.Sprintf("projects.%s.windows.%d.live_window_id", name, legacyWindow.ID))
			}
			report.BrowserURLRecords += len(legacyWindow.SavedURLs)
			dp.Windows = append(dp.Windows, dw)
			report.Windows++
		}
		desired.Projects[w.ProjectID(name)] = dp
	}

	for profileName, profile := range desired.Profiles {
		if profileName != desired.ActiveProfile {
			continue
		}
		for slot, projectID := range profile.Assignments {
			project := legacy.Projects[string(projectID)]
			layout, ok := migrateProjectLayout(projectID, w.WorkspaceID(slot), project.Windows)
			if !ok {
				continue
			}
			dp := desired.Projects[projectID]
			dp.Layouts[w.WorkspaceID(slot)] = layout
			desired.Projects[projectID] = dp
			desired.AcceptedLayouts[projectID] = map[w.WorkspaceID]w.DesiredLayout{w.WorkspaceID(slot): layout}
		}
	}

	sort.Strings(report.QuarantinedFields)
	return desired, report, nil
}

func DesiredWorldFromLegacyStateWithPrivatePayloadStore(ctx context.Context, data []byte, privateStore browser.PrivatePayloadStore) (w.DesiredWorld, LegacyReport, LegacyPrivatePayloadReport, error) {
	desired, report, err := DesiredWorldFromLegacyState(data)
	if err != nil {
		return w.DesiredWorld{}, LegacyReport{}, LegacyPrivatePayloadReport{}, err
	}
	privateReport, err := migrateLegacySavedURLsToPrivatePayloadStore(ctx, data, privateStore, &desired)
	if err != nil {
		return w.DesiredWorld{}, LegacyReport{}, LegacyPrivatePayloadReport{}, err
	}
	pruneBrowserWindowsWithoutPrivatePayload(&desired, &privateReport)
	pruneMissingProjectRoots(&desired, &report)
	sort.Strings(report.QuarantinedFields)
	return desired, report, privateReport, nil
}

func MigrateLegacySavedURLsToPrivatePayloadStore(ctx context.Context, data []byte, privateStore browser.PrivatePayloadStore) (LegacyPrivatePayloadReport, error) {
	return migrateLegacySavedURLsToPrivatePayloadStore(ctx, data, privateStore, nil)
}

func migrateLegacySavedURLsToPrivatePayloadStore(ctx context.Context, data []byte, privateStore browser.PrivatePayloadStore, desired *w.DesiredWorld) (LegacyPrivatePayloadReport, error) {
	if privateStore == nil {
		return LegacyPrivatePayloadReport{}, fmt.Errorf("migration: private payload store is required")
	}
	var legacy legacyState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return LegacyPrivatePayloadReport{}, fmt.Errorf("migration: parse legacy state: %w", err)
	}
	report := LegacyPrivatePayloadReport{PrivatePayloadRedacted: true}
	for _, projectName := range sortedKeys(legacy.Projects) {
		project := legacy.Projects[projectName]
		for _, legacyWindow := range project.Windows {
			if len(legacyWindow.SavedURLs) == 0 {
				continue
			}
			valid := make([]string, 0, len(legacyWindow.SavedURLs))
			for range legacyWindow.SavedURLs {
				report.Discovered++
			}
			for _, raw := range legacyWindow.SavedURLs {
				if !validLegacyBrowserURL(raw) {
					report.SkippedInvalid++
					continue
				}
				valid = append(valid, raw)
			}
			if len(valid) == 0 {
				continue
			}
			token, err := privateStore.Put(ctx, browser.PrivatePayload{URLs: valid})
			if err != nil {
				return LegacyPrivatePayloadReport{}, fmt.Errorf("migration: store private browser payload: %w", err)
			}
			if desired != nil {
				if err := attachBrowserPrivatePayloadRef(desired, projectName, legacyWindow, token, len(valid), len(legacyWindow.SavedURLs)-len(valid)); err != nil {
					return LegacyPrivatePayloadReport{}, err
				}
			}
			report.MigratedToPrivatePayload += len(valid)
		}
	}
	return report, nil
}

func pruneBrowserWindowsWithoutPrivatePayload(desired *w.DesiredWorld, report *LegacyPrivatePayloadReport) {
	dropped := map[w.DesiredWindowID]bool{}
	for projectID, project := range desired.Projects {
		windows := project.Windows[:0]
		for _, window := range project.Windows {
			if window.Kind == w.WindowBrowser && (window.Browser == nil || len(window.Browser.URLPayloadRefs) == 0) {
				dropped[window.ID] = true
				report.DroppedBrowserWindowsWithoutPayload++
				continue
			}
			windows = append(windows, window)
		}
		project.Windows = windows
		project.Layouts = pruneLayouts(project.Layouts, dropped)
		desired.Projects[projectID] = project
	}
	desired.AcceptedLayouts = pruneAcceptedLayouts(desired.AcceptedLayouts, dropped)
}

func pruneMissingProjectRoots(desired *w.DesiredWorld, report *LegacyReport) {
	missing := map[w.ProjectID]bool{}
	for projectID, project := range desired.Projects {
		if project.Root == "" {
			missing[projectID] = true
		} else if _, err := os.Stat(project.Root); err != nil {
			missing[projectID] = true
		}
		if !missing[projectID] {
			continue
		}
		project.Archived = true
		desired.Projects[projectID] = project
		report.MissingProjectRoots++
		report.QuarantinedFields = append(report.QuarantinedFields, fmt.Sprintf("projects.%s.cwd", projectID))
		delete(desired.AcceptedLayouts, projectID)
	}
	if len(missing) == 0 {
		return
	}
	for profileID, profile := range desired.Profiles {
		assignments := map[w.SlotID]w.ProjectID{}
		for slot, projectID := range profile.Assignments {
			if !missing[projectID] {
				assignments[slot] = projectID
			}
		}
		profile.Assignments = assignments
		desired.Profiles[profileID] = profile
	}
}

func pruneAcceptedLayouts(layouts map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout, dropped map[w.DesiredWindowID]bool) map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout {
	out := make(map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout, len(layouts))
	for projectID, perWorkspace := range layouts {
		pruned := pruneLayouts(perWorkspace, dropped)
		if len(pruned) > 0 {
			out[projectID] = pruned
		}
	}
	return out
}

func pruneLayouts(layouts map[w.WorkspaceID]w.DesiredLayout, dropped map[w.DesiredWindowID]bool) map[w.WorkspaceID]w.DesiredLayout {
	out := make(map[w.WorkspaceID]w.DesiredLayout, len(layouts))
	for workspace, layout := range layouts {
		columns := layout.Columns[:0]
		for _, column := range layout.Columns {
			windows := column.Windows[:0]
			for _, id := range column.Windows {
				if !dropped[id] {
					windows = append(windows, id)
				}
			}
			if len(windows) == 0 {
				continue
			}
			column.Windows = windows
			columns = append(columns, column)
		}
		if len(columns) == 0 {
			continue
		}
		layout.Columns = columns
		out[workspace] = layout
	}
	return out
}

func attachBrowserPrivatePayloadRef(desired *w.DesiredWorld, projectName string, legacy legacyWindow, token string, validURLs int, invalidURLs int) error {
	projectID := w.ProjectID(projectName)
	project, ok := desired.Projects[projectID]
	if !ok {
		return fmt.Errorf("migration: browser payload project %q missing from desired world", projectName)
	}
	for i := range project.Windows {
		window := &project.Windows[i]
		if window.ID.Index != legacy.ID || window.Kind != w.WindowBrowser {
			continue
		}
		session := window.Browser
		if session == nil {
			session = &w.DesiredBrowserSession{
				PrivacyMode:       w.BrowserSnapshotPrivateContent,
				RedactionPolicyID: "legacy-saved-urls-v1",
			}
		}
		session.URLPayloadRefs = append(session.URLPayloadRefs, w.PrivatePayloadRef(token))
		session.URLCount += validURLs
		session.InvalidURLCount += invalidURLs
		session.RestoreURLs = false
		window.Browser = session
		desired.Projects[projectID] = project
		return nil
	}
	return fmt.Errorf("migration: browser payload window project=%q id=%d missing from desired world", projectName, legacy.ID)
}

func migrateWindow(projectName string, projectRoot string, legacy legacyWindow) (w.DesiredWindow, error) {
	if legacy.ID < 1 {
		return w.DesiredWindow{}, fmt.Errorf("migration: project %q has invalid window id %d", projectName, legacy.ID)
	}
	kind, err := migrateWindowKind(legacy.Kind)
	if err != nil {
		return w.DesiredWindow{}, fmt.Errorf("migration: project %q window %d: %w", projectName, legacy.ID, err)
	}
	dw := w.DesiredWindow{
		ID:   w.DesiredWindowID{Project: w.ProjectID(projectName), Kind: kind, Index: legacy.ID},
		Kind: kind,
	}
	switch kind {
	case w.WindowAI:
		dw.App = w.AppRequirement{Capability: w.CapabilityTerminal, BundleID: "com.mitchellh.ghostty", AppPath: "/Applications/Ghostty.app"}
		dw.TitleContract = w.TitleContract{Authority: w.TitleControllerOwned, Expected: fmt.Sprintf("ai-%d:%s", legacy.ID, projectName), Drift: w.TitleDriftRepair}
		dw.MatchHints = []w.MatchHint{{Kind: w.MatchByTitlePrefix, Pattern: fmt.Sprintf("ai-%d:%s", legacy.ID, projectName), Confidence: w.MatchStrong}}
	case w.WindowShell:
		dw.App = w.AppRequirement{Capability: w.CapabilityTerminal, BundleID: "com.mitchellh.ghostty", AppPath: "/Applications/Ghostty.app"}
		dw.TitleContract = w.TitleContract{Authority: w.TitleControllerOwned, Expected: fmt.Sprintf("shell-%d:%s", legacy.ID, projectName), Drift: w.TitleDriftRepair}
		dw.MatchHints = []w.MatchHint{{Kind: w.MatchByTitlePrefix, Pattern: fmt.Sprintf("shell-%d:%s", legacy.ID, projectName), Confidence: w.MatchStrong}}
	case w.WindowEditor:
		dw.App = w.AppRequirement{Capability: w.CapabilityEditor, BundleID: "dev.zed.Zed", AppPath: "/Applications/Zed.app"}
		dw.TitleContract = w.TitleContract{Authority: w.TitleAppOwned, Expected: filepath.Base(projectRoot), Drift: w.TitleDriftRematch}
		dw.MatchHints = []w.MatchHint{{Kind: w.MatchByBundleID, Pattern: "dev.zed.Zed", Confidence: w.MatchStrong}}
	case w.WindowBrowser:
		dw.App = w.AppRequirement{Capability: w.CapabilityBrowser, BundleID: "com.vivaldi.Vivaldi", AppPath: "/Applications/Vivaldi.app"}
		dw.TitleContract = w.TitleContract{Authority: w.TitleAppOwned, Drift: w.TitleDriftObserveOnly}
		dw.MatchHints = []w.MatchHint{{Kind: w.MatchByBundleID, Pattern: "com.vivaldi.Vivaldi", Confidence: w.MatchWeak}}
	}
	return dw, nil
}

func validLegacyBrowserURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	switch parsed.Scheme {
	case "http", "https":
		return true
	default:
		return false
	}
}

func migrateWindowKind(kind string) (w.WindowKind, error) {
	switch kind {
	case "ai":
		return w.WindowAI, nil
	case "shell":
		return w.WindowShell, nil
	case "editor":
		return w.WindowEditor, nil
	case "browser":
		return w.WindowBrowser, nil
	default:
		return "", fmt.Errorf("unknown legacy window kind %q", kind)
	}
}

func migrateProjectLayout(projectID w.ProjectID, workspace w.WorkspaceID, windows []legacyWindow) (w.DesiredLayout, bool) {
	type positioned struct {
		window legacyWindow
	}
	columns := map[int][]positioned{}
	hasLayout := false
	for _, window := range windows {
		if window.Layout == nil {
			continue
		}
		hasLayout = true
		columns[window.Layout.Column] = append(columns[window.Layout.Column], positioned{window: window})
	}
	if !hasLayout {
		return w.DesiredLayout{}, false
	}
	columnIndexes := make([]int, 0, len(columns))
	for idx := range columns {
		columnIndexes = append(columnIndexes, idx)
	}
	sort.Ints(columnIndexes)
	out := w.DesiredLayout{Workspace: workspace, Source: w.LayoutAuthorityImported}
	for _, columnIndex := range columnIndexes {
		items := columns[columnIndex]
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].window.Layout.Stack < items[j].window.Layout.Stack
		})
		col := w.DesiredColumn{Mode: w.ColumnSolo}
		for _, item := range items {
			kind, err := migrateWindowKind(item.window.Kind)
			if err != nil {
				continue
			}
			col.Windows = append(col.Windows, w.DesiredWindowID{Project: projectID, Kind: kind, Index: item.window.ID})
			if item.window.Layout.Tabbed {
				col.Mode = w.ColumnTabbed
			}
		}
		if len(col.Windows) > 1 && col.Mode == w.ColumnSolo {
			col.Mode = w.ColumnStacked
		}
		out.Columns = append(out.Columns, col)
	}
	return out, true
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
