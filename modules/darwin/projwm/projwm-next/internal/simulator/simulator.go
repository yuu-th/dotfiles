// Package simulator predicts the effect of an Operation on PredictedWorld.
// L0 = effect simulator, L1 = semantic layout simulator. design.md §11, impl-design §1665.
package simulator

import (
	"fmt"

	"github.com/yuu-th/projwm-next/internal/op"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Apply returns a new PredictedWorld with the operation's expected effects applied.
// Pure (does not observe).
func Apply(pred w.PredictedWorld, oper op.Operation) (w.PredictedWorld, error) {
	out := clonePredicted(pred)
	for _, e := range oper.ExpectedEffects {
		switch e.Kind {
		case op.EffectSpawnWindow:
			// Cockpit / SystemWindow spawn: no project DesiredWindowID
			// or Workspace. Simulator records the predicted live window
			// keyed off SystemWindowID; layout is untouched (cockpit is
			// scratchpad-floating, not a tile column).
			if e.SystemWindow != nil {
				sw := *e.SystemWindow
				id := w.LiveWindowID(fmt.Sprintf("predicted-cockpit-%s-%d", sw.Kind, sw.Index))
				out.Windows[id] = w.ObservedWindow{
					ID:              id,
					Title:           w.ObservedTitle{Value: fmt.Sprintf("projwm-cockpit-%d", sw.Index)},
					Kind:            e.WindowKind,
					SystemMatchedTo: &sw,
				}
				break
			}
			if e.Desired == nil || e.Workspace == nil {
				return out, fmt.Errorf("simulator: spawn missing desired or workspace")
			}
			id := w.LiveWindowID(fmt.Sprintf("predicted-%s-%s-%d", e.Desired.Project, e.Desired.Kind, e.Desired.Index))
			d := *e.Desired
			out.Windows[id] = w.ObservedWindow{
				ID:        id,
				Title:     w.ObservedTitle{Value: predictedTitle(d, e.WindowKind)},
				Workspace: *e.Workspace,
				MatchedTo: &d,
				Kind:      effectiveKind(e.WindowKind, d.Kind),
			}
			ol := out.Layouts[*e.Workspace]
			ol.Workspace = *e.Workspace
			ol.Columns = append(ol.Columns, w.ObservedColumn{Windows: []w.LiveWindowID{id}, Mode: w.ColumnSolo})
			out.Layouts[*e.Workspace] = ol
		case op.EffectCloseWindow:
			if e.Window == nil {
				return out, fmt.Errorf("simulator: close missing window")
			}
			ow, ok := out.Windows[*e.Window]
			if !ok {
				continue
			}
			delete(out.Windows, *e.Window)
			out.Layouts[ow.Workspace] = removeFromCols(out.Layouts[ow.Workspace], *e.Window)
			if out.Focus.Window == *e.Window {
				out.Focus.Window = ""
			}
		case op.EffectMoveWindow:
			if e.Window == nil || e.Workspace == nil {
				return out, fmt.Errorf("simulator: move missing window/workspace")
			}
			ow, ok := out.Windows[*e.Window]
			if !ok {
				continue
			}
			from := ow.Workspace
			out.Layouts[from] = removeFromCols(out.Layouts[from], *e.Window)
			ow.Workspace = *e.Workspace
			out.Windows[*e.Window] = ow
			ol := out.Layouts[*e.Workspace]
			ol.Workspace = *e.Workspace
			ol.Columns = append(ol.Columns, w.ObservedColumn{Windows: []w.LiveWindowID{*e.Window}, Mode: w.ColumnSolo})
			out.Layouts[*e.Workspace] = ol
		case op.EffectFocusWorkspace:
			if e.FocusedWS == nil {
				continue
			}
			out.Focus.Workspace = *e.FocusedWS
			ol := out.Layouts[*e.FocusedWS]
			if len(ol.Columns) > 0 && len(ol.Columns[0].Windows) > 0 {
				out.Focus.Window = ol.Columns[0].Windows[0]
			} else {
				out.Focus.Window = ""
			}
		case op.EffectFocusWindow:
			if e.FocusedWin == nil {
				continue
			}
			ow, ok := out.Windows[*e.FocusedWin]
			if !ok {
				continue
			}
			out.Focus = w.ObservedFocus{Workspace: ow.Workspace, Window: *e.FocusedWin}
		case op.EffectReorderColumns:
			if e.Workspace == nil {
				continue
			}
			// Map desired columns through current MatchedTo to live IDs.
			byDesired := map[w.DesiredWindowID]w.LiveWindowID{}
			for id, wn := range out.Windows {
				if wn.Workspace == *e.Workspace && wn.MatchedTo != nil {
					byDesired[*wn.MatchedTo] = id
				}
			}
			cols := []w.ObservedColumn{}
			for _, dc := range e.Columns {
				stack := []w.LiveWindowID{}
				for _, dwid := range dc.Windows {
					if id, ok := byDesired[dwid]; ok {
						stack = append(stack, id)
					}
				}
				if len(stack) == 0 {
					continue
				}
				mode := w.ColumnSolo
				if len(stack) > 1 {
					mode = w.ColumnStacked
				}
				cols = append(cols, w.ObservedColumn{Windows: stack, Mode: mode})
			}
			ol := out.Layouts[*e.Workspace]
			ol.Workspace = *e.Workspace
			ol.Columns = cols
			out.Layouts[*e.Workspace] = ol
		}
	}
	return out, nil
}

func predictedTitle(d w.DesiredWindowID, k w.WindowKind) string {
	if k == w.WindowViewer {
		// d is the AI's DesiredWindowID (viewers share it with Kind=Viewer), so
		// d.Index is the AI's own index N. The viewer's title mirrors its AI:
		// ai-view-N (matching naming.viewerTitleForAI = "ai-view"+strip("ai") and
		// naming/ssot_l0_identity_test: ai-view-1 ↔ ai-1). A previous "+1"
		// predicted ai-view-(N+1), so the verifier compared a wrong predicted
		// title against the real ai-view-N and never matched the viewer.
		return fmt.Sprintf("ai-view-%d:%s", d.Index, d.Project)
	}
	return fmt.Sprintf("%s:%s:%d", d.Project, d.Kind, d.Index)
}

func effectiveKind(want, fromDesired w.WindowKind) w.WindowKind {
	if want != "" {
		return want
	}
	return fromDesired
}

func clonePredicted(p w.PredictedWorld) w.PredictedWorld {
	out := w.PredictedWorld{
		ObservedWorld: w.ObservedWorld{
			Workspaces: map[w.WorkspaceID]w.ObservedWorkspace{},
			Windows:    map[w.LiveWindowID]w.ObservedWindow{},
			Layouts:    map[w.WorkspaceID]w.ObservedLayout{},
			Focus:      p.Focus,
			Displays:   p.Displays,
		},
		BasedOnEpoch: p.BasedOnEpoch,
	}
	for k, v := range p.Workspaces {
		out.Workspaces[k] = v
	}
	for k, v := range p.Windows {
		if v.MatchedTo != nil {
			d := *v.MatchedTo
			v.MatchedTo = &d
		}
		out.Windows[k] = v
	}
	for k, v := range p.Layouts {
		cols := make([]w.ObservedColumn, len(v.Columns))
		for i, c := range v.Columns {
			cols[i] = w.ObservedColumn{Windows: append([]w.LiveWindowID(nil), c.Windows...), Mode: c.Mode}
		}
		out.Layouts[k] = w.ObservedLayout{Workspace: v.Workspace, Columns: cols}
	}
	return out
}

func removeFromCols(ol w.ObservedLayout, id w.LiveWindowID) w.ObservedLayout {
	out := w.ObservedLayout{Workspace: ol.Workspace}
	for _, c := range ol.Columns {
		nc := []w.LiveWindowID{}
		for _, x := range c.Windows {
			if x != id {
				nc = append(nc, x)
			}
		}
		if len(nc) > 0 {
			out.Columns = append(out.Columns, w.ObservedColumn{Windows: nc, Mode: c.Mode})
		}
	}
	return out
}

// FromObserved produces an initial PredictedWorld from an ObservedWorld.
func FromObserved(o w.ObservedWorld, epoch w.Epoch) w.PredictedWorld {
	return clonePredicted(w.PredictedWorld{ObservedWorld: o, BasedOnEpoch: epoch})
}
