package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yuu-th/projwm/internal/state"
)

func paths() (state.Paths, error) {
	if rootStateDir != "" {
		return state.Paths{
			Dir:        rootStateDir,
			StateFile:  filepath.Join(rootStateDir, "state.json"),
			BackupFile: filepath.Join(rootStateDir, "state.json.bak"),
			LockFile:   filepath.Join(rootStateDir, "lock"),
			LogsDir:    filepath.Join(rootStateDir, "logs"),
		}, nil
	}
	return state.DefaultPaths()
}

func loadStore() (*state.Store, *state.State, error) {
	p, err := paths()
	if err != nil {
		return nil, nil, err
	}
	s := state.NewStore(p)
	st, err := s.Load()
	if err != nil {
		return nil, nil, err
	}
	return s, st, nil
}

func newStateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "state",
		Short: "Inspect or edit state.json directly",
	}
	c.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print state.json (pretty)",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, st, err := loadStore()
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(st, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open state.json in $EDITOR",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := paths()
			if err != nil {
				return err
			}
			s := state.NewStore(p)
			if err := s.Touch(); err != nil { // ensure file exists
				return err
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "nvim"
			}
			fmt.Printf("Open %s with %s manually (auto-launch not implemented).\n", p.StateFile, editor)
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "repair",
		Short: "Drop invalid entries to recover",
		RunE: func(_ *cobra.Command, _ []string) error {
			s, _, err := loadStore()
			if err != nil {
				return err
			}
			return s.Mutate(func(st *state.State) error {
				if st.ActiveProfile != "" {
					if _, ok := st.Profiles[st.ActiveProfile]; !ok {
						st.ActiveProfile = ""
					}
				}
				for pname, prof := range st.Profiles {
					for slot, proj := range prof.Assignments {
						if _, ok := st.Projects[proj]; !ok {
							delete(prof.Assignments, slot)
						}
					}
					st.Profiles[pname] = prof
				}
				return nil
			})
		},
	})
	return c
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Sanity-check the projwm environment",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := paths()
			if err != nil {
				return err
			}
			fmt.Printf("state dir:  %s\n", p.Dir)
			if _, err := os.Stat(p.StateFile); err == nil {
				fmt.Printf("state file: present\n")
			} else {
				fmt.Printf("state file: not yet created (will be on first mutate)\n")
			}
			return nil
		},
	}
}
