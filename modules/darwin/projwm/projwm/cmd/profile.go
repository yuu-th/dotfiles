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
			// (j) old active で out になる project の layout を mutate 前に snapshot
			//     (slot は old profile での slot 必要なので mutate 前に取る)
			// 注: newProjects は mutate 後に再 load するが、 ここでは old active
			//     視点で居る project のみ対象なので、 oldActive を渡す。
			//     差分計算は new load 後に行うが、 snapshot 自体は全 oldProjects
			//     に対して行う (どうせ slot 解決して slot ws の現状を撮るだけ)。
			//     ただし「new でも active な project」も含めて snapshot しても
			//     後で restore 時に上書きされるだけで害はない。 シンプルさ優先。
			for _, projName := range oldProjects {
				snapshotLayoutForProject(oldActive, projName)
			}
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
			//    browser windows を snapshot + close (focus 不要、 並列で OK)
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
			// 3) 新 profile で active 復帰する project の browser windows を **bulk** spawn
			//    (origWS save/restore を 1 回に集約、 各 project の slot に順次 focus
			//    切替して spawn → 全完了で origWS 戻す。 race / 二重切替なし)
			toSpawn := diffProjects(newProjects, oldProjects)
			if len(toSpawn) > 0 {
				if err := spawnBrowserWindowsForMultipleProjects(newActive, toSpawn); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: bulk spawn browsers: %v\n", err)
				}
			}
			// (j) layout restore: 新 profile で active 復帰する project に対し
			//     spawn 完了 (reconcile + browser) 後に column/stack を再構築。
			for _, projName := range diffProjects(newProjects, oldProjects) {
				restoreLayoutForProject(newActive, projName)
			}
			// viewer (WS A) を slot_names 順に整列。reconcile の async spawn で
			// column 順が不定になるため、全 restore 完了後に修正する。
			fixViewerOrderForProfile(newActive)
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
			projName := args[1]
			var becameActive bool
			if err := s.Mutate(func(st *state.State) error {
				if st.ActiveProfile == "" {
					return errors.New("no active profile")
				}
				prof := st.Profiles[st.ActiveProfile]
				if prof.Assignments == nil {
					prof.Assignments = map[string]string{}
				}
				// 既に他 slot で同じ project が assigned なら active 状態は不変
				wasActive := false
				for _, v := range prof.Assignments {
					if v == projName {
						wasActive = true
						break
					}
				}
				prof.Assignments[args[0]] = projName
				st.Profiles[st.ActiveProfile] = prof
				becameActive = !wasActive
				return nil
			}); err != nil {
				return err
			}
			// reconcile (ai/shell/editor の spawn) + browser spawn
			if !rootNoReconcile {
				if err := runReconcileOnce(); err != nil {
					return err
				}
			}
			if becameActive {
				if err := spawnBrowserWindowsForProject(projName); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: spawn browser on assign: %v\n", err)
				}
			}
			return nil
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
			var unassignedProjects []string
			if err := s.Mutate(func(st *state.State) error {
				if st.ActiveProfile == "" {
					return errors.New("no active profile")
				}
				prof := st.Profiles[st.ActiveProfile]
				if v, ok := prof.Assignments[args[0]]; ok {
					unassignedProjects = append(unassignedProjects, v)
					delete(prof.Assignments, args[0])
				} else {
					for k, v := range prof.Assignments {
						if v == args[0] {
							unassignedProjects = append(unassignedProjects, v)
							delete(prof.Assignments, k)
						}
					}
				}
				st.Profiles[st.ActiveProfile] = prof
				return nil
			}); err != nil {
				return err
			}
			// active 外になった project の browser を close
			// (ai/shell/editor は次の reconcile で close される)
			for _, projName := range unassignedProjects {
				// 同 profile の他 slot に同じ project がまだあるか確認
				_, st2, _ := loadStore()
				stillActive := false
				if prof, ok := st2.Profiles[st2.ActiveProfile]; ok {
					for _, v := range prof.Assignments {
						if v == projName {
							stillActive = true
							break
						}
					}
				}
				if !stillActive {
					if err := snapshotAndCloseBrowserWindowsForProject(projName); err != nil {
						fmt.Fprintf(os.Stderr, "WARN: close browser on unassign: %v\n", err)
					}
				}
			}
			if !rootNoReconcile {
				return runReconcileOnce()
			}
			return nil
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
