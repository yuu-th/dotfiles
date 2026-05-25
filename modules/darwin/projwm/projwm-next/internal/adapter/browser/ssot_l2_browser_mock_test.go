package browser

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSSOTL2BrowserAdapterDoesNotExposePrivatePayloadOnOpenError(t *testing.T) {
	ctx := context.Background()
	store, err := NewFilePrivatePayloadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePrivatePayloadStore: %v", err)
	}
	const secretURL = "https://SHOULD-NOT-LEAK.example/private"
	token, err := store.Put(ctx, PrivatePayload{URLs: []string{secretURL}})
	if err != nil {
		t.Fatalf("put payload: %v", err)
	}
	adapter := NewVivaldiAdapter(store, &recordingAppOpener{err: errors.New("failed " + secretURL + " " + token)}, "")

	_, err = adapter.OpenInProfile(ctx, VivaldiAutomationProfile, token)
	if err == nil {
		t.Fatal("expected open error")
	}
	if strings.Contains(err.Error(), secretURL) || strings.Contains(err.Error(), token) {
		t.Fatalf("SSOT L2 browser adapter leaked private payload data: %v", err)
	}
}
