package browser

import (
	"reflect"
	"testing"
)

func TestParseInspectTabsByWindow_HappyPath(t *testing.T) {
	// Two windows: "browser-1:dotfiles" with 2 tabs, "browser-1:manaflow"
	// with 1 tab. Separators match the AppleScript producer.
	raw := "browser-1:dotfiles\x1fhttps://a\x1dhttps://b\x1ebrowser-1:manaflow\x1fhttps://c\x1e"
	got := parseInspectTabsByWindow(raw)
	want := []WindowTabs{
		{Title: "browser-1:dotfiles", URLs: []string{"https://a", "https://b"}},
		{Title: "browser-1:manaflow", URLs: []string{"https://c"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %#v\nwant = %#v", got, want)
	}
}

func TestParseInspectTabsByWindow_EmptyInput(t *testing.T) {
	if got := parseInspectTabsByWindow(""); len(got) != 0 {
		t.Errorf("expected nil/empty for empty input, got %#v", got)
	}
	if got := parseInspectTabsByWindow("   \n  "); len(got) != 0 {
		t.Errorf("expected nil/empty for whitespace-only input, got %#v", got)
	}
}

// User-profile window with no managed title — observer should still see
// it (with arbitrary title) and let naming.IdentityFromTitle classify
// it as External.
func TestParseInspectTabsByWindow_UnmanagedWindowKept(t *testing.T) {
	raw := "My Personal Vivaldi Window\x1fhttps://random.example\x1e"
	got := parseInspectTabsByWindow(raw)
	want := []WindowTabs{
		{Title: "My Personal Vivaldi Window", URLs: []string{"https://random.example"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %#v\nwant = %#v", got, want)
	}
}

// Window with zero tabs (e.g., just-opened blank window). Title is
// kept; URLs slice is nil.
func TestParseInspectTabsByWindow_WindowWithoutTabs(t *testing.T) {
	raw := "browser-1:dotfiles\x1f\x1e"
	got := parseInspectTabsByWindow(raw)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1: %#v", len(got), got)
	}
	if got[0].Title != "browser-1:dotfiles" {
		t.Errorf("Title = %q, want browser-1:dotfiles", got[0].Title)
	}
	if len(got[0].URLs) != 0 {
		t.Errorf("URLs = %v, want empty", got[0].URLs)
	}
}

// Malformed record (missing fieldSep) is dropped silently rather than
// poisoning the entire batch.
func TestParseInspectTabsByWindow_MalformedRecordSkipped(t *testing.T) {
	raw := "good-title\x1fhttps://a\x1eMALFORMED-NO-FIELDSEP\x1eother-title\x1fhttps://b\x1e"
	got := parseInspectTabsByWindow(raw)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (malformed skipped): %#v", len(got), got)
	}
	if got[0].Title != "good-title" || got[1].Title != "other-title" {
		t.Errorf("titles = [%q %q], want [good-title other-title]", got[0].Title, got[1].Title)
	}
}
