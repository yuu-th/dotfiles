package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

type Manifest struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Source        string                `json:"source"`
	Environment   EnvironmentContract   `json:"environment"`
	Authorities   []AuthorityAssignment `json:"authorities"`
	Writers       []Writer              `json:"writers"`
}

type EnvironmentContract struct {
	WMBackend       string   `json:"wmBackend"`
	ViewerWorkspace string   `json:"viewerWorkspace"`
	SlotWorkspaces  []string `json:"slotWorkspaces"`
}

type AuthorityAssignment struct {
	Resource string `json:"resource"`
	Owner    string `json:"owner"`
}

type Writer struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Resources []string `json:"resources"`
}

var allowedOwners = []string{
	"nix",
	"persistent-store",
	"observer",
	"predicted-world",
	"projwmd",
	"private-payload-store",
}

func Decode(r io.Reader) (Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, err
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion <= 0 {
		return errors.New("schemaVersion is required")
	}
	if m.Source != "nix" {
		return fmt.Errorf("source must be nix, got %q", m.Source)
	}
	if m.Environment.WMBackend == "" {
		return errors.New("environment.wmBackend is required")
	}
	if m.Environment.ViewerWorkspace == "" {
		return errors.New("environment.viewerWorkspace is required")
	}
	if len(m.Environment.SlotWorkspaces) == 0 {
		return errors.New("environment.slotWorkspaces must not be empty")
	}
	for _, slot := range m.Environment.SlotWorkspaces {
		if slot == m.Environment.ViewerWorkspace {
			return fmt.Errorf("viewer workspace %q must not also be a slot workspace", slot)
		}
	}

	owners := map[string]string{}
	for _, a := range m.Authorities {
		if a.Resource == "" || a.Owner == "" {
			return errors.New("authority resource and owner are required")
		}
		if !slices.Contains(allowedOwners, a.Owner) {
			return fmt.Errorf("unknown authority owner %q", a.Owner)
		}
		if prev, ok := owners[a.Resource]; ok && prev != a.Owner {
			return fmt.Errorf("resource %q has duplicate owners %q and %q", a.Resource, prev, a.Owner)
		}
		owners[a.Resource] = a.Owner
	}

	normalMutationOwners := map[string]string{}
	for _, w := range m.Writers {
		if w.Name == "" || w.Kind == "" {
			return errors.New("writer name and kind are required")
		}
		if w.Kind != "normal-mutator" {
			continue
		}
		if w.Name != "projwmd" {
			return fmt.Errorf("normal mutation writer must be projwmd, got %q", w.Name)
		}
		for _, r := range w.Resources {
			if prev, ok := normalMutationOwners[r]; ok && prev != w.Name {
				return fmt.Errorf("resource %q has conflicting normal writers %q and %q", r, prev, w.Name)
			}
			normalMutationOwners[r] = w.Name
		}
	}
	return nil
}
