package main

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

// daemonClient is the projwm CLI's IPC client to projwmd.
//
// In Phase 1 it exposes:
//   - SubmitIntent (one-shot intent → response): for writable subcommands
//
// Phase 3 extends this with QueryWorld / Subscribe for cockpit live updates.
type daemonClient struct {
	socketPath     string
	manifestPath   string
	manifestDigest string
	dialTimeout    time.Duration
}

func newDaemonClient(gf globalFlags) *daemonClient {
	return &daemonClient{
		socketPath:     gf.socketPath,
		manifestPath:   gf.manifestPath,
		manifestDigest: gf.manifestDigest,
		dialTimeout:    3 * time.Second,
	}
}

// reachable returns true if the daemon socket exists, the manifest binds
// to it, and a TCP-like Dial succeeds within dialTimeout.
//
// Used by status / profile-list / archive-list to decide whether to
// query the daemon (preferred) or fall back to store_reader.
func (c *daemonClient) reachable() bool {
	if c.socketPath == "" || c.manifestPath == "" || c.manifestDigest == "" {
		return false
	}
	if err := clientauth.VerifyManagedSocket("projwm", c.socketPath, c.manifestPath, c.manifestDigest); err != nil {
		return false
	}
	conn, err := net.DialTimeout("unix", c.socketPath, c.dialTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Query dials projwmd, performs the handshake, and issues a read-only
// QueryRequest. The returned body is JSON-RawMessage that the caller can
// decode against the per-kind schema. Returns an error if the daemon is
// unreachable or the response carries an Error.
func (c *daemonClient) Query(ctx context.Context, kind ipc.QueryKind, traceID string) (ipc.QueryResponse, error) {
	conn, err := c.dialAndHandshake(ctx)
	if err != nil {
		return ipc.QueryResponse{}, err
	}
	defer conn.Close()
	req, err := ipc.NewEnvelope(ipc.MsgQueryRequest, ipc.QueryRequest{
		RequestID: fmt.Sprintf("projwm-q-%d", time.Now().UnixNano()),
		Kind:      kind,
		TraceID:   traceID,
	})
	if err != nil {
		return ipc.QueryResponse{}, err
	}
	if err := ipc.WriteEnvelope(conn, req); err != nil {
		return ipc.QueryResponse{}, err
	}
	respEnv, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return ipc.QueryResponse{}, err
	}
	if respEnv.Type != ipc.MsgQueryResponse {
		return ipc.QueryResponse{}, fmt.Errorf("expected query-response, got %q", respEnv.Type)
	}
	var resp ipc.QueryResponse
	if err := json.Unmarshal(respEnv.Payload, &resp); err != nil {
		return ipc.QueryResponse{}, err
	}
	if resp.Error != nil {
		return resp, resp.Error
	}
	return resp, nil
}

// dialAndHandshake performs the manifest-verification + Unix-socket dial
// + Hello/Welcome flow used by both Query and SubmitIntent.
func (c *daemonClient) dialAndHandshake(ctx context.Context) (net.Conn, error) {
	if c.socketPath == "" {
		return nil, fmt.Errorf("daemon socket path is required (--socket-path or PROJWM_NEXT_SOCKET_PATH)")
	}
	if c.manifestPath == "" {
		return nil, fmt.Errorf("managed-environment path is required (--managed-environment or PROJWM_NEXT_MANAGED_ENVIRONMENT)")
	}
	if c.manifestDigest == "" {
		return nil, fmt.Errorf("manifest digest is required (--manifest-digest or PROJWM_NEXT_MANIFEST_DIGEST)")
	}
	if err := clientauth.VerifyManagedSocket("projwm", c.socketPath, c.manifestPath, c.manifestDigest); err != nil {
		return nil, err
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", c.socketPath, err)
	}
	hello, err := ipc.NewEnvelope(ipc.MsgHello, ipc.Hello{
		ProtocolVersion:    ipc.ProtocolVersion,
		StoreSchemaVersion: ipc.StoreSchemaVersion,
		ManifestDigest:     c.manifestDigest,
		ClientName:         "projwm",
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

// SubmitIntent dials projwmd, performs the handshake, and submits an intent.
// Returns the daemon's IntentResponse (with TransactionID + CommittedGeneration
// when accepted), or an error.
func (c *daemonClient) SubmitIntent(ctx context.Context, in intent.Intent) (ipc.IntentResponse, error) {
	conn, err := c.dialAndHandshake(ctx)
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	defer conn.Close()

	body, err := json.Marshal(in)
	if err != nil {
		return ipc.IntentResponse{}, fmt.Errorf("marshal intent: %w", err)
	}
	req, err := ipc.NewEnvelope(ipc.MsgIntentRequest, ipc.IntentRequest{
		RequestID: fmt.Sprintf("projwm-%d", time.Now().UnixNano()),
		Kind:      in.Kind(),
		Payload:   body,
	})
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if err := ipc.WriteEnvelope(conn, req); err != nil {
		return ipc.IntentResponse{}, err
	}

	rawResp, err := ipc.ReadEnvelope(conn)
	if err != nil {
		return ipc.IntentResponse{}, err
	}
	if rawResp.Type != ipc.MsgIntentResponse {
		return ipc.IntentResponse{}, fmt.Errorf("expected intent-response, got %q", rawResp.Type)
	}
	var resp ipc.IntentResponse
	if err := json.Unmarshal(rawResp.Payload, &resp); err != nil {
		return ipc.IntentResponse{}, fmt.Errorf("decode response: %w", err)
	}
	if resp.Error != nil {
		return resp, resp.Error
	}
	return resp, nil
}

// formatIntentResponse formats an IntentResponse for human display.
func formatIntentResponse(resp ipc.IntentResponse) string {
	if resp.AcceptedTransaction != nil && resp.CommittedGeneration != nil {
		return fmt.Sprintf("ok request=%s txn=%s gen=%s\n", resp.RequestID, *resp.AcceptedTransaction, *resp.CommittedGeneration)
	}
	if resp.CommittedGeneration != nil {
		return fmt.Sprintf("ok request=%s gen=%s\n", resp.RequestID, *resp.CommittedGeneration)
	}
	return fmt.Sprintf("ok request=%s\n", resp.RequestID)
}
