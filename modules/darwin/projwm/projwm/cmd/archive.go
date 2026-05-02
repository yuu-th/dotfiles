package cmd

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/state"
)

func newArchiveCmd() *cobra.Command {
	c := &cobra.Command{Use: "archive", Short: "Archive management"}

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List archived projects",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			names := []string{}
			for n, p := range st.Projects {
				if p.Archived {
					names = append(names, n)
				}
			}
			sort.Strings(names)
			if len(names) == 0 {
				fmt.Println("(no archived projects)")
				return nil
			}
			for _, n := range names {
				p := st.Projects[n]
				fmt.Printf("%s  %s  (%d windows in state)\n", n, p.CWD, len(p.Windows))
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "purge <project>",
		Short: "Remove archived project from state entirely (unrecoverable)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			yes, _ := c.Flags().GetBool("yes")
			if !yes {
				return errors.New("--yes required (purge is unrecoverable)")
			}
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			return s.Mutate(func(st *state.State) error {
				p, ok := st.Projects[args[0]]
				if !ok {
					return fmt.Errorf("project %q not found", args[0])
				}
				if !p.Archived {
					return fmt.Errorf("project %q is not archived (archive first)", args[0])
				}
				delete(st.Projects, args[0])
				return nil
			})
		},
	})
	c.PersistentFlags().Bool("yes", false, "confirm destructive operation")

	// `projwm archive <project>` (top-level) — make it a sibling of subcommands
	// by overriding the parent's Args/Run. Simpler: add an alias top-level command.
	return c
}
