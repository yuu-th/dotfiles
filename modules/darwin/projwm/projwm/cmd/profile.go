package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/state"
)

func newProfileCmd() *cobra.Command {
	c := &cobra.Command{Use: "profile", Short: "Manage profiles"}

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(st.Profiles))
			for n := range st.Profiles {
				names = append(names, n)
			}
			sort.Strings(names)
			if len(names) == 0 {
				fmt.Println("(no profiles)")
				return nil
			}
			for _, n := range names {
				marker := " "
				if n == st.ActiveProfile {
					marker = "*"
				}
				p := st.Profiles[n]
				fmt.Printf("%s %s  (%d assignments)  %s\n", marker, n, len(p.Assignments), p.Description)
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "show [name]",
		Short: "Show profile details (default: active)",
		RunE: func(_ *cobra.Command, args []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			name := st.ActiveProfile
			if len(args) > 0 {
				name = args[0]
			}
			if name == "" {
				return errors.New("no active profile and no name given")
			}
			p, ok := st.Profiles[name]
			if !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			fmt.Printf("profile: %s%s\n", name, marker(name == st.ActiveProfile))
			if p.Description != "" {
				fmt.Printf("description: %s\n", p.Description)
			}
			slots := make([]string, 0, len(p.Assignments))
			for s := range p.Assignments {
				slots = append(slots, s)
			}
			sort.Strings(slots)
			for _, s := range slots {
				fmt.Printf("  [%s] %s\n", s, p.Assignments[s])
			}
			return nil
		},
	})

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			desc, _ := c.Flags().GetString("description")
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			return s.Mutate(func(st *state.State) error {
				if _, ok := st.Profiles[args[0]]; ok {
					return fmt.Errorf("profile %q already exists", args[0])
				}
				st.Profiles[args[0]] = state.Profile{
					Description: desc,
					Assignments: map[string]string{},
				}
				if st.ActiveProfile == "" {
					st.ActiveProfile = args[0]
				}
				return nil
			})
		},
	}
	createCmd.Flags().String("description", "", "human-readable description")
	c.AddCommand(createCmd)

	c.AddCommand(&cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a profile (rejected if active)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			return s.Mutate(func(st *state.State) error {
				if args[0] == st.ActiveProfile {
					return fmt.Errorf("cannot delete active profile %q (switch first)", args[0])
				}
				if _, ok := st.Profiles[args[0]]; !ok {
					return fmt.Errorf("profile %q not found", args[0])
				}
				delete(st.Profiles, args[0])
				return nil
			})
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "switch <name>",
		Short: "Switch active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			s, st, err := loadStore()
			if err != nil {
				return err
			}
			oldActive := st.ActiveProfile
			newActive := args[0]
			oldProjects := activeProjectsOfProfile(st, oldActive)
			err = s.Mutate(func(st *state.State) error {
				if _, ok := st.Profiles[newActive]; !ok {
					return fmt.Errorf("profile %q not found", newActive)
				}
				st.ActiveProfile = newActive
				return nil
			})
			if err != nil {
				return err
			}
			// browser ライフサイクル (v12 paradigm C, FR-29):
			// reconcile は browser に触らないので、ここで明示的に close/spawn を呼ぶ。
			// 1) 旧 profile で active だが新 profile で active 外になる project の
			//    browser windows を snapshot + close
			_, st2, _ := loadStore()
			newProjects := activeProjectsOfProfile(st2, newActive)
			for _, projName := range diffProjects(oldProjects, newProjects) {
				if err := snapshotAndCloseBrowserWindowsForProject(projName); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: close browser for %s: %v\n", projName, err)
				}
			}
			// 2) reconcile (ai/shell/editor の close + spawn)
			if !rootNoReconcile {
				if err := runReconcileOnce(); err != nil {
					return err
				}
			}
			// 3) 新 profile で active 復帰する project の browser windows を spawn
			for _, projName := range diffProjects(newProjects, oldProjects) {
				if err := spawnBrowserWindowsForProject(projName); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: spawn browser for %s: %v\n", projName, err)
				}
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a profile (active follows automatically)",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			return s.Mutate(func(st *state.State) error {
				old, nw := args[0], args[1]
				if _, ok := st.Profiles[old]; !ok {
					return fmt.Errorf("profile %q not found", old)
				}
				if _, ok := st.Profiles[nw]; ok {
					return fmt.Errorf("profile %q already exists", nw)
				}
				st.Profiles[nw] = st.Profiles[old]
				delete(st.Profiles, old)
				if st.ActiveProfile == old {
					st.ActiveProfile = nw
				}
				return nil
			})
		},
	})

	assignCmd := &cobra.Command{
		Use:   "assign <slot> <project>",
		Short: "Add a slot→project assignment to active profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			return s.Mutate(func(st *state.State) error {
				if st.ActiveProfile == "" {
					return errors.New("no active profile")
				}
				prof := st.Profiles[st.ActiveProfile]
				if prof.Assignments == nil {
					prof.Assignments = map[string]string{}
				}
				prof.Assignments[args[0]] = args[1]
				st.Profiles[st.ActiveProfile] = prof
				return nil
			})
		},
	}
	c.AddCommand(assignCmd)

	c.AddCommand(&cobra.Command{
		Use:   "unassign <slot-or-project>",
		Short: "Remove a slot from active profile (slot or project name)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			return s.Mutate(func(st *state.State) error {
				if st.ActiveProfile == "" {
					return errors.New("no active profile")
				}
				prof := st.Profiles[st.ActiveProfile]
				if _, ok := prof.Assignments[args[0]]; ok {
					delete(prof.Assignments, args[0])
				} else {
					for k, v := range prof.Assignments {
						if v == args[0] {
							delete(prof.Assignments, k)
						}
					}
				}
				st.Profiles[st.ActiveProfile] = prof
				return nil
			})
		},
	})

	return c
}

func marker(active bool) string {
	if active {
		return " (active)"
	}
	return ""
}
