package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/naming"
)

// newBrowserCmd は `projwm browser ...` の root command（v12 paradigm C, read-only）。
//
// 状態 query のみ。bind は `projwm add-browser`、close/spawn は明示
// イベント (profile switch / archive / unarchive / remove) に内包される。
func newBrowserCmd() *cobra.Command {
	c := &cobra.Command{Use: "browser", Short: "Browser window inspection (read-only, v12 paradigm C)"}

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List browser windows registered in state",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			projNames := make([]string, 0, len(st.Projects))
			for n := range st.Projects {
				projNames = append(projNames, n)
			}
			sort.Strings(projNames)
			any := false
			for _, n := range projNames {
				p := st.Projects[n]
				for _, w := range p.Windows {
					if w.Kind != naming.KindBrowser {
						continue
					}
					live := "—"
					if w.LiveWindowID != "" {
						live = w.LiveWindowID
					}
					fmt.Printf("%s\tbrowser-%d\tprofile=%s\tsaved=%d\twid=%s\n",
						n, w.ID, w.BrowserProfile, len(w.SavedURLs), live)
					any = true
				}
			}
			if !any {
				fmt.Println("(no browser windows)")
			}
			return nil
		},
	})

	return c
}
