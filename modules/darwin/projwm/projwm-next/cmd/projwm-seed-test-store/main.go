// projwm-seed-test-store is a tiny helper used by the E2E CLI smoke test
// to bootstrap a test-kind PersistentStore directly via internal/store,
// bypassing projwmstore-bootstrap's production safety checks.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func main() {
	storeDir := flag.String("store-dir", "", "store directory to bootstrap")
	flag.Parse()
	if *storeDir == "" {
		fmt.Fprintln(os.Stderr, "--store-dir required")
		os.Exit(2)
	}
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {
				ID:             "work",
				Description:    "primary",
				InactivePolicy: w.InactivePolicyRemove,
				Assignments:    map[w.SlotID]w.ProjectID{"Q": "dotfiles"},
			},
			"misc": {
				ID:             "misc",
				InactivePolicy: w.InactivePolicyKeep,
				Assignments:    map[w.SlotID]w.ProjectID{},
			},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {ID: "dotfiles"},
			"spike-x":  {ID: "spike-x"},
			"old":      {ID: "old", Archived: true},
		},
	}
	if _, err := store.OpenFileStore(context.Background(), *storeDir, store.StoreKindTest, desired); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("seeded test store at %s\n", *storeDir)
}
