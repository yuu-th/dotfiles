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

// cmdProfile dispatches `projwm profile <subcmd>`.
func cmdProfile(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("profile: subcommand required (list|show|switch|assign|unassign|create|delete|rename)")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdProfileList(gf, rest, stdout, stderr)
	case "show":
		return cmdProfileShow(gf, rest, stdout, stderr)
	case "switch":
		return cmdProfileSwitch(gf, rest, stdout, stderr)
	case "assign":
		return cmdProfileAssign(gf, rest, stdout, stderr)
	case "unassign":
		return cmdProfileUnassign(gf, rest, stdout, stderr)
	case "create":
		return cmdProfileCreate(gf, rest, stdout, stderr)
	case "delete":
		return cmdProfileDelete(gf, rest, stdout, stderr)
	case "rename":
		return cmdProfileRename(gf, rest, stdout, stderr)
	default:
		return fmt.Errorf("profile: unknown subcommand %q", sub)
	}
}

func cmdProfileList(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("profile list: no arguments expected")
	}
	snap, err := loadSnapshotWithTimeout(gf, 5*time.Second)
	if err != nil {
		return err
	}
	renderProfileList(snap, stdout)
	return nil
}

func cmdProfileShow(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	name := w.ProfileID("")
	if len(args) == 1 {
		name = w.ProfileID(args[0])
	} else if len(args) > 1 {
		return fmt.Errorf("profile show: usage: projwm profile show [<NAME>]")
	}
	snap, err := loadSnapshotWithTimeout(gf, 5*time.Second)
	if err != nil {
		return err
	}
	return renderProfileShow(snap, name, stdout)
}

func cmdProfileSwitch(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("profile switch: usage: projwm profile switch <NAME>")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.SwitchProfile{To: w.ProfileID(args[0])})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

func cmdProfileAssign(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("profile assign: usage: projwm profile assign <SLOT> <PROJECT>")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.AssignProject{
		Slot:    w.SlotID(args[0]),
		Project: w.ProjectID(args[1]),
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

func cmdProfileUnassign(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("profile unassign: usage: projwm profile unassign <SLOT>")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.UnassignSlot{Slot: w.SlotID(args[0])})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

func cmdProfileCreate(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("profile create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	desc := fs.String("description", "", "human description")
	policy := fs.String("inactive-policy", "remove", "remove or keep")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("profile create: usage: projwm profile create <NAME> [--description <TEXT>] [--inactive-policy remove|keep]")
	}
	pol := w.InactivePolicy(*policy)
	if pol != w.InactivePolicyRemove && pol != w.InactivePolicyKeep {
		return fmt.Errorf("profile create: --inactive-policy must be 'remove' or 'keep'")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.CreateProfile{
		ID:             w.ProfileID(fs.Arg(0)),
		Description:    *desc,
		InactivePolicy: pol,
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

func cmdProfileDelete(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("profile delete: usage: projwm profile delete <NAME>")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.DeleteProfile{ID: w.ProfileID(args[0])})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}

func cmdProfileRename(gf globalFlags, args []string, stdout, stderr io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("profile rename: usage: projwm profile rename <OLD> <NEW>")
	}
	c := newDaemonClient(gf)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.SubmitIntent(ctx, intent.RenameProfile{
		Old: w.ProfileID(args[0]),
		New: w.ProfileID(args[1]),
	})
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, formatIntentResponse(resp))
	return nil
}
