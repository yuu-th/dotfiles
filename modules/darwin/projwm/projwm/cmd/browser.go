package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/browserwrap/vivaldi"
	"github.com/yuu-th/projwm/internal/state"
)

// newBrowserCmd は `projwm browser ...` の root command を返す。
func newBrowserCmd() *cobra.Command {
	c := &cobra.Command{Use: "browser", Short: "Browser workspace integration (Vivaldi)"}

	// browser set <project> <browser> <workspace-name>
	c.AddCommand(&cobra.Command{
		Use:   "set <project> <browser> <workspace-name>",
		Short: "Bind a browser workspace to a project",
		Long:  "Currently only `vivaldi` is supported as the browser argument (v12).",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			projName, browser, wsName := args[0], args[1], args[2]
			if browser != "vivaldi" {
				return fmt.Errorf("unsupported browser %q (only \"vivaldi\" in v12)", browser)
			}
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			return s.Mutate(func(st *state.State) error {
				p, ok := st.Projects[projName]
				if !ok {
					return fmt.Errorf("project %q not found", projName)
				}
				p.BrowserWorkspace = &state.BrowserWorkspace{Browser: browser, Name: wsName}
				st.Projects[projName] = p
				return nil
			})
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "unset <project>",
		Short: "Remove browser workspace binding from a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			return s.Mutate(func(st *state.State) error {
				p, ok := st.Projects[args[0]]
				if !ok {
					return fmt.Errorf("project %q not found", args[0])
				}
				p.BrowserWorkspace = nil
				st.Projects[args[0]] = p
				return nil
			})
		},
	})

	// browser switch <project>
	c.AddCommand(&cobra.Command{
		Use:   "switch <project>",
		Short: "Switch the bound browser to the project's workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			p, ok := st.Projects[args[0]]
			if !ok {
				return fmt.Errorf("project %q not found", args[0])
			}
			if p.BrowserWorkspace == nil {
				return fmt.Errorf("project %q has no browser_workspace set", args[0])
			}
			return switchBrowserWorkspace(p.BrowserWorkspace)
		},
	})

	// browser ensure: read all projects with BrowserWorkspace and seed them in Vivaldi
	c.AddCommand(&cobra.Command{
		Use:   "ensure",
		Short: "Ensure all bound workspaces exist in Vivaldi (creates missing ones via Preferences edit)",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			names := collectVivaldiWorkspaceNames(st)
			if len(names) == 0 {
				fmt.Println("(no projects with vivaldi browser_workspace)")
				return nil
			}
			d := vivaldi.New()
			d.Logger = os.Stderr
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := d.EnsureWorkspaces(ctx, names); err != nil {
				return err
			}
			fmt.Printf("ensured %d workspaces: %v\n", len(names), names)
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List browser_workspace bindings per project",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(st.Projects))
			for n := range st.Projects {
				names = append(names, n)
			}
			sort.Strings(names)
			any := false
			for _, n := range names {
				bw := st.Projects[n].BrowserWorkspace
				if bw == nil {
					continue
				}
				fmt.Printf("%s\t%s:%s\n", n, bw.Browser, bw.Name)
				any = true
			}
			if !any {
				fmt.Println("(no bindings)")
			}
			return nil
		},
	})

	return c
}

// switchBrowserWorkspace は browser kind 別 driver で workspace 切替を実行する。
func switchBrowserWorkspace(bw *state.BrowserWorkspace) error {
	if bw == nil || bw.Name == "" {
		return fmt.Errorf("empty browser_workspace")
	}
	switch bw.Browser {
	case "vivaldi":
		d := vivaldi.New()
		d.Logger = os.Stderr
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return d.SwitchWorkspace(ctx, bw.Name)
	default:
		return fmt.Errorf("unsupported browser %q", bw.Browser)
	}
}

// collectVivaldiWorkspaceNames は state 内の全 vivaldi workspace 名（重複排除済）を返す。
func collectVivaldiWorkspaceNames(st *state.State) []string {
	seen := map[string]bool{}
	for _, p := range st.Projects {
		if p.Archived || p.BrowserWorkspace == nil {
			continue
		}
		if p.BrowserWorkspace.Browser == "vivaldi" && p.BrowserWorkspace.Name != "" {
			seen[p.BrowserWorkspace.Name] = true
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
