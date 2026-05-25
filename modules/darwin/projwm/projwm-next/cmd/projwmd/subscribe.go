package main

import (
	"context"
	"encoding/json"
	"net"

	"github.com/yuu-th/projwm-next/internal/controller"
	"github.com/yuu-th/projwm-next/internal/ipc"
)

// connHub is the process-wide subscription manager. Wired into the
// controller via attachHubToController so commit completions trigger
// hub.Broadcast.
var connHub = NewConnHub()

// handleSubscribeEnvelope processes a MsgSubscribe envelope:
//  1. parse the SubscribeRequest payload
//  2. register a Subscriber with the kinds
//  3. ack
//  4. block on hub.Pump until the connection errors or a cancel arrives
func handleSubscribeEnvelope(ctx context.Context, conn net.Conn, ctrl *controller.Controller, env ipc.Envelope) {
	var req ipc.SubscribeRequest
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		writeSubscribeAck(conn, "", ipc.ErrProtocolMismatch, err.Error())
		return
	}
	sub := connHub.Register(req.Kinds)
	defer connHub.Remove(sub)
	writeSubscribeAck(conn, req.RequestID, "", "")

	// Send one snapshot of current ActiveCards so the client doesn't
	// need to issue a separate QueryCards round trip just to populate
	// initial state.
	if sub.Kinds["card-added"] || sub.Kinds["*"] {
		state := ctrl.State()
		for _, card := range state.Meta.ActiveCards {
			connHub.Broadcast("card-added", card, "")
		}
	}

	// Pump in this goroutine until either the connection breaks (read
	// loop noticing EOF) or the subscriber is removed.
	go func() {
		for {
			env, err := ipc.ReadEnvelope(conn)
			if err != nil {
				connHub.Remove(sub)
				return
			}
			if env.Type == ipc.MsgSubscriptionCancel {
				connHub.Remove(sub)
				return
			}
			// Any other envelope is ignored on a subscribe stream; the
			// client should open a separate connection for queries.
		}
	}()
	connHub.Pump(conn, sub)
}

func writeSubscribeAck(conn net.Conn, id string, code ipc.ErrorCode, msg string) {
	ack := ipc.SubscribeAck{RequestID: id}
	if code != "" {
		ack.Error = &ipc.Error{Code: code, Message: msg}
	}
	env, _ := ipc.NewEnvelope(ipc.MsgSubscribeAck, ack)
	_ = ipc.WriteEnvelope(conn, env)
}
