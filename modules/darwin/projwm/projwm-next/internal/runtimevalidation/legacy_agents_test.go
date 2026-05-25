package runtimevalidation

import (
	"context"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

type fakeProbe map[string]string

func (s fakeProbe) Status(ctx context.Context, label string) (string, string, error) {
	if status, ok := s[label]; ok {
		return status, "fake", nil
	}
	return agentStatusUnknown, "fake unknown", nil
}

func TestValidatorReportsLegacyAgentsDeterministically(t *testing.T) {
	env := w.ManagedEnvironment{Daemons: w.DaemonEnvironment{LegacyAgents: []w.LegacyAgentPolicy{
		{Label: "z-agent", Action: "report"},
		{Label: "a-agent", Action: "remove"},
	}}}
	v := Validator{Probe: fakeProbe{"z-agent": agentStatusActive, "a-agent": agentStatusAbsent}}

	reports, blocking, err := v.ValidateEnvironment(context.Background(), env)
	if err != nil {
		t.Fatalf("ValidateEnvironment: %v", err)
	}
	if blocking {
		t.Fatalf("absent remove/report policies should not block: %+v", reports)
	}
	if len(reports) != 2 || reports[0].Subject != "a-agent" || reports[1].Subject != "z-agent" {
		t.Fatalf("reports not sorted by label: %+v", reports)
	}
	if reports[0].Status != agentStatusAbsent || reports[0].Action != "removal-satisfied" || reports[0].Blocking {
		t.Fatalf("remove/absent report mismatch: %+v", reports[0])
	}
	if reports[1].Status != agentStatusActive || reports[1].Action != "report" || reports[1].Blocking {
		t.Fatalf("report/active report mismatch: %+v", reports[1])
	}
}

func TestValidatorBlocksActiveRemoveLegacyAgent(t *testing.T) {
	env := w.ManagedEnvironment{Daemons: w.DaemonEnvironment{LegacyAgents: []w.LegacyAgentPolicy{
		{Label: "old-writer", Action: "remove"},
	}}}
	v := Validator{Probe: fakeProbe{"old-writer": agentStatusActive}}

	reports, blocking, err := v.ValidateEnvironment(context.Background(), env)
	if err != nil {
		t.Fatalf("ValidateEnvironment: %v", err)
	}
	if !blocking || len(reports) != 1 || !reports[0].Blocking || reports[0].Action != "remove-by-nix-rebuild" {
		t.Fatalf("active remove agent did not block: blocking=%v reports=%+v", blocking, reports)
	}
}

func TestValidatorTreatsUnknownAsReportOnly(t *testing.T) {
	env := w.ManagedEnvironment{Daemons: w.DaemonEnvironment{LegacyAgents: []w.LegacyAgentPolicy{
		{Label: "maybe-writer", Action: "remove"},
	}}}
	v := Validator{Probe: fakeProbe{}}

	reports, blocking, err := v.ValidateEnvironment(context.Background(), env)
	if err != nil {
		t.Fatalf("ValidateEnvironment: %v", err)
	}
	if blocking || len(reports) != 1 || reports[0].Status != agentStatusUnknown || reports[0].Action != "report-unverified" {
		t.Fatalf("unknown status should be report-only: blocking=%v reports=%+v", blocking, reports)
	}
}
