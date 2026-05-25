package session

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"
)

type fakeExec struct {
	calls   [][]string
	results []fakeResult
}

type fakeResult struct {
	out []byte
	err error
}

func (f *fakeExec) Run(ctx context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	if len(f.results) == 0 {
		return nil, nil
	}
	r := f.results[0]
	f.results = f.results[1:]
	return r.out, r.err
}

// fakeExitErr mimics *exec.ExitError for HasSession's negative path.
func fakeExitErr() error {
	// Run a command guaranteed to exit non-zero so we get a real
	// *exec.ExitError without needing platform-specific construction.
	cmd := exec.Command("sh", "-c", "exit 1")
	return cmd.Run()
}

func TestHasSessionExists(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{{nil, nil}}}
	c := &Client{Exec: fe}
	got, err := c.HasSession(context.Background(), "ai-1/dotfiles")
	if err != nil || !got {
		t.Fatalf("HasSession=%v err=%v want true,nil", got, err)
	}
	want := []string{"has-session", "-t", "=ai-1/dotfiles"}
	if !reflect.DeepEqual(fe.calls[0], want) {
		t.Fatalf("call=%v want %v", fe.calls[0], want)
	}
}

func TestHasSessionMissing(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{{nil, fakeExitErr()}}}
	c := &Client{Exec: fe}
	got, err := c.HasSession(context.Background(), "missing")
	if err != nil {
		t.Fatalf("err=%v want nil", err)
	}
	if got {
		t.Fatalf("HasSession=true want false")
	}
}

func TestHasSessionTransportError(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{{nil, errors.New("boom")}}}
	c := &Client{Exec: fe}
	if _, err := c.HasSession(context.Background(), "x"); err == nil {
		t.Fatal("expected transport error")
	}
}

func TestEnsureSessionCreates(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{
		{nil, fakeExitErr()}, // has-session: missing
		{nil, nil},           // new-session: ok
	}}
	c := &Client{Exec: fe}
	created, err := c.EnsureSession(context.Background(), "shell-1/dotfiles", "/tmp")
	if err != nil || !created {
		t.Fatalf("created=%v err=%v want true,nil", created, err)
	}
	want := []string{"new-session", "-d", "-s", "shell-1/dotfiles", "-c", "/tmp"}
	if !reflect.DeepEqual(fe.calls[1], want) {
		t.Fatalf("new-session call=%v want %v", fe.calls[1], want)
	}
}

func TestEnsureSessionAlreadyExists(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{{nil, nil}}}
	c := &Client{Exec: fe}
	created, err := c.EnsureSession(context.Background(), "x", "")
	if err != nil || created {
		t.Fatalf("created=%v err=%v want false,nil", created, err)
	}
	if len(fe.calls) != 1 {
		t.Fatalf("expected 1 call (has-session), got %d", len(fe.calls))
	}
}

func TestEnsureGroupedSession(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{
		{nil, fakeExitErr()}, // has-session: missing
		{nil, nil},           // new-session -t: ok
	}}
	c := &Client{Exec: fe}
	if err := c.EnsureGroupedSession(context.Background(), "ai-1/dotfiles", "ai-1/dotfiles_v"); err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []string{"new-session", "-d", "-t", "ai-1/dotfiles", "-s", "ai-1/dotfiles_v"}
	if !reflect.DeepEqual(fe.calls[1], want) {
		t.Fatalf("call=%v want %v", fe.calls[1], want)
	}
}

func TestSendKeysAppendsEnter(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{{nil, nil}}}
	c := &Client{Exec: fe}
	if err := c.SendKeys(context.Background(), "ai-1/dotfiles", "claude"); err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []string{"send-keys", "-t", "ai-1/dotfiles", "claude", "C-m"}
	if !reflect.DeepEqual(fe.calls[0], want) {
		t.Fatalf("call=%v want %v", fe.calls[0], want)
	}
}

func TestListSessionsParsesLines(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{{[]byte("ai-1/dotfiles\nshell-1/dotfiles\n"), nil}}}
	c := &Client{Exec: fe}
	got, err := c.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []string{"ai-1/dotfiles", "shell-1/dotfiles"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestListSessionsNoServer(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{{[]byte("no server running on /tmp/tmux-x\n"), errors.New("exit 1")}}}
	c := &Client{Exec: fe}
	got, err := c.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got=%v want empty", got)
	}
}
