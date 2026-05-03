// Package cmd は projwm の cobra CLI ルート + サブコマンド群。
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	rootStateDir     string
	rootNoReconcile  bool
	rootProfileFlag  string
)

// Execute は CLI エントリポイント。
func Execute(version string) error {
	root := &cobra.Command{
		Use:           "projwm",
		Short:         "AI workspace manager built on OmniWM + tmux + Zed",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&rootStateDir, "state-dir", "", "override state dir (default ~/.local/state/projwm)")
	root.PersistentFlags().BoolVar(&rootNoReconcile, "no-reconcile", false, "skip the post-command reconcile pass")
	root.PersistentFlags().StringVar(&rootProfileFlag, "profile", "", "operate on a non-active profile (only some commands)")

	root.AddCommand(newStateCmd())
	root.AddCommand(newProfileCmd())
	root.AddCommand(newArchiveCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newReconcileCmd())
	root.AddCommand(newUpCmd())
	root.AddCommand(newJumpCmd())
	root.AddCommand(newAddAICmd())
	root.AddCommand(newAddShellCmd())
	root.AddCommand(newAddEditorCmd())
	root.AddCommand(newRemoveCmd())
	root.AddCommand(newArchiveTopCmd())
	root.AddCommand(newUnarchiveTopCmd())
	root.AddCommand(newTUICmd())
	root.AddCommand(newBrowserCmd())

	return root.Execute()
}

// fmtErr は internal helpers から共通で使うエラーフォーマット。
func fmtErr(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
