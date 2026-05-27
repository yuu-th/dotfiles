package browserprivacy

import (
	"errors"
	"strings"
	"testing"
)

func TestSensitiveValuesRedactStringConversion(t *testing.T) {
	u := NewSensitiveURL("https://secret.example.test/path?q=SHOULD_NOT_APPEAR", Descriptor{Class: "https", HasQuery: true}, nil)
	title := NewSensitiveTitle("SHOULD_NOT_APPEAR private title", Descriptor{Class: "title"}, nil)
	if strings.Contains(u.String(), "SHOULD_NOT_APPEAR") {
		t.Fatal("URL string conversion leaked raw content")
	}
	if strings.Contains(title.String(), "SHOULD_NOT_APPEAR") {
		t.Fatal("title string conversion leaked raw content")
	}
}

func TestPersistentSnapshotOmitsRawURLAndTitleByDefault(t *testing.T) {
	tab := ObservedTab{
		ID:     "tab-1",
		URL:    NewSensitiveURL("https://secret.example.test/path?q=SHOULD_NOT_APPEAR", Descriptor{Class: "https", HasQuery: true}, nil),
		Title:  NewSensitiveTitle("SHOULD_NOT_APPEAR private title", Descriptor{Class: "title"}, nil),
		Active: true,
	}
	persistent := SnapshotTab(tab, Policy{})
	b, err := MarshalPersistent(persistent, "SHOULD_NOT_APPEAR", "secret.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "https://") || strings.Contains(string(b), "private title") {
		t.Fatalf("persistent snapshot leaked raw content: %s", b)
	}
}

func TestPrivatePayloadRefIsOpaqueWhenEnabled(t *testing.T) {
	ref := PrivatePayloadRef("payload_123")
	tab := ObservedTab{
		ID:     "tab-1",
		URL:    NewSensitiveURL("https://secret.example.test/path", Descriptor{Class: "https"}, &ref),
		Title:  NewSensitiveTitle("secret title", Descriptor{Class: "title"}, nil),
		Active: true,
	}
	persistent := SnapshotTab(tab, Policy{PersistPrivateURLPayload: true})
	b, err := MarshalPersistent(persistent, "secret.example.test", "secret title")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "payload_123") {
		t.Fatalf("expected opaque payload ref, got %s", b)
	}
}

func TestMarshalPersistentRejectsLeakCanary(t *testing.T) {
	_, err := MarshalPersistent(map[string]string{"url": "https://SHOULD_NOT_APPEAR.example.test"}, "SHOULD_NOT_APPEAR")
	if !errors.Is(err, ErrRawBrowserContent) {
		t.Fatalf("err = %v, want ErrRawBrowserContent", err)
	}
}

func TestLogEventOmitsRawURLAndTitle(t *testing.T) {
	tab := leakCanaryTab()
	event := LogTabObserved(tab)
	b, err := MarshalPersistent(event, "SHOULD_NOT_APPEAR", "secret.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "https://") || strings.Contains(string(b), "private title") {
		t.Fatalf("log event leaked raw content: %s", b)
	}
}

func TestIPCPayloadOmitsRawURLAndTitle(t *testing.T) {
	payload := IPCSnapshot(leakCanaryTab())
	b, err := MarshalPersistent(payload, "SHOULD_NOT_APPEAR", "secret.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "https://") || strings.Contains(string(b), "private title") {
		t.Fatalf("IPC payload leaked raw content: %s", b)
	}
}

func TestDiagnosticsOmitsRawURLAndTitle(t *testing.T) {
	diag := DiagnosticsSnapshot(leakCanaryTab())
	b, err := MarshalPersistent(diag, "SHOULD_NOT_APPEAR", "secret.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "https://") || strings.Contains(string(b), "private title") {
		t.Fatalf("diagnostics leaked raw content: %s", b)
	}
}

func leakCanaryTab() ObservedTab {
	return ObservedTab{
		ID:     "tab-1",
		URL:    NewSensitiveURL("https://secret.example.test/path?q=SHOULD_NOT_APPEAR", Descriptor{Class: "https", HasQuery: true}, nil),
		Title:  NewSensitiveTitle("SHOULD_NOT_APPEAR private title", Descriptor{Class: "title"}, nil),
		Active: true,
	}
}
