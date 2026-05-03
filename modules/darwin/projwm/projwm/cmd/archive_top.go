package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/reconcile"
	"github.com/yuu-th/projwm/internal/state"
)

func newArchiveTopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive-project <project>",
		Short: "Archive a project (kill tmux, close windows, keep state)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			name := args[0]
			cfgRes, err := config.LoadFromDefaultPath()
			if err != nil {
				return err
			}
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			// browser kind 用に snapshot+close を archive 前に呼ぶ (paradigm C)。
			// archive 後だと state.Project が p.Archived=true で project lookup
			// 経路が変わる場合があるので、archived flag セット前に処理しておく。
			if err := snapshotAndCloseBrowserWindowsForProject(name); err != nil {
				fmt.Fprintf(os.Stderr, "WARN: close browser before archive: %v\n", err)
			}
			err = s.Mutate(func(st *state.State) error {
				p, ok := st.Projects[name]
				if !ok {
					return fmt.Errorf("project %q not found", name)
				}
				if p.Archived {
					fmt.Printf("%s already archived (no-op)\n", name)
					return nil
				}
				p.Archived = true
				st.Projects[name] = p
				// 全 profile から assignment を解除
				for pn, prof := range st.Profiles {
					for slot, target := range prof.Assignments {
						if target == name {
							delete(prof.Assignments, slot)
						}
					}
					st.Profiles[pn] = prof
				}
				return nil
			})
			if err != nil {
				return err
			}
			if !rootNoReconcile {
				_, st, _ := loadStore()
				r := reconcile.New(cfgRes.Config)
				acts, _ := r.Run(context.Background(), st, reconcile.Options{Logger: os.Stderr})
				fmt.Printf("archived %s, %d reconcile action(s)\n", name, len(acts))
			}
			return nil
		},
	}
}

func newUnarchiveTopCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "unarchive <project>",
		Short: "Unarchive a project (becomes parked, no auto-assign)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			profile, _ := c.Flags().GetString("profile")
			slot, _ := c.Flags().GetString("slot")
			name := args[0]
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			isNowActive := false
			if err := s.Mutate(func(st *state.State) error {
				p, ok := st.Projects[name]
				if !ok {
					return fmt.Errorf("project %q not found", name)
				}
				if !p.Archived {
					fmt.Printf("%s not archived (no-op)\n", name)
					return nil
				}
				p.Archived = false
				st.Projects[name] = p
				if profile != "" && slot != "" {
					prof, ok := st.Profiles[profile]
					if !ok {
						return fmt.Errorf("profile %q not found", profile)
					}
					if prof.Assignments == nil {
						prof.Assignments = map[string]string{}
					}
					if other, taken := prof.Assignments[slot]; taken && other != name {
						return fmt.Errorf("slot %q already occupied by %q in profile %q", slot, other, profile)
					}
					prof.Assignments[slot] = name
					st.Profiles[profile] = prof
					if profile == st.ActiveProfile {
						isNowActive = true
					}
				} else if profile != "" || slot != "" {
					return errors.New("--profile and --slot must be specified together")
				}
				fmt.Printf("unarchived %s\n", name)
				return nil
			}); err != nil {
				return err
			}
			// active profile に戻ってきたなら browser を spawn (paradigm C)
			if isNowActive {
				if err := spawnBrowserWindowsForProject(name); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: spawn browser after unarchive: %v\n", err)
				}
			}
			return nil
		},
	}
	c.Flags().String("profile", "", "assign to this profile (optional)")
	c.Flags().String("slot", "", "assign to this slot (optional, requires --profile)")
	return c
}

// 共有用 mock
var _ = state.New
