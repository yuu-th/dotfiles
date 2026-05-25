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

// cmdArchive dispatches `projwm archive <subcmd-or-PROJECT>`.
//
// Supported forms:
//   - projwm archive <PROJECT>           → ArchiveProject intent
//   - projwm archive list                → read-only listing of archived
//   - projwm archive purge <PROJECT> --yes → DeleteProject{Purge:true} intent
func cmdArchive(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("archive: usage: projwm archive <PROJECT> | projwm archive list | projwm archive purge <PROJECT> --yes")
	}
	switch args[0] {
	case "list":
		return cmdArchiveList(gf, args[1:], stdout, stderr)
	case "purge":
		return cmdArchivePurge(gf, args[1:], stdout, stderr)
	default:
		return cmdArchiveProject(gf, args, stdout, stderr)
	}
}

func cmdArchiveList(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("archive list: no arguments expected")
	}
	snap, err := loadSnapshotWithTimeout(gf, 5*time.Second)
	if err != nil {
		return err
	}
	renderArchiveList(snap, stdout)
	return nil
}

func cmdArchiveProject(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("archive: usage: projwm archive <PROJECT>")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.ArchiveProject{Project: w.ProjectID(args[0])})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

func cmdArchivePurge(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("archive purge", flag.ContinueOnError)
	fs.SetOutput(stderr)
	yes := fs.Bool("yes", false, "confirm destructive purge")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("archive purge: usage: projwm archive purge <PROJECT> --yes")
	}
	if !*yes {
		return fmt.Errorf("archive purge: refusing without --yes (destructive)")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.DeleteProject{
		ID:    w.ProjectID(fs.Arg(0)),
		Purge: true,
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

// cmdUnarchive implements `projwm unarchive <PROJECT> [--profile X] [--slot Y]`.
//
// SSOT §4.5: unarchive returns the project to park state. To place it in a
// slot, follow with `projwm profile assign <slot> <project>`.
func cmdUnarchive(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("unarchive", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("unarchive: usage: projwm unarchive <PROJECT>")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.UnarchiveProject{
		Project: w.ProjectID(fs.Arg(0)),
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}
