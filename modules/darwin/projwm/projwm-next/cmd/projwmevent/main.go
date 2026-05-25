package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/yuu-th/projwm-next/internal/clientauth"
	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/ipc"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func buildHello(manifestDigest string) (ipc.Envelope, error) {
	return ipc.NewEnvelope(ipc.MsgHello, ipc.Hello{
		ProtocolVersion:    ipc.ProtocolVersion,
		StoreSchemaVersion: ipc.StoreSchemaVersion,
		ManifestDigest:     manifestDigest,
		ClientName:         "projwmevent",
		ClientVersion:      "0.1.0",
	})
}

func submitEvent(socketPath, manifestPath, manifestDigest, hintID string, source event.Source, kind event.Kind, epoch w.Epoch) (ipc.EventAck, error) {
	if err := clientauth.VerifyManagedSocket("projwmevent", socketPath, manifestPath, manifestDigest); err != nil {
		return ipc.EventAck{}, err
	}
	conn, err := net.DialTimeout("unix", socketPath, 3*time.Second)
	if err != nil {
		return ipc.EventAck{}, fmt.Errorf("projwmevent: dial %s: %w", socketPath, err)
	}
	defer conn.Close()

	hello, err := buildHello(manifestDigest)
	if err != nil {
		return ipc.EventAck{}, err
	}
	if err := ipc.WriteEnvelope(conn, hello); err != nil {
		return ipc.EventAck{}, err
	}
	welcome, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return ipc.EventAck{}, err
	}
	if welcome.Type == ipc.MsgReject {
		var rej ipc.Reject
		_ = json.Unmarshal(welcome.Payload, &rej)
		return ipc.EventAck{}, &rej.Error
	}
	if welcome.Type != ipc.MsgWelcome {
		return ipc.EventAck{}, fmt.Errorf("projwmevent: expected welcome, got %q", welcome.Type)
	}
	if hintID == "" {
		hintID = fmt.Sprintf("hint-%d", time.Now().UnixNano())
	}
	hint, err := ipc.NewEnvelope(ipc.MsgEventHint, ipc.EventHint{
		HintID: hintID,
		Source: source,
		Kind:   kind,
		Epoch:  epoch,
	})
	if err != nil {
		return ipc.EventAck{}, err
	}
	if err := ipc.WriteEnvelope(conn, hint); err != nil {
		return ipc.EventAck{}, err
	}
	rawAck, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return ipc.EventAck{}, err
	}
	if rawAck.Type != ipc.MsgEventAck {
		return ipc.EventAck{}, fmt.Errorf("projwmevent: expected event-ack, got %q", rawAck.Type)
	}
	var ack ipc.EventAck
	if err := json.Unmarshal(rawAck.Payload, &ack); err != nil {
		return ipc.EventAck{}, fmt.Errorf("projwmevent: decode event ack: %w", err)
	}
	if ack.Error != nil {
		return ack, ack.Error
	}
	return ack, nil
}

func main() {
	socketPath := flag.String("socket-path", os.Getenv("PROJWM_NEXT_SOCKET_PATH"), "projwmd Unix socket path")
	manifestPath := flag.String("managed-environment", os.Getenv("PROJWM_NEXT_MANAGED_ENVIRONMENT"), "managed environment manifest path authorizing socket path")
	manifestDigest := flag.String("manifest-digest", os.Getenv("PROJWM_NEXT_MANIFEST_DIGEST"), "managed environment manifest digest")
	source := flag.String("source", "", "event source: user, window-manager, system, timer")
	kind := flag.String("kind", "", "event kind")
	hintID := flag.String("hint-id", "", "optional event hint id")
	epoch := flag.Uint64("epoch", 0, "optional observation-time controller epoch")
	flag.Parse()
	if *socketPath == "" {
		fmt.Fprintln(os.Stderr, "projwmevent: --socket-path or PROJWM_NEXT_SOCKET_PATH is required")
		os.Exit(2)
	}
	if *source == "" || *kind == "" {
		fmt.Fprintln(os.Stderr, "projwmevent: --source and --kind are required")
		os.Exit(2)
	}
	ack, err := submitEvent(*socketPath, *manifestPath, *manifestDigest, *hintID, event.Source(*source), event.Kind(*kind), w.Epoch(*epoch))
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmevent: %v\n", err)
		os.Exit(1)
	}
	if ack.AcceptedTransaction != nil && ack.CommittedGeneration != nil {
		fmt.Printf("ok hint=%s acceptedTransaction=%s committedGeneration=%s dropped=%t\n", ack.HintID, *ack.AcceptedTransaction, *ack.CommittedGeneration, ack.Dropped)
		return
	}
	if ack.AcceptedTransaction != nil {
		fmt.Printf("ok hint=%s acceptedTransaction=%s dropped=%t\n", ack.HintID, *ack.AcceptedTransaction, ack.Dropped)
		return
	}
	fmt.Printf("ok hint=%s dropped=%t\n", ack.HintID, ack.Dropped)
}
