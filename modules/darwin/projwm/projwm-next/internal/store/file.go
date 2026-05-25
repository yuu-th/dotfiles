package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

const (
	fileStoreSchemaVersion = 1
	artifactDesiredWorld   = "desired_world.json"
	artifactAcceptedLayout = "accepted_layout.json"
	artifactCheckpoint     = "checkpoint.json"
	artifactJournal        = "journal.jsonl"
	artifactManifest       = "manifest.json"
)

type StoreKind string

const (
	StoreKindProduction StoreKind = "production"
	StoreKindTest       StoreKind = "test"
	StoreKindRecovery   StoreKind = "recovery"
)

type storeIdentity struct {
	SchemaVersion int       `json:"schemaVersion"`
	StoreKind     StoreKind `json:"storeKind"`
	CreatedAt     string    `json:"createdAt"`
}

type generationManifest struct {
	Epoch          uint64                     `json:"epoch"`
	TransactionID  string                     `json:"transactionId"`
	Generation     string                     `json:"generation"`
	Parent         string                     `json:"parentGeneration,omitempty"`
	CommittedBy    string                     `json:"committedBy"`
	CommitKind     string                     `json:"commitKind"`
	StoreSchema    int                        `json:"storeSchemaVersion"`
	ArtifactSchema map[string]int             `json:"artifactSchemaVersions"`
	Files          map[string]fileDigestEntry `json:"files"`
	CreatedAt      string                     `json:"createdAt"`
}

type fileDigestEntry struct {
	SHA256 string `json:"sha256"`
}

type fileStagedCommit struct {
	generation w.GenerationID
	parent     w.GenerationID
	dir        string
	body       ControllerCommit
}

type GenerationAncestry struct {
	Current CommittedGeneration
	Root    CommittedGeneration
	Chain   []w.GenerationID
}

// FileStore is the generation-directory PersistentStore from impl-design §4.
type FileStore struct {
	root   string
	kind   StoreKind
	mu     sync.Mutex
	staged map[string]fileStagedCommit
	nextID uint64
}

// OpenFileStore opens or initializes a generation-directory store. If the store
// is empty, initial is committed as G000001 through the same artifact protocol.
func OpenFileStore(ctx context.Context, root string, kind StoreKind, initial w.DesiredWorld) (*FileStore, error) {
	return OpenFileStoreWithBootstrapTrace(ctx, root, kind, initial, TransactionTrace{})
}

func OpenFileStoreWithBootstrapTrace(ctx context.Context, root string, kind StoreKind, initial w.DesiredWorld, trace TransactionTrace) (*FileStore, error) {
	if root == "" {
		return nil, fmt.Errorf("store/file: root is required")
	}
	if kind == "" {
		return nil, fmt.Errorf("store/file: store kind is required")
	}
	realRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("store/file: abs root: %w", err)
	}
	fs := &FileStore{root: realRoot, kind: kind, staged: map[string]fileStagedCommit{}, nextID: 1}
	if err := fs.initialize(ctx, initial, trace); err != nil {
		return nil, err
	}
	return fs, nil
}

// OpenExistingFileStore opens an already-bootstrapped generation-directory
// store. Daemon startup uses this path so an empty production store cannot be
// silently initialized from the environment manifest.
func OpenExistingFileStore(ctx context.Context, root string, kind StoreKind) (*FileStore, error) {
	if root == "" {
		return nil, fmt.Errorf("store/file: root is required")
	}
	if kind == "" {
		return nil, fmt.Errorf("store/file: store kind is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	realRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("store/file: abs root: %w", err)
	}
	fs := &FileStore{root: realRoot, kind: kind, staged: map[string]fileStagedCommit{}, nextID: 1}
	if _, err := os.Stat(filepath.Join(fs.root, ".store_identity.json")); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("store/file: existing store identity required: %w", err)
		}
		return nil, fmt.Errorf("store/file: stat identity: %w", err)
	}
	if _, err := readCurrentName(fs.root); err != nil {
		return nil, fmt.Errorf("store/file: existing store required: %w", err)
	}
	if err := fs.ensureIdentity(); err != nil {
		return nil, err
	}
	if err := fs.refreshNextID(); err != nil {
		return nil, err
	}
	return fs, nil
}

func (s *FileStore) initialize(ctx context.Context, initial w.DesiredWorld, trace TransactionTrace) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.root, "generations"), 0o755); err != nil {
		return fmt.Errorf("store/file: create generations: %w", err)
	}
	for _, dir := range []string{".staging", "quarantine", "migrations", "repair", "traces"} {
		if err := os.MkdirAll(filepath.Join(s.root, dir), 0o755); err != nil {
			return fmt.Errorf("store/file: create %s: %w", dir, err)
		}
	}
	if err := s.ensureIdentity(); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(s.root, "CURRENT")); err == nil {
		return s.refreshNextID()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("store/file: stat CURRENT: %w", err)
	}
	commit := ControllerCommit{
		Parent:     "",
		Desired:    initial,
		Checkpoint: ControllerCheckpoint{Epoch: 0},
		Trace:      trace,
	}
	if err := flockExclusive(filepath.Join(s.root, "LOCK"), func() error {
		staged, err := s.stage(ctx, "G000001", "", commit)
		if err != nil {
			return err
		}
		return s.publish(ctx, staged)
	}); err != nil {
		return err
	}
	s.nextID = 2
	return nil
}

func (s *FileStore) ensureIdentity() error {
	path := filepath.Join(s.root, ".store_identity.json")
	if b, err := os.ReadFile(path); err == nil {
		var id storeIdentity
		if err := json.Unmarshal(b, &id); err != nil {
			return fmt.Errorf("store/file: parse identity: %w", err)
		}
		if id.SchemaVersion != fileStoreSchemaVersion {
			return fmt.Errorf("store/file: identity schema %d unsupported", id.SchemaVersion)
		}
		if id.StoreKind != s.kind {
			return fmt.Errorf("store/file: store kind mismatch (got %q, want %q)", id.StoreKind, s.kind)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("store/file: read identity: %w", err)
	}
	id := storeIdentity{SchemaVersion: fileStoreSchemaVersion, StoreKind: s.kind, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := writeJSONAtomic(path, id); err != nil {
		return err
	}
	return fsyncDir(s.root)
}

func (s *FileStore) refreshNextID() error {
	entries, err := os.ReadDir(filepath.Join(s.root, "generations"))
	if err != nil {
		return fmt.Errorf("store/file: read generations: %w", err)
	}
	var max uint64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var n uint64
		if _, err := fmt.Sscanf(e.Name(), "G%06d", &n); err == nil && n > max {
			max = n
		}
	}
	if max == 0 {
		s.nextID = 1
	} else {
		s.nextID = max + 1
	}
	return nil
}

func (s *FileStore) LoadCurrentGeneration(ctx context.Context) (CommittedGeneration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return CommittedGeneration{}, err
	}
	name, err := readCurrentName(s.root)
	if err != nil {
		return CommittedGeneration{}, err
	}
	return s.loadGeneration(name)
}

func (s *FileStore) LoadGenerationAncestry(ctx context.Context) (GenerationAncestry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return GenerationAncestry{}, err
	}
	currentName, err := readCurrentName(s.root)
	if err != nil {
		return GenerationAncestry{}, err
	}
	seen := map[w.GenerationID]bool{}
	var (
		current CommittedGeneration
		root    CommittedGeneration
		chain   []w.GenerationID
		name    = w.GenerationID(currentName)
	)
	for {
		if seen[name] {
			return GenerationAncestry{}, fmt.Errorf("store/file: generation ancestry cycle at %s", name)
		}
		seen[name] = true
		gen, err := s.loadGeneration(string(name))
		if err != nil {
			return GenerationAncestry{}, err
		}
		if len(chain) == 0 {
			current = gen
		}
		chain = append(chain, gen.ID)
		if gen.Trace.CommittedGeneration != "" && gen.Trace.CommittedGeneration != gen.ID {
			return GenerationAncestry{}, fmt.Errorf("store/file: generation %s trace committed generation mismatch %s", gen.ID, gen.Trace.CommittedGeneration)
		}
		parent := w.GenerationID("")
		if gen.Parent != nil {
			parent = *gen.Parent
		}
		if gen.Trace.ParentGeneration != "" && gen.Trace.ParentGeneration != parent {
			return GenerationAncestry{}, fmt.Errorf("store/file: generation %s trace parent mismatch %s != %s", gen.ID, gen.Trace.ParentGeneration, parent)
		}
		if gen.Parent == nil {
			root = gen
			break
		}
		name = *gen.Parent
	}
	return GenerationAncestry{Current: current, Root: root, Chain: chain}, nil
}

func (s *FileStore) BeginCommit(ctx context.Context, c ControllerCommit) (StagedCommit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return StagedCommit{}, err
	}
	var staged StagedCommit
	if err := flockExclusive(filepath.Join(s.root, "LOCK"), func() error {
		current, err := readCurrentName(s.root)
		if err != nil {
			return err
		}
		if c.Parent != w.GenerationID(current) {
			return fmt.Errorf("store/file: parent generation mismatch (got %s, current %s)", c.Parent, current)
		}
		generation := w.GenerationID(fmt.Sprintf("G%06d", s.nextID))
		s.nextID++
		next, err := s.stage(ctx, generation, c.Parent, c)
		if err != nil {
			return err
		}
		staged = next
		id := staged.id
		s.staged[id] = fileStagedCommit{generation: generation, parent: c.Parent, dir: staged.id, body: c}
		return nil
	}); err != nil {
		return StagedCommit{}, err
	}
	return staged, nil
}

func (s *FileStore) Commit(ctx context.Context, staged StagedCommit) (w.GenerationID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.staged[staged.id]
	if !ok {
		return "", fmt.Errorf("store/file: unknown staged commit %s", staged.id)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := flockExclusive(filepath.Join(s.root, "LOCK"), func() error {
		current, err := readCurrentName(s.root)
		if err != nil {
			return err
		}
		if meta.parent != w.GenerationID(current) {
			return fmt.Errorf("store/file: parent changed before commit (staged parent %s, current %s)", meta.parent, current)
		}
		return s.publish(ctx, StagedCommit{id: meta.dir, body: meta.body})
	}); err != nil {
		return "", err
	}
	delete(s.staged, staged.id)
	return meta.generation, nil
}

func (s *FileStore) Abort(ctx context.Context, staged StagedCommit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.staged[staged.id]
	if !ok {
		return nil
	}
	delete(s.staged, staged.id)
	return os.RemoveAll(filepath.Join(s.root, ".staging", meta.dir))
}

func (s *FileStore) RecordTransactionTrace(ctx context.Context, trace TransactionTrace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if trace.TransactionID == "" {
		return fmt.Errorf("store/file: trace transactionId is required")
	}
	name := string(trace.TransactionID)
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		return fmt.Errorf("store/file: invalid trace transactionId %q", name)
	}
	dir := filepath.Join(s.root, "traces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store/file: create traces: %w", err)
	}
	if err := writeJSONAtomic(filepath.Join(dir, name+".json"), trace); err != nil {
		return err
	}
	return fsyncDir(dir)
}

func (s *FileStore) stage(ctx context.Context, generation w.GenerationID, parent w.GenerationID, c ControllerCommit) (StagedCommit, error) {
	if err := ctx.Err(); err != nil {
		return StagedCommit{}, err
	}
	if c.TransactionID == "" {
		c.TransactionID = w.TransactionID("txn-" + string(generation))
	}
	c.Trace.TransactionID = c.TransactionID
	c.Trace.ParentGeneration = parent
	c.Trace.CommittedGeneration = generation
	c.Trace.CommitKind = commitKind(parent)
	c.Trace.CommittedBy = "controller"
	stageName := fmt.Sprintf("txn-%s-%d", generation, time.Now().UnixNano())
	stageDir := filepath.Join(s.root, ".staging", stageName)
	if err := os.Mkdir(stageDir, 0o755); err != nil {
		return StagedCommit{}, fmt.Errorf("store/file: create staging: %w", err)
	}
	artifacts := map[string]any{
		artifactDesiredWorld:   c.Desired,
		artifactAcceptedLayout: c.AcceptedLayouts,
		artifactCheckpoint:     c.Checkpoint,
	}
	for name, body := range artifacts {
		if err := writeJSONAtomic(filepath.Join(stageDir, name), body); err != nil {
			return StagedCommit{}, err
		}
	}
	journal := struct {
		TransactionID    string           `json:"transactionId"`
		Generation       string           `json:"generation"`
		ParentGeneration string           `json:"parentGeneration,omitempty"`
		CommitKind       string           `json:"commitKind"`
		CommittedBy      string           `json:"committedBy"`
		Trace            TransactionTrace `json:"trace"`
	}{
		TransactionID:    string(c.TransactionID),
		Generation:       string(generation),
		ParentGeneration: string(parent),
		CommitKind:       commitKind(parent),
		CommittedBy:      "controller",
		Trace:            c.Trace,
	}
	journalData, err := json.Marshal(journal)
	if err != nil {
		return StagedCommit{}, fmt.Errorf("store/file: marshal journal: %w", err)
	}
	journalData = append(journalData, '\n')
	if err := writeFileAtomic(filepath.Join(stageDir, artifactJournal), journalData); err != nil {
		return StagedCommit{}, err
	}
	if err := fsyncDir(stageDir); err != nil {
		return StagedCommit{}, err
	}
	files := map[string]fileDigestEntry{}
	for _, name := range []string{artifactDesiredWorld, artifactAcceptedLayout, artifactCheckpoint, artifactJournal} {
		sum, err := sha256File(filepath.Join(stageDir, name))
		if err != nil {
			return StagedCommit{}, err
		}
		files[name] = fileDigestEntry{SHA256: sum}
	}
	manifest := generationManifest{
		Epoch:         uint64(c.Checkpoint.Epoch),
		TransactionID: string(c.TransactionID),
		Generation:    string(generation),
		Parent:        string(parent),
		CommittedBy:   "controller",
		CommitKind:    commitKind(parent),
		StoreSchema:   fileStoreSchemaVersion,
		ArtifactSchema: map[string]int{
			"desiredWorld":   1,
			"acceptedLayout": 1,
			"checkpoint":     1,
			"journal":        1,
		},
		Files:     files,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeJSONAtomic(filepath.Join(stageDir, artifactManifest), manifest); err != nil {
		return StagedCommit{}, err
	}
	if err := fsyncDir(stageDir); err != nil {
		return StagedCommit{}, err
	}
	return StagedCommit{id: stageName, body: c}, nil
}

func commitKind(parent w.GenerationID) string {
	if parent == "" {
		return "migration-bootstrap"
	}
	return "controller-commit"
}

func (s *FileStore) publish(ctx context.Context, staged StagedCommit) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	stageDir := filepath.Join(s.root, ".staging", staged.id)
	manifestPath := filepath.Join(stageDir, artifactManifest)
	var manifest generationManifest
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("store/file: read staged manifest: %w", err)
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		return fmt.Errorf("store/file: parse staged manifest: %w", err)
	}
	if err := validateManifestFiles(stageDir, manifest); err != nil {
		return err
	}
	finalDir := filepath.Join(s.root, "generations", manifest.Generation)
	if err := os.Rename(stageDir, finalDir); err != nil {
		return fmt.Errorf("store/file: publish generation: %w", err)
	}
	if err := fsyncDir(filepath.Join(s.root, "generations")); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(s.root, "CURRENT"), []byte(manifest.Generation+"\n")); err != nil {
		return err
	}
	return fsyncDir(s.root)
}

func (s *FileStore) loadGeneration(name string) (CommittedGeneration, error) {
	if strings.Contains(name, "/") || name == "" {
		return CommittedGeneration{}, fmt.Errorf("store/file: invalid CURRENT value %q", name)
	}
	dir := filepath.Join(s.root, "generations", name)
	var manifest generationManifest
	if err := readJSON(filepath.Join(dir, artifactManifest), &manifest); err != nil {
		return CommittedGeneration{}, err
	}
	if manifest.Generation != name {
		return CommittedGeneration{}, fmt.Errorf("store/file: manifest generation %q != CURRENT %q", manifest.Generation, name)
	}
	if err := validateManifestFiles(dir, manifest); err != nil {
		return CommittedGeneration{}, err
	}
	var desired w.DesiredWorld
	if err := readJSON(filepath.Join(dir, artifactDesiredWorld), &desired); err != nil {
		return CommittedGeneration{}, err
	}
	// Bug 2026-05-19 round 2: migrate AI/Shell/Viewer windows from the
	// legacy TitlePrefixOwned/Prefix encoding to the ControllerOwned/
	// Expected encoding the current code path assumes. Projects created
	// before this migration would otherwise be unresolvable (ClassWeak
	// only) and the planner would refuse mutation forever. Idempotent:
	// re-writes on every load until the next commit captures the
	// migrated form into persistent storage.
	migrateDesiredTitleContracts(&desired)
	var accepted map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout
	if err := readJSON(filepath.Join(dir, artifactAcceptedLayout), &accepted); err != nil {
		return CommittedGeneration{}, err
	}
	var checkpoint ControllerCheckpoint
	if err := readJSON(filepath.Join(dir, artifactCheckpoint), &checkpoint); err != nil {
		return CommittedGeneration{}, err
	}
	var journal struct {
		TransactionID    string           `json:"transactionId"`
		Generation       string           `json:"generation"`
		ParentGeneration string           `json:"parentGeneration,omitempty"`
		CommitKind       string           `json:"commitKind"`
		CommittedBy      string           `json:"committedBy"`
		Trace            TransactionTrace `json:"trace"`
	}
	if err := readJSON(filepath.Join(dir, artifactJournal), &journal); err != nil {
		return CommittedGeneration{}, err
	}
	if journal.Generation != manifest.Generation || journal.ParentGeneration != manifest.Parent || journal.CommitKind != manifest.CommitKind || journal.CommittedBy != manifest.CommittedBy || journal.TransactionID != manifest.TransactionID {
		return CommittedGeneration{}, fmt.Errorf("store/file: journal/manifest mismatch for generation %s", name)
	}
	if journal.Trace.CommitKind != "" && journal.Trace.CommitKind != journal.CommitKind {
		return CommittedGeneration{}, fmt.Errorf("store/file: trace commit kind mismatch for generation %s", name)
	}
	if journal.Trace.CommittedBy != "" && journal.Trace.CommittedBy != journal.CommittedBy {
		return CommittedGeneration{}, fmt.Errorf("store/file: trace committedBy mismatch for generation %s", name)
	}
	var parent *w.GenerationID
	if manifest.Parent != "" {
		p := w.GenerationID(manifest.Parent)
		parent = &p
	}
	return CommittedGeneration{
		ID:              w.GenerationID(name),
		Parent:          parent,
		Desired:         desired,
		AcceptedLayouts: accepted,
		Checkpoint:      checkpoint,
		Trace:           journal.Trace,
		StoreVersion:    w.StoreVersion(fmt.Sprint(manifest.StoreSchema)),
	}, nil
}

// migrateDesiredTitleContracts upgrades AI/Shell/Viewer DesiredWindow
// TitleContracts from the legacy `TitlePrefixOwned + Prefix` form to the
// canonical `TitleControllerOwned + Expected` form. The legacy form
// always classifies as ClassWeak in identity.Resolve, which makes the
// planner's "refusing mutation without unique-strong evidence" guard
// trip on every reconcile after a project's first spawn, blocking
// AssignProject / UnarchiveProject. Idempotent: ControllerOwned rows
// pass through unchanged; Prefix is migrated to Expected (the live
// title in this environment is exactly the prefix value, because tmux
// in our setup does not modify window titles).
func migrateDesiredTitleContracts(d *w.DesiredWorld) {
	if d == nil {
		return
	}
	for pid, pr := range d.Projects {
		changed := false
		for i := range pr.Windows {
			win := &pr.Windows[i]
			switch win.Kind {
			case w.WindowAI, w.WindowShell, w.WindowViewer:
				if win.TitleContract.Authority != w.TitlePrefixOwned {
					continue
				}
				if win.TitleContract.Prefix == "" {
					continue
				}
				win.TitleContract.Authority = w.TitleControllerOwned
				win.TitleContract.Expected = win.TitleContract.Prefix
				win.TitleContract.Prefix = ""
				if win.TitleContract.Drift == w.TitleDriftRematch || win.TitleContract.Drift == "" {
					win.TitleContract.Drift = w.TitleDriftRepair
				}
				changed = true
			}
		}
		if changed {
			d.Projects[pid] = pr
		}
	}
}

func readCurrentName(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "CURRENT"))
	if err != nil {
		return "", fmt.Errorf("store/file: read CURRENT: %w", err)
	}
	name := strings.TrimSpace(string(b))
	if name == "" || strings.Contains(name, "/") {
		return "", fmt.Errorf("store/file: invalid CURRENT value %q", name)
	}
	return name, nil
}

func validateManifestFiles(dir string, manifest generationManifest) error {
	if manifest.StoreSchema != fileStoreSchemaVersion {
		return fmt.Errorf("store/file: unsupported store schema %d", manifest.StoreSchema)
	}
	names := make([]string, 0, len(manifest.Files))
	for name := range manifest.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if strings.Contains(name, "/") {
			return fmt.Errorf("store/file: manifest file path escapes generation: %q", name)
		}
		got, err := sha256File(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if want := manifest.Files[name].SHA256; got != want {
			return fmt.Errorf("store/file: checksum mismatch for %s (got %s want %s)", name, got, want)
		}
	}
	return nil
}

func writeJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("store/file: marshal %s: %w", filepath.Base(path), err)
	}
	b = append(b, '\n')
	return writeFileAtomic(path, b)
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("store/file: open temp %s: %w", tmp, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("store/file: write temp %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("store/file: fsync temp %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("store/file: close temp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("store/file: rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("store/file: read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("store/file: parse %s: %w", path, err)
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("store/file: open for checksum %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("store/file: checksum %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fsyncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("store/file: open dir %s: %w", path, err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("store/file: fsync dir %s: %w", path, err)
	}
	return nil
}

func flockExclusive(path string, fn func() error) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("store/file: open lock: %w", err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("store/file: flock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

// Root returns the absolute on-disk root the FileStore was opened against.
// Used by the daemon's Query handlers to locate the traces directory.
func (s *FileStore) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

var _ PersistentStore = (*FileStore)(nil)
var _ TransactionTraceRecorder = (*FileStore)(nil)
