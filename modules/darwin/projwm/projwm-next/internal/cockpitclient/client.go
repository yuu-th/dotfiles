// Package cockpitclient is the projwm-cockpit IPC client. Lives in
// internal/ so the bubbletea TUI package can import it without
// circular deps. Mirrors the (former) cmd/projwm-cockpit/client.go.
package cockpitclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/yuu-th/projwm-next/internal/clientauth"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/ipc"
)

// Config carries the IPC handshake parameters.
type Config struct {
	SocketPath     string
	ManifestPath   string
	ManifestDigest string
}

// Client wraps a daemon connection for query + subscribe + intent.
type Client struct {
	cfg Config
}

func New(cfg Config) *Client { return &Client{cfg: cfg} }

// Reachable returns true if a fresh dial + handshake succeeds.
func (c *Client) Reachable() bool {
	if c.cfg.SocketPath == "" || c.cfg.ManifestPath == "" || c.cfg.ManifestDigest == "" {
		return false
	}
	if err := clientauth.VerifyManagedSocket("projwm-cockpit", c.cfg.SocketPath, c.cfg.ManifestPath, c.cfg.ManifestDigest); err != nil {
		return false
	}
	conn, err := net.DialTimeout("unix", c.cfg.SocketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (c *Client) dial(ctx context.Context) (net.Conn, error) {
	if err := clientauth.VerifyManagedSocket("projwm-cockpit", c.cfg.SocketPath, c.cfg.ManifestPath, c.cfg.ManifestDigest); err != nil {
		return nil, err
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", c.cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	hello, err := ipc.NewEnvelope(ipc.MsgHello, ipc.Hello{
		ProtocolVersion:    ipc.ProtocolVersion,
		StoreSchemaVersion: ipc.StoreSchemaVersion,
		ManifestDigest:     c.cfg.ManifestDigest,
		ClientName:         "projwm-cockpit",
		ClientVersion:      "0.1.0",
	})
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := ipc.WriteEnvelope(conn, hello); err != nil {
		conn.Close()
		return nil, err
	}
	welcome, err := ipc.ReadEnvelope(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if welcome.Type == ipc.MsgReject {
		conn.Close()
		var rej ipc.Reject
		_ = json.Unmarshal(welcome.Payload, &rej)
		return nil, &rej.Error
	}
	if welcome.Type != ipc.MsgWelcome {
		conn.Close()
		return nil, fmt.Errorf("expected welcome, got %q", welcome.Type)
	}
	return conn, nil
}

// QueryWorld returns the raw JSON body of QueryWorld.
func (c *Client) QueryWorld(ctx context.Context) ([]byte, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	req, err := ipc.NewEnvelope(ipc.MsgQueryRequest, ipc.QueryRequest{
		RequestID: fmt.Sprintf("cockpit-%d", time.Now().UnixNano()),
		Kind:      ipc.QueryWorld,
	})
	if err != nil {
		return nil, err
	}
	if err := ipc.WriteEnvelope(conn, req); err != nil {
		return nil, err
	}
	respEnv, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return nil, err
	}
	if respEnv.Type != ipc.MsgQueryResponse {
		return nil, fmt.Errorf("expected query-response, got %q", respEnv.Type)
	}
	var resp ipc.QueryResponse
	if err := json.Unmarshal(respEnv.Payload, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Body, nil
}

// SubscribeStream is a thin wrapper over the daemon's subscribe protocol.
type SubscribeStream struct {
	conn net.Conn
}

func (s *SubscribeStream) Close() error {
	if s.conn == nil {
		return nil
	}
	_ = ipc.WriteEnvelope(s.conn, mustEnvelope(ipc.MsgSubscriptionCancel, ipc.SubscriptionCancel{RequestID: "cockpit"}))
	return s.conn.Close()
}

func (s *SubscribeStream) Next() (ipc.SubscriptionPush, error) {
	env, err := ipc.ReadEnvelope(s.conn)
	if err != nil {
		return ipc.SubscriptionPush{}, err
	}
	if env.Type != ipc.MsgSubscriptionPush {
		return ipc.SubscriptionPush{}, fmt.Errorf("expected subscription-push, got %q", env.Type)
	}
	var push ipc.SubscriptionPush
	if err := json.Unmarshal(env.Payload, &push); err != nil {
		return ipc.SubscriptionPush{}, err
	}
	return push, nil
}

// Subscribe opens a MsgSubscribe stream for the given event kinds.
func (c *Client) Subscribe(ctx context.Context, kinds []string) (*SubscribeStream, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	req, err := ipc.NewEnvelope(ipc.MsgSubscribe, ipc.SubscribeRequest{
		RequestID: "cockpit",
		Kinds:     kinds,
	})
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := ipc.WriteEnvelope(conn, req); err != nil {
		conn.Close()
		return nil, err
	}
	respEnv, err := ipc.ReadEnvelope(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if respEnv.Type != ipc.MsgSubscribeAck {
		conn.Close()
		return nil, fmt.Errorf("expected subscribe-ack, got %q", respEnv.Type)
	}
	var ack ipc.SubscribeAck
	if err := json.Unmarshal(respEnv.Payload, &ack); err != nil {
		conn.Close()
		return nil, err
	}
	if ack.Error != nil {
		conn.Close()
		return nil, ack.Error
	}
	return &SubscribeStream{conn: conn}, nil
}

// SubmitIntent dials projwmd and submits an intent.
func (c *Client) SubmitIntent(ctx context.Context, in intent.Intent) error {
	if !c.Reachable() {
		return fmt.Errorf("daemon unreachable")
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := ipc.NewEnvelope(ipc.MsgIntentRequest, ipc.IntentRequest{
		RequestID: fmt.Sprintf("cockpit-%d", time.Now().UnixNano()),
		Kind:      in.Kind(),
		Payload:   body,
	})
	if err != nil {
		return err
	}
	if err := ipc.WriteEnvelope(conn, req); err != nil {
		return err
	}
	respEnv, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return err
	}
	if respEnv.Type != ipc.MsgIntentResponse {
		return fmt.Errorf("unexpected response type %s", respEnv.Type)
	}
	var resp ipc.IntentResponse
	if err := json.Unmarshal(respEnv.Payload, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return resp.Error
	}
	return nil
}

func mustEnvelope(t ipc.MessageType, payload any) ipc.Envelope {
	env, err := ipc.NewEnvelope(t, payload)
	if err != nil {
		panic(err)
	}
	return env
}
