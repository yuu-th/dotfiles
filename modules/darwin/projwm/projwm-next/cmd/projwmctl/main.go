// projwmctl is the projwm-next CLI client.
//
// 参照: implementation-design.md §5.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/yuu-th/projwm-next/internal/clientauth"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/ipc"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// buildHello は controller へ送る最初の handshake envelope。
//
// manifestDigest は projwmctl が読み込んだ manifest から計算したものを
// そのまま渡す。daemon 側 digest と一致しなければ daemon が
// ErrProtocolMismatch で reject する。
func buildHello(manifestDigest string) (ipc.Envelope, error) {
	return ipc.NewEnvelope(ipc.MsgHello, ipc.Hello{
		ProtocolVersion:    ipc.ProtocolVersion,
		StoreSchemaVersion: ipc.StoreSchemaVersion,
		ManifestDigest:     manifestDigest,
		ClientName:         "projwmctl",
		ClientVersion:      "0.1.0",
	})
}

// buildIntentRequest は intent payload を IPC envelope に詰める。
// real client は generated request ID を使う; ここでは contract 検証用。
func buildIntentRequest(requestID string, in intent.Intent) (ipc.Envelope, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return ipc.Envelope{}, fmt.Errorf("projwmctl: marshal intent: %w", err)
	}
	return ipc.NewEnvelope(ipc.MsgIntentRequest, ipc.IntentRequest{
		RequestID: requestID,
		Kind:      in.Kind(),
		Payload:   body,
	})
}

func parseIntent(args []string) (intent.Intent, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing command")
	}
	switch args[0] {
	case "reconcile":
		return intent.Reconcile{}, nil
	case "switch-profile":
		if len(args) != 2 {
			return nil, fmt.Errorf("usage: projwmctl switch-profile <profile>")
		}
		return intent.SwitchProfile{To: w.ProfileID(args[1])}, nil
	case "archive":
		if len(args) != 2 {
			return nil, fmt.Errorf("usage: projwmctl archive <project>")
		}
		return intent.ArchiveProject{Project: w.ProjectID(args[1])}, nil
	case "unarchive":
		// SSOT §4.5: unarchive returns to park state; slot assignment is
		// a separate `assign <slot> <project>` step.
		if len(args) != 2 {
			return nil, fmt.Errorf("usage: projwmctl unarchive <project>")
		}
		return intent.UnarchiveProject{Project: w.ProjectID(args[1])}, nil
	case "assign":
		if len(args) != 3 {
			return nil, fmt.Errorf("usage: projwmctl assign <slot> <project>")
		}
		return intent.AssignProject{Slot: w.SlotID(args[1]), Project: w.ProjectID(args[2])}, nil
	case "unassign":
		if len(args) != 2 {
			return nil, fmt.Errorf("usage: projwmctl unassign <slot>")
		}
		return intent.UnassignSlot{Slot: w.SlotID(args[1])}, nil
	case "accept-manual-layout":
		// v2.3 deprecation: AcceptManualLayout is replaced by the
		// daemon-internal AutoSyncLayout (Tier 2 auto-overwrite).
		// projwmctl no longer exposes this wire path so the CLI surface
		// matches requirements §3.2 / I18.
		return nil, fmt.Errorf("projwmctl: accept-manual-layout removed; Tier 2 layout sync is automatic (requirements v2.3)")
	case "validate-environment":
		return intent.ValidateEnvironment{}, nil
	case "show-scratch-shell":
		// SSOT §4.1 OP11: 通常は shortcut から呼ばれるが、debug / script
		// 経路として projwmctl からも発行可能にしておく。
		return intent.ShowScratchShell{}, nil
	case "hide-scratch-shell":
		return intent.HideScratchShell{}, nil
	default:
		return nil, fmt.Errorf("unknown command %q", args[0])
	}
}

func submit(socketPath, manifestPath, manifestDigest string, in intent.Intent) (ipc.IntentResponse, error) {
	if err := clientauth.VerifyManagedSocket("projwmctl", socketPath, manifestPath, manifestDigest); err != nil {
		return ipc.IntentResponse{}, err
	}
	conn, err := net.DialTimeout("unix", socketPath, 3*time.Second)
	if err != nil {
		return ipc.IntentResponse{}, fmt.Errorf("projwmctl: dial %s: %w", socketPath, err)
	}
	defer conn.Close()

	hello, err := buildHello(manifestDigest)
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if err := ipc.WriteEnvelope(conn, hello); err != nil {
		return ipc.IntentResponse{}, err
	}
	welcome, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if welcome.Type == ipc.MsgReject {
		var rej ipc.Reject
		_ = json.Unmarshal(welcome.Payload, &rej)
		return ipc.IntentResponse{}, &rej.Error
	}
	if welcome.Type != ipc.MsgWelcome {
		return ipc.IntentResponse{}, fmt.Errorf("projwmctl: expected welcome, got %q", welcome.Type)
	}

	reqID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	req, err := buildIntentRequest(reqID, in)
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if err := ipc.WriteEnvelope(conn, req); err != nil {
		return ipc.IntentResponse{}, err
	}
	rawResp, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if rawResp.Type != ipc.MsgIntentResponse {
		return ipc.IntentResponse{}, fmt.Errorf("projwmctl: expected intent-response, got %q", rawResp.Type)
	}
	var resp ipc.IntentResponse
	if err := json.Unmarshal(rawResp.Payload, &resp); err != nil {
		return ipc.IntentResponse{}, fmt.Errorf("projwmctl: decode response: %w", err)
	}
	if resp.Error != nil {
		return resp, resp.Error
	}
	return resp, nil
}

func formatIntentResponse(resp ipc.IntentResponse) string {
	if resp.AcceptedTransaction != nil && resp.CommittedGeneration != nil {
		return fmt.Sprintf("ok request=%s acceptedTransaction=%s committedGeneration=%s\n", resp.RequestID, *resp.AcceptedTransaction, *resp.CommittedGeneration)
	}
	if resp.CommittedGeneration != nil {
		return fmt.Sprintf("ok request=%s committedGeneration=%s\n", resp.RequestID, *resp.CommittedGeneration)
	}
	return fmt.Sprintf("ok request=%s\n", resp.RequestID)
}

func main() {
	socketPath := flag.String("socket-path", os.Getenv("PROJWM_NEXT_SOCKET_PATH"), "projwmd Unix socket path")
	manifestPath := flag.String("managed-environment", os.Getenv("PROJWM_NEXT_MANAGED_ENVIRONMENT"), "managed environment manifest path authorizing socket path")
	manifestDigest := flag.String("manifest-digest", os.Getenv("PROJWM_NEXT_MANIFEST_DIGEST"), "managed environment manifest digest")
	flag.Parse()
	if *socketPath == "" {
		fmt.Fprintln(os.Stderr, "projwmctl: --socket-path or PROJWM_NEXT_SOCKET_PATH is required")
		os.Exit(2)
	}
	in, err := parseIntent(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmctl: %v\n", err)
		os.Exit(2)
	}
	resp, err := submit(*socketPath, *manifestPath, *manifestDigest, in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmctl: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(formatIntentResponse(resp))
}
