package main

import (
	"errors"
	"testing"
	"time"

	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/eventqueue"
	"github.com/yuu-th/projwm-next/internal/ipc"
)

func resetGlobals(t *testing.T) {
	t.Helper()
	orig := submitOnceFn
	origBackoff := initialBackoff
	t.Cleanup(func() {
		submitOnceFn = orig
		initialBackoff = origBackoff
	})
	initialBackoff = 1 * time.Millisecond // speed up retries in tests
}

func TestSubmitWithRetry_SucceedsAfterTransientFailures(t *testing.T) {
	resetGlobals(t)
	calls := 0
	submitOnceFn = func(_, _, _ string, r eventqueue.Record) (ipc.EventAck, error) {
		calls++
		if calls < 3 {
			return ipc.EventAck{}, errors.New("dial: connection refused")
		}
		return ipc.EventAck{HintID: r.HintID}, nil
	}
	ack, err := submitWithRetry("/sock", "/m", "deadbeef", eventqueue.Record{HintID: "h"})
	if err != nil {
		t.Fatalf("submitWithRetry: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (two failures + one success)", calls)
	}
	if ack.HintID != "h" {
		t.Errorf("ack.HintID = %q, want %q", ack.HintID, "h")
	}
}

func TestSubmitWithRetry_StopsAfterMaxAttempts(t *testing.T) {
	resetGlobals(t)
	calls := 0
	wantErr := errors.New("dial: down")
	submitOnceFn = func(_, _, _ string, _ eventqueue.Record) (ipc.EventAck, error) {
		calls++
		return ipc.EventAck{}, wantErr
	}
	_, err := submitWithRetry("/sock", "/m", "deadbeef", eventqueue.Record{HintID: "h"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wrap of %v", err, wantErr)
	}
	if calls != retryAttempts {
		t.Errorf("calls = %d, want %d", calls, retryAttempts)
	}
}

// SSOT robustness contract (S20 G3): when projwmd is unreachable, the
// new event must be persisted on disk so the next sidecar invocation
// can replay it. Exit must be success so launchd does not enter the
// throttle-restart loop.
func TestRunMain_QueuesEventWhenSubmitFails(t *testing.T) {
	resetGlobals(t)
	submitOnceFn = func(_, _, _ string, _ eventqueue.Record) (ipc.EventAck, error) {
		return ipc.EventAck{}, errors.New("dial: down")
	}
	queueDir := t.TempDir()
	if err := runMain("/sock", "/m", "deadbeef", queueDir, "hint-1", event.Source("window-manager"), event.Kind("windows-changed"), 0); err != nil {
		t.Fatalf("runMain (expected exit 0 even when submit fails): %v", err)
	}
	q, err := eventqueue.New(queueDir)
	if err != nil {
		t.Fatalf("New queue: %v", err)
	}
	n, err := q.Len()
	if err != nil {
		t.Fatalf("Len: %v", err)
	}
	if n != 1 {
		t.Errorf("queue.Len = %d, want 1 (the failed event must be persisted)", n)
	}
}

// SSOT robustness contract: on next invocation when projwmd is back,
// the queued event is replayed before the new event.
func TestRunMain_DrainsQueueOnSuccess(t *testing.T) {
	resetGlobals(t)
	queueDir := t.TempDir()
	// Seed queue with 2 stranded records.
	q, err := eventqueue.New(queueDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := q.Enqueue(eventqueue.Record{Source: "window-manager", Kind: "windows-changed", HintID: hintID(i)}); err != nil {
			t.Fatalf("seed enqueue: %v", err)
		}
	}

	var got []string
	submitOnceFn = func(_, _, _ string, r eventqueue.Record) (ipc.EventAck, error) {
		got = append(got, r.HintID)
		return ipc.EventAck{HintID: r.HintID}, nil
	}

	if err := runMain("/sock", "/m", "deadbeef", queueDir, "fresh", event.Source("window-manager"), event.Kind("display-changed"), 0); err != nil {
		t.Fatalf("runMain: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("submitOnceFn called %d times, want 3 (2 drained + 1 new): got=%v", len(got), got)
	}
	if got[0] != "h0" || got[1] != "h1" || got[2] != "fresh" {
		t.Errorf("submit order = %v, want [h0 h1 fresh]", got)
	}
	if n, _ := q.Len(); n != 0 {
		t.Errorf("queue.Len after drain = %d, want 0", n)
	}
}

// If drain hits a failure mid-batch, surviving records stay queued and
// the new event is also queued (rather than skipping ahead of pending
// records — preserves enqueue order across daemon downtime).
func TestRunMain_PartialDrainPreservesPendingAndQueuesNew(t *testing.T) {
	resetGlobals(t)
	queueDir := t.TempDir()
	q, err := eventqueue.New(queueDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := q.Enqueue(eventqueue.Record{Source: "window-manager", Kind: "windows-changed", HintID: hintID(i)}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	calls := 0
	submitOnceFn = func(_, _, _ string, r eventqueue.Record) (ipc.EventAck, error) {
		calls++
		if r.HintID == "h1" || r.HintID == "fresh" {
			return ipc.EventAck{}, errors.New("dial: down mid-stream")
		}
		return ipc.EventAck{HintID: r.HintID}, nil
	}

	if err := runMain("/sock", "/m", "deadbeef", queueDir, "fresh", event.Source("window-manager"), event.Kind("display-changed"), 0); err != nil {
		t.Fatalf("runMain: %v", err)
	}
	n, _ := q.Len()
	if n != 3 {
		t.Errorf("queue.Len = %d, want 3 (h1, h2 stranded + fresh)", n)
	}
}

func hintID(i int) string {
	return "h" + string(rune('0'+i))
}
