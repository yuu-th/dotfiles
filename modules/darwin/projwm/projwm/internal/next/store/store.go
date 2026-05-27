package store

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Generation string

const InitialGeneration Generation = "E0001-G000000"

type Store struct {
	root     string
	identity Identity
}

type Commit struct {
	store    *Store
	parent   Generation
	next     Generation
	staging  string
	files    map[string]string
	aborted  bool
	finished bool
}

type Manifest struct {
	Epoch                  int               `json:"epoch"`
	ParentGeneration       Generation        `json:"parentGeneration"`
	Generation             Generation        `json:"generation"`
	CommittedBy            string            `json:"committedBy"`
	CommitKind             string            `json:"commitKind"`
	StoreSchemaVersion     int               `json:"storeSchemaVersion"`
	ArtifactSchemaVersions map[string]int    `json:"artifactSchemaVersions"`
	Files                  map[string]string `json:"files"`
}

type Identity struct {
	StoreKind    string `json:"storeKind"`
	SchemaFamily string `json:"schemaFamily"`
}

var ErrStaleGeneration = errors.New("stale generation")
var ErrMissingArtifact = errors.New("missing artifact")

var requiredArtifacts = []string{
	"desired_world.json",
	"accepted_layout.json",
	"browser_snapshot.json",
	"checkpoint.json",
	"journal.jsonl",
}

func Open(root string) (*Store, error) {
	identity := Identity{StoreKind: "test", SchemaFamily: "projwm-next"}
	if err := os.MkdirAll(filepath.Join(root, "generations"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, ".staging"), 0o755); err != nil {
		return nil, err
	}
	identityPath := filepath.Join(root, ".store_identity.json")
	if _, err := os.Stat(identityPath); errors.Is(err, os.ErrNotExist) {
		if err := writeJSONFile(identityPath, identity); err != nil {
			return nil, err
		}
	} else if err == nil {
		b, err := os.ReadFile(identityPath)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &identity); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(filepath.Join(root, "CURRENT")); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(filepath.Join(root, "CURRENT"), []byte(InitialGeneration+"\n"), 0o644); err != nil {
			return nil, err
		}
	}
	return &Store{root: root, identity: identity}, nil
}

func (s *Store) Current() (Generation, error) {
	b, err := os.ReadFile(filepath.Join(s.root, "CURRENT"))
	if err != nil {
		return "", err
	}
	return Generation(strings.TrimSpace(string(b))), nil
}

func (s *Store) BeginCommit(expected Generation) (*Commit, error) {
	current, err := s.Current()
	if err != nil {
		return nil, err
	}
	if current != expected {
		return nil, ErrStaleGeneration
	}
	next, err := nextGeneration(current)
	if err != nil {
		return nil, err
	}
	staging := filepath.Join(s.root, ".staging", string(next))
	if err := os.Mkdir(staging, 0o755); err != nil {
		return nil, err
	}
	return &Commit{store: s, parent: current, next: next, staging: staging, files: map[string]string{}}, nil
}

func (c *Commit) WriteArtifact(name string, value any) error {
	if c.aborted || c.finished {
		return errors.New("commit is not writable")
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(c.staging, name)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	c.files[name] = fmt.Sprintf("%x", sum[:])
	return nil
}

func (c *Commit) Commit(kind string) (Generation, error) {
	if c.aborted || c.finished {
		return "", errors.New("commit already closed")
	}
	for _, name := range requiredArtifacts {
		if _, ok := c.files[name]; !ok {
			return "", fmt.Errorf("%w: %s", ErrMissingArtifact, name)
		}
	}
	manifest := Manifest{
		Epoch:              1,
		ParentGeneration:   c.parent,
		Generation:         c.next,
		CommittedBy:        "controller",
		CommitKind:         kind,
		StoreSchemaVersion: 1,
		ArtifactSchemaVersions: map[string]int{
			"desiredWorld":    1,
			"acceptedLayout":  1,
			"browserSnapshot": 1,
			"checkpoint":      1,
			"journal":         1,
		},
		Files: c.files,
	}
	if err := writeJSONFile(filepath.Join(c.staging, "manifest.json"), manifest); err != nil {
		return "", err
	}
	final := filepath.Join(c.store.root, "generations", string(c.next))
	if err := os.Rename(c.staging, final); err != nil {
		return "", err
	}
	tmpCurrent := filepath.Join(c.store.root, "CURRENT.tmp")
	if err := os.WriteFile(tmpCurrent, []byte(c.next+"\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpCurrent, filepath.Join(c.store.root, "CURRENT")); err != nil {
		return "", err
	}
	c.finished = true
	return c.next, nil
}

func (s *Store) LoadManifest(g Generation) (Manifest, error) {
	b, err := os.ReadFile(filepath.Join(s.root, "generations", string(g), "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return Manifest{}, err
	}
	for name, want := range manifest.Files {
		b, err := os.ReadFile(filepath.Join(s.root, "generations", string(g), name))
		if err != nil {
			return Manifest{}, err
		}
		got := sha256.Sum256(b)
		if fmt.Sprintf("%x", got[:]) != want {
			return Manifest{}, fmt.Errorf("checksum mismatch for %s", name)
		}
	}
	return manifest, nil
}

func (c *Commit) Abort() error {
	if c.finished {
		return errors.New("commit already finished")
	}
	c.aborted = true
	return os.RemoveAll(c.staging)
}

func nextGeneration(g Generation) (Generation, error) {
	s := string(g)
	i := strings.LastIndex(s, "G")
	if i < 0 {
		return "", fmt.Errorf("invalid generation %q", s)
	}
	n, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return "", fmt.Errorf("invalid generation %q: %w", s, err)
	}
	return Generation(fmt.Sprintf("%sG%06d", s[:i], n+1)), nil
}

func writeJSONFile(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
