package controller

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// fakePrivatePayloadStore is an in-memory PrivatePayloadStore that records
// every Put / Get / Forget call so the controller wiring can be verified.
type fakePrivatePayloadStore struct {
	mu      sync.Mutex
	puts    []browser.PrivatePayload
	forgets []string
	gets    []string
	tokens  map[string]browser.PrivatePayload
	seq     int
	putErr  error
}

func newFakePrivatePayloadStore() *fakePrivatePayloadStore {
	return &fakePrivatePayloadStore{tokens: map[string]browser.PrivatePayload{}}
}

func (s *fakePrivatePayloadStore) Put(_ context.Context, payload browser.PrivatePayload) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putErr != nil {
		return "", s.putErr
	}
	s.seq++
	// 32 hex chars is the controller-side IsPayloadToken validator's requirement.
	token := "browser-payload-v1-" + strings.Repeat("a", 31) + hexDigit(s.seq)
	s.puts = append(s.puts, payload)
	s.tokens[token] = payload
	return token, nil
}

func (s *fakePrivatePayloadStore) Get(_ context.Context, token string) (browser.PrivatePayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets = append(s.gets, token)
	return s.tokens[token], nil
}

func (s *fakePrivatePayloadStore) Forget(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forgets = append(s.forgets, token)
	delete(s.tokens, token)
	return nil
}

func hexDigit(n int) string {
	const hex = "0123456789abcdef"
	return string(hex[n%16])
}

func browserProjectFixture() (w.ManagedEnvironment, w.DesiredWorld) {
	env := w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		Workspaces: w.WorkspaceEnvironment{
			Slots:      []w.SlotSpec{{ID: "Q", Workspace: "Q", Order: 1}},
			Workspaces: []w.WorkspaceSpec{{ID: "Q", RawName: "Q", DisplayName: "Q", Role: w.WorkspaceProject}},
		},
	}
	winID := w.DesiredWindowID{Project: "p1", Kind: w.WindowBrowser, Index: 1}
	desired := w.DesiredWorld{
		ActiveProfile: "default",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"default": {ID: "default", Assignments: map[w.SlotID]w.ProjectID{"Q": "p1"}},
		},
		Projects: map[w.ProjectID]w.DesiredProject{
			"p1": {
				ID: "p1",
				Windows: []w.DesiredWindow{{
					ID:      winID,
					Kind:    w.WindowBrowser,
					Browser: &w.DesiredBrowserSession{},
				}},
			},
		},
	}
	return env, desired
}

func newControllerWithPayloadStore(t *testing.T) (*Controller, *fakePrivatePayloadStore, w.DesiredWindowID) {
	t.Helper()
	env, desired := browserProjectFixture()
	st := store.NewMemoryStore(desired)
	ctrl := New(env, desired, wm.NewFake(env), st)
	store := newFakePrivatePayloadStore()
	ctrl.PayloadStore = store
	winID := w.DesiredWindowID{Project: "p1", Kind: w.WindowBrowser, Index: 1}
	return ctrl, store, winID
}

// SSOT §4.1 OP14 + §4.4 BR-PRIV-NOSTORE: BrowserAddTab routes URL through
// PrivatePayloadStore. DesiredWorld must hold the token, NOT the literal URL.
func TestControllerBrowserAddTab_RoutesURLThroughPayloadStore(t *testing.T) {
	ctrl, payloadStore, winID := newControllerWithPayloadStore(t)
	const secretURL = "https://example.com/secret"

	if _, err := ctrl.ApplyIntent(context.Background(), intent.BrowserAddTab{
		Project: "p1", WindowID: winID, URL: secretURL,
	}); err != nil {
		t.Fatalf("ApplyIntent: %v", err)
	}

	if len(payloadStore.puts) != 1 {
		t.Fatalf("expected 1 Put call, got %d", len(payloadStore.puts))
	}
	if got := payloadStore.puts[0].URLs; len(got) != 1 || got[0] != secretURL {
		t.Errorf("Put payload URLs = %v, want [%q]", got, secretURL)
	}
	refs := ctrl.state.Desired.Projects["p1"].Windows[0].Browser.URLPayloadRefs
	if len(refs) != 1 {
		t.Fatalf("URLPayloadRefs len = %d, want 1", len(refs))
	}
	if !browser.IsPayloadToken(string(refs[0])) {
		t.Errorf("URLPayloadRefs[0] = %q is not an opaque token (SSOT §4.4 BR-PRIV-NOSTORE)", refs[0])
	}
	if strings.Contains(string(refs[0]), secretURL) {
		t.Errorf("URLPayloadRefs[0] = %q leaks raw URL %q (SSOT privacy violation)", refs[0], secretURL)
	}
}

// SSOT §4.1 OP15: BrowserRemoveTab Forgets the opaque ref of the removed tab.
func TestControllerBrowserRemoveTab_ForgetsRemovedRef(t *testing.T) {
	ctrl, payloadStore, winID := newControllerWithPayloadStore(t)
	ctx := context.Background()
	if _, err := ctrl.ApplyIntent(ctx, intent.BrowserAddTab{Project: "p1", WindowID: winID, URL: "https://a"}); err != nil {
		t.Fatalf("seed AddTab: %v", err)
	}
	if _, err := ctrl.ApplyIntent(ctx, intent.BrowserAddTab{Project: "p1", WindowID: winID, URL: "https://b"}); err != nil {
		t.Fatalf("seed AddTab 2: %v", err)
	}
	tokenToForget := string(ctrl.state.Desired.Projects["p1"].Windows[0].Browser.URLPayloadRefs[0])

	if _, err := ctrl.ApplyIntent(ctx, intent.BrowserRemoveTab{Project: "p1", WindowID: winID, Tab: 1}); err != nil {
		t.Fatalf("RemoveTab: %v", err)
	}

	if len(payloadStore.forgets) != 1 {
		t.Fatalf("expected 1 Forget call, got %d (%v)", len(payloadStore.forgets), payloadStore.forgets)
	}
	if payloadStore.forgets[0] != tokenToForget {
		t.Errorf("Forget called with %q, want %q (the removed tab's token)", payloadStore.forgets[0], tokenToForget)
	}
	if got := len(ctrl.state.Desired.Projects["p1"].Windows[0].Browser.URLPayloadRefs); got != 1 {
		t.Errorf("URLPayloadRefs len after remove = %d, want 1", got)
	}
}

// SSOT §4.1 OP16: BrowserChangeTabURL Forgets old ref and Puts new URL.
func TestControllerBrowserChangeTabURL_RotatesPayload(t *testing.T) {
	ctrl, payloadStore, winID := newControllerWithPayloadStore(t)
	ctx := context.Background()
	if _, err := ctrl.ApplyIntent(ctx, intent.BrowserAddTab{Project: "p1", WindowID: winID, URL: "https://old"}); err != nil {
		t.Fatalf("seed AddTab: %v", err)
	}
	oldToken := string(ctrl.state.Desired.Projects["p1"].Windows[0].Browser.URLPayloadRefs[0])

	if _, err := ctrl.ApplyIntent(ctx, intent.BrowserChangeTabURL{Project: "p1", WindowID: winID, Tab: 1, URL: "https://new"}); err != nil {
		t.Fatalf("ChangeTabURL: %v", err)
	}

	if len(payloadStore.forgets) != 1 || payloadStore.forgets[0] != oldToken {
		t.Errorf("Forget calls = %v, want [%q]", payloadStore.forgets, oldToken)
	}
	if len(payloadStore.puts) != 2 {
		t.Fatalf("expected 2 Put calls (add + change), got %d", len(payloadStore.puts))
	}
	if got := payloadStore.puts[1].URLs; len(got) != 1 || got[0] != "https://new" {
		t.Errorf("second Put URLs = %v, want [\"https://new\"]", got)
	}
	newToken := string(ctrl.state.Desired.Projects["p1"].Windows[0].Browser.URLPayloadRefs[0])
	if newToken == oldToken {
		t.Errorf("ChangeTabURL kept old token %q, expected rotation", newToken)
	}
	if !browser.IsPayloadToken(newToken) {
		t.Errorf("post-change ref %q is not a valid token", newToken)
	}
}

// SSOT §4.1 OP17: BrowserReorderTabs does NOT touch PrivatePayloadStore — refs
// are only rearranged.
func TestControllerBrowserReorderTabs_NoPayloadStoreCalls(t *testing.T) {
	ctrl, payloadStore, winID := newControllerWithPayloadStore(t)
	ctx := context.Background()
	if _, err := ctrl.ApplyIntent(ctx, intent.BrowserAddTab{Project: "p1", WindowID: winID, URL: "https://a"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := ctrl.ApplyIntent(ctx, intent.BrowserAddTab{Project: "p1", WindowID: winID, URL: "https://b"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	putsBefore := len(payloadStore.puts)
	forgetsBefore := len(payloadStore.forgets)

	if _, err := ctrl.ApplyIntent(ctx, intent.BrowserReorderTabs{Project: "p1", WindowID: winID, From: 1, To: 2}); err != nil {
		t.Fatalf("ReorderTabs: %v", err)
	}

	if len(payloadStore.puts) != putsBefore {
		t.Errorf("ReorderTabs triggered Put (delta=%d), should be no-op", len(payloadStore.puts)-putsBefore)
	}
	if len(payloadStore.forgets) != forgetsBefore {
		t.Errorf("ReorderTabs triggered Forget (delta=%d), should be no-op", len(payloadStore.forgets)-forgetsBefore)
	}
}

// PayloadStore=nil is the S14 第一段階 fallback: URLs go in literal.
// This keeps existing tests / migration path working until the daemon wires
// a real store. Promoting OP-14-17 ledger requires that the daemon wires
// PayloadStore non-nil — separate from this test.
func TestControllerBrowserAddTab_NilPayloadStoreStoresLiteral(t *testing.T) {
	env, desired := browserProjectFixture()
	winID := w.DesiredWindowID{Project: "p1", Kind: w.WindowBrowser, Index: 1}
	ctrl := New(env, desired, wm.NewFake(env), store.NewMemoryStore(desired))
	// ctrl.PayloadStore intentionally left nil.

	const literalURL = "https://nil-store-mode"
	if _, err := ctrl.ApplyIntent(context.Background(), intent.BrowserAddTab{
		Project: "p1", WindowID: winID, URL: literalURL,
	}); err != nil {
		t.Fatalf("ApplyIntent: %v", err)
	}
	refs := ctrl.state.Desired.Projects["p1"].Windows[0].Browser.URLPayloadRefs
	if len(refs) != 1 || string(refs[0]) != literalURL {
		t.Errorf("nil PayloadStore mode: URLPayloadRefs = %v, want [%q]", refs, literalURL)
	}
}
