package session

import (
	"context"
	"reflect"
	"testing"
)

func TestSSOTL2SessionEnsureRecreatesMissingTmuxSession(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{
		{nil, fakeExitErr()},
		{nil, nil},
	}}
	c := &Client{Exec: fe}
	created, err := c.EnsureSession(context.Background(), "shell-1/dotfiles", "/tmp/dotfiles")
	if err != nil || !created {
		t.Fatalf("EnsureSession created=%v err=%v, want true,nil", created, err)
	}
	want := []string{"new-session", "-d", "-s", "shell-1/dotfiles", "-c", "/tmp/dotfiles"}
	if !reflect.DeepEqual(fe.calls[1], want) {
		t.Fatalf("new-session call=%v want %v", fe.calls[1], want)
	}
}

func TestSSOTL2SessionEnsureExistingTmuxSessionIsNoop(t *testing.T) {
	fe := &fakeExec{results: []fakeResult{{nil, nil}}}
	c := &Client{Exec: fe}
	created, err := c.EnsureSession(context.Background(), "shell-1/dotfiles", "/tmp/dotfiles")
	if err != nil || created {
		t.Fatalf("EnsureSession created=%v err=%v, want false,nil", created, err)
	}
	if len(fe.calls) != 1 {
		t.Fatalf("existing session must only call has-session, got %d calls", len(fe.calls))
	}
}
