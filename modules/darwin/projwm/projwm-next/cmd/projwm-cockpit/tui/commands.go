package tui

import (
	"context"
	"encoding/json"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yuu-th/projwm-next/internal/cockpitclient"
	"github.com/yuu-th/projwm-next/internal/cockpitsnap"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/ipc"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// loadSnapshotCmd fetches the world snapshot. Prefer daemon
// QueryWorld, fall back to store-direct read. Result is wrapped in
// snapshotMsg{Snap, Err}.
func loadSnapshotCmd(client *cockpitclient.Client, storeDir, manifestPath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if client != nil && client.Reachable() {
			resp, err := client.QueryWorld(ctx)
			if err == nil {
				var snap cockpitsnap.Snapshot
				if err := json.Unmarshal(resp, &snap); err == nil {
					snap.Source = "daemon"
					return snapshotMsg{Snap: snap}
				}
			}
		}
		if storeDir == "" {
			return snapshotMsg{Err: errNoSource}
		}
		snap, err := cockpitsnap.LoadFromStore(ctx, storeDir, manifestPath)
		if err != nil {
			return snapshotMsg{Err: err}
		}
		return snapshotMsg{Snap: snap}
	}
}

// listenSubscribeCmd reads one push from the subscribe channel and
// returns a subscriptionMsg. The channel is fed by a long-running
// goroutine inside Run() (see main.go); the channel decouples
// concurrent reads from tea's single-msg-at-a-time consumption.
func listenSubscribeCmd(ch <-chan ipc.SubscriptionPush) tea.Cmd {
	return func() tea.Msg {
		push, ok := <-ch
		if !ok {
			return subscriptionMsg{Err: errChanClosed}
		}
		return subscriptionMsg{Push: push}
	}
}

// submitIntentCmd submits an intent in the background and returns an
// intentSubmittedMsg with the outcome.
func submitIntentCmd(client *cockpitclient.Client, in intent.Intent) tea.Cmd {
	kind := string(in.Kind())
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := client.SubmitIntent(ctx, in)
		return intentSubmittedMsg{Kind: kind, Err: err}
	}
}

// setVisibilityCmd is a convenience wrapper around
// SetCockpitVisibility intent. Used at K1.5 (force-show on card-added)
// and on hide/Ctrl+C/Esc-at-top.
func setVisibilityCmd(client *cockpitclient.Client, v w.CockpitVisibility) tea.Cmd {
	return submitIntentCmd(client, intent.SetCockpitVisibility{Visibility: v})
}

// tickEveryCmd schedules a tickMsg every d.
func tickEveryCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type sentinelErr string

func (e sentinelErr) Error() string { return string(e) }

const (
	errNoSource   sentinelErr = "no snapshot source (daemon unreachable, store-dir empty)"
	errChanClosed sentinelErr = "subscribe channel closed"
)
