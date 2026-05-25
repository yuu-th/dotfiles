package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// cmdCockpit implements `projwm cockpit <show|hide>`.
//
// SSOT N-06 (2026-05-20): cockpit is summon-only. The legacy `toggle` and
// `focus` sub-actions are mapped onto SetCockpitVisibility{Shown} —
// pressing the hotkey always brings cockpit to the user, never away.
// Hiding is reached by SetCockpitVisibility{Hidden} (or Esc inside the
// TUI), not by a toggle press.
func cmdCockpit(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("cockpit: usage: projwm cockpit <show|hide>")
	}
	action := args[0]
	var in intent.Intent
	switch action {
	case "show", "toggle", "focus":
		in = intent.SetCockpitVisibility{Visibility: w.CockpitShown}
	case "hide":
		in = intent.SetCockpitVisibility{Visibility: w.CockpitHidden}
	default:
		return fmt.Errorf("cockpit: unknown action %q (want show|hide)", action)
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, in)
	if err != nil {
		return fmt.Errorf("cockpit %s: %w", action, err)
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}
