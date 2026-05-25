// Package verifier compares PredictedWorld with ObservedWorld and classifies
// drift. design.md §11. Pure.
package verifier

import (
	"fmt"
	"sort"

	w "github.com/yuu-th/projwm-next/internal/world"
)

type DiffClass string

const (
	DiffMissing DiffClass = "missing"
	DiffExtra   DiffClass = "extra"
	DiffDrift   DiffClass = "drift"
)

type Entry struct {
	Class  DiffClass
	Window w.LiveWindowID
	Detail string
}

type WorldDiff struct {
	Entries []Entry
}

func (d WorldDiff) Empty() bool { return len(d.Entries) == 0 }

func Diff(predicted, observed w.ObservedWorld) WorldDiff {
	var entries []Entry
	pwins := groupedWindows(predicted.Windows)
	owins := groupedWindows(observed.Windows)
	pkeys := sortedKeys(pwins)
	okeys := sortedKeys(owins)
	pset := map[string]bool{}
	for _, key := range pkeys {
		pset[key] = true
	}
	oset := map[string]bool{}
	for _, key := range okeys {
		oset[key] = true
	}
	for _, key := range pkeys {
		if !oset[key] {
			for _, win := range pwins[key] {
				entries = append(entries, Entry{Class: DiffMissing, Window: win.ID, Detail: detail("predicted but not observed", win)})
			}
		}
	}
	for _, key := range okeys {
		if !pset[key] {
			for _, win := range owins[key] {
				entries = append(entries, Entry{Class: DiffExtra, Window: win.ID, Detail: detail("observed but not predicted", win)})
			}
		}
	}
	for _, key := range pkeys {
		if oset[key] {
			pgroup := pwins[key]
			ogroup := owins[key]
			if len(pgroup) > len(ogroup) {
				for _, win := range pgroup[len(ogroup):] {
					entries = append(entries, Entry{Class: DiffMissing, Window: win.ID, Detail: detail("duplicate predicted window not observed", win)})
				}
			}
			if len(ogroup) > len(pgroup) {
				for _, win := range ogroup[len(pgroup):] {
					entries = append(entries, Entry{Class: DiffExtra, Window: win.ID, Detail: detail("duplicate observed window", win)})
				}
			}
			limit := len(pgroup)
			if len(ogroup) < limit {
				limit = len(ogroup)
			}
			for i := 0; i < limit; i++ {
				entries = append(entries, windowDriftEntries(pgroup[i], ogroup[i])...)
			}
		}
	}
	entries = append(entries, layoutDiffEntries(predicted, observed)...)
	entries = append(entries, focusDiffEntries(predicted, observed)...)
	entries = append(entries, displayDiffEntries(predicted.Displays, observed.Displays)...)
	return WorldDiff{Entries: entries}
}

func detail(prefix string, win w.ObservedWindow) string {
	matched := "<nil>"
	if win.MatchedTo != nil {
		matched = fmt.Sprintf("%s:%s:%d", win.MatchedTo.Project, win.MatchedTo.Kind, win.MatchedTo.Index)
	}
	return fmt.Sprintf("%s id=%s kind=%s workspace=%s matched=%s title=%q bundle=%q", prefix, win.ID, win.Kind, win.Workspace, matched, win.Title.Value, win.App.BundleID)
}

func groupedWindows(windows map[w.LiveWindowID]w.ObservedWindow) map[string][]w.ObservedWindow {
	out := map[string][]w.ObservedWindow{}
	for id, win := range windows {
		key := comparableKey(id, win)
		out[key] = append(out[key], win)
	}
	for key := range out {
		sort.Slice(out[key], func(i, j int) bool { return out[key][i].ID < out[key][j].ID })
	}
	return out
}

func comparableKey(id w.LiveWindowID, win w.ObservedWindow) string {
	if win.MatchedTo != nil {
		return fmt.Sprintf("desired:%s:%s:%d:%s", win.MatchedTo.Project, win.MatchedTo.Kind, win.MatchedTo.Index, win.Kind)
	}
	return "live:" + string(id)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func windowDriftEntries(predicted, observed w.ObservedWindow) []Entry {
	var entries []Entry
	if predicted.Kind != observed.Kind {
		entries = append(entries, Entry{Class: DiffDrift, Window: observed.ID, Detail: fmt.Sprintf("kind differs predicted=%s observed=%s; %s", predicted.Kind, observed.Kind, detail("observed", observed))})
	}
	if predicted.Workspace != observed.Workspace {
		entries = append(entries, Entry{Class: DiffDrift, Window: observed.ID, Detail: fmt.Sprintf("workspace differs predicted=%s observed=%s; %s", predicted.Workspace, observed.Workspace, detail("observed", observed))})
	}
	if predicted.Title.Value != observed.Title.Value {
		entries = append(entries, Entry{Class: DiffDrift, Window: observed.ID, Detail: fmt.Sprintf("title differs predicted=%q observed=%q; %s", predicted.Title.Value, observed.Title.Value, detail("observed", observed))})
	}
	if predicted.App.BundleID != "" && observed.App.BundleID != "" && predicted.App.BundleID != observed.App.BundleID {
		entries = append(entries, Entry{Class: DiffDrift, Window: observed.ID, Detail: fmt.Sprintf("bundle differs predicted=%q observed=%q; %s", predicted.App.BundleID, observed.App.BundleID, detail("observed", observed))})
	}
	return entries
}

func layoutDiffEntries(predicted, observed w.ObservedWorld) []Entry {
	var entries []Entry
	pkeys := sortedLayoutKeys(predicted.Layouts)
	okeys := sortedLayoutKeys(observed.Layouts)
	seen := map[w.WorkspaceID]bool{}
	for _, ws := range pkeys {
		seen[ws] = true
		if normalizeLayout(predicted.Layouts[ws], predicted.Windows) != normalizeLayout(observed.Layouts[ws], observed.Windows) {
			entries = append(entries, Entry{Class: DiffDrift, Detail: fmt.Sprintf("layout differs workspace=%s predicted=%s observed=%s", ws, normalizeLayout(predicted.Layouts[ws], predicted.Windows), normalizeLayout(observed.Layouts[ws], observed.Windows))})
		}
	}
	for _, ws := range okeys {
		if !seen[ws] {
			entries = append(entries, Entry{Class: DiffExtra, Detail: fmt.Sprintf("observed layout without prediction workspace=%s observed=%s", ws, normalizeLayout(observed.Layouts[ws], observed.Windows))})
		}
	}
	return entries
}

func focusDiffEntries(predicted, observed w.ObservedWorld) []Entry {
	pwin := normalizeWindowRef(predicted.Focus.Window, predicted.Windows)
	owin := normalizeWindowRef(observed.Focus.Window, observed.Windows)
	if predicted.Focus.Workspace == observed.Focus.Workspace && pwin == owin {
		return nil
	}
	return []Entry{{Class: DiffDrift, Window: observed.Focus.Window, Detail: fmt.Sprintf("focus differs predicted=%s/%s observed=%s/%s", predicted.Focus.Workspace, pwin, observed.Focus.Workspace, owin)}}
}

func displayDiffEntries(predicted, observed w.ObservedDisplayState) []Entry {
	if normalizeDisplays(predicted) == normalizeDisplays(observed) {
		return nil
	}
	return []Entry{{Class: DiffDrift, Detail: fmt.Sprintf("display state differs predicted=%s observed=%s", normalizeDisplays(predicted), normalizeDisplays(observed))}}
}

func sortedLayoutKeys(layouts map[w.WorkspaceID]w.ObservedLayout) []w.WorkspaceID {
	out := make([]w.WorkspaceID, 0, len(layouts))
	for ws := range layouts {
		out = append(out, ws)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func normalizeLayout(layout w.ObservedLayout, windows map[w.LiveWindowID]w.ObservedWindow) string {
	parts := make([]string, 0, len(layout.Columns))
	for _, col := range layout.Columns {
		wins := make([]string, 0, len(col.Windows))
		for _, id := range col.Windows {
			wins = append(wins, normalizeWindowRef(id, windows))
		}
		parts = append(parts, fmt.Sprintf("%s[%s]", col.Mode, join(wins, ",")))
	}
	return join(parts, "|")
}

func normalizeWindowRef(id w.LiveWindowID, windows map[w.LiveWindowID]w.ObservedWindow) string {
	if id == "" {
		return ""
	}
	if win, ok := windows[id]; ok {
		return comparableKey(id, win)
	}
	return "live:" + string(id)
}

func normalizeDisplays(state w.ObservedDisplayState) string {
	ids := make([]w.DisplayID, 0, len(state.Displays))
	for id := range state.Displays {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := make([]string, 0, len(ids)+1)
	primary := ""
	if state.Primary != nil {
		primary = string(*state.Primary)
	}
	parts = append(parts, "primary="+primary)
	for _, id := range ids {
		display := state.Displays[id]
		parts = append(parts, fmt.Sprintf("%s:%t", id, display.Connected))
	}
	return join(parts, ",")
}

func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, part := range parts[1:] {
		out += sep + part
	}
	return out
}
