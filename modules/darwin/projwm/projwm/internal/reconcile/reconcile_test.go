package reconcile

import (
	"context"
	"strings"
	"testing"

	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/naming"
	"github.com/yuu-th/projwm/internal/omniwm"
	"github.com/yuu-th/projwm/internal/state"
	"github.com/yuu-th/projwm/internal/tmuxwrap"
)

type fakeOmniwmExec struct {
	windows  []omniwm.Window
	wss      []omniwm.Workspace
	commands [][]string
}

func (f *fakeOmniwmExec) Run(_ context.Context, args ...string) ([]byte, error) {
	f.commands = append(f.commands, args)
	if len(args) >= 2 && args[0] == "query" && args[1] == "windows" {
		// reply with windows JSON
		var b strings.Builder
		b.WriteString(`{"ok":true,"result":{"kind":"windows","payload":{"windows":[`)
		for i, w := range f.windows {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"id":"`)
			b.WriteString(w.ID)
			b.WriteString(`","title":"`)
			b.WriteString(w.Title)
			b.WriteString(`","app":{"bundleId":"`)
			b.WriteString(w.BundleID)
			b.WriteString(`","name":"`)
			b.WriteString(w.AppName)
			b.WriteString(`"},"pid":`)
			b.WriteString("0")
			b.WriteString(`,"workspace":{"id":"x","number":0,"rawName":"`)
			b.WriteString(w.Workspace.RawName)
			b.WriteString(`","displayName":"`)
			b.WriteString(w.Workspace.DisplayName)
			b.WriteString(`"}}`)
		}
		b.WriteString(`]}}}`)
		return []byte(b.String()), nil
	}
	if len(args) >= 2 && args[0] == "query" && args[1] == "workspaces" {
		var b strings.Builder
		b.WriteString(`{"ok":true,"result":{"kind":"workspaces","payload":{"workspaces":[`)
		for i, w := range f.wss {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"id":"x","rawName":"`)
			b.WriteString(w.RawName)
			b.WriteString(`","displayName":"`)
			b.WriteString(w.DisplayName)
			b.WriteString(`","number":`)
			b.WriteString(itoa(w.Number))
			b.WriteString(`,"isCurrent":false}`)
		}
		b.WriteString(`]}}}`)
		return []byte(b.String()), nil
	}
	return []byte(`{"ok":true,"result":{"kind":"void","payload":{}}}`), nil
}

func itoa(n int) string {
	// simple int → string
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d [20]byte
	i := len(d)
	for n > 0 {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		d[i] = '-'
	}
	return string(d[i:])
}

type fakeTmuxExec struct {
	sessions map[string]bool
	commands [][]string
}

func (f *fakeTmuxExec) Run(_ context.Context, args ...string) ([]byte, error) {
	f.commands = append(f.commands, args)
	if len(args) >= 1 && args[0] == "has-session" {
		// args = [has-session -t =name]
		name := strings.TrimPrefix(args[2], "=")
		if f.sessions[name] {
			return nil, nil
		}
		return nil, &fakeExitErr{msg: "no session"}
	}
	if len(args) >= 1 && args[0] == "new-session" {
		// extract -s
		for i, a := range args {
			if a == "-s" && i+1 < len(args) {
				if f.sessions == nil {
					f.sessions = map[string]bool{}
				}
				f.sessions[args[i+1]] = true
			}
		}
	}
	if len(args) >= 1 && args[0] == "kill-session" {
		name := strings.TrimPrefix(args[2], "=")
		delete(f.sessions, name)
	}
	return nil, nil
}

type fakeExitErr struct{ msg string }

func (e *fakeExitErr) Error() string { return e.msg }

type fakeGhostty struct{ spawns []string }

func (f *fakeGhostty) Spawn(_ context.Context, title, cwd, session string) error {
	f.spawns = append(f.spawns, title+"|"+cwd+"|"+session)
	return nil
}

type fakeZed struct{ spawns []string }

func (f *fakeZed) Spawn(_ context.Context, cwd string) error {
	f.spawns = append(f.spawns, cwd)
	return nil
}

func TestReconcileSpawnsMissingWindowsForActive(t *testing.T) {
	st := state.New()
	st.Projects["dotfiles"] = state.Project{
		CWD: "/Users/yuta/dev/dotfiles",
		Windows: []state.Window{
			{ID: 1, Kind: naming.KindAI, AI: naming.AIClaude},
			{ID: 1, Kind: naming.KindShell},
			{ID: 1, Kind: naming.KindEditor},
		},
	}
	st.Profiles["work"] = state.Profile{Assignments: map[string]string{"Q": "dotfiles"}}
	st.ActiveProfile = "work"

	fOmni := &fakeOmniwmExec{
		windows: nil, // 何も無い
		wss:     []omniwm.Workspace{{RawName: "Q", DisplayName: "Q", Number: 14}, {RawName: "A", DisplayName: "A", Number: 13}},
	}
	fTmux := &fakeTmuxExec{}
	fGhostty := &fakeGhostty{}
	fZed := &fakeZed{}

	r := &Reconciler{
		Cfg:     config.Default(),
		OmniWM:  omniwm.New(fOmni),
		Tmux:    tmuxwrap.New(fTmux),
		Ghostty: fGhostty,
		Zed:     fZed,
	}
	acts, err := r.Run(context.Background(), st, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) == 0 {
		t.Fatal("expected actions, got 0")
	}

	// AI(1) + AI viewer(1) + shell(1) + editor(1) ぶんの spawn が出るはず
	spawnCount := 0
	for _, a := range acts {
		if a.Op == "spawn-ghostty" || a.Op == "spawn-zed" {
			spawnCount++
		}
	}
	if spawnCount < 4 {
		t.Errorf("expected >=4 spawns (ai+viewer+shell+editor), got %d. acts=%+v", spawnCount, acts)
	}

	// tmux session も新設されているか
	if !fTmux.sessions["ai-1/dotfiles"] {
		t.Error("expected ai-1/dotfiles tmux session created")
	}
	if !fTmux.sessions["shell-1/dotfiles"] {
		t.Error("expected shell-1/dotfiles tmux session created")
	}
	if !fTmux.sessions["ai-1/dotfiles_v"] {
		t.Error("expected viewer grouped session ai-1/dotfiles_v created")
	}

	// ghostty が呼ばれた
	if len(fGhostty.spawns) < 3 { // ai + ai_view + shell
		t.Errorf("expected >=3 ghostty spawns, got %d: %+v", len(fGhostty.spawns), fGhostty.spawns)
	}
	// zed が呼ばれた
	if len(fZed.spawns) != 1 || fZed.spawns[0] != "/Users/yuta/dev/dotfiles" {
		t.Errorf("expected 1 zed spawn for dotfiles, got %+v", fZed.spawns)
	}
}

func TestReconcileDryRunEmitsNoSpawns(t *testing.T) {
	st := state.New()
	st.Projects["dotfiles"] = state.Project{
		CWD: "/Users/yuta/dev/dotfiles",
		Windows: []state.Window{
			{ID: 1, Kind: naming.KindShell},
		},
	}
	st.Profiles["work"] = state.Profile{Assignments: map[string]string{"Q": "dotfiles"}}
	st.ActiveProfile = "work"

	fOmni := &fakeOmniwmExec{wss: []omniwm.Workspace{{RawName: "Q", Number: 14}}}
	fTmux := &fakeTmuxExec{}
	fGhostty := &fakeGhostty{}
	fZed := &fakeZed{}
	r := &Reconciler{
		Cfg:     config.Default(),
		OmniWM:  omniwm.New(fOmni),
		Tmux:    tmuxwrap.New(fTmux),
		Ghostty: fGhostty,
		Zed:     fZed,
	}
	acts, err := r.Run(context.Background(), st, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) == 0 {
		t.Error("expected planned actions even in dry-run")
	}
	if len(fGhostty.spawns) != 0 {
		t.Errorf("dry-run should not spawn ghostty, got %v", fGhostty.spawns)
	}
	if len(fZed.spawns) != 0 {
		t.Errorf("dry-run should not spawn zed, got %v", fZed.spawns)
	}
	if len(fTmux.sessions) != 0 {
		t.Errorf("dry-run should not create tmux sessions, got %v", fTmux.sessions)
	}
}

func TestReconcileEmptyState(t *testing.T) {
	st := state.New()
	r := &Reconciler{
		Cfg:     config.Default(),
		OmniWM:  omniwm.New(&fakeOmniwmExec{}),
		Tmux:    tmuxwrap.New(&fakeTmuxExec{}),
		Ghostty: &fakeGhostty{},
		Zed:     &fakeZed{},
	}
	acts, err := r.Run(context.Background(), st, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 0 {
		t.Errorf("empty state should produce 0 actions, got %d: %+v", len(acts), acts)
	}
}

func TestAllProjectTitles(t *testing.T) {
	p := state.Project{
		CWD: "/x/dotfiles",
		Windows: []state.Window{
			{ID: 1, Kind: naming.KindAI, AI: naming.AIClaude},
			{ID: 2, Kind: naming.KindAI, AI: naming.AICopilot},
			{ID: 1, Kind: naming.KindShell},
			{ID: 1, Kind: naming.KindEditor},
		},
	}
	got := allProjectTitles("dotfiles", p)
	want := []string{
		"ai-1:dotfiles", "ai-view-1:dotfiles",
		"ai-2:dotfiles", "ai-view-2:dotfiles",
		"shell-1:dotfiles",
		"dotfiles", // editor's title is basename
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %q in %v", w, got)
		}
	}
}
