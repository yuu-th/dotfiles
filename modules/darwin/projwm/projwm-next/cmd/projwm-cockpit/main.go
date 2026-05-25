// projwm-cockpit is the projwm-next user-facing TUI cockpit.
//
// Requirements §8-§10 / design v3 §3.2. The TUI itself lives in the
// internal `tui` subpackage (charmbracelet/bubbletea); main is the
// minimal entry point: flag parsing, daemon subscribe goroutine, and
// program lifecycle.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yuu-th/projwm-next/cmd/projwm-cockpit/tui"
	"github.com/yuu-th/projwm-next/internal/cockpitclient"
	"github.com/yuu-th/projwm-next/internal/ipc"
)

func main() {
	var (
		socketPath     = flag.String("socket-path", envFallback("PROJWM_NEXT_SOCKET_PATH"), "projwmd Unix socket path")
		manifestPath   = flag.String("managed-environment", envFallback("PROJWM_NEXT_MANAGED_ENVIRONMENT"), "managed environment manifest path")
		manifestDigest = flag.String("manifest-digest", envFallback("PROJWM_NEXT_MANIFEST_DIGEST"), "manifest digest for handshake")
		storeDir       = flag.String("store-dir", envFallback("PROJWM_NEXT_STORE_DIR"), "PersistentStore directory (used for fallback rendering)")
		refresh        = flag.Duration("refresh", 2*time.Second, "periodic refresh interval")
		oneShot        = flag.Bool("one-shot", false, "render once and exit (test/debug)")
	)
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := cockpitclient.New(cockpitclient.Config{
		SocketPath:     *socketPath,
		ManifestPath:   *manifestPath,
		ManifestDigest: *manifestDigest,
	})

	cfg := tui.Config{
		Client:          client,
		StoreDir:        *storeDir,
		ManifestPath:    *manifestPath,
		RefreshInterval: int(refresh.Seconds()),
	}

	if *oneShot {
		// One-shot mode: render a single frame, then exit. Useful for
		// tests + manual smoke checks. Builds the model directly and
		// calls View() without entering bubbletea's event loop.
		m := tui.New(cfg)
		// Run Init's snapshot loader synchronously so the first frame
		// has data.
		if msg := runInitOnce(m); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(tui.Model)
		}
		fmt.Print(m.View())
		return
	}

	// Start the subscribe goroutine — it owns the long-lived
	// connection to the daemon and feeds tea via a channel. The
	// channel buffer absorbs short bursts.
	subscribeCh := make(chan ipc.SubscriptionPush, 32)
	go runSubscriber(ctx, client, subscribeCh)
	cfg.SubscribeCh = subscribeCh

	m := tui.New(cfg)
	p := tea.NewProgram(m,
		tea.WithContext(ctx),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "projwm-cockpit: %v\n", err)
		os.Exit(1)
	}
}

// runSubscriber runs the daemon subscribe loop. It reconnects every
// 5s on failure. Each push goes onto ch with a non-blocking send (so
// a slow TUI doesn't backpressure the daemon).
func runSubscriber(ctx context.Context, client *cockpitclient.Client, ch chan<- ipc.SubscriptionPush) {
	for {
		if ctx.Err() != nil {
			return
		}
		if !client.Reachable() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		stream, err := client.Subscribe(ctx, []string{"*"})
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		for {
			push, err := stream.Next()
			if err != nil {
				_ = stream.Close()
				break
			}
			select {
			case ch <- push:
			default:
				// Drop on overflow — tea will refresh on the next
				// tick anyway.
			}
		}
	}
}

// runInitOnce runs Model.Init() in foreground for one-shot mode, then
// drains the resulting cmds for one snapshotMsg so View() has data.
// Returns the snapshot message (or nil if Init produced nothing).
func runInitOnce(m tui.Model) tea.Msg {
	cmd := m.Init()
	if cmd == nil {
		return nil
	}
	return cmd()
}

func envFallback(key string) string { return os.Getenv(key) }
