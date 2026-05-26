// projwmevent is a 1-shot CLI invoked by launchd-managed sidecars
// (omniwmctl watch, wake-watcher, etc.) to push observed events into
// projwmd's IPC socket.
//
// S20 Step 2 robustness: when projwmd is down (e.g., crashed and
// launchd is still restarting it within ThrottleInterval=10s), a naive
// 1-shot would lose the event. To meet the "either daemon may die
// without losing events" requirement, projwmevent:
//
//  1. Drains the on-disk queue (eventqueue.Queue) on startup.
//  2. Attempts to submit the new event with retry+backoff (3 tries).
//  3. On total failure, enqueues the new event and exits 0 — the next
//     sidecar invocation will retry.
//
// Stale-epoch events are dropped by projwmd controller.isStaleEvent, so
// queue durability is safe even across long downtime.
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
	"github.com/yuu-th/projwm-next/internal/eventqueue"
	"github.com/yuu-th/projwm-next/internal/ipc"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// retryAttempts is the number of submit tries before falling through to
// queue persistence. Backoff: 100ms, 200ms, 400ms (~700ms total). Short
// enough that the user does not perceive launchd delay; long enough to
// ride through a fast projwmd restart.
const retryAttempts = 3

var (
	initialBackoff = 100 * time.Millisecond

	// submitOnceFn is the indirection seam tests use to swap in a fake
	// transport. Production points it at the real Unix-socket submit.
	submitOnceFn = submitOnce
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

// submitOnce performs the full handshake + EventHint submit + ack read.
// Returns the ack on success. All transient failures (dial / read / write)
// are returned as errors so caller can retry.
func submitOnce(socketPath, manifestPath, manifestDigest string, r eventqueue.Record) (ipc.EventAck, error) {
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
	hint, err := ipc.NewEnvelope(ipc.MsgEventHint, ipc.EventHint{
		HintID: r.HintID,
		Source: r.Source,
		Kind:   r.Kind,
		Epoch:  r.Epoch,
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

// submitWithRetry tries submitOnce up to retryAttempts times with
// exponential backoff. Returns the last error if all attempts fail.
// All errors (including auth) are retried — auth failures will then
// queue and surface via stderr, giving the operator visibility without
// burning launchd ThrottleInterval restarts on a fast-failing 1-shot.
func submitWithRetry(socketPath, manifestPath, manifestDigest string, r eventqueue.Record) (ipc.EventAck, error) {
	var lastErr error
	backoff := initialBackoff
	for i := 0; i < retryAttempts; i++ {
		ack, err := submitOnceFn(socketPath, manifestPath, manifestDigest, r)
		if err == nil {
			return ack, nil
		}
		lastErr = err
		if i < retryAttempts-1 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return ipc.EventAck{}, lastErr
}

func runMain(socketPath, manifestPath, manifestDigest, queueDir, hintID string, source event.Source, kind event.Kind, epoch w.Epoch) error {
	queue, err := eventqueue.New(queueDir)
	if err != nil {
		return fmt.Errorf("projwmevent: queue init: %w", err)
	}

	// 1. Drain any queued records first. If draining hits an error, we
	//    still try to deliver the new event — if projwmd is back up, the
	//    new event has a chance; if not, the new event will also queue.
	drained, drainErr := queue.Flush(func(r eventqueue.Record) error {
		_, e := submitWithRetry(socketPath, manifestPath, manifestDigest, r)
		return e
	})
	if drained > 0 {
		fmt.Fprintf(os.Stderr, "projwmevent: drained %d queued events\n", drained)
	}

	// 2. Submit the new event.
	newRecord := eventqueue.Record{
		HintID:     hintID,
		Source:     source,
		Kind:       kind,
		Epoch:      epoch,
		EnqueuedAt: time.Now().UTC(),
	}
	if newRecord.HintID == "" {
		newRecord.HintID = fmt.Sprintf("hint-%d", time.Now().UnixNano())
	}
	ack, err := submitWithRetry(socketPath, manifestPath, manifestDigest, newRecord)
	if err == nil {
		// 3a. Success — print ack details for launchd log readability.
		printAck(ack)
		return nil
	}
	// 3b. Submit failed; enqueue and exit 0 so the launchd sidecar does
	//     not enter the throttle-restart loop. Surfaces a warning on
	//     stderr so the operator can grep logs.
	fmt.Fprintf(os.Stderr, "projwmevent: submit failed (%v) — enqueued for retry; drain-err=%v\n", err, drainErr)
	if enqErr := queue.Enqueue(newRecord); enqErr != nil {
		return fmt.Errorf("projwmevent: enqueue: %w (after submit fail: %v)", enqErr, err)
	}
	fmt.Printf("queued hint=%s kind=%s source=%s\n", newRecord.HintID, newRecord.Kind, newRecord.Source)
	return nil
}

func printAck(ack ipc.EventAck) {
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

func defaultQueueDir() string {
	if v := os.Getenv("PROJWM_NEXT_EVENT_QUEUE_DIR"); v != "" {
		return v
	}
	if state := os.Getenv("PROJWM_NEXT_STATE_DIR"); state != "" {
		return state + "/event-queue"
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home + "/.local/state/projwm-next/event-queue"
	}
	return "./projwm-next-event-queue"
}

func main() {
	socketPath := flag.String("socket-path", os.Getenv("PROJWM_NEXT_SOCKET_PATH"), "projwmd Unix socket path")
	manifestPath := flag.String("managed-environment", os.Getenv("PROJWM_NEXT_MANAGED_ENVIRONMENT"), "managed environment manifest path authorizing socket path")
	manifestDigest := flag.String("manifest-digest", os.Getenv("PROJWM_NEXT_MANIFEST_DIGEST"), "managed environment manifest digest")
	queueDir := flag.String("queue-dir", defaultQueueDir(), "directory for on-disk event queue (used when projwmd is unreachable)")
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
	if err := runMain(*socketPath, *manifestPath, *manifestDigest, *queueDir, *hintID, event.Source(*source), event.Kind(*kind), w.Epoch(*epoch)); err != nil {
		fmt.Fprintf(os.Stderr, "projwmevent: %v\n", err)
		os.Exit(1)
	}
}
