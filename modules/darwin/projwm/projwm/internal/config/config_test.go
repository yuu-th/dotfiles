package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	c := Default()
	if c.ViewerWorkspace != "A" {
		t.Errorf("viewer=%q", c.ViewerWorkspace)
	}
	if len(c.SlotNames) != 10 {
		t.Errorf("slot_names len=%d, want 10", len(c.SlotNames))
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	res, err := Load(filepath.Join(dir, "nonexistent.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.UsedDefault {
		t.Error("expected UsedDefault=true")
	}
}

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.toml")
	os.WriteFile(p, []byte(`viewer_workspace = "A"
slot_names = ["Q","W","E"]
`), 0o644)
	res, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.UsedDefault {
		t.Error("UsedDefault should be false")
	}
	if len(res.Config.SlotNames) != 3 {
		t.Errorf("slot_names=%d", len(res.Config.SlotNames))
	}
}

func TestLoadEmptySlots(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.toml")
	os.WriteFile(p, []byte(`viewer_workspace = "A"
slot_names = []
`), 0o644)
	if _, err := Load(p); err == nil {
		t.Error("expected error: empty slot_names")
	}
}

func TestLoadCollidingViewer(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.toml")
	os.WriteFile(p, []byte(`viewer_workspace = "Q"
slot_names = ["Q","W"]
`), 0o644)
	if _, err := Load(p); err == nil {
		t.Error("expected error: viewer in slots")
	}
}

func TestLoadUndecoded(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.toml")
	os.WriteFile(p, []byte(`viewer_workspace = "A"
slot_names = ["Q"]
future_field = "ok"
`), 0o644)
	res, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UndecodedKeys) == 0 {
		t.Error("expected undecoded keys for forward compat")
	}
}

func TestLoadParseError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.toml")
	os.WriteFile(p, []byte(`this is not valid {{`), 0o644)
	if _, err := Load(p); err == nil {
		t.Error("expected parse error")
	}
}
