package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// cmdBrowser dispatches `projwm browser <subcmd>` per SSOT §4.1 OP14-17.
//
// Subcommands:
//   - add-tab        --project <P> --window <W> --url <URL>
//   - remove-tab     --project <P> --window <W> --tab <N>
//   - change-tab-url --project <P> --window <W> --tab <N> --url <URL>
//   - reorder-tabs   --project <P> --window <W> --from <N> --to <M>
//
// Window arg `<W>` is the kind-index form "browser-1" or the project's
// canonical title "browser-1:dotfiles" — the CLI parses it into a
// DesiredWindowID. Project is mandatory (CLI doesn't try to deduce from
// active profile here — explicit > implicit for browser tab edits).
func cmdBrowser(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("browser: subcommand required (add-tab|remove-tab|change-tab-url|reorder-tabs)")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add-tab":
		return cmdBrowserAddTab(gf, rest, stdout, stderr)
	case "remove-tab":
		return cmdBrowserRemoveTab(gf, rest, stdout, stderr)
	case "change-tab-url":
		return cmdBrowserChangeTabURL(gf, rest, stdout, stderr)
	case "reorder-tabs":
		return cmdBrowserReorderTabs(gf, rest, stdout, stderr)
	default:
		return fmt.Errorf("browser: unknown subcommand %q", sub)
	}
}

// parseBrowserWindowFlag は --window 引数を DesiredWindowID に変換する。
// 形式: "browser-N" (e.g. "browser-1") — 既存 SSOT §7.3 命名規約に従う。
func parseBrowserWindowFlag(project, window string) (w.DesiredWindowID, error) {
	kind, idx, err := parseWindowSpec(window)
	if err != nil {
		return w.DesiredWindowID{}, fmt.Errorf("--window: %w", err)
	}
	if kind != w.WindowBrowser {
		return w.DesiredWindowID{}, fmt.Errorf("--window must reference a browser window (got %s)", kind)
	}
	return w.DesiredWindowID{Project: w.ProjectID(project), Kind: kind, Index: idx}, nil
}

func cmdBrowserAddTab(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("browser add-tab", flag.ContinueOnError)
	fs.SetOutput(stderr)
	project := fs.String("project", "", "project ID (required)")
	window := fs.String("window", "browser-1", "browser window (browser-N)")
	url := fs.String("url", "", "URL to add (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" || *url == "" {
		return fmt.Errorf("browser add-tab: --project and --url are required")
	}
	wid, err := parseBrowserWindowFlag(*project, *window)
	if err != nil {
		return err
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.BrowserAddTab{
		Project: w.ProjectID(*project), WindowID: wid, URL: *url,
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

func cmdBrowserRemoveTab(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("browser remove-tab", flag.ContinueOnError)
	fs.SetOutput(stderr)
	project := fs.String("project", "", "project ID (required)")
	window := fs.String("window", "browser-1", "browser window (browser-N)")
	tab := fs.Int("tab", 0, "1-based tab index to remove (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" || *tab < 1 {
		return fmt.Errorf("browser remove-tab: --project and --tab (>=1) are required")
	}
	wid, err := parseBrowserWindowFlag(*project, *window)
	if err != nil {
		return err
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.BrowserRemoveTab{
		Project: w.ProjectID(*project), WindowID: wid, Tab: *tab,
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

func cmdBrowserChangeTabURL(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("browser change-tab-url", flag.ContinueOnError)
	fs.SetOutput(stderr)
	project := fs.String("project", "", "project ID (required)")
	window := fs.String("window", "browser-1", "browser window (browser-N)")
	tab := fs.Int("tab", 0, "1-based tab index (required)")
	url := fs.String("url", "", "new URL (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" || *tab < 1 || *url == "" {
		return fmt.Errorf("browser change-tab-url: --project, --tab (>=1) and --url are required")
	}
	wid, err := parseBrowserWindowFlag(*project, *window)
	if err != nil {
		return err
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.BrowserChangeTabURL{
		Project: w.ProjectID(*project), WindowID: wid, Tab: *tab, URL: *url,
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

func cmdBrowserReorderTabs(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("browser reorder-tabs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	project := fs.String("project", "", "project ID (required)")
	window := fs.String("window", "browser-1", "browser window (browser-N)")
	from := fs.Int("from", 0, "1-based tab index to move from (required)")
	to := fs.Int("to", 0, "1-based tab index to move to (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" || *from < 1 || *to < 1 {
		return fmt.Errorf("browser reorder-tabs: --project, --from (>=1), --to (>=1) are required")
	}
	wid, err := parseBrowserWindowFlag(*project, *window)
	if err != nil {
		return err
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.BrowserReorderTabs{
		Project: w.ProjectID(*project), WindowID: wid, From: *from, To: *to,
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}
