package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/ipc"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// recordedIntents captures the IntentRequests a fake daemon received.
type recordedIntents struct {
	mu   sync.Mutex
	last *ipc.IntentRequest
}

func (r *recordedIntents) set(ir ipc.IntentRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := ir
	r.last = &cp
}

func (r *recordedIntents) take() (ipc.IntentRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last == nil {
		return ipc.IntentRequest{}, false
	}
	ir := *r.last
	r.last = nil
	return ir, true
}

// startFakeDaemon listens on socketPath and, per connection, completes the
// hello/welcome handshake, records the submitted IntentRequest, and replies
// with a canned accepted IntentResponse. It lets us drive the real `run()`
// CLI entrypoint and assert the (command args → intent) mapping without a
// live projwmd — SSOT §5.7 / §10.9 GAP-14.
func startFakeDaemon(t *testing.T, socketPath, digest string) *recordedIntents {
	t.Helper()
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen %s: %v", socketPath, err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	rec := &recordedIntents{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveFakeConn(conn, rec, digest)
		}
	}()
	return rec
}

func serveFakeConn(conn net.Conn, rec *recordedIntents, digest string) {
	defer conn.Close()
	hello, err := ipc.ReadEnvelope(conn)
	if err != nil || hello.Type != ipc.MsgHello {
		return
	}
	welcome, err := ipc.NewEnvelope(ipc.MsgWelcome, ipc.Welcome{
		ProtocolVersion:    ipc.ProtocolVersion,
		StoreSchemaVersion: ipc.StoreSchemaVersion,
		ManifestDigest:     digest,
		DaemonVersion:      "fake-test",
		CurrentGeneration:  w.GenerationID("G000001"),
	})
	if err != nil {
		return
	}
	if err := ipc.WriteEnvelope(conn, welcome); err != nil {
		return
	}
	req, err := ipc.ReadEnvelope(conn)
	if err != nil || req.Type != ipc.MsgIntentRequest {
		return
	}
	var ir ipc.IntentRequest
	if err := json.Unmarshal(req.Payload, &ir); err != nil {
		return
	}
	rec.set(ir)
	txn := w.TransactionID("txn-fake")
	gen := w.GenerationID("G000002")
	resp, err := ipc.NewEnvelope(ipc.MsgIntentResponse, ipc.IntentResponse{
		RequestID:           ir.RequestID,
		AcceptedTransaction: &txn,
		CommittedGeneration: &gen,
	})
	if err != nil {
		return
	}
	_ = ipc.WriteEnvelope(conn, resp)
}

// TestSSOTCLICommandsSubmitCorrectIntent is the owner test for SSOT §5.7 /
// §10.9 GAP-14: each writable CLI command must construct and submit the
// correct intent Kind from its arguments (not merely exist in the usage
// surface). We drive the real run() entrypoint against a fake daemon and
// assert the recorded intent Kind. Commands that resolve their target from a
// live store (e.g. add-* without --project) are exercised here with explicit
// args so the path stays daemon-only.
func TestSSOTCLICommandsSubmitCorrectIntent(t *testing.T) {
	dir := t.TempDir()
	// macOS caps unix socket paths at ~104 chars; t.TempDir() is too long,
	// so place the socket under /tmp with a short unique name.
	sockDir, err := os.MkdirTemp("/tmp", "pwcli")
	if err != nil {
		t.Fatalf("mkdir sock dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	sock := filepath.Join(sockDir, "d.sock")

	// Minimal valid managed-environment manifest whose daemons.socketPath
	// authorizes our fake socket (clientauth requires the equality).
	withSock := `{"schemaVersion":1,"authority":"nix","windowManager":{"backend":"omniwm"},` +
		`"workspaces":[{"id":"8","rawName":"8","role":"general"}],"slots":[],` +
		`"apps":[{"bundleId":"com.mitchellh.ghostty","capability":"terminal",` +
		`"lifecycleRemoval":{"method":"ax-close-guarded","allowed":true,"allowedKinds":["ai","shell","viewer"]}}],` +
		`"daemons":{"socketPath":` + jsonString(sock) + `}}`
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(withSock), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	sum := sha256.Sum256([]byte(withSock))
	digest := hex.EncodeToString(sum[:])

	rec := startFakeDaemon(t, sock, digest)
	base := []string{
		"--socket-path", sock,
		"--managed-environment", manifestPath,
		"--manifest-digest", digest,
	}

	cases := []struct {
		name string
		args []string
		want intent.Kind
	}{
		{"archive", []string{"archive", "myproj"}, intent.KindArchiveProject},
		{"unarchive", []string{"unarchive", "myproj"}, intent.KindUnarchiveProject},
		{"profile-create", []string{"profile", "create", "p1"}, intent.KindCreateProfile},
		{"profile-delete", []string{"profile", "delete", "p1"}, intent.KindDeleteProfile},
		{"profile-switch", []string{"profile", "switch", "p1"}, intent.KindSwitchProfile},
		{"profile-rename", []string{"profile", "rename", "old", "new"}, intent.KindRenameProfile},
		{"profile-assign", []string{"profile", "assign", "Q", "myproj"}, intent.KindAssignProject},
		{"profile-unassign", []string{"profile", "unassign", "Q"}, intent.KindUnassignSlot},
		{"reconcile", []string{"reconcile"}, intent.KindReconcile},
		{"add-shell", []string{"add-shell", "--project", "myproj"}, intent.KindAddWindow},
		{"add-ai", []string{"add-ai", "--ai", "claude", "--project", "myproj"}, intent.KindAddWindow},
		{"add-editor", []string{"add-editor", "--project", "myproj"}, intent.KindAddWindow},
		{"add-browser", []string{"add-browser", "--project", "myproj"}, intent.KindAddWindow},
		{"remove", []string{"remove", "--window", "shell-1", "--project", "myproj"}, intent.KindRemoveWindow},
		{"browser-add-tab", []string{"browser", "add-tab", "--project", "myproj", "--window", "browser-1", "--url", "https://x.test"}, intent.KindBrowserAddTab},
		{"browser-remove-tab", []string{"browser", "remove-tab", "--project", "myproj", "--window", "browser-1", "--tab", "2"}, intent.KindBrowserRemoveTab},
		{"browser-change-tab-url", []string{"browser", "change-tab-url", "--project", "myproj", "--window", "browser-1", "--tab", "1", "--url", "https://y.test"}, intent.KindBrowserChangeTabURL},
		{"browser-reorder-tabs", []string{"browser", "reorder-tabs", "--project", "myproj", "--window", "browser-1", "--from", "2", "--to", "1"}, intent.KindBrowserReorderTabs},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := run(append(append([]string{}, base...), c.args...), io.Discard, io.Discard); err != nil {
				t.Fatalf("run %v: %v", c.args, err)
			}
			ir, ok := rec.take()
			if !ok {
				t.Fatalf("command %v submitted no intent", c.args)
			}
			if ir.Kind != c.want {
				t.Errorf("command %v submitted intent Kind=%q, want %q", c.args, ir.Kind, c.want)
			}
		})
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
