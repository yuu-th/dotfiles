package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the projwm cockpit TUI (alt+space target)",
		RunE: func(_ *cobra.Command, _ []string) error {
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			cfgRes, err := config.LoadFromDefaultPath()
			if err != nil {
				return err
			}
			return tui.Run(s, cfgRes.Config)
		},
	}
}
