package omniwm

import (
	"context"
	"strings"
	"testing"
)

type mockExec struct {
	calls    [][]string
	response map[string][]byte
	err      error
}

func (m *mockExec) Run(_ context.Context, args ...string) ([]byte, error) {
	m.calls = append(m.calls, args)
	if m.err != nil {
		return nil, m.err
	}
	key := strings.Join(args, " ")
	if v, ok := m.response[key]; ok {
		return v, nil
	}
	return []byte(`{"ok":true,"result":{"kind":"void","payload":{}}}`), nil
}

func TestQueryWindows(t *testing.T) {
	m := &mockExec{response: map[string][]byte{
		"query windows --json": []byte(`{
			"ok": true,
			"result": {"kind":"windows","payload":{"windows":[
				{"id":"ow_x","title":"ai-1:dotfiles","app":{"bundleId":"com.mitchellh.ghostty","name":"ghostty"},"pid":12345,"isFocused":true,"isVisible":true,"workspace":{"id":"ws1","number":14,"rawName":"Q","displayName":"Q"}}
			]}}
		}`),
	}}
	c := New(m)
	ws, err := c.QueryWindows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 1 {
		t.Fatalf("got %d windows", len(ws))
	}
	if ws[0].Title != "ai-1:dotfiles" || ws[0].Workspace.RawName != "Q" || ws[0].Workspace.Number != 14 {
		t.Errorf("unexpected: %+v", ws[0])
	}
}

func TestWorkspaceNumberByName(t *testing.T) {
	m := &mockExec{response: map[string][]byte{
		"query workspaces --json": []byte(`{
			"ok":true,
			"result":{"kind":"workspaces","payload":{"workspaces":[
				{"id":"ws1","rawName":"Q","displayName":"Q","number":14,"isCurrent":false},
				{"id":"ws2","rawName":"M","displayName":"M","number":10,"isCurrent":false}
			]}}
		}`),
	}}
	c := New(m)
	n, err := c.WorkspaceNumberByName(context.Background(), "Q")
	if err != nil {
		t.Fatal(err)
	}
	if n != 14 {
		t.Errorf("Q number=%d, want 14", n)
	}
	if _, err := c.WorkspaceNumberByName(context.Background(), "Z"); err == nil {
		t.Error("expected not-found error for unknown name")
	}
}
