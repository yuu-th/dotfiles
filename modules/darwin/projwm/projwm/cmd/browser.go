package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/browserwrap/chromium"
	"github.com/yuu-th/projwm/internal/naming"
)

// newBrowserCmd は `projwm browser ...` の root command（v12 paradigm C 用）。
//
// 状態 query と user の手動 navigation 補助のみ。bind は `projwm add-browser`、
// remove は `projwm remove --window=browser-N`、archive 等は既存 cmd 経由で。
// destructive 操作は本 sub-command には置かない（reconcile が一元管理）。
func newBrowserCmd() *cobra.Command {
	c := &cobra.Command{Use: "browser", Short: "Browser window inspection (read-only, v12 paradigm C)"}

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List browser windows registered in state and live tab counts",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			d := chromium.New()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			running := d.IsRunning()

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
					any = true
					line := fmt.Sprintf("%s\tbrowser-%d\tprofile=%s\tsaved=%d", n, w.ID, w.BrowserProfile, len(w.SavedURLs))
					if running {
						if wid, found, _ := d.FindWindowByMarker(ctx, n, w.ID); found {
							if tabs, terr := d.ListTabsInWindow(ctx, wid); terr == nil {
								line += fmt.Sprintf("\tlive=%d (wid=%s)", len(tabs), wid)
							}
						} else {
							line += "\tlive=(no window)"
						}
					}
					fmt.Println(line)
				}
			}
			if !any {
				fmt.Println("(no browser windows)")
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "focus <project> [browser-id]",
		Short: "Bring the project's browser window to front (default id=1)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			p, ok := st.Projects[args[0]]
			if !ok {
				return fmt.Errorf("project %q not found", args[0])
			}
			id := 1
			if len(args) == 2 {
				if _, e := fmt.Sscanf(args[1], "%d", &id); e != nil {
					return fmt.Errorf("invalid browser id %q", args[1])
				}
			}
			found := false
			for _, w := range p.Windows {
				if w.Kind == naming.KindBrowser && w.ID == id {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("project %q has no browser-%d window", args[0], id)
			}
			d := chromium.New()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			wid, ok2, err := d.FindWindowByMarker(ctx, args[0], id)
			if err != nil {
				return err
			}
			if !ok2 {
				return fmt.Errorf("no live Vivaldi window for %s browser-%d (run `projwm reconcile` to spawn)", args[0], id)
			}
			// chrome-cli activate -t は tab 単位、focus したい時はそれで OK。
			// ここでは window の active tab を activate (--focus で window も前面)。
			tabs, err := d.ListTabsInWindow(ctx, wid)
			if err != nil || len(tabs) == 0 {
				return fmt.Errorf("no tabs in window %s", wid)
			}
			cmd := exec.Command(d.ChromeCli, "activate", "-t", tabs[0].ID, "--focus")
			cmd.Env = append(os.Environ(), "CHROME_BUNDLE_IDENTIFIER="+d.BundleID)
			return cmd.Run()
		},
	})

	return c
}
