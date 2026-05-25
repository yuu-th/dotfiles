package main

import (
	"encoding/json"
	"net"
	"sync"

	"github.com/yuu-th/projwm-next/internal/ipc"
)

// ConnHub is the daemon's broadcast manager for MsgSubscribe subscribers.
//
// Lifecycle:
//   - handleSubscribeEnvelope registers a Subscriber per connection.
//   - Controller.commit completion calls hub.Broadcast on relevant kinds.
//   - Subscriber goroutines fan-out the push to their socket.
//   - Connection close (peer disconnect or MsgSubscriptionCancel) calls
//     hub.Remove which closes the per-subscriber channel.
//
// Backpressure: each subscriber owns a buffered channel (size 64). If
// the channel is full when Broadcast tries to enqueue, the push for
// that subscriber is dropped (logged) so a stuck subscriber cannot
// block the controller commit pipeline.
type ConnHub struct {
	mu          sync.Mutex
	subscribers map[*Subscriber]struct{}
}

// Subscriber is one open MsgSubscribe stream.
type Subscriber struct {
	Kinds map[string]bool
	out   chan ipc.SubscriptionPush
	done  chan struct{}
}

// NewConnHub returns an empty hub.
func NewConnHub() *ConnHub {
	return &ConnHub{subscribers: map[*Subscriber]struct{}{}}
}

// Register adds a Subscriber to the hub. Caller owns the lifetime of the
// returned channel (must drain it until done is closed).
func (h *ConnHub) Register(kinds []string) *Subscriber {
	s := &Subscriber{
		Kinds: map[string]bool{},
		out:   make(chan ipc.SubscriptionPush, 64),
		done:  make(chan struct{}),
	}
	for _, k := range kinds {
		s.Kinds[k] = true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribers[s] = struct{}{}
	return s
}

// Remove de-registers and closes the subscriber. Safe to call twice.
func (h *ConnHub) Remove(s *Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subscribers[s]; !ok {
		return
	}
	delete(h.subscribers, s)
	close(s.done)
	close(s.out)
}

// Broadcast enqueues a push for every subscriber that asked for kind.
// Drops on full channel to enforce backpressure isolation.
func (h *ConnHub) Broadcast(kind string, body any, generation string) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return
	}
	push := ipc.SubscriptionPush{
		Kind:       kind,
		Body:       bodyBytes,
		Generation: generation,
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subscribers {
		if !s.Kinds[kind] && !s.Kinds["*"] {
			continue
		}
		select {
		case s.out <- push:
		default:
			// Subscriber full → drop. Cockpit reconnects refresh state via Query.
		}
	}
}

// Pump streams pushes from one Subscriber to its socket. Returns when the
// subscriber is removed or the connection errors out.
func (h *ConnHub) Pump(conn net.Conn, s *Subscriber) {
	for {
		select {
		case push, ok := <-s.out:
			if !ok {
				return
			}
			env, err := ipc.NewEnvelope(ipc.MsgSubscriptionPush, push)
			if err != nil {
				return
			}
			if err := ipc.WriteEnvelope(conn, env); err != nil {
				return
			}
		case <-s.done:
			return
		}
	}
}
