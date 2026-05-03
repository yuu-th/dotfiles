package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/state"
)

// pickBrowserProject は指定 profile の Assignments を slot 標準順で探索し、
// browser_workspace を持つ最初の project とその BrowserWorkspace を返す。
//
// 標準順 Q,W,E,R,T,Y,U,I,O,P (projwm-spec.md §4.2)。
func pickBrowserProject(profileName string) (string, *state.BrowserWorkspace) {
	_, st, err := loadStore()
	if err != nil {
		return "", nil
	}
	prof, ok := st.Profiles[profileName]
	if !ok {
		return "", nil
	}
	slotOrder := []string{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"}
	for _, slot := range slotOrder {
		projName, ok := prof.Assignments[slot]
		if !ok {
			continue
		}
		p, ok := st.Projects[projName]
		if !ok || p.Archived {
			continue
		}
		if p.BrowserWorkspace != nil && p.BrowserWorkspace.Name != "" {
			return projName, p.BrowserWorkspace
		}
	}
	return "", nil
}

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
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			err = s.Mutate(func(st *state.State) error {
				if _, ok := st.Profiles[args[0]]; !ok {
					return fmt.Errorf("profile %q not found", args[0])
				}
				st.ActiveProfile = args[0]
				return nil
			})
			if err != nil {
				return err
			}
			// 切替後に reconcile を呼んで windows 操作を実行する
			// (旧 profile の windows close + 新 profile の windows spawn)
			if !rootNoReconcile {
				if err := runReconcileOnce(); err != nil {
					return err
				}
			}
			// browser workspace 統合 (v12, projwm-roadmap.md):
			// active profile の中で browser_workspace を持つ最初の project (slot 標準
			// 順 Q,W,E,R,T,Y,U,I,O,P) を選び、その browser を切り替える。
			// 失敗は warn して reconcile 結果は損なわない。
			if proj, bw := pickBrowserProject(args[0]); bw != nil {
				if err := switchBrowserWorkspace(bw); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: browser switch (%s -> %s:%s) failed: %v\n", proj, bw.Browser, bw.Name, err)
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
