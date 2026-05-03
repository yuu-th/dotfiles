// Package state は projwm の state.json (source of truth) を読み書きする。
//
// 排他制御: flock(2) で lock ファイルを取得、書き込みは tmpfile + atomic rename。
// スキーマ: projwm-design.md §6.3。
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gofrs/flock"
	"github.com/yuu-th/projwm/internal/naming"
)

// Window は project が持つ 1 つの window の意図状態。
//
// title / tmux session 名は **保存しない**。projwm が naming パッケージで
// (kind, id, project) から決定的に算出する（v11.1 §6.3.2）。
//
// kind="browser" の挙動は v12 paradigm C で確定予定（chrome-cli + Chromium profile）。
// 現状は未実装、reconcile は no-op。
type Window struct {
	ID             int         `json:"id"`
	Kind           naming.Kind `json:"kind"`
	AI             naming.AI   `json:"ai,omitempty"`              // kind=="ai" のみ必須
	BrowserProfile string      `json:"browser_profile,omitempty"` // kind=="browser" のみ: Chromium user profile 名
	SavedURLs      []string    `json:"saved_urls,omitempty"`      // kind=="browser" のみ: project archive 時に snapshot
}

// Project は 1 つの作業 cwd（典型的には 1 git worktree）。
type Project struct {
	CWD      string   `json:"cwd"`
	Archived bool     `json:"archived"`
	Windows  []Window `json:"windows"`
}

// Profile は slot 割当の名前付きセット。
type Profile struct {
	Description string            `json:"description,omitempty"`
	// Assignments は slot 名 (e.g. "Q") → project 名のマップ。
	Assignments map[string]string `json:"assignments"`
}

// State は state.json のルート。
type State struct {
	ActiveProfile string             `json:"active_profile"`
	Profiles      map[string]Profile `json:"profiles"`
	Projects      map[string]Project `json:"projects"`
}

// New は空の初期 state を返す（projwm-design.md §6.6.5）。
func New() *State {
	return &State{
		ActiveProfile: "",
		Profiles:      map[string]Profile{},
		Projects:      map[string]Project{},
	}
}

// Paths は state file 周辺の物理 path 群。
type Paths struct {
	Dir       string // ~/.local/state/projwm
	StateFile string // state.json
	BackupFile string // state.json.bak
	LockFile  string // lock
	LogsDir   string // logs/
}

// DefaultPaths は XDG_STATE_HOME (or ~/.local/state) ベースの既定 path を返す。
func DefaultPaths() (Paths, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("user home dir: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(base, "projwm")
	return Paths{
		Dir:        dir,
		StateFile:  filepath.Join(dir, "state.json"),
		BackupFile: filepath.Join(dir, "state.json.bak"),
		LockFile:   filepath.Join(dir, "lock"),
		LogsDir:    filepath.Join(dir, "logs"),
	}, nil
}

// EnsureDirs は必要なディレクトリを作成する。
func (p Paths) EnsureDirs() error {
	for _, d := range []string{p.Dir, p.LogsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

// Store は state file の読み書きを統括する（flock + atomic rename）。
type Store struct {
	paths Paths
}

// NewStore は Store を生成。
func NewStore(p Paths) *Store {
	return &Store{paths: p}
}

// Load は state.json を読み取り、無ければ New() を返す。lock 不要（atomic rename
// により部分書込みは観測されない）。
func (s *Store) Load() (*State, error) {
	data, err := os.ReadFile(s.paths.StateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(), nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		// 破損時は backup から復旧を試みる
		if bdata, berr := os.ReadFile(s.paths.BackupFile); berr == nil {
			var bst State
			if jerr := json.Unmarshal(bdata, &bst); jerr == nil {
				return &bst, fmt.Errorf("state.json broken (recovered from backup): %w", err)
			}
		}
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if st.Profiles == nil {
		st.Profiles = map[string]Profile{}
	}
	if st.Projects == nil {
		st.Projects = map[string]Project{}
	}
	return &st, nil
}

// Mutate は flock 取得 → Load → fn(state) → 検証 → atomic save を 1 トランザクションで行う。
// fn は state を mutate して nil または error を返す。
func (s *Store) Mutate(fn func(*State) error) error {
	if err := s.paths.EnsureDirs(); err != nil {
		return err
	}
	lock := flock.New(s.paths.LockFile)
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Unlock()

	cur, err := s.Load()
	if err != nil {
		return err
	}
	if err := fn(cur); err != nil {
		return err
	}
	if err := Validate(cur); err != nil {
		return fmt.Errorf("post-mutate validation: %w", err)
	}
	return s.atomicSave(cur)
}

// atomicSave は tmpfile + rename で書き込み、前回内容を .bak に退避する。
func (s *Store) atomicSave(st *State) error {
	if err := s.paths.EnsureDirs(); err != nil {
		return err
	}
	// backup（既存があれば）
	if _, err := os.Stat(s.paths.StateFile); err == nil {
		if data, err := os.ReadFile(s.paths.StateFile); err == nil {
			_ = os.WriteFile(s.paths.BackupFile, data, 0o644)
		}
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp, err := os.CreateTemp(s.paths.Dir, "state.json.tmp.*")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmpName, s.paths.StateFile); err != nil {
		return fmt.Errorf("rename tmp: %w", err)
	}
	return nil
}

// Validate は不変条件 (projwm-design.md §6.3.3) を検査する。
func Validate(st *State) error {
	// active_profile は必ず profiles の既存キー（空文字は初期状態として許容）
	if st.ActiveProfile != "" {
		if _, ok := st.Profiles[st.ActiveProfile]; !ok {
			return fmt.Errorf("active_profile %q not in profiles", st.ActiveProfile)
		}
	}
	// profile.assignments の値は projects に存在
	for pname, prof := range st.Profiles {
		seenProjects := map[string]string{} // project -> slot
		for slot, proj := range prof.Assignments {
			if _, ok := st.Projects[proj]; !ok {
				return fmt.Errorf("profile %q slot %q assigns to unknown project %q", pname, slot, proj)
			}
			// archived な project は active profile に居てはいけない
			if pname == st.ActiveProfile && st.Projects[proj].Archived {
				return fmt.Errorf("active profile %q contains archived project %q", pname, proj)
			}
			// 同 profile 内で同一 project が複数 slot に居ない
			if otherSlot, dup := seenProjects[proj]; dup {
				return fmt.Errorf("profile %q maps project %q to multiple slots: %q and %q",
					pname, proj, otherSlot, slot)
			}
			seenProjects[proj] = slot
		}
	}
	// projects 内の windows は (kind, id) 一意 / kind="ai" は ai 必須 / それ以外は ai 不在
	for name, proj := range st.Projects {
		seen := map[string]bool{}
		for _, w := range proj.Windows {
			if !naming.IsValidKind(w.Kind) {
				return fmt.Errorf("project %q: invalid kind %q", name, w.Kind)
			}
			if w.ID < 1 {
				return fmt.Errorf("project %q: window id must be >= 1, got %d", name, w.ID)
			}
			key := fmt.Sprintf("%s-%d", w.Kind, w.ID)
			if seen[key] {
				return fmt.Errorf("project %q: duplicate (kind,id) %s", name, key)
			}
			seen[key] = true
			if w.Kind == naming.KindAI {
				if !naming.IsValidAI(w.AI) {
					return fmt.Errorf("project %q: window %s missing/invalid ai field: %q", name, key, w.AI)
				}
			} else {
				if w.AI != "" {
					return fmt.Errorf("project %q: window %s has ai field but kind != ai", name, key)
				}
			}
			if w.Kind != naming.KindBrowser {
				if w.BrowserProfile != "" || len(w.SavedURLs) > 0 {
					return fmt.Errorf("project %q: window %s has browser_* field but kind != browser", name, key)
				}
			}
		}
	}
	// active な全 project の cwd basename は一意 (NFR-12)
	if st.ActiveProfile != "" {
		seen := map[string]string{}
		for _, proj := range st.Profiles[st.ActiveProfile].Assignments {
			p, ok := st.Projects[proj]
			if !ok || p.Archived {
				continue
			}
			base := filepath.Base(p.CWD)
			if other, dup := seen[base]; dup && other != proj {
				return fmt.Errorf("basename collision in active profile: %q and %q both basename %q", other, proj, base)
			}
			seen[base] = proj
		}
	}
	return nil
}

// NextWindowID は project 内の指定 kind の最大 id + 1 を返す。down で穴が空いても
// 再利用しない（projwm-design.md §5.1.1）。
func NextWindowID(p Project, kind naming.Kind) int {
	maxID := 0
	for _, w := range p.Windows {
		if w.Kind == kind && w.ID > maxID {
			maxID = w.ID
		}
	}
	return maxID + 1
}

// SortedWindows は windows[] を (kind, id) でソートして返す（表示順）。
func SortedWindows(p Project) []Window {
	out := make([]Window, len(p.Windows))
	copy(out, p.Windows)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return kindOrder(out[i].Kind) < kindOrder(out[j].Kind)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func kindOrder(k naming.Kind) int {
	switch k {
	case naming.KindAI:
		return 0
	case naming.KindShell:
		return 1
	case naming.KindEditor:
		return 2
	case naming.KindBrowser:
		return 3
	}
	return 99
}

// IsParked は archived ではなく、いずれの profile の assignments にも入っていない
// 無所属 project かを返す（projwm-design.md §6.6.6）。
func IsParked(st *State, projectName string) bool {
	p, ok := st.Projects[projectName]
	if !ok || p.Archived {
		return false
	}
	for _, prof := range st.Profiles {
		for _, assigned := range prof.Assignments {
			if assigned == projectName {
				return false
			}
		}
	}
	return true
}

// AssignedSlot は active profile での project の slot を返す（無ければ空文字）。
func AssignedSlot(st *State, projectName string) string {
	if st.ActiveProfile == "" {
		return ""
	}
	for slot, proj := range st.Profiles[st.ActiveProfile].Assignments {
		if proj == projectName {
			return slot
		}
	}
	return ""
}

// Touch は state file が新しいフォーマットでなくとも書き出して整える（init 時に有用）。
func (s *Store) Touch() error {
	return s.Mutate(func(*State) error { return nil })
}

// Now は logs などのタイムスタンプ用（テストで差し替えられるよう関数化）。
var Now = func() time.Time { return time.Now() }
