package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/reducer"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// cmdUp implements `projwm up --ai <name> --slot <SLOT> [--cwd <PATH>] [--as <NAME>]`.
//
// Semantics (requirements §5.8):
//   - If project (defaults to basename of cwd unless --as is given) does not
//     exist, CreateProject{ID, Path, Windows: default ai+shell+editor}.
//   - Then AssignProject{Slot, Project}.
//
// The CLI submits two intents back-to-back. Each is committed independently;
// if the second fails, the project is left created-but-unassigned (no slot).
func cmdUp(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(stderr)
	aiName := fs.String("ai", "", "AI window name (e.g. claude, gemini)")
	slot := fs.String("slot", "", "target slot id (e.g. Q)")
	cwd := fs.String("cwd", "", "project root (defaults to $PWD)")
	asName := fs.String("as", "", "explicit project name (default: basename of cwd)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *aiName == "" || *slot == "" {
		return fmt.Errorf("up: --ai and --slot are required")
	}
	if *cwd == "" {
		v, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("up: get cwd: %w", err)
		}
		*cwd = v
	}
	abs, err := filepath.Abs(*cwd)
	if err != nil {
		return fmt.Errorf("up: abs cwd: %w", err)
	}
	projectName := *asName
	if projectName == "" {
		projectName = filepath.Base(abs)
	}
	pid := w.ProjectID(projectName)

	snap, err := loadSnapshotWithTimeout(gf, 5*time.Second)
	if err != nil {
		// Can't read store — daemon may be the only authority. Try CreateProject anyway.
	}
	c := newDaemonClient(gf)

	if _, exists := snap.Desired.Projects[pid]; !exists {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := c.SubmitIntent(ctx, intent.CreateProject{
			ID:      pid,
			Path:    abs,
			Windows: defaultProjectWindows(pid, *aiName),
		})
		if err != nil {
			return fmt.Errorf("up: create project: %w", err)
		}
		fmt.Fprintf(stdout, "created project %s: %s", pid, formatIntentResponse(resp))
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()
	resp, err := c.SubmitIntent(ctx2, intent.AssignProject{
		Slot:    w.SlotID(*slot),
		Project: pid,
	})
	if err != nil {
		return fmt.Errorf("up: assign project: %w", err)
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

// defaultProjectWindows wraps reducer.DefaultProjectWindows so the CLI
// and the daemon emit the same DesiredWindow shape. SSOT §7.3 mandates
// title `ai-N:<project>` (no AI name embedded); §4.4 routes the AI launch
// command via DesiredAISession populated by the reducer.
func defaultProjectWindows(pid w.ProjectID, aiName string) []w.DesiredWindow {
	return reducer.DefaultProjectWindows(pid, aiName)
}

// cmdAddAI / cmdAddShell / cmdAddEditor: thin wrappers around AddWindow.
func cmdAddAI(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	return cmdAddKind(gf, args, stdout, stderr, w.WindowAI, "add-ai")
}
func cmdAddShell(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	return cmdAddKind(gf, args, stdout, stderr, w.WindowShell, "add-shell")
}
func cmdAddEditor(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	return cmdAddKind(gf, args, stdout, stderr, w.WindowEditor, "add-editor")
}

func cmdAddKind(gf globalFlags, args []string, stdout, stderr io.Writer, kind w.WindowKind, fsName string) error {
	fs := flag.NewFlagSet(fsName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	aiName := fs.String("ai", "", "AI name (only used for add-ai)")
	project := fs.String("project", "", "target project (default: project on focused slot)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pid, err := resolveProjectArg(gf, *project)
	if err != nil {
		return fmt.Errorf("%s: %w", fsName, err)
	}
	if kind == w.WindowAI && *aiName == "" {
		return fmt.Errorf("%s: --ai is required for AI window", fsName)
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.AddWindow{
		Project:    pid,
		WindowKind: kind,
		Index:      0, // auto-pick
		AIName:     *aiName,
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

// cmdRemove implements `projwm remove --window <KIND-N> [--project <P>]`.
func cmdRemove(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	fs.SetOutput(stderr)
	winSpec := fs.String("window", "", "window to remove (e.g. ai-1, shell-2, editor-1)")
	project := fs.String("project", "", "target project (default: project on focused slot)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *winSpec == "" {
		return fmt.Errorf("remove: --window is required")
	}
	pid, err := resolveProjectArg(gf, *project)
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	kind, idx, err := parseWindowSpec(*winSpec)
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.RemoveWindow{
		Project:  pid,
		WindowID: w.DesiredWindowID{Project: pid, Kind: kind, Index: idx},
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

// parseWindowSpec accepts "ai-1", "shell-3", "editor-1" forms.
func parseWindowSpec(spec string) (w.WindowKind, int, error) {
	dash := strings.LastIndex(spec, "-")
	if dash <= 0 {
		return "", 0, fmt.Errorf("window spec %q must be KIND-N (e.g. ai-1)", spec)
	}
	kindStr := spec[:dash]
	var n int
	if _, err := fmt.Sscanf(spec[dash+1:], "%d", &n); err != nil || n < 1 {
		return "", 0, fmt.Errorf("window spec %q must have positive numeric suffix", spec)
	}
	switch kindStr {
	case "ai":
		return w.WindowAI, n, nil
	case "shell":
		return w.WindowShell, n, nil
	case "editor":
		return w.WindowEditor, n, nil
	case "browser":
		return w.WindowBrowser, n, nil
	default:
		return "", 0, fmt.Errorf("unknown window kind %q (expect ai|shell|editor|browser)", kindStr)
	}
}

// resolveProjectArg returns the user-supplied project, or — when empty —
// tries to deduce it from the active profile's first non-empty assignment
// (best effort; daemon will reject if mismatched).
func resolveProjectArg(gf globalFlags, explicit string) (w.ProjectID, error) {
	if explicit != "" {
		return w.ProjectID(explicit), nil
	}
	snap, err := loadSnapshotWithTimeout(gf, 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("--project required (store unavailable: %v)", err)
	}
	prof, ok := snap.Desired.Profiles[snap.Desired.ActiveProfile]
	if !ok {
		return "", fmt.Errorf("--project required (no active profile)")
	}
	// First assigned slot in slot order.
	for _, sid := range snap.Environment.SlotOrder() {
		if pid, ok := prof.Assignments[sid]; ok && pid != "" {
			return pid, nil
		}
	}
	return "", fmt.Errorf("--project required (no slot is assigned in active profile)")
}
