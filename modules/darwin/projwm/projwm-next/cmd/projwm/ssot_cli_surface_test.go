package main

import (
	"strings"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestSSOTCLIExposesBrowserTabOperations(t *testing.T) {
	required := []string{
		"projwm browser add-tab",
		"projwm browser remove-tab",
		"projwm browser change-tab-url",
		"projwm browser reorder-tabs",
	}
	var missing []string
	for _, snippet := range required {
		if !strings.Contains(usage, snippet) {
			missing = append(missing, snippet)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("SSOT §4.1 CLI surface is incomplete: usage missing %v", missing)
	}
}

func TestSSOTCLIUpUsesCanonicalDefaultWindowTitles(t *testing.T) {
	windows := defaultProjectWindows("dotfiles", "claude")
	want := map[w.WindowKind]string{
		w.WindowAI:     "ai-1:dotfiles",
		w.WindowShell:  "shell-1:dotfiles",
		w.WindowEditor: "dotfiles",
	}
	for _, win := range windows {
		expected, ok := want[win.Kind]
		if !ok {
			t.Fatalf("unexpected default window kind %s", win.Kind)
		}
		if win.TitleContract.Expected != expected {
			t.Fatalf("default %s title = %q, want %q", win.Kind, win.TitleContract.Expected, expected)
		}
	}
}
