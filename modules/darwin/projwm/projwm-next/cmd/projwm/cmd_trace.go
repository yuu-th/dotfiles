package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/yuu-th/projwm-next/internal/ipc"
	"github.com/yuu-th/projwm-next/internal/store"
)

// cmdTrace implements `projwm trace [--last | <txid>]`.
//
// Strategy: prefer daemon QueryTrace (so trace is always-fresh and the
// CLI doesn't need the store-dir env). Fall back to direct trace dir
// read when the daemon is unreachable.
func cmdTrace(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("trace", flag.ContinueOnError)
	fs.SetOutput(stderr)
	last := fs.Bool("last", false, "show the most recent transaction trace")
	asJSON := fs.Bool("json", false, "emit raw JSON instead of human format")
	if err := fs.Parse(args); err != nil {
		return err
	}
	traceID := ""
	if fs.NArg() == 1 {
		traceID = fs.Arg(0)
	} else if !*last {
		return fmt.Errorf("trace: usage: projwm trace --last  |  projwm trace <txid>")
	}

	// 1) Try daemon Query first.
	c := newDaemonClient(gf)
	if c.reachable() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := c.Query(ctx, ipc.QueryTrace, traceID)
		if err == nil && resp.Body != nil {
			if *asJSON {
				_, err := stdout.Write(resp.Body)
				return err
			}
			var t store.TransactionTrace
			if err := json.Unmarshal(resp.Body, &t); err == nil {
				scrubTrace(&t)
				renderTrace(t, stdout)
				return nil
			}
		}
	}

	// 2) Fallback: direct trace dir read.
	if gf.storeDir == "" {
		return fmt.Errorf("trace: daemon unreachable and --store-dir not provided")
	}
	tracesDir := filepath.Join(gf.storeDir, "traces")
	if traceID == "" {
		return showLatestTrace(tracesDir, *asJSON, stdout)
	}
	return showTraceByID(tracesDir, traceID, *asJSON, stdout)
}

func showLatestTrace(tracesDir string, asJSON bool, out io.Writer) error {
	entries, err := os.ReadDir(tracesDir)
	if err != nil {
		return fmt.Errorf("trace: read traces dir: %w", err)
	}
	var newest os.DirEntry
	var newestMtime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newestMtime) {
			newest = e
			newestMtime = info.ModTime()
		}
	}
	if newest == nil {
		return fmt.Errorf("trace: no traces found in %s", tracesDir)
	}
	return showTraceFile(filepath.Join(tracesDir, newest.Name()), asJSON, out)
}

func showTraceByID(tracesDir, id string, asJSON bool, out io.Writer) error {
	// Try exact filename first.
	exact := filepath.Join(tracesDir, id+".json")
	if _, err := os.Stat(exact); err == nil {
		return showTraceFile(exact, asJSON, out)
	}
	// Prefix match (some traces use txn-NNNNN naming).
	entries, err := os.ReadDir(tracesDir)
	if err != nil {
		return fmt.Errorf("trace: read traces dir: %w", err)
	}
	matches := []string{}
	for _, e := range entries {
		if !e.IsDir() && fileBasenameWithoutExt(e.Name()) == id {
			matches = append(matches, e.Name())
		}
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return fmt.Errorf("trace: no trace matching %q in %s", id, tracesDir)
	}
	return showTraceFile(filepath.Join(tracesDir, matches[0]), asJSON, out)
}

func fileBasenameWithoutExt(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[:i]
		}
	}
	return name
}

func showTraceFile(path string, asJSON bool, out io.Writer) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("trace: read %s: %w", path, err)
	}
	if asJSON {
		_, err = out.Write(data)
		return err
	}
	var t store.TransactionTrace
	if err := json.Unmarshal(data, &t); err != nil {
		return fmt.Errorf("trace: parse %s: %w", path, err)
	}
	// Daemon-side traces never embed raw URLs, but defensively scrub
	// anything URL-shaped in places where the daemon currently doesn't
	// write them. (Belt-and-suspenders against trace schema drift.)
	scrubTrace(&t)
	renderTrace(t, out)
	return nil
}

// scrubTrace zeroes any obviously-private fields that should not surface
// at CLI output. Daemon-side records are already redacted; this is a
// second line of defense per requirements §13.3.
func scrubTrace(t *store.TransactionTrace) {
	// Nothing currently to scrub — TransactionTrace doesn't embed URLs.
	// Hook left in place for forward compatibility.
	_ = t
}

// Ensure context import stays in use.
var _ context.Context
