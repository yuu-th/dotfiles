package eventqueue

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuu-th/projwm-next/internal/event"
)

func newTestQueue(t *testing.T) *Queue {
	t.Helper()
	q, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return q
}

func TestQueue_EnqueueThenFlushDrainsInOrder(t *testing.T) {
	q := newTestQueue(t)
	for i, k := range []event.Kind{"windows-changed", "display-changed", "wake"} {
		if err := q.Enqueue(Record{Source: event.Source("window-manager"), Kind: k, HintID: hintIDFor(i)}); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}
	n, err := q.Len()
	if err != nil || n != 3 {
		t.Fatalf("Len after Enqueue = %d, %v; want 3", n, err)
	}

	var got []event.Kind
	drained, err := q.Flush(func(r Record) error {
		got = append(got, r.Kind)
		return nil
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if drained != 3 {
		t.Errorf("drained = %d, want 3", drained)
	}
	want := []event.Kind{"windows-changed", "display-changed", "wake"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Flush order[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if n, _ := q.Len(); n != 0 {
		t.Errorf("Len after successful Flush = %d, want 0", n)
	}
}

// Critical: if submit fails on record N, records 0..N-1 are deleted,
// record N stays so the next Flush retries it.
func TestQueue_FlushStopsAndPreservesFailingRecord(t *testing.T) {
	q := newTestQueue(t)
	for i, k := range []event.Kind{"k0", "k1-fail", "k2"} {
		if err := q.Enqueue(Record{Kind: k, HintID: hintIDFor(i)}); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	want := errors.New("boom")
	drained, err := q.Flush(func(r Record) error {
		if r.Kind == "k1-fail" {
			return want
		}
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("Flush err = %v, want wrapping %v", err, want)
	}
	if drained != 1 {
		t.Errorf("drained = %d, want 1 (only k0)", drained)
	}
	n, _ := q.Len()
	if n != 2 {
		t.Errorf("Len after partial Flush = %d, want 2 (k1-fail + k2)", n)
	}

	// Retry — this time succeed; queue should empty.
	drained2, err := q.Flush(func(r Record) error { return nil })
	if err != nil {
		t.Fatalf("retry Flush: %v", err)
	}
	if drained2 != 2 {
		t.Errorf("retry drained = %d, want 2", drained2)
	}
	if n, _ := q.Len(); n != 0 {
		t.Errorf("Len after retry Flush = %d, want 0", n)
	}
}

// Poison records (corrupt JSON) must not block the queue.
func TestQueue_FlushDropsPoisonRecord(t *testing.T) {
	q := newTestQueue(t)
	if err := os.WriteFile(filepath.Join(q.Dir(), "00000000000000000001-0000000001.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed poison: %v", err)
	}
	if err := q.Enqueue(Record{Kind: "windows-changed", HintID: "good"}); err != nil {
		t.Fatalf("Enqueue good: %v", err)
	}

	var got []event.Kind
	drained, err := q.Flush(func(r Record) error {
		got = append(got, r.Kind)
		return nil
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if drained != 1 {
		t.Errorf("drained = %d, want 1 (poison dropped, good submitted)", drained)
	}
	if len(got) != 1 || got[0] != "windows-changed" {
		t.Errorf("Flush got = %v, want [windows-changed]", got)
	}
	if n, _ := q.Len(); n != 0 {
		t.Errorf("Len after poison drop = %d, want 0", n)
	}
}

// Tmp files (in-flight Enqueue from a crashed writer) must not be
// picked up by Flush.
func TestQueue_FlushIgnoresTmpFiles(t *testing.T) {
	q := newTestQueue(t)
	if err := os.WriteFile(filepath.Join(q.Dir(), "in-flight.json.tmp"), []byte(`{"kind":"never"}`), 0o600); err != nil {
		t.Fatalf("seed tmp: %v", err)
	}
	called := false
	drained, err := q.Flush(func(r Record) error { called = true; return nil })
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if called || drained != 0 {
		t.Errorf("Flush picked up .tmp: called=%v drained=%d", called, drained)
	}
	if n, _ := q.Len(); n != 0 {
		t.Errorf("Len = %d, want 0 (tmp not counted)", n)
	}
}

func TestQueue_EnqueueGeneratesHintIDIfEmpty(t *testing.T) {
	q := newTestQueue(t)
	if err := q.Enqueue(Record{Kind: "windows-changed"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	var got Record
	_, err := q.Flush(func(r Record) error {
		got = r
		return nil
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got.HintID == "" {
		t.Error("HintID was not generated when caller left it empty")
	}
}

func TestQueue_NewRejectsEmptyDir(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("New(\"\") expected error")
	}
}

func hintIDFor(i int) string {
	return "hint-" + string(rune('a'+i))
}
