package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/naming"
	"github.com/yuu-th/projwm/internal/omniwm"
	"github.com/yuu-th/projwm/internal/reconcile"
	"github.com/yuu-th/projwm/internal/state"
)

func newAddAICmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "add-ai",
		Short: "Add another AI window to the current project",
		RunE: func(c *cobra.Command, _ []string) error {
			ai, _ := c.Flags().GetString("ai")
			project, _ := c.Flags().GetString("project")
			if ai == "" {
				return errors.New("--ai is required")
			}
			if !naming.IsValidAI(naming.AI(ai)) {
				return fmt.Errorf("invalid ai %q", ai)
			}
			return modifyCurrentProject(project, func(st *state.State, name string) error {
				p := st.Projects[name]
				newID := state.NextWindowID(p, naming.KindAI)
				p.Windows = append(p.Windows, state.Window{ID: newID, Kind: naming.KindAI, AI: naming.AI(ai)})
				st.Projects[name] = p
				fmt.Printf("added ai-%d (%s) to %s\n", newID, ai, name)
				return nil
			})
		},
	}
	c.Flags().String("ai", "", "claude | copilot (required)")
	c.Flags().String("project", "", "project name (default: project assigned to focused slot)")
	return c
}

func newAddShellCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "add-shell",
		Short: "Add another shell window to the current project",
		RunE: func(c *cobra.Command, _ []string) error {
			project, _ := c.Flags().GetString("project")
			return modifyCurrentProject(project, func(st *state.State, name string) error {
				p := st.Projects[name]
				newID := state.NextWindowID(p, naming.KindShell)
				p.Windows = append(p.Windows, state.Window{ID: newID, Kind: naming.KindShell})
				st.Projects[name] = p
				fmt.Printf("added shell-%d to %s\n", newID, name)
				return nil
			})
		},
	}
	c.Flags().String("project", "", "project name (default: project assigned to focused slot)")
	return c
}

func newAddBrowserCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "add-browser",
		Short: "Add a browser window (Chromium-based) bound to a profile and URL list (v12, paradigm C)",
		Long: `Add a browser-kind window to the project. Opens immediately in the
specified Chromium user profile with the given URLs.

reconcile は browser に触らない (FR-29)。spawn/close はこの cmd や
profile switch / archive 等の明示イベントでのみ発火する。`,
		RunE: func(c *cobra.Command, _ []string) error {
			project, _ := c.Flags().GetString("project")
			profile, _ := c.Flags().GetString("profile")
			urls, _ := c.Flags().GetStringArray("url")
			if profile == "" {
				return errors.New("--profile is required (Chromium user profile name, e.g. work / client-x)")
			}
			var resolvedName string
			if err := modifyCurrentProject(project, func(st *state.State, name string) error {
				p := st.Projects[name]
				newID := state.NextWindowID(p, naming.KindBrowser)
				p.Windows = append(p.Windows, state.Window{
					ID:             newID,
					Kind:           naming.KindBrowser,
					BrowserProfile: profile,
					SavedURLs:      urls,
				})
				st.Projects[name] = p
				resolvedName = name
				fmt.Printf("added browser-%d (profile=%s, %d urls) to %s\n", newID, profile, len(urls), name)
				return nil
			}); err != nil {
				return err
			}
			// 直後 spawn (reconcile では触らないので明示的に呼ぶ)
			if resolvedName != "" {
				if err := spawnBrowserWindowsForProject(resolvedName); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: spawn browser: %v\n", err)
				}
			}
			return nil
		},
	}
	c.Flags().String("project", "", "project name (default: focused-slot project)")
	c.Flags().String("profile", "", "Chromium user profile name (required, e.g. work / personal)")
	c.Flags().StringArray("url", nil, "URL to open in the new browser window (repeatable)")
	return c
}

func newAddEditorCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "add-editor",
		Short: "Add another Zed editor window to the current project",
		RunE: func(c *cobra.Command, _ []string) error {
			project, _ := c.Flags().GetString("project")
			return modifyCurrentProject(project, func(st *state.State, name string) error {
				p := st.Projects[name]
				newID := state.NextWindowID(p, naming.KindEditor)
				p.Windows = append(p.Windows, state.Window{ID: newID, Kind: naming.KindEditor})
				st.Projects[name] = p
				fmt.Printf("added editor-%d to %s (note: 多 editor は OI-15、Zed の挙動次第)\n", newID, name)
				return nil
			})
		},
	}
	c.Flags().String("project", "", "project name")
	return c
}


func newRemoveCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "remove",
		Short: "Remove a single window (e.g. ai-2, shell-1, editor-1, browser-1)",
		RunE: func(c *cobra.Command, _ []string) error {
			win, _ := c.Flags().GetString("window")
			project, _ := c.Flags().GetString("project")
			if win == "" {
				return errors.New("--window is required (e.g. ai-2)")
			}
			kind, id, err := parseWinSpec(win)
			if err != nil {
				return err
			}
			// browser kind なら state mutate 前に live window を close (paradigm C)
			if kind == naming.KindBrowser {
				_, st, _ := loadStore()
				name := project
				if name == "" {
					if n, e := resolveCurrentProject(st); e == nil {
						name = n
					}
				}
				if name != "" {
					if p, ok := st.Projects[name]; ok {
						for _, w := range p.Windows {
							if w.Kind == naming.KindBrowser && w.ID == id && w.LiveWindowID != "" {
								// 単一 window の close を呼ぶ
								tmpProj := state.Project{Windows: []state.Window{w}}
								_ = tmpProj // 使わない
								if err := snapshotAndCloseSingleBrowser(name, w); err != nil {
									fmt.Fprintf(os.Stderr, "WARN: close browser before remove: %v\n", err)
								}
							}
						}
					}
				}
			}
			return modifyCurrentProject(project, func(st *state.State, name string) error {
				p := st.Projects[name]
				newWindows := p.Windows[:0]
				removed := false
				for _, w := range p.Windows {
					if w.Kind == kind && w.ID == id {
						removed = true
						continue
					}
					newWindows = append(newWindows, w)
				}
				if !removed {
					return fmt.Errorf("no such window %s in project %s", win, name)
				}
				p.Windows = newWindows
				st.Projects[name] = p
				fmt.Printf("removed %s from %s\n", win, name)
				return nil
			})
		},
	}
	c.Flags().String("window", "", "window spec, e.g. ai-2 / shell-1 / editor-1 / browser-1 (required)")
	c.Flags().String("project", "", "project name")
	return c
}

func parseWinSpec(s string) (naming.Kind, int, error) {
	idx := strings.LastIndex(s, "-")
	if idx <= 0 {
		return "", 0, fmt.Errorf("malformed window spec %q (want kind-id e.g. ai-2)", s)
	}
	kind := naming.Kind(s[:idx])
	if !naming.IsValidKind(kind) {
		return "", 0, fmt.Errorf("invalid kind %q in %q", kind, s)
	}
	id, err := strconv.Atoi(s[idx+1:])
	if err != nil || id < 1 {
		return "", 0, fmt.Errorf("invalid id in %q", s)
	}
	return kind, id, nil
}

// modifyCurrentProject は project を解決し、mutate fn を実行、reconcile を呼ぶ。
func modifyCurrentProject(projectFlag string, fn func(*state.State, string) error) error {
	cfgRes, err := config.LoadFromDefaultPath()
	if err != nil {
		return err
	}
	s, _, err := loadStore()
	if err != nil {
		return err
	}
	var resolvedName string
	err = s.Mutate(func(st *state.State) error {
		name := projectFlag
		if name == "" {
			n, err := resolveCurrentProject(st)
			if err != nil {
				return err
			}
			name = n
		}
		if _, ok := st.Projects[name]; !ok {
			return fmt.Errorf("project %q not found (specify --project)", name)
		}
		resolvedName = name
		return fn(st, name)
	})
	if err != nil {
		return err
	}
	if !rootNoReconcile {
		_, st, _ := loadStore()
		r := reconcile.New(cfgRes.Config)
		acts, err := r.Run(context.Background(), st, reconcile.Options{Logger: os.Stderr})
		if err != nil {
			return err
		}
		fmt.Printf("(%s: %d reconcile action(s))\n", resolvedName, len(acts))
	}
	return nil
}

// resolveCurrentProject は focused window の workspace から active profile の
// assignments を引き、project 名を返す。
func resolveCurrentProject(st *state.State) (string, error) {
	if st.ActiveProfile == "" {
		return "", errors.New("no active profile (specify --project)")
	}
	cli := omniwmClient()
	wins, err := cli.QueryWindows(context.Background(), "--focused")
	if err != nil {
		return "", err
	}
	if len(wins) == 0 {
		return "", errors.New("no focused window (specify --project)")
	}
	wsName := wins[0].Workspace.RawName
	if wsName == "" {
		wsName = wins[0].Workspace.DisplayName
	}
	prof := st.Profiles[st.ActiveProfile]
	if name, ok := prof.Assignments[wsName]; ok {
		return name, nil
	}
	return "", fmt.Errorf("focused workspace %q is not assigned in profile %q (specify --project)", wsName, st.ActiveProfile)
}

func omniwmClient() *omniwm.Client {
	return omniwm.New(nil)
}
