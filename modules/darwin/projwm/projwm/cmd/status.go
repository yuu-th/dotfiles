package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/state"
)

func newStatusCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "Show projwm state summary",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			archived := 0
			for _, p := range st.Projects {
				if p.Archived {
					archived++
				}
			}
			parked := 0
			for n := range st.Projects {
				if state.IsParked(st, n) {
					parked++
				}
			}
			fmt.Printf("profile: %s    archive: %d    parked: %d\n",
				orDash(st.ActiveProfile), archived, parked)
			fmt.Println("───────────────────────────────────────────")
			if st.ActiveProfile != "" {
				prof := st.Profiles[st.ActiveProfile]
				slots := make([]string, 0, len(prof.Assignments))
				for s := range prof.Assignments {
					slots = append(slots, s)
				}
				sort.Strings(slots)
				for _, s := range slots {
					name := prof.Assignments[s]
					p := st.Projects[name]
					fmt.Printf("[%s] %s    %s\n", s, name, p.CWD)
					for _, w := range state.SortedWindows(p) {
						extra := ""
						if w.AI != "" {
							extra = "  " + string(w.AI)
						}
						fmt.Printf("     %s-%d%s\n", w.Kind, w.ID, extra)
					}
				}
			} else {
				fmt.Println("(no active profile)")
			}
			return nil
		},
	}
	return c
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
