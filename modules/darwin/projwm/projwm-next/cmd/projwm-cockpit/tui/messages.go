package tui

import (
	"time"

	"github.com/yuu-th/projwm-next/internal/cockpitsnap"
	"github.com/yuu-th/projwm-next/internal/ipc"
)

// snapshotMsg arrives whenever the loader cmd returns a fresh world
// snapshot. Source = "daemon" or "store".
type snapshotMsg struct {
	Snap cockpitsnap.Snapshot
	Err  error
}

// subscriptionMsg arrives for each daemon push event. The TUI looks at
// Push.Kind to decide what to do (force-show on "card-added", refresh
// snapshot on "*", etc.).
type subscriptionMsg struct {
	Push ipc.SubscriptionPush
	Err  error
}

// intentSubmittedMsg signals that a submitted intent finished. Err is
// non-nil on rejection / dial failure.
type intentSubmittedMsg struct {
	Kind string
	Err  error
}

// statusMsg sets the status line.
type statusMsg struct{ Text string }

// tickMsg drives the 1s relative-time refresh tick (cards' CreatedAt
// display). Carries the wall clock time so the View can compute "Xs
// ago" labels without snapshotting time again.
type tickMsg time.Time

// promptSubmitMsg fires when the user presses Enter inside a prompt.
type promptSubmitMsg struct {
	Value string
}

// quitMsg asks the bubbletea program to exit (after hideCockpit).
type quitMsg struct{}
