package wm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// appcontract.go — per-app spawn helpers used by SigWM.Spawn.
//
// implementation-design.md §6 (App minimum contracts) constrains identity
// evidence and spawn preconditions. The helpers here only handle the launch
// step; resolver / settle / verify are owned by the Executor and SigWM.Spawn.

// resolveAppPath returns the .app path for a SpawnRequest, falling back to the
// ManagedEnvironment's ManagedAppPolicy when the request does not carry one.
func (s *SigWM) resolveAppPath(r SpawnRequest) (appPath, bundleID string) {
	appPath = r.AppPath
	bundleID = r.BundleID
	if appPath == "" && bundleID != "" {
		for _, p := range s.Env.Apps.ManagedApps {
			if p.BundleID == bundleID && p.AppPath != "" {
				appPath = p.AppPath
				break
			}
		}
	}
	return appPath, bundleID
}

// spawnGhostty launches Ghostty with a controller-owned --title argument so
// the spawned window's title matches r.Title exactly. impl-design §6 mandates
// "bundle ID == Ghostty AND exact expected title" for unique-strong identity.
//
// When SpawnRequest.TmuxSession is non-empty, this function additionally
// ensures the named tmux session exists (creating it detached against
// r.ProjectPath, or as a grouped clone of r.ViewerSourceTmuxSession for
// viewers) and instructs Ghostty to attach to it via `-e tmux new-session
// -A -s <name>`. The returned createdSession bool is true only when the
// tmux session was freshly created by this call (used by the caller to
// decide whether to send-keys an AI command after settle).
func (s *SigWM) spawnGhostty(ctx context.Context, r SpawnRequest) (createdSession bool, err error) {
	if r.Title == "" {
		return false, fmt.Errorf("ghostty: SpawnRequest.Title is required for controller-owned title")
	}
	appPath, bundleID := s.resolveAppPath(r)
	args := []string{fmt.Sprintf("--title=%s", r.Title)}

	if r.TmuxSession != "" && s.Tmux != nil {
		if r.ViewerSourceTmuxSession != "" {
			// Viewer: ensure source AI session exists (best-effort: if
			// missing, create empty placeholder rooted at ProjectPath)
			// then grouped-clone. Production callers ensure the AI window
			// is spawned first, so missing-source is unusual.
			if exists, herr := s.Tmux.HasSession(ctx, r.ViewerSourceTmuxSession); herr == nil && !exists {
				if _, eerr := s.Tmux.EnsureSession(ctx, r.ViewerSourceTmuxSession, r.ProjectPath); eerr != nil {
					return false, fmt.Errorf("ghostty: ensure viewer source session: %w", eerr)
				}
			}
			if eerr := s.Tmux.EnsureGroupedSession(ctx, r.ViewerSourceTmuxSession, r.TmuxSession); eerr != nil {
				return false, fmt.Errorf("ghostty: ensure grouped session: %w", eerr)
			}
			// Treat grouped clone as not-newly-created for AI auto-launch
			// purposes (viewer never sends AICommand).
		} else {
			created, eerr := s.Tmux.EnsureSession(ctx, r.TmuxSession, r.ProjectPath)
			if eerr != nil {
				return false, fmt.Errorf("ghostty: ensure session: %w", eerr)
			}
			createdSession = created
		}
		// Have Ghostty attach (or create-and-attach) the tmux session.
		args = append(args,
			"-e", "tmux", "new-session", "-A",
			"-s", r.TmuxSession,
		)
		if r.ProjectPath != "" {
			args = append(args, "-c", r.ProjectPath)
		}
	}

	args = append(args, r.ExtraArgs...)
	if err := s.Launcher.Launch(ctx, appPath, bundleID, args); err != nil {
		return createdSession, err
	}
	return createdSession, nil
}

// spawnZed launches Zed scoped to a project directory. impl-design §6 mandates
// "bundle ID == Zed AND exact project/cwd title AND cwd exists".
func (s *SigWM) spawnZed(ctx context.Context, r SpawnRequest) error {
	if err := validateZedProjectPath(r.ProjectPath); err != nil {
		return err
	}
	if launcher, ok := s.Launcher.(ZedProjectLauncher); ok {
		return launcher.LaunchZedProject(ctx, r.ProjectPath, r.ExtraArgs)
	}
	appPath, bundleID := s.resolveAppPath(r)
	args := []string{r.ProjectPath}
	args = append(args, r.ExtraArgs...)
	return s.Launcher.Launch(ctx, appPath, bundleID, args)
}

func validateZedProjectPath(projectPath string) error {
	if projectPath == "" {
		return fmt.Errorf("zed: SpawnRequest.ProjectPath is required")
	}
	info, err := os.Stat(projectPath)
	if err != nil {
		return fmt.Errorf("zed: stat project path %q: %w", projectPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("zed: project path %q is not a directory", projectPath)
	}
	return nil
}

func (s *SigWM) spawnVivaldi(ctx context.Context, r SpawnRequest) error {
	if r.Title != "" {
		return fmt.Errorf("vivaldi: controller-owned browser title is not supported; got %q", r.Title)
	}
	if s.Browser == nil {
		return fmt.Errorf("vivaldi: BrowserCapabilityAdapter is required for browser restore")
	}
	if r.BrowserPayloadToken == "" {
		return fmt.Errorf("vivaldi: private browser payload token is required for browser restore")
	}
	if strings.TrimSpace(r.BrowserProfile) == "" || r.BrowserProfile == "default" {
		return fmt.Errorf("vivaldi: automation-owned non-default profile %q is required for browser restore", browser.VivaldiAutomationProfile)
	}
	if _, err := s.Browser.OpenInProfile(ctx, r.BrowserProfile, r.BrowserPayloadToken); err != nil {
		return err
	}
	return nil
}

// Compile-time guard: the helpers consume world.WindowKind values referenced
// in SigWM.Spawn so package builds break on enum drift.
var _ = []w.WindowKind{w.WindowShell, w.WindowAI, w.WindowEditor, w.WindowBrowser, w.WindowViewer}
