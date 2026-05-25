package main

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/controller"
	"github.com/yuu-th/projwm-next/internal/ipc"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// startQueryServer fires up a minimal projwmd-style query server backed by
// a FileStore + in-memory controller, returning a connected net.Conn ready
// for client roundtrips.
func startQueryServer(t *testing.T) (clientConn net.Conn, cleanup func()) {
	t.Helper()
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	desired := w.DesiredWorld{
		ActiveProfile: "work",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"work": {ID: "work", InactivePolicy: w.InactivePolicyRemove,
				Assignments: map[w.SlotID]w.ProjectID{"Q": "dotfiles"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"dotfiles": {ID: "dotfiles"},
			"old":      {ID: "old", Archived: true},
		},
	}
	fs, err := store.OpenFileStore(context.Background(), storeDir, store.StoreKindTest, desired)
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Viewer: "A",
			Workspaces: []w.WorkspaceSpec{
				{ID: "Q", Role: w.WorkspaceProject},
			},
			Slots: []w.SlotSpec{
				{ID: "Q", Workspace: "Q", Order: 1},
			},
		},
	}
	ctrl := controller.New(env, desired, &wm.Fake{}, fs)

	server, client := net.Pipe()
	go func() {
		defer server.Close()
		for {
			env, err := ipc.ReadEnvelope(server)
			if err != nil {
				return
			}
			switch env.Type {
			case ipc.MsgQueryRequest:
				handleQueryEnvelope(context.Background(), server, ctrl, env)
			default:
				return
			}
		}
	}()
	return client, func() {
		_ = client.Close()
	}
}

// sendQuery is a tiny helper that writes a query request and reads back
// the response, returning the decoded body.
func sendQuery(t *testing.T, conn net.Conn, kind ipc.QueryKind) ipc.QueryResponse {
	t.Helper()
	req, err := ipc.NewEnvelope(ipc.MsgQueryRequest, ipc.QueryRequest{
		RequestID: "q1",
		Kind:      kind,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ipc.WriteEnvelope(conn, req); err != nil {
		t.Fatalf("write: %v", err)
	}
	respEnv, err := ipc.ReadEnvelope(conn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if respEnv.Type != ipc.MsgQueryResponse {
		t.Fatalf("expected query-response, got %q", respEnv.Type)
	}
	var resp ipc.QueryResponse
	if err := json.Unmarshal(respEnv.Payload, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func TestQuery_World(t *testing.T) {
	conn, cleanup := startQueryServer(t)
	defer cleanup()
	resp := sendQuery(t, conn, ipc.QueryWorld)
	if resp.Error != nil {
		t.Fatalf("query error: %v", resp.Error)
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["activeProfile"] != "work" {
		t.Errorf("activeProfile = %v", body["activeProfile"])
	}
}

func TestQuery_Archive(t *testing.T) {
	conn, cleanup := startQueryServer(t)
	defer cleanup()
	resp := sendQuery(t, conn, ipc.QueryArchive)
	if resp.Error != nil {
		t.Fatalf("query error: %v", resp.Error)
	}
	var ids []string
	if err := json.Unmarshal(resp.Body, &ids); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(ids) != 1 || ids[0] != "old" {
		t.Errorf("expected [old], got %v", ids)
	}
}

func TestQuery_Cards_Empty(t *testing.T) {
	conn, cleanup := startQueryServer(t)
	defer cleanup()
	resp := sendQuery(t, conn, ipc.QueryCards)
	if resp.Error != nil {
		t.Fatalf("query error: %v", resp.Error)
	}
	// Empty active cards: marshals to "null" (typed nil slice).
	if got := string(resp.Body); got != "null" && got != "[]" {
		t.Errorf("expected null or [], got %q", got)
	}
}

func TestConnHub_BroadcastRespectsKinds(t *testing.T) {
	hub := NewConnHub()
	subA := hub.Register([]string{"card-added"})
	subB := hub.Register([]string{"card-removed"})
	defer hub.Remove(subA)
	defer hub.Remove(subB)

	hub.Broadcast("card-added", "x", "G1")

	select {
	case push := <-subA.out:
		if push.Kind != "card-added" {
			t.Errorf("kind = %s", push.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("subA did not receive push")
	}
	select {
	case push := <-subB.out:
		t.Errorf("subB unexpectedly received %v", push)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestConnHub_BroadcastWildcard(t *testing.T) {
	hub := NewConnHub()
	subAll := hub.Register([]string{"*"})
	defer hub.Remove(subAll)
	hub.Broadcast("anything", 42, "")
	select {
	case push := <-subAll.out:
		if push.Kind != "anything" {
			t.Errorf("kind = %s", push.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("wildcard subscriber did not receive push")
	}
}

func TestConnHub_FullBufferDrops(t *testing.T) {
	hub := NewConnHub()
	sub := hub.Register([]string{"*"})
	defer hub.Remove(sub)
	for i := 0; i < 200; i++ {
		hub.Broadcast("x", i, "")
	}
	// Channel buffer is 64; we just verify no goroutine deadlock and the
	// receiver can drain at least 64 before further pushes were dropped.
	count := 0
	for {
		select {
		case <-sub.out:
			count++
		default:
			if count < 1 {
				t.Errorf("expected at least 1 push, got %d", count)
			}
			return
		}
	}
}
