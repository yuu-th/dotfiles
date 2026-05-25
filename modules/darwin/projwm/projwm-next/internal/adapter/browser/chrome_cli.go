package browser

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	w "github.com/yuu-th/projwm-next/internal/world"
)

type CLIExecutor interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type CmdCLIExecutor struct {
	Bin string
}

func (c CmdCLIExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := c.Bin
	if bin == "" {
		bin = "chrome-cli"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("browser/chrome-cli: command failed: %w (output bytes=%d)", err, len(out))
	}
	return out, nil
}

type ChromeCLIAdapter struct {
	Exec           CLIExecutor
	PrivateStore   PrivatePayloadStore
	DefaultProfile string
	ObserveContent bool
	SettleTimeout  time.Duration
}

func NewChromeCLIAdapter(exec CLIExecutor, privateStore PrivatePayloadStore) *ChromeCLIAdapter {
	if exec == nil {
		exec = CmdCLIExecutor{}
	}
	return &ChromeCLIAdapter{
		Exec:           exec,
		PrivateStore:   privateStore,
		DefaultProfile: "default",
		SettleTimeout:  10 * time.Second,
	}
}

func (a *ChromeCLIAdapter) ObserveWindows(ctx context.Context) ([]WindowSnapshot, error) {
	if a.Exec == nil {
		return nil, errors.New("browser/chrome-cli: executor is required")
	}
	out, err := a.Exec.Run(ctx, "list", "windows")
	if err != nil {
		return nil, redactedCLIError(ctx, "list windows", err)
	}
	ids := parseChromeCLIWindowIDs(string(out))
	snapshots := make([]WindowSnapshot, 0, len(ids))
	for _, id := range ids {
		snapshot := WindowSnapshot{
			WindowID:        w.LiveWindowID(id),
			BrowserWindowID: id,
			ProfileID:       a.DefaultProfile,
		}
		if a.ObserveContent && a.PrivateStore != nil {
			tabsOut, err := a.Exec.Run(ctx, "list", "tabs", "-w", id)
			if err != nil {
				return nil, redactedCLIError(ctx, "list tabs", err)
			}
			tabs := parseChromeCLITabs(string(tabsOut))
			snapshot.TabCount = len(tabs)
			if len(tabs) > 0 {
				urls := make([]string, 0, len(tabs))
				for _, tab := range tabs {
					urls = append(urls, tab.URL)
				}
				token, err := a.PrivateStore.Put(ctx, PrivatePayload{URLs: urls})
				if err != nil {
					return nil, err
				}
				snapshot.PayloadToken = token
			}
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func (a *ChromeCLIAdapter) FocusWindow(ctx context.Context, id w.LiveWindowID) error {
	if a.Exec == nil {
		return errors.New("browser/chrome-cli: executor is required")
	}
	tabsOut, err := a.Exec.Run(ctx, "list", "tabs", "-w", string(id))
	if err != nil {
		return redactedCLIError(ctx, "list tabs", err)
	}
	tabs := parseChromeCLITabs(string(tabsOut))
	if len(tabs) == 0 {
		return fmt.Errorf("browser/chrome-cli: window %s has no tabs to focus", id)
	}
	if _, err := a.Exec.Run(ctx, "activate", "-t", tabs[0].ID, "--focus"); err != nil {
		return redactedCLIError(ctx, "activate tab", err)
	}
	return nil
}

func (a *ChromeCLIAdapter) OpenInProfile(ctx context.Context, profile string, payloadToken string) (OpenResult, error) {
	if a.Exec == nil {
		return OpenResult{}, errors.New("browser/chrome-cli: executor is required")
	}
	if a.PrivateStore == nil {
		return OpenResult{}, errors.New("browser/chrome-cli: private payload store is required")
	}
	if payloadToken == "" {
		return OpenResult{}, errors.New("browser/chrome-cli: private payload token is required")
	}
	if profile != "" && a.DefaultProfile != "" && profile != a.DefaultProfile {
		return OpenResult{}, fmt.Errorf("browser/chrome-cli: profile %q is not supported by chrome-cli adapter", profile)
	}
	payload, err := a.PrivateStore.Get(ctx, payloadToken)
	if err != nil {
		return OpenResult{}, err
	}
	if len(payload.URLs) == 0 {
		return OpenResult{}, errors.New("browser/chrome-cli: private payload has no URLs")
	}
	before, err := a.windowIDSet(ctx)
	if err != nil {
		return OpenResult{}, err
	}
	if _, err := a.Exec.Run(ctx, "open", payload.URLs[0], "-n"); err != nil {
		return OpenResult{}, redactedCLIError(ctx, "open new window", err)
	}
	windowID, err := a.settleNewWindow(ctx, before)
	if err != nil {
		return OpenResult{}, err
	}
	for _, rawURL := range payload.URLs[1:] {
		if _, err := a.Exec.Run(ctx, "open", rawURL, "-w", string(windowID)); err != nil {
			return OpenResult{}, redactedCLIError(ctx, "open tab", err)
		}
	}
	return OpenResult{BrowserWindowID: string(windowID)}, nil
}

func (a *ChromeCLIAdapter) CloseWindow(ctx context.Context, id w.LiveWindowID) error {
	if a.Exec == nil {
		return errors.New("browser/chrome-cli: executor is required")
	}
	_, err := a.Exec.Run(ctx, "close", "-w", string(id))
	return redactedCLIError(ctx, "close window", err)
}

func (a *ChromeCLIAdapter) windowIDSet(ctx context.Context) (map[string]struct{}, error) {
	out, err := a.Exec.Run(ctx, "list", "windows")
	if err != nil {
		return nil, redactedCLIError(ctx, "list windows", err)
	}
	ids := parseChromeCLIWindowIDs(string(out))
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

func redactedCLIError(ctx context.Context, action string, err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return fmt.Errorf("browser/chrome-cli: %s failed (%T)", action, err)
}

func (a *ChromeCLIAdapter) settleNewWindow(ctx context.Context, before map[string]struct{}) (w.LiveWindowID, error) {
	timeout := a.SettleTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var lastCount int
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		now, err := a.windowIDSet(ctx)
		if err != nil {
			return "", err
		}
		var candidates []string
		for id := range now {
			if _, existed := before[id]; !existed {
				candidates = append(candidates, id)
			}
		}
		lastCount = len(candidates)
		if len(candidates) == 1 {
			return w.LiveWindowID(candidates[0]), nil
		}
		if len(candidates) > 1 {
			return "", fmt.Errorf("browser/chrome-cli: ambiguous new browser windows count=%d", len(candidates))
		}
		time.Sleep(150 * time.Millisecond)
	}
	return "", fmt.Errorf("browser/chrome-cli: timeout waiting for new browser window (last count=%d)", lastCount)
}

type chromeCLITab struct {
	ID    string
	URL   string
	Title string
}

var (
	windowLineRE = regexp.MustCompile(`(?i)\bwindow\s+([0-9]+)\b`)
	tabLineRE    = regexp.MustCompile(`^\s*\[([0-9]+)\]\s+(\S+)(?:\s+(.*))?$`)
)

func parseChromeCLIWindowIDs(out string) []string {
	var ids []string
	seen := map[string]struct{}{}
	for _, line := range strings.Split(out, "\n") {
		m := windowLineRE.FindStringSubmatch(line)
		if len(m) != 2 {
			continue
		}
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		ids = append(ids, m[1])
	}
	return ids
}

func parseChromeCLITabs(out string) []chromeCLITab {
	var tabs []chromeCLITab
	for _, line := range strings.Split(out, "\n") {
		m := tabLineRE.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		tab := chromeCLITab{ID: m[1], URL: m[2]}
		if len(m) > 3 {
			tab.Title = strings.TrimSpace(m[3])
		}
		tabs = append(tabs, tab)
	}
	return tabs
}

var _ BrowserCapabilityAdapter = (*ChromeCLIAdapter)(nil)
