package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/ipc"
)

// cmdReconcile implements `projwm reconcile [--dry-run] [--verbose]`.
//
// `--dry-run` asks projwmd for a Query("plan-preview"): the daemon runs the
// planner against its live WorldState and DesiredWorld and returns the
// resulting operations without committing or mutating the WM. Requirements
// §5.9 / §16.1 fully covered: operations list, predicted diff, and the
// "Already converged (0 ops)" message.
func cmdReconcile(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "compute planner ops without committing")
	verbose := fs.Bool("verbose", false, "include per-operation target / risk detail")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dryRun {
		return runDryRunPreview(gf, *verbose, stdout)
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.Reconcile{})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

// runDryRunPreview submits Query("plan-preview") and renders the result.
//
// Output format (intentionally human-readable; --json variant could be added):
//
//	Dry-run preview (plan <plan-id>, base epoch <n>, reason reconcile)
//	  spawn-1     SpawnProjectTerminal  desired=dw_..  risk=medium
//	  layout-2    ReorderColumns        workspace=Q    risk=low
//	  ...
//	Total: 12 operations (0 risk=high, 8 medium, 4 low)
//
// Empty plan → "Already converged (0 ops)."
func runDryRunPreview(gf globalFlags, verbose bool, stdout io.Writer) error {
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.Query(ctx, ipc.QueryPlanPreview, "")
	if err != nil {
		return fmt.Errorf("reconcile --dry-run: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("reconcile --dry-run: daemon: %s", resp.Error.Message)
	}
	var preview struct {
		PlanID     string                   `json:"planId"`
		BaseEpoch  int                      `json:"baseEpoch"`
		Reason     string                   `json:"reason"`
		Scope      []string                 `json:"scope"`
		Operations []map[string]interface{} `json:"operations"`
		Converged  bool                     `json:"converged"`
	}
	if err := json.Unmarshal(resp.Body, &preview); err != nil {
		return fmt.Errorf("reconcile --dry-run: parse response: %w", err)
	}
	if preview.Converged {
		fmt.Fprintln(stdout, "Already converged (0 ops).")
		return nil
	}
	fmt.Fprintf(stdout, "Dry-run preview (plan %s, base epoch %d, reason %s)\n",
		preview.PlanID, preview.BaseEpoch, preview.Reason)
	risks := map[string]int{}
	for _, op := range preview.Operations {
		kind, _ := op["kind"].(string)
		id, _ := op["id"].(string)
		risk, _ := op["risk"].(string)
		risks[risk]++
		if verbose {
			fmt.Fprintf(stdout, "  %s\t%s\trisk=%s", id, kind, risk)
			for _, field := range []string{"liveWindow", "desiredWindow", "workspace", "systemWindow", "idempotencyKey"} {
				if v, ok := op[field]; ok {
					fmt.Fprintf(stdout, " %s=%v", field, v)
				}
			}
			fmt.Fprintln(stdout)
		} else {
			fmt.Fprintf(stdout, "  %s\t%s\trisk=%s\n", id, kind, risk)
		}
	}
	fmt.Fprintf(stdout, "Total: %d operations (high=%d medium=%d low=%d)\n",
		len(preview.Operations), risks["high"], risks["medium"], risks["low"])
	return nil
}

// cmdValidateEnvironment implements `projwm validate-environment`.
//
// Submits the ValidateEnvironment intent (daemon-side runtime check).
// The wire response is plain ok/fail; doctor surfaces more detail.
func cmdValidateEnvironment(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("validate-environment: no arguments expected")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.ValidateEnvironment{})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}
