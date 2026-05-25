package runtimevalidation

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

const (
	agentStatusActive  = "active"
	agentStatusAbsent  = "absent"
	agentStatusUnknown = "unknown"
)

type LegacyAgentProbe interface {
	Status(ctx context.Context, label string) (status string, detail string, err error)
}

type Validator struct {
	Probe LegacyAgentProbe
}

func NewLaunchctlValidator() Validator {
	return Validator{Probe: LaunchctlProbe{}}
}

func (v Validator) ValidateEnvironment(ctx context.Context, env w.ManagedEnvironment) ([]store.RuntimeValidationReport, bool, error) {
	if len(env.Daemons.LegacyAgents) == 0 {
		return nil, false, nil
	}
	probe := v.Probe
	if probe == nil {
		probe = LaunchctlProbe{}
	}
	agents := append([]w.LegacyAgentPolicy(nil), env.Daemons.LegacyAgents...)
	sort.Slice(agents, func(i, j int) bool { return agents[i].Label < agents[j].Label })

	reports := make([]store.RuntimeValidationReport, 0, len(agents))
	blocking := false
	for _, policy := range agents {
		status, detail, err := probe.Status(ctx, policy.Label)
		if err != nil {
			return nil, false, err
		}
		action := "none"
		block := false
		switch {
		case status == agentStatusActive && policy.Action == "remove":
			action = "remove-by-nix-rebuild"
			block = true
			detail = detailOrDefault(detail, "legacy writer is active; mutation is blocked until Nix removes the agent")
		case status == agentStatusAbsent && policy.Action == "remove":
			action = "removal-satisfied"
			detail = detailOrDefault(detail, "legacy writer is absent; remove policy is satisfied")
		case status == agentStatusUnknown:
			action = "report-unverified"
			detail = detailOrDefault(detail, "legacy writer status could not be verified")
		default:
			action = "report"
			detail = detailOrDefault(detail, "legacy writer was reported according to manifest policy")
		}
		if block {
			blocking = true
		}
		reports = append(reports, store.RuntimeValidationReport{
			Kind:     "legacy-agent",
			Subject:  policy.Label,
			Policy:   policy.Action,
			Status:   status,
			Action:   action,
			Blocking: block,
			Detail:   detail,
		})
	}
	return reports, blocking, nil
}

type LaunchctlProbe struct{}

func (LaunchctlProbe) Status(ctx context.Context, label string) (string, string, error) {
	if label == "" {
		return "", "", fmt.Errorf("runtimevalidation: legacy agent label is required")
	}
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
	cmd := exec.CommandContext(ctx, "/bin/launchctl", "print", target)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	body := strings.TrimSpace(out.String())
	if err == nil {
		return agentStatusActive, "launchctl reported the legacy writer as loaded", nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", "", ctxErr
	}
	if isLaunchctlNotFound(body) {
		return agentStatusAbsent, "launchctl did not find the legacy writer", nil
	}
	return agentStatusUnknown, redactedLaunchctlFailure(body), nil
}

func isLaunchctlNotFound(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "could not find service") ||
		strings.Contains(lower, "service is not loaded") ||
		strings.Contains(lower, "no such process") ||
		strings.Contains(lower, "domain does not support specified action")
}

func redactedLaunchctlFailure(body string) string {
	if body == "" {
		return "launchctl returned a non-zero status without detail"
	}
	fields := strings.Fields(body)
	if len(fields) > 16 {
		fields = fields[:16]
	}
	return "launchctl status unknown: " + strings.Join(fields, " ")
}

func detailOrDefault(detail, fallback string) string {
	if strings.TrimSpace(detail) == "" {
		return fallback
	}
	return detail
}
