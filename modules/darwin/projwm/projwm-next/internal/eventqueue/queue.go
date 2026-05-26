// Package eventqueue provides on-disk durable queueing for sidecar events
// when projwmd is temporarily unreachable.
//
// Design (S20 Step 2): The projwmevent CLI is invoked as a 1-shot from
// launchd-managed sidecars (omniwmctl watch, wake watcher, etc.). When
// projwmd dies (launchd KeepAlive restarts it within ~10s via
// ThrottleInterval), a naive 1-shot CLI would lose the event because the
// dial fails and the process exits.
//
// To meet the "either daemon may die without losing events" robustness
// requirement, projwmevent uses this queue:
//
//  1. On startup, drain the queue: for each pending record, attempt to
//     submit it. Stop on first failure (likely daemon still down).
//  2. Attempt to submit the new event with retry+backoff (3 tries).
//  3. If all retries fail, enqueue the new event and exit 0 — the next
//     sidecar invocation will retry.
//
// Stale events are not the queue's concern: projwmd's controller already
// drops stale-epoch events (isStaleEvent). The queue's job is only to
// guarantee at-least-once delivery to the controller.
package eventqueue

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yuu-th/projwm-next/internal/event"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Record is the durable form of an EventHint queued to disk. Body is
// intentionally omitted: today's sidecars (windows-changed, display-changed,
// wake, layout-changed) submit body-less hints. If a body-bearing event
// kind appears later, extend Record + the file format.
type Record struct {
	HintID     string       `json:"hintId"`
	Source     event.Source `json:"source"`
	Kind       event.Kind   `json:"kind"`
	Epoch      w.Epoch      `json:"epoch,omitempty"`
	EnqueuedAt time.Time    `json:"enqueuedAt"`
}

// Queue is the file-based at-least-once delivery queue used by
// projwmevent.
type Queue struct {
	dir string
	seq atomic.Uint64
}

// New ensures dir exists with 0o700 perms (records may carry HintID +
// source/kind, which are not secrets but should not be world-readable).
func New(dir string) (*Queue, error) {
	if dir == "" {
		return nil, errors.New("eventqueue: dir required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("eventqueue: resolve dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("eventqueue: mkdir: %w", err)
	}
	if err := os.Chmod(abs, 0o700); err != nil {
		return nil, fmt.Errorf("eventqueue: chmod: %w", err)
	}
	return &Queue{dir: abs}, nil
}

// Dir returns the absolute queue directory path.
func (q *Queue) Dir() string { return q.dir }

// Enqueue writes the record to a fresh file. File naming is timestamp +
// monotonic seq so List() returns records in enqueue order even when
// many records are written in the same wall-clock millisecond. Uses
// tmp+rename so a partial write cannot be picked up by a concurrent
// Flush.
func (q *Queue) Enqueue(r Record) error {
	if r.EnqueuedAt.IsZero() {
		r.EnqueuedAt = time.Now().UTC()
	}
	if r.HintID == "" {
		r.HintID = fmt.Sprintf("queue-%d-%d", r.EnqueuedAt.UnixNano(), q.seq.Add(1))
	}
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("eventqueue: marshal: %w", err)
	}
	seq := q.seq.Add(1)
	name := fmt.Sprintf("%020d-%010d.json", r.EnqueuedAt.UnixNano(), seq)
	final := filepath.Join(q.dir, name)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("eventqueue: write tmp: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("eventqueue: commit: %w", err)
	}
	return nil
}

// List returns committed records in enqueue order. .tmp files (partial
// writes from a crashed process) are ignored. Caller-visible failures
// during read are skipped with a best-effort attitude — Flush callers
// should tolerate per-record errors rather than failing the whole batch.
func (q *Queue) List() ([]queuedFile, error) {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return nil, fmt.Errorf("eventqueue: readdir: %w", err)
	}
	var files []queuedFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		files = append(files, queuedFile{
			path: filepath.Join(q.dir, e.Name()),
			name: e.Name(),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}

type queuedFile struct {
	path string
	name string
}

// Flush replays each queued record through submit in order. Returns the
// first error encountered; the failing record stays on disk and is
// retried on the next Flush. Records preceding the failing one are
// removed once submit succeeds.
//
// Records that fail to decode are removed (poison-pill drop): we cannot
// retry an unparseable record forever, and the consequence of dropping
// is one missed event (vs. blocking the whole queue indefinitely).
func (q *Queue) Flush(submit func(Record) error) (drained int, err error) {
	files, listErr := q.List()
	if listErr != nil {
		return 0, listErr
	}
	for _, f := range files {
		data, readErr := os.ReadFile(f.path)
		if readErr != nil {
			if errors.Is(readErr, fs.ErrNotExist) {
				// concurrent removal — skip silently
				continue
			}
			return drained, fmt.Errorf("eventqueue: read %s: %w", f.name, readErr)
		}
		var r Record
		if jsonErr := json.Unmarshal(data, &r); jsonErr != nil {
			// poison pill: drop and continue
			_ = os.Remove(f.path)
			continue
		}
		if submitErr := submit(r); submitErr != nil {
			return drained, submitErr
		}
		if rmErr := os.Remove(f.path); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			return drained, fmt.Errorf("eventqueue: remove %s after submit: %w", f.name, rmErr)
		}
		drained++
	}
	return drained, nil
}

// Len reports how many committed records are pending.
func (q *Queue) Len() (int, error) {
	files, err := q.List()
	if err != nil {
		return 0, err
	}
	return len(files), nil
}
