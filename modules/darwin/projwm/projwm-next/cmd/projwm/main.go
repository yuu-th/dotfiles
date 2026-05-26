// projwm is the user-facing CLI for projwm-next.
//
// 参照: queue/projwm-cockpit-requirements.md (v2.3), queue/projwm-cockpit-implementation-design.md (v3).
//
// Layer 構成:
//   - Layer 1: projwmctl (低レベル IPC client、debug 用)
//   - Layer 2: projwm (本ファイル、ユーザ向け CLI)
//   - Layer 3: projwm-cockpit (常駐 TUI)
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

const usage = `projwm — projwm-next user CLI

Usage:
  projwm status [--json]
  projwm doctor
  projwm trace [--last | <txid>]
  projwm up --ai <name> --slot <SLOT> [--cwd <PATH>] [--as <NAME>]
  projwm add-ai --ai <name> [--project <P>]
  projwm add-shell [--project <P>]
  projwm add-editor [--project <P>]
  projwm add-browser [--project <P>]
  projwm remove --window <KIND-N> [--project <P>]
  projwm profile create <NAME> [--description <TEXT>] [--inactive-policy remove|keep]
  projwm profile switch <NAME>
  projwm profile assign <SLOT> <PROJECT>
  projwm profile unassign <SLOT>
  projwm profile delete <NAME>
  projwm profile rename <OLD> <NEW>
  projwm profile list
  projwm profile show [<NAME>]
  projwm archive <PROJECT>
  projwm archive list
  projwm archive purge <PROJECT> --yes
  projwm unarchive <PROJECT> [--profile <X>] [--slot <Y>]
  projwm jump <TARGET>
  projwm cockpit <show|hide|toggle|focus>
  projwm browser add-tab --project <P> --window <W> --url <URL>
  projwm browser remove-tab --project <P> --window <W> --tab <N>
  projwm browser change-tab-url --project <P> --window <W> --tab <N> --url <URL>
  projwm browser reorder-tabs --project <P> --window <W> --from <N> --to <M>
  projwm reconcile [--dry-run] [--verbose]
  projwm validate-environment
  projwm tui

Global flags (read from env if not specified):
  --socket-path <path>          (env: PROJWM_NEXT_SOCKET_PATH)
  --managed-environment <path>  (env: PROJWM_NEXT_MANAGED_ENVIRONMENT)
  --manifest-digest <hex>       (env: PROJWM_NEXT_MANIFEST_DIGEST)
  --store-dir <path>            (env: PROJWM_NEXT_STORE_DIR)
`

// globalFlags holds projwm CLI flags shared across subcommands.
type globalFlags struct {
	socketPath     string
	manifestPath   string
	manifestDigest string
	storeDir       string
}

// envDefault returns the value of envKey if set, else fallback.
func envDefault(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}

// parseGlobalFlags strips known --global args from the input slice,
// populating gf. Returns the remaining args (the subcommand + its args).
//
// Global flags can appear before OR after the subcommand. The first
// non-flag positional is the subcommand.
func parseGlobalFlags(args []string) (gf globalFlags, rest []string) {
	gf.socketPath = envDefault("PROJWM_NEXT_SOCKET_PATH", "")
	gf.manifestPath = envDefault("PROJWM_NEXT_MANAGED_ENVIRONMENT", "")
	gf.manifestDigest = envDefault("PROJWM_NEXT_MANIFEST_DIGEST", "")
	gf.storeDir = envDefault("PROJWM_NEXT_STORE_DIR", "")

	rest = make([]string, 0, len(args))
	i := 0
	for i < len(args) {
		a := args[i]
		consumed := false
		switch {
		case a == "--socket-path" && i+1 < len(args):
			gf.socketPath = args[i+1]
			i += 2
			consumed = true
		case a == "--managed-environment" && i+1 < len(args):
			gf.manifestPath = args[i+1]
			i += 2
			consumed = true
		case a == "--manifest-digest" && i+1 < len(args):
			gf.manifestDigest = args[i+1]
			i += 2
			consumed = true
		case a == "--store-dir" && i+1 < len(args):
			gf.storeDir = args[i+1]
			i += 2
			consumed = true
		}
		if !consumed {
			rest = append(rest, a)
			i++
		}
	}
	return gf, rest
}

// runSubcommand dispatches the parsed subcommand to its handler.
// gf carries shared CLI flags. args is the subcommand-specific tail.
func runSubcommand(gf globalFlags, sub string, args []string, stdout, stderr io.Writer) error {
	switch sub {
	case "status":
		return cmdStatus(gf, args, stdout, stderr)
	case "doctor":
		return cmdDoctor(gf, args, stdout, stderr)
	case "trace":
		return cmdTrace(gf, args, stdout, stderr)
	case "up":
		return cmdUp(gf, args, stdout, stderr)
	case "add-ai":
		return cmdAddAI(gf, args, stdout, stderr)
	case "add-shell":
		return cmdAddShell(gf, args, stdout, stderr)
	case "add-editor":
		return cmdAddEditor(gf, args, stdout, stderr)
	case "add-browser":
		return cmdAddBrowser(gf, args, stdout, stderr)
	case "remove":
		return cmdRemove(gf, args, stdout, stderr)
	case "profile":
		return cmdProfile(gf, args, stdout, stderr)
	case "archive":
		return cmdArchive(gf, args, stdout, stderr)
	case "unarchive":
		return cmdUnarchive(gf, args, stdout, stderr)
	case "jump":
		return cmdJump(gf, args, stdout, stderr)
	case "cockpit":
		return cmdCockpit(gf, args, stdout, stderr)
	case "browser":
		return cmdBrowser(gf, args, stdout, stderr)
	case "reconcile":
		return cmdReconcile(gf, args, stdout, stderr)
	case "validate-environment":
		return cmdValidateEnvironment(gf, args, stdout, stderr)
	case "tui":
		return cmdTUI(gf, args, stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		return fmt.Errorf("unknown subcommand %q (run `projwm help` for usage)", sub)
	}
}

// run is the testable entry point. It parses global flags + dispatches.
func run(args []string, stdout, stderr io.Writer) error {
	gf, rest := parseGlobalFlags(args)
	if len(rest) == 0 {
		fmt.Fprint(stderr, usage)
		return errors.New("subcommand required")
	}
	return runSubcommand(gf, rest[0], rest[1:], stdout, stderr)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "projwm: %v\n", err)
		os.Exit(1)
	}
}
