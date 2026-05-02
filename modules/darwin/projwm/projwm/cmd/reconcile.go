package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/reconcile"
)

func newReconcileCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile state.json with the running OmniWM/tmux/ghostty/Zed reality",
		RunE: func(c *cobra.Command, _ []string) error {
			dry, _ := c.Flags().GetBool("dry-run")
			verb, _ := c.Flags().GetBool("verbose")
			gc, _ := c.Flags().GetBool("gc")
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			cfgRes, err := config.LoadFromDefaultPath()
			if err != nil {
				return err
			}
			r := reconcile.New(cfgRes.Config)
			ctx := context.Background()
			acts, err := r.Run(ctx, st, reconcile.Options{
				DryRun: dry, Verbose: verb, GC: gc, Logger: os.Stderr,
			})
			if err != nil {
				return err
			}
			if dry {
				fmt.Println("(dry-run) planned actions:")
			} else {
				fmt.Println("actions:")
			}
			for _, a := range acts {
				suffix := ""
				if a.OnError != nil {
					suffix = "  ERROR=" + a.OnError.Error()
				}
				fmt.Printf("  %-15s %s  %s%s\n", a.Op, a.Target, a.Detail, suffix)
			}
			if len(acts) == 0 {
				fmt.Println("  (no diff)")
			}
			return nil
		},
	}
	c.Flags().Bool("dry-run", false, "print planned actions without executing")
	c.Flags().Bool("verbose", false, "verbose logging")
	c.Flags().Bool("gc", false, "close orphan windows that look like projwm-managed but have no state entry")
	return c
}
