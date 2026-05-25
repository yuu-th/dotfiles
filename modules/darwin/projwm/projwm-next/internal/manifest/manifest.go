// Package manifest reads and validates the ManagedEnvironment manifest.
//
// On-disk schema follows SSOT §10.7: `workspaces`, `slots`, `apps` are
// top-level JSON arrays (not nested under "workspaces.workspaces" etc.).
// The viewer workspace is identified by role=="viewer" rather than a
// dedicated "viewer" field.
//
// The in-memory `world.ManagedEnvironment` keeps the nested grouping for
// caller ergonomics — only the JSON shape and parser change here.
package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// Document is the on-disk JSON schema. SSOT §10.7.
type Document struct {
	SchemaVersion    int            `json:"schemaVersion"`
	Authority        string         `json:"authority"`
	Source           string         `json:"source,omitempty"`
	MinDaemonVersion string         `json:"minProjwmdVersion,omitempty"`
	WindowManager    DocWindowMgr   `json:"windowManager"`
	Workspaces       []DocWorkspace `json:"workspaces"`
	Slots            []DocSlot      `json:"slots"`
	Apps             []DocApp       `json:"apps"`
	Daemons          DocDaemons     `json:"daemons"`
}

type DocWindowMgr struct {
	Backend string    `json:"backend"`
	Layout  DocLayout `json:"layout,omitempty"`
	Focus   DocFocus  `json:"focus,omitempty"`
}

type DocLayout struct {
	DefaultColumnWidth  float64   `json:"defaultColumnWidth,omitempty"`
	ColumnWidthPresets  []float64 `json:"columnWidthPresets,omitempty"`
	MaxVisibleColumns   int       `json:"maxVisibleColumns,omitempty"`
	MaxWindowsPerColumn int       `json:"maxWindowsPerColumn,omitempty"`
	CenterFocusedColumn string    `json:"centerFocusedColumn,omitempty"`
	AlwaysCenterSingle  bool      `json:"alwaysCenterSingle,omitempty"`
}

type DocFocus struct {
	FollowsMouse             bool `json:"followsMouse,omitempty"`
	FollowsWindowToMonitor   bool `json:"followsWindowToMonitor,omitempty"`
	MoveMouseToFocusedWindow bool `json:"moveMouseToFocusedWindow,omitempty"`
}

type DocWorkspace struct {
	ID          w.WorkspaceID   `json:"id"`
	RawName     string          `json:"rawName"`
	DisplayName string          `json:"displayName,omitempty"`
	Role        w.WorkspaceRole `json:"role"`
}

type DocSlot struct {
	ID        w.SlotID      `json:"id"`
	Workspace w.WorkspaceID `json:"workspace"`
	Order     int           `json:"order"`
}

type DocApp struct {
	Capability       w.AppCapability            `json:"capability"`
	BundleID         string                     `json:"bundleId"`
	AppPath          string                     `json:"appPath,omitempty"`
	LifecycleRemoval *DocLifecycleRemovalPolicy `json:"lifecycleRemoval,omitempty"`
}

type DocLifecycleRemovalPolicy struct {
	Allowed          bool                     `json:"allowed"`
	Method           w.LifecycleRemovalMethod `json:"method"`
	AllowedKinds     []w.WindowKind           `json:"allowedKinds,omitempty"`
	RequiredEvidence []string                 `json:"requiredEvidence,omitempty"`
}

type DocDaemons struct {
	Controller   string           `json:"controller,omitempty"`
	SocketPath   string           `json:"socketPath,omitempty"`
	LegacyAgents string           `json:"legacyAgents,omitempty"`
	Agents       []DocLegacyAgent `json:"agents,omitempty"`
	EventSources []DocEventSource `json:"eventSources,omitempty"`
}

type DocLegacyAgent struct {
	Label  string `json:"label"`
	Action string `json:"action"`
}

type DocEventSource struct {
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	Mode      string `json:"mode"`
	Authority string `json:"authority"`
	Label     string `json:"label"`
}

// LoadFromFile reads a manifest path and validates.
func LoadFromFile(path string) (w.ManagedEnvironment, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return w.ManagedEnvironment{}, fmt.Errorf("manifest: read %s: %w", path, err)
	}
	return Parse(b, "0.1.0")
}

// Parse parses a manifest document and validates against the daemon version.
// Unknown top-level fields are rejected.
func Parse(data []byte, daemonVersion string) (w.ManagedEnvironment, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return w.ManagedEnvironment{}, fmt.Errorf("manifest: parse: %w", err)
	}
	if err := Validate(doc, daemonVersion); err != nil {
		return w.ManagedEnvironment{}, err
	}
	return ToManagedEnvironment(doc), nil
}

// Validate runs schema/version/min-daemon-version checks.
//
// Per SSOT §10.7, the minimum L3 test fixture has:
//   - empty daemons object (daemons.socketPath optional)
//   - no minProjwmdVersion (optional)
//   - no windowManager.layout (defaults apply)
//   - no viewer workspace (viewer is identified by role=="viewer"; absence
//     means this manifest has no viewer slot — valid for tests)
//
// Production manifests are richer and validated by additional checks at
// the use site (e.g., daemons.socketPath at daemon startup).
func Validate(d Document, daemonVersion string) error {
	if d.SchemaVersion != 1 {
		return fmt.Errorf("manifest: unsupported schemaVersion %d", d.SchemaVersion)
	}
	if d.Authority != "nix" {
		return fmt.Errorf("manifest: authority must be \"nix\", got %q", d.Authority)
	}
	if d.MinDaemonVersion != "" && cmpSemver(daemonVersion, d.MinDaemonVersion) < 0 {
		return fmt.Errorf("manifest: daemon version %s < required %s", daemonVersion, d.MinDaemonVersion)
	}
	if d.WindowManager.Backend == "" {
		return fmt.Errorf("manifest: windowManager.backend missing")
	}
	// Layout limits: enforce positivity only if specified. Zero means "no
	// configured ceiling" (defaults apply at the planner level).
	if d.WindowManager.Layout.MaxVisibleColumns < 0 {
		return fmt.Errorf("manifest: layout.maxVisibleColumns must be >= 0")
	}
	if d.WindowManager.Layout.MaxWindowsPerColumn < 0 {
		return fmt.Errorf("manifest: layout.maxWindowsPerColumn must be >= 0")
	}
	if d.Daemons.LegacyAgents != "" && d.Daemons.LegacyAgents != "remove" && d.Daemons.LegacyAgents != "report" {
		return fmt.Errorf("manifest: daemons.legacyAgents must be remove or report, got %q", d.Daemons.LegacyAgents)
	}
	for _, agent := range d.Daemons.Agents {
		if agent.Label == "" {
			return fmt.Errorf("manifest: daemons.agents.label missing")
		}
		if agent.Action != "remove" && agent.Action != "report" {
			return fmt.Errorf("manifest: daemons.agents[%s].action must be remove or report, got %q", agent.Label, agent.Action)
		}
	}
	if err := validateEventSources(d.Daemons.EventSources); err != nil {
		return err
	}
	wsByID := map[w.WorkspaceID]bool{}
	viewerCount := 0
	for _, ws := range d.Workspaces {
		if ws.ID == "" {
			return fmt.Errorf("manifest: workspace.id missing")
		}
		if wsByID[ws.ID] {
			return fmt.Errorf("manifest: duplicate workspace id %q", ws.ID)
		}
		wsByID[ws.ID] = true
		if ws.Role == w.WorkspaceViewer {
			viewerCount++
		}
	}
	if viewerCount > 1 {
		return fmt.Errorf("manifest: at most one workspace may have role=viewer, got %d", viewerCount)
	}
	for _, s := range d.Slots {
		if s.ID == "" {
			return fmt.Errorf("manifest: slot.id missing")
		}
		if !wsByID[s.Workspace] {
			return fmt.Errorf("manifest: slot %q references unknown workspace %q", s.ID, s.Workspace)
		}
	}
	for _, app := range d.Apps {
		if app.BundleID == "" {
			return fmt.Errorf("manifest: app bundleId missing")
		}
		if app.LifecycleRemoval != nil && app.LifecycleRemoval.Allowed && app.LifecycleRemoval.Method == "" {
			return fmt.Errorf("manifest: lifecycleRemoval.method missing for %s", app.BundleID)
		}
		if app.LifecycleRemoval != nil && app.LifecycleRemoval.Allowed && len(app.LifecycleRemoval.AllowedKinds) == 0 {
			return fmt.Errorf("manifest: lifecycleRemoval.allowedKinds missing for %s", app.BundleID)
		}
	}
	return nil
}

// ToManagedEnvironment converts the validated document into the in-memory
// world.ManagedEnvironment shape (which keeps the nested grouping for
// caller ergonomics).
func ToManagedEnvironment(d Document) w.ManagedEnvironment {
	wss := make([]w.WorkspaceSpec, 0, len(d.Workspaces))
	var viewer w.WorkspaceID
	for _, ws := range d.Workspaces {
		wss = append(wss, w.WorkspaceSpec{
			ID: ws.ID, RawName: ws.RawName, DisplayName: ws.DisplayName, Role: ws.Role,
		})
		if ws.Role == w.WorkspaceViewer {
			viewer = ws.ID
		}
	}
	slots := make([]w.SlotSpec, 0, len(d.Slots))
	for _, s := range d.Slots {
		slots = append(slots, w.SlotSpec{ID: s.ID, Workspace: s.Workspace, Order: s.Order})
	}
	apps := make([]w.ManagedAppPolicy, 0, len(d.Apps))
	for _, a := range d.Apps {
		policy := w.LifecycleRemovalPolicy{Method: w.LifecycleRemovalBlocked}
		if a.LifecycleRemoval != nil {
			policy = w.LifecycleRemovalPolicy{
				Allowed:          a.LifecycleRemoval.Allowed,
				Method:           a.LifecycleRemoval.Method,
				AllowedKinds:     append([]w.WindowKind(nil), a.LifecycleRemoval.AllowedKinds...),
				RequiredEvidence: append([]string(nil), a.LifecycleRemoval.RequiredEvidence...),
			}
		}
		apps = append(apps, w.ManagedAppPolicy{Capability: a.Capability, BundleID: a.BundleID, AppPath: a.AppPath, LifecycleRemoval: policy})
	}
	legacy := make([]w.LegacyAgentPolicy, 0, len(d.Daemons.Agents))
	for _, a := range d.Daemons.Agents {
		legacy = append(legacy, w.LegacyAgentPolicy{Label: a.Label, Action: a.Action})
	}
	eventSources := make([]w.EventSourceSpec, 0, len(d.Daemons.EventSources))
	for _, src := range d.Daemons.EventSources {
		eventSources = append(eventSources, w.EventSourceSpec{
			Kind:      src.Kind,
			Source:    src.Source,
			Mode:      src.Mode,
			Authority: src.Authority,
			Label:     src.Label,
		})
	}
	return w.ManagedEnvironment{
		SchemaVersion:    d.SchemaVersion,
		Authority:        d.Authority,
		Source:           d.Source,
		MinDaemonVersion: d.MinDaemonVersion,
		WindowManager: w.WindowManagerEnvironment{
			Backend: d.WindowManager.Backend,
			Layout: w.LayoutTuning{
				DefaultColumnWidth:  d.WindowManager.Layout.DefaultColumnWidth,
				ColumnWidthPresets:  d.WindowManager.Layout.ColumnWidthPresets,
				MaxVisibleColumns:   d.WindowManager.Layout.MaxVisibleColumns,
				MaxWindowsPerColumn: d.WindowManager.Layout.MaxWindowsPerColumn,
				CenterFocusedColumn: d.WindowManager.Layout.CenterFocusedColumn,
				AlwaysCenterSingle:  d.WindowManager.Layout.AlwaysCenterSingle,
			},
			Focus: w.FocusTuning{
				FollowsMouse:             d.WindowManager.Focus.FollowsMouse,
				FollowsWindowToMonitor:   d.WindowManager.Focus.FollowsWindowToMonitor,
				MoveMouseToFocusedWindow: d.WindowManager.Focus.MoveMouseToFocusedWindow,
			},
		},
		Workspaces: w.WorkspaceEnvironment{
			Viewer:     viewer,
			Workspaces: wss,
			Slots:      slots,
		},
		Apps:    w.AppEnvironment{ManagedApps: apps},
		Daemons: w.DaemonEnvironment{ControllerLabel: d.Daemons.Controller, SocketPath: d.Daemons.SocketPath, LegacyAgents: legacy, EventSources: eventSources},
	}
}

func validateEventSources(sources []DocEventSource) error {
	seen := map[string]bool{}
	for _, src := range sources {
		if src.Kind == "" || src.Source == "" || src.Mode == "" || src.Authority == "" || src.Label == "" {
			return fmt.Errorf("manifest: daemons.eventSources entries require kind, source, mode, authority, and label")
		}
		if src.Mode != "sidecar" && src.Mode != "in-process" {
			return fmt.Errorf("manifest: daemons.eventSources[%s].mode must be sidecar or in-process, got %q", src.Label, src.Mode)
		}
		if src.Authority != "hint" && src.Authority != "evidence" {
			return fmt.Errorf("manifest: daemons.eventSources[%s].authority must be hint or evidence, got %q", src.Label, src.Authority)
		}
		if src.Mode == "sidecar" && src.Authority != "hint" {
			return fmt.Errorf("manifest: daemons.eventSources[%s] sidecar authority must be hint, got %q", src.Label, src.Authority)
		}
		key := src.Kind + "\x00" + src.Source + "\x00" + src.Mode + "\x00" + src.Label
		if seen[key] {
			return fmt.Errorf("manifest: duplicate daemons.eventSources entry for kind=%s source=%s mode=%s label=%s", src.Kind, src.Source, src.Mode, src.Label)
		}
		seen[key] = true
	}
	return nil
}

// cmpSemver compares two dotted versions of unbounded length. Returns -1/0/+1.
func cmpSemver(a, b string) int {
	pa := splitDots(a)
	pb := splitDots(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = atoi(pa[i])
		}
		if i < len(pb) {
			y = atoi(pb[i])
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

func splitDots(s string) []string {
	out := []string{}
	cur := ""
	for _, c := range s {
		if c == '.' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}
