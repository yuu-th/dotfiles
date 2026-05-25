package browser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilePrivatePayloadStoreRoundTripAndForget(t *testing.T) {
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	token, err := store.Put(context.Background(), PrivatePayload{URLs: []string{"https://browser-fixture.test/SHOULD_NOT_APPEAR"}})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !validPayloadToken(token) || strings.Contains(token, "SHOULD_NOT_APPEAR") || strings.Contains(token, "browser-fixture") {
		t.Fatalf("token is not opaque: %q", token)
	}
	got, err := store.Get(context.Background(), token)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.URLs) != 1 || got.URLs[0] != "https://browser-fixture.test/SHOULD_NOT_APPEAR" {
		t.Fatalf("payload = %+v", got)
	}
	if err := store.Forget(context.Background(), token); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if _, err := store.Get(context.Background(), token); err == nil {
		t.Fatal("Get succeeded after Forget")
	}
}

func TestFilePrivatePayloadStoreUsesPrivateFiles(t *testing.T) {
	root := t.TempDir()
	store, err := NewFilePrivatePayloadStore(root)
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	token, err := store.Put(context.Background(), PrivatePayload{URLs: []string{"https://browser-fixture.test/private"}})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if rootInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("root permissions = %o, want private", rootInfo.Mode().Perm())
	}
	payloadInfo, err := os.Stat(filepath.Join(root, token+".json"))
	if err != nil {
		t.Fatalf("stat payload: %v", err)
	}
	if payloadInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("payload permissions = %o, want private", payloadInfo.Mode().Perm())
	}
}

func TestFilePrivatePayloadStoreRejectsInvalidTokenShape(t *testing.T) {
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	if _, err := store.Get(context.Background(), "../browser-payload-v1-00000000000000000000000000000000"); err == nil {
		t.Fatal("Get accepted invalid token")
	}
	if err := store.Forget(context.Background(), "../browser-payload-v1-00000000000000000000000000000000"); err != nil {
		t.Fatalf("Forget invalid token should be a safe no-op, got %v", err)
	}
}
