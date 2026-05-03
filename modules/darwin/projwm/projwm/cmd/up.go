package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/naming"
	"github.com/yuu-th/projwm/internal/reconcile"
	"github.com/yuu-th/projwm/internal/state"
)

func newUpCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "up",
		Short: "Register cwd as a project and spawn ai+shell+editor",
		Long: `up は cwd を project として登録し、active profile の空き slot に割り当てる。
既定で AI 1 個 + shell 1 個 + editor 1 個 (Zed) を起動。
--ai は必須（暗黙のデフォルトを持たない）。
basename uniqueness を validate（衝突時は --as <内部名> で内部名を分離可）。`,
		RunE: func(c *cobra.Command, _ []string) error {
			ai, _ := c.Flags().GetString("ai")
			cwdFlag, _ := c.Flags().GetString("cwd")
			profileFlag, _ := c.Flags().GetString("profile")
			slotFlag, _ := c.Flags().GetString("slot")
			asFlag, _ := c.Flags().GetString("as")
			noEditor, _ := c.Flags().GetBool("no-editor")

			if ai == "" {
				return errors.New("--ai is required (e.g. claude or copilot)")
			}
			if !naming.IsValidAI(naming.AI(ai)) {
				return fmt.Errorf("invalid --ai %q (use claude or copilot)", ai)
			}
			cwd := cwdFlag
			if cwd == "" {
				w, err := os.Getwd()
				if err != nil {
					return err
				}
				cwd = w
			}
			absCwd, err := filepath.Abs(cwd)
			if err != nil {
				return err
			}
			projName := asFlag
			if projName == "" {
				projName = filepath.Base(absCwd)
			}

			cfgRes, err := config.LoadFromDefaultPath()
			if err != nil {
				return err
			}
			s, _, err := loadStore()
			if err != nil {
				return err
			}

			err = s.Mutate(func(st *state.State) error {
				// プロファイルが空なら作成
				profileName := profileFlag
				if profileName == "" {
					profileName = st.ActiveProfile
				}
				if profileName == "" {
					return errors.New("no active profile (use `projwm profile create <name>` first or pass --profile)")
				}
				if _, ok := st.Profiles[profileName]; !ok {
					return fmt.Errorf("profile %q not found", profileName)
				}

				// project 既存？
				p, exists := st.Projects[projName]
				if !exists {
					p = state.Project{
						CWD:      absCwd,
						Archived: false,
						Windows:  nil,
					}
				} else if p.Archived {
					return fmt.Errorf("project %q is archived (unarchive first)", projName)
				}

				// 既定 windows[]: ai-1 + shell-1 + editor-1（既存があれば再利用）
				if !hasWindow(p, naming.KindAI, 1) {
					p.Windows = append(p.Windows, state.Window{
						ID: state.NextWindowID(p, naming.KindAI), Kind: naming.KindAI, AI: naming.AI(ai),
					})
				}
				if !hasWindow(p, naming.KindShell, 1) {
					p.Windows = append(p.Windows, state.Window{
						ID: state.NextWindowID(p, naming.KindShell), Kind: naming.KindShell,
					})
				}
				if !noEditor && !hasWindow(p, naming.KindEditor, 1) {
					p.Windows = append(p.Windows, state.Window{
						ID: state.NextWindowID(p, naming.KindEditor), Kind: naming.KindEditor,
					})
				}
				st.Projects[projName] = p

				// slot 割当
				prof := st.Profiles[profileName]
				if prof.Assignments == nil {
					prof.Assignments = map[string]string{}
				}
				slot := slotFlag
				if slot == "" {
					slot = pickFreeSlot(prof.Assignments, cfgRes.Config.SlotNames)
					if slot == "" {
						return errors.New("no free slot in profile (specify --slot)")
					}
				}
				// 既に他 project が居る？
				if other, taken := prof.Assignments[slot]; taken && other != projName {
					return fmt.Errorf("slot %q already occupied by %q (use a different --slot)", slot, other)
				}
				prof.Assignments[slot] = projName
				st.Profiles[profileName] = prof

				if st.ActiveProfile == "" {
					st.ActiveProfile = profileName
				}
				return nil
			})
			if err != nil {
				return err
			}

			// reconcile 実行
			if !rootNoReconcile {
				_, st, err := loadStore()
				if err != nil {
					return err
				}
				r := reconcile.New(cfgRes.Config)
				acts, err := r.Run(context.Background(), st, reconcile.Options{Logger: os.Stderr})
				if err != nil {
					return err
				}
				fmt.Printf("up: registered %s, %d reconcile action(s)\n", projName, len(acts))
				errCount := 0
				for _, a := range acts {
					if a.OnError != nil {
						errCount++
						fmt.Fprintf(os.Stderr, "  ERROR %s %s: %v\n", a.Op, a.Target, a.OnError)
					}
				}
				if errCount > 0 {
					return fmt.Errorf("%d action(s) failed during reconcile (see ERROR lines above)", errCount)
				}
			} else {
				fmt.Printf("up: registered %s (reconcile skipped)\n", projName)
			}
			return nil
		},
	}
	c.Flags().String("ai", "", "AI to launch: claude | copilot (required)")
	c.Flags().String("cwd", "", "working directory (default: current)")
	c.Flags().String("profile", "", "target profile (default: active)")
	c.Flags().String("slot", "", "target slot letter (default: first free)")
	c.Flags().String("as", "", "internal project name (default: basename of cwd)")
	c.Flags().Bool("no-editor", false, "skip Zed editor spawn")
	return c
}

// slotFlagOrFirstAssigned は project の active profile での slot 名を返す（情報メッセージ用）。
func slotFlagOrFirstAssigned(projName string) string {
	_, st, err := loadStore()
	if err != nil || st.ActiveProfile == "" {
		return "?"
	}
	for slot, name := range st.Profiles[st.ActiveProfile].Assignments {
		if name == projName {
			return slot
		}
	}
	return "?"
}

func hasWindow(p state.Project, kind naming.Kind, id int) bool {
	for _, w := range p.Windows {
		if w.Kind == kind && w.ID == id {
			return true
		}
	}
	return false
}

func pickFreeSlot(assigned map[string]string, slots []string) string {
	for _, s := range slots {
		if _, taken := assigned[s]; !taken {
			return s
		}
	}
	return ""
}

func newJumpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "jump <slot|project|profile>",
		Short: "Jump to a workspace by slot letter, project name, or profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			arg := args[0]
			cfgRes, err := config.LoadFromDefaultPath()
			if err != nil {
				return err
			}
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			cli := omniwmClient()

			// 1) slot letter（1 文字 + slot_names に含まれる）
			for _, s := range cfgRes.Config.SlotNames {
				if s == arg {
					return cli.FocusWorkspaceByName(context.Background(), s)
				}
			}
			if arg == cfgRes.Config.ViewerWorkspace {
				return cli.FocusWorkspaceByName(context.Background(), arg)
			}
			// 2) project name → active profile の slot を探す
			if st.ActiveProfile != "" {
				for slot, name := range st.Profiles[st.ActiveProfile].Assignments {
					if name == arg {
						return cli.FocusWorkspaceByName(context.Background(), slot)
					}
				}
			}
			// 3) profile name → switch
			if _, ok := st.Profiles[arg]; ok {
				s, _, _ := loadStore()
				return s.Mutate(func(st *state.State) error {
					st.ActiveProfile = arg
					return nil
				})
			}
			return fmt.Errorf("nothing matches %q (slot/project/profile)", arg)
		},
	}
}

// helper: project の windows を kind ソートで列挙する用（status と共用）。
func sortedSlots(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
