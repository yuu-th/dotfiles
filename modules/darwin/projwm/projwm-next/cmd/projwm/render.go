package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// renderHuman writes a human-readable status report for snap to out.
// Format intentionally mirrors the original projwm CLI's `projwm status` shape
// so that long-time users can read it without retraining.
func renderHuman(snap WorldSnapshot, out io.Writer) {
	fmt.Fprintf(out, "Generation: %s\n", snap.Generation)
	if snap.ParentGeneration != nil {
		fmt.Fprintf(out, "Parent:     %s\n", *snap.ParentGeneration)
	}
	fmt.Fprintf(out, "Epoch:      %d\n", snap.Checkpoint.Epoch)
	fmt.Fprintf(out, "Active:     %s\n", snap.Desired.ActiveProfile)

	if active, ok := snap.Desired.Profiles[snap.Desired.ActiveProfile]; ok {
		if active.Description != "" {
			fmt.Fprintf(out, "  description: %s\n", active.Description)
		}
		fmt.Fprintf(out, "  inactive-policy: %s\n", policyLabel(active.InactivePolicy))
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Active profile assignments:")
	assignments := snap.activeAssignments()
	if len(assignments) == 0 {
		fmt.Fprintln(out, "  (no slots)")
	}
	for _, sa := range assignments {
		proj := "(unassigned)"
		if sa.Project != "" {
			proj = string(sa.Project)
		}
		fmt.Fprintf(out, "  %s (workspace=%s) → %s\n", sa.Slot, sa.Workspace, proj)
		if sa.Project != "" {
			renderProjectWindows(out, snap, sa.Project, "    ")
		}
	}

	others := otherProfiles(snap)
	if len(others) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Other profiles:")
		for _, p := range others {
			fmt.Fprintf(out, "  %s — %d slot(s), policy=%s\n",
				p.ID, len(p.Assignments), policyLabel(p.InactivePolicy))
		}
	}

	parked := snap.parkedProjects()
	if len(parked) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Parked projects:")
		for _, pid := range parked {
			fmt.Fprintf(out, "  %s\n", pid)
		}
	}

	archived := snap.archivedProjects()
	if len(archived) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Archived projects:")
		for _, pid := range archived {
			fmt.Fprintf(out, "  %s\n", pid)
		}
	}
}

func renderProjectWindows(out io.Writer, snap WorldSnapshot, pid w.ProjectID, indent string) {
	pr, ok := snap.Desired.Projects[pid]
	if !ok || len(pr.Windows) == 0 {
		return
	}
	wins := append([]w.DesiredWindow(nil), pr.Windows...)
	sort.Slice(wins, func(i, j int) bool {
		if wins[i].Kind != wins[j].Kind {
			return wins[i].Kind < wins[j].Kind
		}
		return wins[i].ID.Index < wins[j].ID.Index
	})
	for _, dw := range wins {
		fmt.Fprintf(out, "%s%s-%d\n", indent, dw.Kind, dw.ID.Index)
	}
}

func otherProfiles(snap WorldSnapshot) []w.DesiredProfile {
	out := make([]w.DesiredProfile, 0)
	for id, p := range snap.Desired.Profiles {
		if id == snap.Desired.ActiveProfile {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func policyLabel(p w.InactivePolicy) string {
	if p == "" {
		return string(w.InactivePolicyRemove) + " (default)"
	}
	return string(p)
}

// renderProfileList writes one line per profile, marking the active one.
func renderProfileList(snap WorldSnapshot, out io.Writer) {
	ids := make([]w.ProfileID, 0, len(snap.Desired.Profiles))
	for id := range snap.Desired.Profiles {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		p := snap.Desired.Profiles[id]
		mark := " "
		if id == snap.Desired.ActiveProfile {
			mark = "*"
		}
		fmt.Fprintf(out, "%s %s (%d slot(s), policy=%s)\n",
			mark, id, len(p.Assignments), policyLabel(p.InactivePolicy))
	}
}

// renderProfileShow writes detailed information for one profile.
// Falls back to active profile if name is empty.
func renderProfileShow(snap WorldSnapshot, name w.ProfileID, out io.Writer) error {
	if name == "" {
		name = snap.Desired.ActiveProfile
	}
	p, ok := snap.Desired.Profiles[name]
	if !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	fmt.Fprintf(out, "Profile: %s%s\n", name, activeMark(name == snap.Desired.ActiveProfile))
	if p.Description != "" {
		fmt.Fprintf(out, "  description: %s\n", p.Description)
	}
	fmt.Fprintf(out, "  inactive-policy: %s\n", policyLabel(p.InactivePolicy))
	fmt.Fprintf(out, "  assignments:\n")
	slots := snap.Environment.SlotOrder()
	for _, sid := range slots {
		spec, _ := snap.Environment.SlotByID(sid)
		pid := p.Assignments[sid]
		if pid == "" {
			fmt.Fprintf(out, "    %s (workspace=%s) → (unassigned)\n", sid, spec.Workspace)
		} else {
			fmt.Fprintf(out, "    %s (workspace=%s) → %s\n", sid, spec.Workspace, pid)
		}
	}
	return nil
}

func activeMark(active bool) string {
	if active {
		return " (active)"
	}
	return ""
}

// renderArchiveList writes one line per archived project.
func renderArchiveList(snap WorldSnapshot, out io.Writer) {
	ids := snap.archivedProjects()
	if len(ids) == 0 {
		fmt.Fprintln(out, "(no archived projects)")
		return
	}
	for _, id := range ids {
		fmt.Fprintln(out, id)
	}
}

// renderTrace writes a redacted trace summary.
func renderTrace(t store.TransactionTrace, out io.Writer) {
	fmt.Fprintf(out, "TransactionID: %s\n", t.TransactionID)
	if t.Command != "" {
		fmt.Fprintf(out, "Command:       %s\n", t.Command)
	}
	if t.Reason != "" {
		fmt.Fprintf(out, "Reason:        %s\n", t.Reason)
	}
	if t.TriggerSource != "" || t.TriggerKind != "" {
		fmt.Fprintf(out, "Trigger:       %s/%s\n", t.TriggerSource, t.TriggerKind)
	}
	if t.EventID != "" {
		fmt.Fprintf(out, "EventID:       %s (epoch=%d)\n", t.EventID, t.EventEpoch)
	}
	fmt.Fprintf(out, "Generation:    parent=%s committed=%s\n", t.ParentGeneration, t.CommittedGeneration)
	fmt.Fprintf(out, "Epoch:         %d\n", t.ControllerEpoch)
	fmt.Fprintf(out, "Started:       %s\n", t.StartedAt)
	fmt.Fprintf(out, "Finished:      %s\n", t.FinishedAt)
	fmt.Fprintf(out, "Converged:     %v\n", t.Converged)
	if t.Discarded {
		fmt.Fprintf(out, "Discarded:     %v (%s)\n", t.Discarded, t.DiscardReason)
	}
	fmt.Fprintf(out, "Operations:    total=%d mutation=%d executed=%d\n",
		t.TotalOperations, t.MutationOperations, t.ExecutedMutations)
	if t.VerifierRan {
		fmt.Fprintf(out, "Verifier:      mode=%s diffs=%d unacceptable=%d\n",
			t.VerifierMode, t.VerifierDiffEntries, t.LastUnacceptableDiffEntries)
	}
	if t.NoCommitReason != "" {
		fmt.Fprintf(out, "NoCommit:      %s\n", t.NoCommitReason)
	}
	if t.ObservationRefreshFailed {
		fmt.Fprintf(out, "ObsRefresh:    failed (%s)\n", t.ObservationRefreshError)
	}
	if len(t.InvariantViolations) > 0 {
		fmt.Fprintln(out, "Invariants violated:")
		for _, v := range t.InvariantViolations {
			fmt.Fprintf(out, "  - %s\n", v)
		}
	}
	if len(t.RuntimeValidationReports) > 0 {
		fmt.Fprintln(out, "Runtime validation:")
		for _, r := range t.RuntimeValidationReports {
			fmt.Fprintf(out, "  - %s/%s: %s (blocking=%v)\n", r.Kind, r.Subject, r.Status, r.Blocking)
		}
	}
	if len(t.PlanIterations) > 0 {
		fmt.Fprintln(out, "Plan iterations:")
		for _, p := range t.PlanIterations {
			fmt.Fprintf(out, "  iter=%d plan=%s ops=%d mutation=%d executed=%d\n",
				p.Iteration, p.PlanID, p.PlannedOperations, p.MutationOperations, p.ExecutedMutations)
			for _, op := range p.Operations {
				fmt.Fprintf(out, "    op=%s kind=%s mutation=%v executed=%v\n",
					op.ID, op.Kind, op.Mutation, op.Executed)
			}
		}
	}
}

// joinIDs returns "a, b, c" for slice elements that stringify cleanly.
func joinIDs[T ~string](in []T) string {
	if len(in) == 0 {
		return "(none)"
	}
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return strings.Join(out, ", ")
}
