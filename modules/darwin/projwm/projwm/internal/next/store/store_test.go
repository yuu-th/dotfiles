package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInitialGeneration(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Current()
	if err != nil {
		t.Fatal(err)
	}
	if got != InitialGeneration {
		t.Fatalf("initial generation = %q, want %q", got, InitialGeneration)
	}
	if _, err := os.Stat(filepath.Join(dir, ".store_identity.json")); err != nil {
		t.Fatal("store identity marker is required:", err)
	}
}

func TestCommitIncrementsGeneration(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.BeginCommit(InitialGeneration)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.WriteArtifact("desired_world.json", map[string]string{"activeProfile": "work"}); err != nil {
		t.Fatal(err)
	}
	writeRemainingArtifacts(t, c)
	next, err := c.Commit("intent")
	if err != nil {
		t.Fatal(err)
	}
	if next != "E0001-G000001" {
		t.Fatalf("next = %q", next)
	}
	current, err := s.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current != next {
		t.Fatalf("current = %q, want %q", current, next)
	}
	manifest, err := s.LoadManifest(next)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.CommittedBy != "controller" {
		t.Fatalf("committedBy = %q, want controller", manifest.CommittedBy)
	}
	for _, name := range requiredArtifacts {
		if manifest.Files[name] == "" {
			t.Fatalf("manifest missing checksum for %s", name)
		}
	}
}

func TestStaleWriteRejected(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.BeginCommit(InitialGeneration)
	if err != nil {
		t.Fatal(err)
	}
	writeAllArtifacts(t, c)
	if _, err := c.Commit("intent"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginCommit(InitialGeneration); !errors.Is(err, ErrStaleGeneration) {
		t.Fatalf("stale begin err = %v, want ErrStaleGeneration", err)
	}
}

func TestAbortDoesNotAdvanceGeneration(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.BeginCommit(InitialGeneration)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.WriteArtifact("desired_world.json", map[string]string{"activeProfile": "work"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Abort(); err != nil {
		t.Fatal(err)
	}
	current, err := s.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current != InitialGeneration {
		t.Fatalf("abort advanced generation to %q", current)
	}
}

func TestCommitRejectsMissingRequiredArtifact(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.BeginCommit(InitialGeneration)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.WriteArtifact("desired_world.json", map[string]string{"activeProfile": "work"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Commit("intent"); !errors.Is(err, ErrMissingArtifact) {
		t.Fatalf("commit err = %v, want ErrMissingArtifact", err)
	}
	current, err := s.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current != InitialGeneration {
		t.Fatalf("missing artifact commit advanced generation to %q", current)
	}
}

func TestReloadPreservesGeneration(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.BeginCommit(InitialGeneration)
	if err != nil {
		t.Fatal(err)
	}
	writeAllArtifacts(t, c)
	next, err := c.Commit("intent")
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	current, err := reloaded.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current != next {
		t.Fatalf("reloaded current = %q, want %q", current, next)
	}
	if _, err := os.Stat(filepath.Join(dir, "generations", string(next), "manifest.json")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadManifestRejectsChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.BeginCommit(InitialGeneration)
	if err != nil {
		t.Fatal(err)
	}
	writeAllArtifacts(t, c)
	next, err := c.Commit("intent")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "generations", string(next), "desired_world.json"), []byte(`{"tampered":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadManifest(next); err == nil {
		t.Fatal("expected checksum mismatch to be rejected")
	}
}

func writeAllArtifacts(t *testing.T, c *Commit) {
	t.Helper()
	for _, name := range requiredArtifacts {
		if err := c.WriteArtifact(name, map[string]string{"artifact": name}); err != nil {
			t.Fatal(err)
		}
	}
}

func writeRemainingArtifacts(t *testing.T, c *Commit) {
	t.Helper()
	for _, name := range requiredArtifacts[1:] {
		if err := c.WriteArtifact(name, map[string]string{"artifact": name}); err != nil {
			t.Fatal(err)
		}
	}
}
