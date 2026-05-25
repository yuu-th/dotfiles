package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// cmdTUI implements `projwm tui`.
//
// Per requirement §8.8 行 7: "ユーザが projwm tui 実行 → プロセスが存在しな
// ければ手動 spawn、存在すれば show". This routes the request to projwmd as
// SetCockpitVisibility{Shown}; the daemon's planner emits SpawnCockpit if
// no live cockpit exists, or ShowCockpitOnDisplay if one exists but is
// currently hidden. Direct binary invocation (the old code path) bypassed
// the daemon and accumulated orphan processes — see §8.1 / §8.8.
func cmdTUI(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("tui: no arguments expected")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.SetCockpitVisibility{Visibility: w.CockpitShown})
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}
