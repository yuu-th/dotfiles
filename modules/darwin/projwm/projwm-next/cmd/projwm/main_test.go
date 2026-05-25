package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestParseGlobalFlags_Defaults(t *testing.T) {
	t.Setenv("PROJWM_NEXT_SOCKET_PATH", "")
	t.Setenv("PROJWM_NEXT_MANAGED_ENVIRONMENT", "")
	t.Setenv("PROJWM_NEXT_MANIFEST_DIGEST", "")
	t.Setenv("PROJWM_NEXT_STORE_DIR", "")
	gf, rest := parseGlobalFlags([]string{"status"})
	if gf.socketPath != "" || gf.manifestPath != "" || gf.manifestDigest != "" || gf.storeDir != "" {
		t.Errorf("expected all global flags empty, got %+v", gf)
	}
	if !reflect.DeepEqual(rest, []string{"status"}) {
		t.Errorf("expected rest=[status], got %v", rest)
	}
}

func TestParseGlobalFlags_FromEnv(t *testing.T) {
	t.Setenv("PROJWM_NEXT_SOCKET_PATH", "/tmp/sock")
	t.Setenv("PROJWM_NEXT_MANAGED_ENVIRONMENT", "/tmp/manifest.json")
	t.Setenv("PROJWM_NEXT_MANIFEST_DIGEST", "abc123")
	t.Setenv("PROJWM_NEXT_STORE_DIR", "/tmp/store")
	gf, rest := parseGlobalFlags([]string{"status"})
	if gf.socketPath != "/tmp/sock" {
		t.Errorf("socketPath: %q", gf.socketPath)
	}
	if gf.manifestPath != "/tmp/manifest.json" {
		t.Errorf("manifestPath: %q", gf.manifestPath)
	}
	if gf.manifestDigest != "abc123" {
		t.Errorf("manifestDigest: %q", gf.manifestDigest)
	}
	if gf.storeDir != "/tmp/store" {
		t.Errorf("storeDir: %q", gf.storeDir)
	}
	if !reflect.DeepEqual(rest, []string{"status"}) {
		t.Errorf("expected rest=[status], got %v", rest)
	}
}

func TestParseGlobalFlags_FlagOverridesEnv(t *testing.T) {
	t.Setenv("PROJWM_NEXT_SOCKET_PATH", "/from-env")
	gf, rest := parseGlobalFlags([]string{"--socket-path", "/from-flag", "status", "--json"})
	if gf.socketPath != "/from-flag" {
		t.Errorf("expected flag to override env, got %q", gf.socketPath)
	}
	if !reflect.DeepEqual(rest, []string{"status", "--json"}) {
		t.Errorf("expected rest=[status, --json], got %v", rest)
	}
}

func TestParseGlobalFlags_AllFlagsStripped(t *testing.T) {
	t.Setenv("PROJWM_NEXT_SOCKET_PATH", "")
	gf, rest := parseGlobalFlags([]string{
		"--socket-path", "/s",
		"--managed-environment", "/m",
		"--manifest-digest", "deadbeef",
		"--store-dir", "/d",
		"profile", "list",
	})
	if gf.socketPath != "/s" || gf.manifestPath != "/m" || gf.manifestDigest != "deadbeef" || gf.storeDir != "/d" {
		t.Errorf("expected all flags set, got %+v", gf)
	}
	if !reflect.DeepEqual(rest, []string{"profile", "list"}) {
		t.Errorf("expected rest=[profile, list], got %v", rest)
	}
}

func TestRun_NoArgs_PrintsUsage(t *testing.T) {
	t.Setenv("PROJWM_NEXT_SOCKET_PATH", "")
	var stdout, stderr bytes.Buffer
	err := run(nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for no subcommand")
	}
	if !strings.Contains(stderr.String(), "projwm — projwm-next user CLI") {
		t.Errorf("expected usage in stderr, got: %s", stderr.String())
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	t.Setenv("PROJWM_NEXT_SOCKET_PATH", "")
	var stdout, stderr bytes.Buffer
	err := run([]string{"frobnicate"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), `unknown subcommand "frobnicate"`) {
		t.Errorf("expected unknown subcommand error, got: %v", err)
	}
}

func TestRun_HelpPrintsUsageToStdout(t *testing.T) {
	t.Setenv("PROJWM_NEXT_SOCKET_PATH", "")
	var stdout, stderr bytes.Buffer
	err := run([]string{"help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected usage in stdout")
	}
}

func TestRun_DispatchToStatus(t *testing.T) {
	// status with no store-dir errors with a helpful message, proving
	// dispatch reached cmdStatus rather than the unknown-subcommand path.
	t.Setenv("PROJWM_NEXT_SOCKET_PATH", "")
	t.Setenv("PROJWM_NEXT_STORE_DIR", "")
	t.Setenv("PROJWM_NEXT_MANAGED_ENVIRONMENT", "")
	var stdout, stderr bytes.Buffer
	err := run([]string{"status"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing store-dir")
	}
	if !strings.Contains(err.Error(), "store-dir is required") {
		t.Errorf("expected store-dir error from cmdStatus, got: %v", err)
	}
}
