package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuu-th/projwm-next/internal/controller"
	"github.com/yuu-th/projwm-next/internal/ipc"
	"github.com/yuu-th/projwm-next/internal/op"
	"github.com/yuu-th/projwm-next/internal/planner"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// handleQueryEnvelope parses a MsgQueryRequest, runs the read-only Query
// against the controller's current state / store, and replies with a
// MsgQueryResponse. The connection stays open so the caller can issue
// more queries (handled by serveLongLivedSession).
func handleQueryEnvelope(ctx context.Context, conn net.Conn, ctrl *controller.Controller, env ipc.Envelope) {
	var req ipc.QueryRequest
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		writeQueryError(conn, "", ipc.ErrProtocolMismatch, fmt.Sprintf("malformed query: %v", err))
		return
	}
	body, err := dispatchQuery(ctx, ctrl, req)
	if err != nil {
		writeQueryError(conn, req.RequestID, ipc.ErrTransactionFailed, err.Error())
		return
	}
	resp, err := ipc.NewEnvelope(ipc.MsgQueryResponse, ipc.QueryResponse{
		RequestID: req.RequestID,
		Body:      body,
	})
	if err != nil {
		writeQueryError(conn, req.RequestID, ipc.ErrProtocolMismatch, err.Error())
		return
	}
	_ = ipc.WriteEnvelope(conn, resp)
}

func writeQueryError(conn net.Conn, id string, code ipc.ErrorCode, msg string) {
	env, _ := ipc.NewEnvelope(ipc.MsgQueryResponse, ipc.QueryResponse{
		RequestID: id,
		Error:     &ipc.Error{Code: code, Message: msg},
	})
	_ = ipc.WriteEnvelope(conn, env)
}

// dispatchQuery returns the JSON body for one QueryRequest.
//
// Read-only — never writes DesiredWorld. Cards (in-memory) come from
// the controller's WorldState. Trace comes from the persistent store.
func dispatchQuery(ctx context.Context, ctrl *controller.Controller, req ipc.QueryRequest) (json.RawMessage, error) {
	switch req.Kind {
	case ipc.QueryWorld:
		return queryWorld(ctx, ctrl)
	case ipc.QueryProfiles:
		return queryProfiles(ctx, ctrl)
	case ipc.QueryArchive:
		return queryArchive(ctx, ctrl)
	case ipc.QueryCards:
		return queryCards(ctx, ctrl)
	case ipc.QueryTrace:
		return queryTrace(ctx, ctrl, req.TraceID)
	case ipc.QueryPlanPreview:
		return queryPlanPreview(ctx, ctrl)
	default:
		return nil, fmt.Errorf("unsupported query kind %q", req.Kind)
	}
}

// queryPlanPreview runs the planner against the controller's current
// WorldState + DesiredWorld and returns the resulting operations without
// committing. Powers `projwm reconcile --dry-run` per requirements §5.9.
//
// The reply shape is intentionally close to op.Plan; private payloads
// (URLs, tokens) are not in Operation so no redaction is needed here.
func queryPlanPreview(_ context.Context, ctrl *controller.Controller) (json.RawMessage, error) {
	state := ctrl.State()
	plan, err := planner.Plan(state, state.Desired, planner.CommandKey("reconcile-preview"), op.ReasonReconcile)
	if err != nil {
		return nil, fmt.Errorf("planner.Plan preview: %w", err)
	}

	ops := make([]map[string]any, 0, len(plan.Operations))
	for _, o := range plan.Operations {
		entry := map[string]any{
			"id":   o.ID,
			"kind": o.Kind,
			"risk": o.Risk,
		}
		if o.Target.LiveWindow != nil {
			entry["liveWindow"] = *o.Target.LiveWindow
		}
		if o.Target.DesiredWindow != nil {
			entry["desiredWindow"] = *o.Target.DesiredWindow
		}
		if o.Target.Workspace != nil {
			entry["workspace"] = *o.Target.Workspace
		}
		if o.Target.SystemWindow != nil {
			entry["systemWindow"] = *o.Target.SystemWindow
		}
		if o.IdempotencyKey != "" {
			entry["idempotencyKey"] = o.IdempotencyKey
		}
		ops = append(ops, entry)
	}

	resp := map[string]any{
		"planId":     plan.ID,
		"baseEpoch":  plan.BaseEpoch,
		"reason":     plan.Reason,
		"scope":      plan.Scope,
		"operations": ops,
		"converged":  len(ops) == 0,
	}
	return json.Marshal(resp)
}

// queryWorld returns the same shape as `projwm status --json` so that
// cockpit and CLI share a single schema.
func queryWorld(ctx context.Context, ctrl *controller.Controller) (json.RawMessage, error) {
	state := ctrl.State()
	gen, err := ctrl.Store.LoadCurrentGeneration(ctx)
	if err != nil {
		return nil, fmt.Errorf("load current generation: %w", err)
	}
	// Build the small "live windows projection" the cockpit needs.
	liveWindows := make([]map[string]any, 0, len(state.Observed.Windows))
	for id, ow := range state.Observed.Windows {
		entry := map[string]any{
			"id":        id,
			"workspace": ow.Workspace,
			"kind":      ow.Kind,
			"focused":   ow.Focused,
		}
		if ow.MatchedTo != nil {
			entry["matchedTo"] = ow.MatchedTo
		}
		liveWindows = append(liveWindows, entry)
	}

	// tmuxSessions{} comes from probing the host. The cockpit only
	// needs presence/absence so we shell out once per query.
	tmuxSessions := probeTmuxSessions(ctx)

	conv := "CONVERGED"
	if len(state.Meta.DirtyScopes) > 0 {
		conv = fmt.Sprintf("CONVERGING(%d)", len(state.Meta.DirtyScopes))
	}

	resp := map[string]any{
		"generation":             gen.ID,
		"parent":                 gen.Parent,
		"epoch":                  gen.Checkpoint.Epoch,
		"activeProfile":          state.Desired.ActiveProfile,
		"profiles":               state.Desired.Profiles,
		"projects":               state.Desired.Projects,
		"acceptedLayouts":        state.Desired.AcceptedLayouts,
		"workspaces":             state.Environment.Workspaces.Workspaces,
		"slots":                  state.Environment.Workspaces.Slots,
		"parked":                 parkedProjects(state.Desired),
		"archived":               archivedProjects(state.Desired),
		"activeCards":            state.Meta.ActiveCards,
		"pendingOrphans":         state.Meta.PendingOrphans,
		"convergenceStatus":      conv,
		"manifestDigestMismatch": false, // daemon ensures digest at startup; mismatch arrives as MANIFEST card
		"tmuxSessions":           tmuxSessions,
		"liveWindows":            liveWindows,
	}
	return json.Marshal(resp)
}

// probeTmuxSessions returns a map session-name → true for every alive
// tmux session on the host. Used by the cockpit to render K5.1
// per-window state.
func probeTmuxSessions(ctx context.Context) map[string]bool {
	out, err := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return map[string]bool{}
	}
	sessions := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			sessions[line] = true
		}
	}
	return sessions
}

func queryProfiles(ctx context.Context, ctrl *controller.Controller) (json.RawMessage, error) {
	state := ctrl.State()
	type entry struct {
		ID             w.ProfileID                `json:"id"`
		Active         bool                       `json:"active"`
		Description    string                     `json:"description,omitempty"`
		InactivePolicy w.InactivePolicy           `json:"inactivePolicy"`
		Assignments    map[w.SlotID]w.ProjectID   `json:"assignments"`
	}
	out := make([]entry, 0, len(state.Desired.Profiles))
	for id, p := range state.Desired.Profiles {
		out = append(out, entry{
			ID:             id,
			Active:         id == state.Desired.ActiveProfile,
			Description:    p.Description,
			InactivePolicy: p.InactivePolicy,
			Assignments:    p.Assignments,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return json.Marshal(out)
}

func queryArchive(ctx context.Context, ctrl *controller.Controller) (json.RawMessage, error) {
	state := ctrl.State()
	out := archivedProjects(state.Desired)
	return json.Marshal(out)
}

func queryCards(ctx context.Context, ctrl *controller.Controller) (json.RawMessage, error) {
	state := ctrl.State()
	return json.Marshal(state.Meta.ActiveCards)
}

func queryTrace(ctx context.Context, ctrl *controller.Controller, traceID string) (json.RawMessage, error) {
	// The controller doesn't keep a direct handle to the trace directory,
	// so we resolve via the FileStore if available.
	fs, ok := ctrl.Store.(*store.FileStore)
	if !ok {
		return nil, fmt.Errorf("trace queries require a FileStore-backed daemon")
	}
	dir := filepath.Join(fileStoreRoot(fs), "traces")
	if traceID == "" {
		return latestTraceJSON(dir)
	}
	return readTraceJSON(filepath.Join(dir, traceID+".json"))
}

// fileStoreRoot extracts the on-disk root from a *FileStore. This relies on
// the helper exported via the store package — if absent we fall back to
// the empty string and let the caller error.
func fileStoreRoot(fs *store.FileStore) string {
	if fs == nil {
		return ""
	}
	return fs.Root()
}

func latestTraceJSON(dir string) (json.RawMessage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read traces dir: %w", err)
	}
	var newest os.DirEntry
	var newestMtime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newestMtime) {
			newest = e
			newestMtime = info.ModTime()
		}
	}
	if newest == nil {
		return nil, fmt.Errorf("no traces in %s", dir)
	}
	return readTraceJSON(filepath.Join(dir, newest.Name()))
}

func readTraceJSON(path string) (json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read trace %s: %w", path, err)
	}
	return json.RawMessage(data), nil
}

func parkedProjects(d w.DesiredWorld) []w.ProjectID {
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
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func archivedProjects(d w.DesiredWorld) []w.ProjectID {
	var ids []w.ProjectID
	for id, p := range d.Projects {
		if p.Archived {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
