// projwmd is the projwm-next daemon entrypoint.
//
// 参照: implementation-design.md §5 (Controller event loop / IPC transport),
// design.md §3.7 (lifecycle transactions).
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/session"
	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/adapter/zed"
	"github.com/yuu-th/projwm-next/internal/controller"
	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/ipc"
	"github.com/yuu-th/projwm-next/internal/manifest"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// daemonVersion は handshake で client へ返す診断情報。
const daemonVersion = "0.1.0"

// handleHandshake は accept した socket から最初に届く Hello envelope を
// 検証し、Welcome / Reject envelope を返す。
//
// This function is used by the real Unix socket server path.
func handleHandshake(raw []byte, manifestDigest string, currentGeneration string, epoch w.Epoch) (ipc.Envelope, error) {
	var env ipc.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ipc.Envelope{}, fmt.Errorf("projwmd: malformed envelope: %w", err)
	}
	if env.Type != ipc.MsgHello {
		return ipc.NewEnvelope(ipc.MsgReject, ipc.Reject{
			Error: ipc.Error{
				Code:    ipc.ErrProtocolMismatch,
				Message: fmt.Sprintf("expected hello, got %q", env.Type),
			},
		})
	}
	var hello ipc.Hello
	if err := json.Unmarshal(env.Payload, &hello); err != nil {
		return ipc.NewEnvelope(ipc.MsgReject, ipc.Reject{
			Error: ipc.Error{Code: ipc.ErrProtocolMismatch, Message: "malformed hello payload"},
		})
	}
	if rej := ipc.CheckHandshake(hello, manifestDigest); rej != nil {
		return ipc.NewEnvelope(ipc.MsgReject, *rej)
	}
	return ipc.NewEnvelope(ipc.MsgWelcome, ipc.Welcome{
		ProtocolVersion:    ipc.ProtocolVersion,
		StoreSchemaVersion: ipc.StoreSchemaVersion,
		ManifestDigest:     manifestDigest,
		DaemonVersion:      daemonVersion,
		CurrentGeneration:  w.GenerationID(currentGeneration),
		Epoch:              epoch,
	})
}

// selectAdapter wires the production daemon WindowManagerAdapter from the
// Nix-authored manifest. Fake/simulator diagnostics live in internal/scenario;
// daemon startup is production-shaped and must not silently fall back to fake.
//
// The returned Vivaldi adapter is the same VivaldiAdapter installed on
// SigWM.Browser, so the daemon can also feed it into Executor.Vivaldi for the
// browser-window-close lifecycle path. The returned Zed adapter is wired
// against the same SigWM instance so it can drive the project-scoped-app
// removal contract via OmniWM-side window observation + AX-driven Cmd-W.
func selectAdapter(env w.ManagedEnvironment, privateStore browser.PrivatePayloadStore) (wm.Adapter, *browser.VivaldiAdapter, *zed.Adapter, string, error) {
	if override := os.Getenv("PROJWM_NEXT_BACKEND"); override != "" {
		return nil, nil, nil, "", fmt.Errorf("projwmd: PROJWM_NEXT_BACKEND is not allowed for daemon startup; backend must come from the managed environment manifest")
	}
	backend := env.WindowManager.Backend
	if backend == "" {
		return nil, nil, nil, "", fmt.Errorf("projwmd: manifest windowManager.backend is required")
	}
	switch backend {
	case "real", "omniwm":
		sig := wm.NewSigWM(env, wm.CmdCtlExecutor{Bin: "/opt/homebrew/bin/omniwmctl"}, nil)
		sig.Tmux = &session.Client{}
		var vivaldi *browser.VivaldiAdapter
		if privateStore != nil {
			vivaldi = browser.NewVivaldiAdapterWithWM(privateStore, nil, managedAppPath(env, "com.vivaldi.Vivaldi"), sig, nil)
			sig.Browser = vivaldi
		}
		zedAdapter := zed.NewAdapter(sig, nil, nil)
		return sig, vivaldi, zedAdapter, "real", nil
	case "fake":
		return nil, nil, nil, "", fmt.Errorf("projwmd: fake backend is not allowed for daemon startup")
	default:
		return nil, nil, nil, "", fmt.Errorf("projwmd: unsupported window manager backend %q", backend)
	}
}

func managedAppPath(env w.ManagedEnvironment, bundleID string) string {
	for _, app := range env.Apps.ManagedApps {
		if app.BundleID == bundleID && app.AppPath != "" {
			return app.AppPath
		}
	}
	return ""
}

func defaultPrivatePayloadDir(storeDir string) string {
	if storeDir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(storeDir), "private-payloads")
}

func runServer(ctx context.Context, socketPath string, ctrl *controller.Controller, manifestDigest string) error {
	if socketPath == "" {
		return fmt.Errorf("projwmd: --socket-path is required")
	}
	if err := prepareUnixSocketPath(socketPath); err != nil {
		return err
	}
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("projwmd: listen unix %s: %w", socketPath, err)
	}
	defer l.Close()
	defer os.Remove(socketPath)

	errCh := make(chan error, 1)
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					errCh <- err
					return
				}
			}
			go handleConn(ctx, conn, ctrl, manifestDigest)
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return fmt.Errorf("projwmd: accept: %w", err)
	}
}

func prepareUnixSocketPath(socketPath string) error {
	info, err := os.Lstat(socketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("projwmd: inspect socket path %s: %w", socketPath, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("projwmd: refusing to replace non-socket path %s", socketPath)
	}
	conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("projwmd: active unix socket already exists at %s", socketPath)
	}
	if err := os.Remove(socketPath); err != nil {
		return fmt.Errorf("projwmd: remove stale unix socket %s: %w", socketPath, err)
	}
	return nil
}

func handleConn(ctx context.Context, conn net.Conn, ctrl *controller.Controller, manifestDigest string) {
	defer conn.Close()
	rawHello, err := ipc.ReadEnvelope(conn)
	if err != nil {
		_ = ipc.WriteEnvelope(conn, mustEnvelope(ipc.MsgReject, ipc.Reject{Error: ipc.Error{Code: ipc.ErrProtocolMismatch, Message: err.Error()}}))
		return
	}
	current, err := ctrl.Store.LoadCurrentGeneration(ctx)
	if err != nil {
		_ = ipc.WriteEnvelope(conn, mustEnvelope(ipc.MsgReject, ipc.Reject{Error: ipc.Error{Code: ipc.ErrTransactionFailed, Message: fmt.Sprintf("load current generation: %v", err)}}))
		return
	}
	helloBytes, _ := json.Marshal(rawHello)
	welcome, err := handleHandshake(helloBytes, manifestDigest, string(current.ID), current.Checkpoint.Epoch)
	if err != nil {
		_ = ipc.WriteEnvelope(conn, mustEnvelope(ipc.MsgReject, ipc.Reject{Error: ipc.Error{Code: ipc.ErrProtocolMismatch, Message: err.Error()}}))
		return
	}
	if err := ipc.WriteEnvelope(conn, welcome); err != nil || welcome.Type == ipc.MsgReject {
		return
	}

	// After handshake, accept a single request (existing intent/event-hint
	// semantics) OR step into the multi-message loop for Query/Subscribe.
	env, err := ipc.ReadEnvelope(conn)
	if err != nil {
		_ = ipc.WriteEnvelope(conn, mustIntentResponse("", ipc.ErrProtocolMismatch, err.Error()))
		return
	}
	switch env.Type {
	case ipc.MsgIntentRequest:
		handleIntentEnvelope(ctx, conn, ctrl, env)
	case ipc.MsgEventHint:
		handleEventHintEnvelope(ctx, conn, ctrl, env)
	case ipc.MsgQueryRequest:
		handleQueryEnvelope(ctx, conn, ctrl, env)
		// After answering the query, leave the connection open for
		// follow-up queries / a subscribe handshake.
		serveLongLivedSession(ctx, conn, ctrl)
	case ipc.MsgSubscribe:
		handleSubscribeEnvelope(ctx, conn, ctrl, env)
	default:
		_ = ipc.WriteEnvelope(conn, mustIntentResponse("", ipc.ErrUnsupported, fmt.Sprintf("unsupported message type %q", env.Type)))
	}
}

// serveLongLivedSession reads further envelopes from a query-mode
// connection until the peer closes it. Used by `projwm-cockpit` and any
// CLI that wants to send multiple Queries on the same socket.
func serveLongLivedSession(ctx context.Context, conn net.Conn, ctrl *controller.Controller) {
	for {
		env, err := ipc.ReadEnvelope(conn)
		if err != nil {
			return
		}
		switch env.Type {
		case ipc.MsgQueryRequest:
			handleQueryEnvelope(ctx, conn, ctrl, env)
		case ipc.MsgSubscribe:
			handleSubscribeEnvelope(ctx, conn, ctrl, env)
			return
		case ipc.MsgSubscriptionCancel:
			return
		default:
			_ = ipc.WriteEnvelope(conn, mustEnvelope(ipc.MsgQueryResponse, ipc.QueryResponse{
				Error: &ipc.Error{Code: ipc.ErrUnsupported, Message: fmt.Sprintf("unsupported message type %q", env.Type)},
			}))
		}
	}
}

func handleIntentEnvelope(ctx context.Context, conn net.Conn, ctrl *controller.Controller, env ipc.Envelope) {
	var req ipc.IntentRequest
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		_ = ipc.WriteEnvelope(conn, mustIntentResponse("", ipc.ErrProtocolMismatch, fmt.Sprintf("malformed intent request: %v", err)))
		return
	}
	in, err := ipc.DecodeIntent(req)
	if err != nil {
		_ = ipc.WriteEnvelope(conn, mustIntentResponse(req.RequestID, ipc.ErrIntentRejected, err.Error()))
		return
	}
	result, err := ctrl.ApplyIntent(ctx, in)
	if err != nil {
		_ = ipc.WriteEnvelope(conn, transactionFailedIntentResponse(req.RequestID, result, err.Error()))
		return
	}
	current, err := ctrl.Store.LoadCurrentGeneration(ctx)
	if err != nil {
		_ = ipc.WriteEnvelope(conn, mustIntentResponse(req.RequestID, ipc.ErrTransactionFailed, fmt.Sprintf("load committed generation: %v", err)))
		return
	}
	resp, err := ipc.NewEnvelope(ipc.MsgIntentResponse, ipc.IntentResponse{
		RequestID:           req.RequestID,
		AcceptedTransaction: &result.TransactionID,
		CommittedGeneration: &current.ID,
	})
	if err != nil {
		_ = ipc.WriteEnvelope(conn, mustIntentResponse(req.RequestID, ipc.ErrProtocolMismatch, err.Error()))
		return
	}
	_ = ipc.WriteEnvelope(conn, resp)
}

func handleEventHintEnvelope(ctx context.Context, conn net.Conn, ctrl *controller.Controller, env ipc.Envelope) {
	var hint ipc.EventHint
	if err := json.Unmarshal(env.Payload, &hint); err != nil {
		_ = ipc.WriteEnvelope(conn, mustEventAck("", ipc.ErrProtocolMismatch, fmt.Sprintf("malformed event hint: %v", err)))
		return
	}
	if hint.HintID == "" {
		hint.HintID = "hint"
	}
	if hint.Source == "" {
		hint.Source = event.SourceSystem
	}
	ev := event.Event{ID: w.EventID(hint.HintID), Source: hint.Source, Kind: hint.Kind, Epoch: hint.Epoch}
	if len(hint.Body) > 0 {
		if err := json.Unmarshal(hint.Body, &ev.Data); err != nil {
			_ = ipc.WriteEnvelope(conn, mustEventAck(hint.HintID, ipc.ErrProtocolMismatch, fmt.Sprintf("malformed event body: %v", err)))
			return
		}
	}
	result, err := ctrl.ApplyEvent(ctx, ev)
	if err != nil {
		_ = ipc.WriteEnvelope(conn, transactionFailedEventAck(hint.HintID, result, err.Error()))
		return
	}
	ack := ipc.EventAck{
		HintID:              hint.HintID,
		AcceptedTransaction: &result.TransactionID,
		Dropped:             result.Trace.Discarded,
	}
	if result.CommittedGeneration != "" {
		ack.CommittedGeneration = &result.CommittedGeneration
	}
	resp, err := ipc.NewEnvelope(ipc.MsgEventAck, ack)
	if err != nil {
		_ = ipc.WriteEnvelope(conn, mustEventAck(hint.HintID, ipc.ErrProtocolMismatch, err.Error()))
		return
	}
	_ = ipc.WriteEnvelope(conn, resp)
}

func mustIntentResponse(requestID string, code ipc.ErrorCode, message string) ipc.Envelope {
	return mustEnvelope(ipc.MsgIntentResponse, ipc.IntentResponse{RequestID: requestID, Error: &ipc.Error{Code: code, Message: message}})
}

func transactionFailedIntentResponse(requestID string, result controller.TransactionResult, message string) ipc.Envelope {
	resp := ipc.IntentResponse{RequestID: requestID, Error: &ipc.Error{Code: ipc.ErrTransactionFailed, Message: message}}
	if result.TransactionID != "" {
		resp.AcceptedTransaction = &result.TransactionID
	}
	if result.CommittedGeneration != "" {
		resp.CommittedGeneration = &result.CommittedGeneration
	}
	return mustEnvelope(ipc.MsgIntentResponse, resp)
}

func mustEventAck(hintID string, code ipc.ErrorCode, message string) ipc.Envelope {
	return mustEnvelope(ipc.MsgEventAck, ipc.EventAck{HintID: hintID, Error: &ipc.Error{Code: code, Message: message}})
}

func transactionFailedEventAck(hintID string, result controller.TransactionResult, message string) ipc.Envelope {
	ack := ipc.EventAck{HintID: hintID, Error: &ipc.Error{Code: ipc.ErrTransactionFailed, Message: message}}
	if result.TransactionID != "" {
		ack.AcceptedTransaction = &result.TransactionID
	}
	if result.CommittedGeneration != "" {
		ack.CommittedGeneration = &result.CommittedGeneration
	}
	ack.Dropped = result.Trace.Discarded
	return mustEnvelope(ipc.MsgEventAck, ack)
}

func mustEnvelope(t ipc.MessageType, payload any) ipc.Envelope {
	env, err := ipc.NewEnvelope(t, payload)
	if err != nil {
		panic(err)
	}
	return env
}

func loadEnvironment(path string, expectedDigest string) (w.ManagedEnvironment, string, error) {
	if path == "" {
		return w.ManagedEnvironment{}, "", fmt.Errorf("projwmd: --managed-environment is required")
	}
	if expectedDigest == "" {
		return w.ManagedEnvironment{}, "", fmt.Errorf("projwmd: --manifest-digest is required for production daemon startup")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return w.ManagedEnvironment{}, "", fmt.Errorf("manifest: read %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	actualDigest := hex.EncodeToString(sum[:])
	if actualDigest != expectedDigest {
		return w.ManagedEnvironment{}, "", fmt.Errorf("projwmd: manifest digest mismatch (computed %s, expected %s)", actualDigest, expectedDigest)
	}
	env, err := manifest.Parse(data, "0.1.0")
	if err != nil {
		return w.ManagedEnvironment{}, "", err
	}
	return env, actualDigest, nil
}

type startupProvenance struct {
	SchemaVersion                  int                   `json:"schemaVersion"`
	DaemonVersion                  string                `json:"daemonVersion"`
	Mode                           string                `json:"mode"`
	PID                            int                   `json:"pid"`
	StartedAt                      string                `json:"startedAt"`
	ManifestPath                   string                `json:"manifestPath"`
	ManifestDigest                 string                `json:"manifestDigest"`
	ManifestSource                 string                `json:"manifestSource"`
	StoreDir                       string                `json:"storeDir"`
	StoreKind                      store.StoreKind       `json:"storeKind"`
	PrivatePayloadDir              string                `json:"privatePayloadDir"`
	CurrentGeneration              w.GenerationID        `json:"currentGeneration"`
	StoreBootstrapCommitKind       string                `json:"storeBootstrapCommitKind,omitempty"`
	StoreBootstrapTriggerSource    string                `json:"storeBootstrapTriggerSource,omitempty"`
	StoreBootstrapTriggerKind      string                `json:"storeBootstrapTriggerKind,omitempty"`
	ProductionAdminBootstrap       bool                  `json:"productionAdminBootstrap"`
	SocketPath                     string                `json:"socketPath"`
	Backend                        string                `json:"backend"`
	LaunchdLabel                   string                `json:"launchdLabel,omitempty"`
	ManagedByManifest              bool                  `json:"managedByManifest"`
	DesiredWorldInjected           bool                  `json:"desiredWorldInjected"`
	DeclaredEventSources           []w.EventSourceSpec   `json:"declaredEventSources,omitempty"`
	RequiredEventSourcesDeclared   bool                  `json:"requiredEventSourcesDeclared"`
	RuntimeLaunchdEventSourceProof string                `json:"runtimeLaunchdEventSourceProof"`
	LaunchdRuntimeProof            []launchdServiceProof `json:"launchdRuntimeProof,omitempty"`
	StartupLifecycleStatus         string                `json:"startupLifecycleStatus"`
	StartupLifecycleBlockedReason  string                `json:"startupLifecycleBlockedReason,omitempty"`
	StartupLifecycleTransaction    w.TransactionID       `json:"startupLifecycleTransaction,omitempty"`
	BootstrapGeneration            w.GenerationID        `json:"bootstrapGeneration"`
	BootstrapManifestDigest        string                `json:"bootstrapManifestDigest,omitempty"`
}

type launchdServiceProof struct {
	Role       string `json:"role"`
	Label      string `json:"label"`
	Kind       string `json:"kind,omitempty"`
	Source     string `json:"source,omitempty"`
	Loaded     bool   `json:"loaded"`
	Running    bool   `json:"running,omitempty"`
	PID        int    `json:"pid,omitempty"`
	PIDMatches bool   `json:"pidMatches,omitempty"`
}

func buildStartupProvenance(env w.ManagedEnvironment, manifestPath, manifestDigest, storeDir, privatePayloadDir string, storeKind store.StoreKind, current store.CommittedGeneration, bootstrap store.CommittedGeneration, socketPath, backend, launchdLabel string, runtimeProof string, launchdProof []launchdServiceProof) (startupProvenance, error) {
	if launchdLabel != "" && env.Daemons.ControllerLabel != "" && launchdLabel != env.Daemons.ControllerLabel {
		return startupProvenance{}, fmt.Errorf("projwmd: launchd label %q does not match manifest controller %q", launchdLabel, env.Daemons.ControllerLabel)
	}
	if env.Daemons.SocketPath == "" {
		return startupProvenance{}, fmt.Errorf("projwmd: manifest daemons.socketPath is required for production daemon startup")
	}
	if socketPath != env.Daemons.SocketPath {
		return startupProvenance{}, fmt.Errorf("projwmd: socket path %q does not match manifest socketPath %q", socketPath, env.Daemons.SocketPath)
	}
	if !requiredProductionEventSourcesDeclared(env.Daemons.EventSources) {
		return startupProvenance{}, fmt.Errorf("projwmd: manifest daemons.eventSources must declare all required production event hint sidecars")
	}
	adminBootstrap := productionAdminBootstrap(bootstrap.Trace, manifestDigest)
	if !adminBootstrap {
		return startupProvenance{}, fmt.Errorf("projwmd: production store bootstrap evidence is missing or not admin-owned")
	}
	absManifest, err := filepath.Abs(manifestPath)
	if err != nil {
		return startupProvenance{}, fmt.Errorf("projwmd: resolve manifest path: %w", err)
	}
	absStore, err := filepath.Abs(storeDir)
	if err != nil {
		return startupProvenance{}, fmt.Errorf("projwmd: resolve store dir: %w", err)
	}
	if privatePayloadDir == "" {
		return startupProvenance{}, fmt.Errorf("projwmd: --private-payload-dir is required for production daemon startup")
	}
	absPrivatePayload, err := filepath.Abs(privatePayloadDir)
	if err != nil {
		return startupProvenance{}, fmt.Errorf("projwmd: resolve private payload dir: %w", err)
	}
	if absPrivatePayload == absStore || strings.HasPrefix(absPrivatePayload, absStore+string(os.PathSeparator)) {
		return startupProvenance{}, fmt.Errorf("projwmd: private payload dir must be outside PersistentStore dir")
	}
	managedByManifest := env.Authority == "nix" && isNixStorePath(absManifest)
	return startupProvenance{
		SchemaVersion:                  1,
		DaemonVersion:                  daemonVersion,
		Mode:                           "production",
		PID:                            os.Getpid(),
		StartedAt:                      time.Now().UTC().Format(time.RFC3339Nano),
		ManifestPath:                   absManifest,
		ManifestDigest:                 manifestDigest,
		ManifestSource:                 env.Source,
		StoreDir:                       absStore,
		StoreKind:                      storeKind,
		PrivatePayloadDir:              absPrivatePayload,
		CurrentGeneration:              current.ID,
		StoreBootstrapCommitKind:       bootstrap.Trace.CommitKind,
		StoreBootstrapTriggerSource:    bootstrap.Trace.TriggerSource,
		StoreBootstrapTriggerKind:      bootstrap.Trace.TriggerKind,
		ProductionAdminBootstrap:       adminBootstrap,
		SocketPath:                     socketPath,
		Backend:                        backend,
		LaunchdLabel:                   launchdLabel,
		ManagedByManifest:              managedByManifest,
		DesiredWorldInjected:           false,
		DeclaredEventSources:           cloneEventSources(env.Daemons.EventSources),
		RequiredEventSourcesDeclared:   requiredProductionEventSourcesDeclared(env.Daemons.EventSources),
		RuntimeLaunchdEventSourceProof: runtimeProof,
		LaunchdRuntimeProof:            append([]launchdServiceProof(nil), launchdProof...),
		StartupLifecycleStatus:         "pending",
		BootstrapGeneration:            bootstrap.ID,
		BootstrapManifestDigest:        bootstrap.Trace.BootstrapManifestDigest,
	}, nil
}

func productionAdminBootstrap(trace store.TransactionTrace, manifestDigest string) bool {
	if trace.CommitKind != "migration-bootstrap" || trace.CommittedBy != "controller" || trace.TriggerSource != "admin" {
		return false
	}
	if trace.BootstrapManifestDigest == "" || trace.BootstrapManifestDigest != manifestDigest {
		return false
	}
	return trace.TriggerKind == "desired-world-bootstrap" || trace.TriggerKind == "legacy-state-migration"
}

func cloneEventSources(in []w.EventSourceSpec) []w.EventSourceSpec {
	out := append([]w.EventSourceSpec(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Label < out[j].Label
	})
	return out
}

func requiredProductionEventSourcesDeclared(sources []w.EventSourceSpec) bool {
	required := map[string]bool{
		"display-changed|system|sidecar|hint":         false,
		"layout-changed|user|sidecar|hint":            false,
		"safety-timer|timer|sidecar|hint":             false,
		"wake|system|sidecar|hint":                    false,
		"windows-changed|window-manager|sidecar|hint": false,
	}
	for _, src := range sources {
		key := src.Kind + "|" + src.Source + "|" + src.Mode + "|" + src.Authority
		if _, ok := required[key]; ok {
			required[key] = true
		}
	}
	for _, ok := range required {
		if !ok {
			return false
		}
	}
	return true
}

func verifyLaunchdRuntimeProof(ctx context.Context, env w.ManagedEnvironment, launchdLabel string, require bool) (string, []launchdServiceProof, error) {
	if !require {
		return "not-observed", nil, nil
	}
	if launchdLabel == "" {
		return "", nil, fmt.Errorf("projwmd: --launchd-label is required when launchd runtime proof is required")
	}
	services := []launchdServiceProof{}
	controller, err := waitLaunchdService(ctx, "controller", launchdLabel, "", "")
	if err != nil {
		return "", nil, err
	}
	controller.PIDMatches = controller.PID == os.Getpid()
	if !controller.Loaded || !controller.PIDMatches {
		return "", nil, fmt.Errorf("projwmd: launchd controller proof failed for %s (loaded=%v pid=%d self=%d)", launchdLabel, controller.Loaded, controller.PID, os.Getpid())
	}
	services = append(services, controller)
	for _, src := range cloneEventSources(env.Daemons.EventSources) {
		if src.Mode != "sidecar" {
			continue
		}
		proof, err := waitLaunchdService(ctx, "event-source", src.Label, src.Kind, src.Source)
		if err != nil {
			return "", nil, err
		}
		if !proof.Loaded {
			return "", nil, fmt.Errorf("projwmd: launchd event source %s is not loaded", src.Label)
		}
		services = append(services, proof)
	}
	return "verified", services, nil
}

func waitLaunchdService(ctx context.Context, role, label, kind, source string) (launchdServiceProof, error) {
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for {
		proof, err := inspectLaunchdServiceFunc(ctx, role, label, kind, source)
		if err == nil {
			return proof, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return launchdServiceProof{}, lastErr
		}
		select {
		case <-ctx.Done():
			return launchdServiceProof{}, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

var inspectLaunchdServiceFunc = inspectLaunchdService

func inspectLaunchdService(ctx context.Context, role, label, kind, source string) (launchdServiceProof, error) {
	if label == "" {
		return launchdServiceProof{}, fmt.Errorf("projwmd: launchd %s label is required", role)
	}
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
	cmd := exec.CommandContext(ctx, "/bin/launchctl", "print", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return launchdServiceProof{}, fmt.Errorf("projwmd: launchctl print %s: %w", target, err)
	}
	body := string(out)
	pid := launchdPID(body)
	return launchdServiceProof{
		Role:    role,
		Label:   label,
		Kind:    kind,
		Source:  source,
		Loaded:  true,
		Running: pid > 0 || strings.Contains(body, "state = running"),
		PID:     pid,
	}, nil
}

func launchdPID(out string) int {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if raw, ok := strings.CutPrefix(line, "pid = "); ok {
			var pid int
			if _, err := fmt.Sscanf(raw, "%d", &pid); err == nil {
				return pid
			}
		}
	}
	return 0
}

func isNixStorePath(path string) bool {
	clean := filepath.Clean(path)
	return clean == "/nix/store" || strings.HasPrefix(clean, "/nix/store/")
}

func writeStartupProvenance(path string, p startupProvenance) error {
	if path == "" {
		return nil
	}
	if strings.Contains(path, "/tmp/") {
		return fmt.Errorf("projwmd: refusing /tmp startup provenance path: %s", path)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("projwmd: create provenance dir: %w", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("projwmd: marshal startup provenance: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, ".startup-provenance-*.tmp")
	if err != nil {
		return fmt.Errorf("projwmd: create provenance temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("projwmd: write startup provenance: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("projwmd: close startup provenance: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("projwmd: publish startup provenance: %w", err)
	}
	return nil
}

func main() {
	var (
		manifestPath               = flag.String("managed-environment", "", "path to Nix-authored managed environment manifest")
		manifestDigest             = flag.String("manifest-digest", "", "expected manifest digest for IPC handshake")
		desiredPath                = flag.String("desired-world", "", "unsupported daemon bootstrap path; pre-initialize PersistentStore instead")
		socketPath                 = flag.String("socket-path", "", "Unix socket path supplied by launchd or an isolated production-shaped acceptance harness")
		storeDir                   = flag.String("store-dir", "", "PersistentStore generation directory")
		storeKind                  = flag.String("store-kind", "production", "PersistentStore kind: production, test, or recovery")
		privatePayloadDir          = flag.String("private-payload-dir", "", "private browser payload directory; defaults to a sibling of --store-dir")
		launchdLabel               = flag.String("launchd-label", "", "expected production launchd label from ManagedEnvironment.daemons.controller")
		requireLaunchdRuntimeProof = flag.Bool("require-launchd-runtime-proof", false, "require launchctl proof that controller and declared sidecar labels are loaded")
		provenancePath             = flag.String("startup-provenance", "", "optional redacted startup provenance JSON path")
	)
	flag.Parse()

	env, actualManifestDigest, err := loadEnvironment(*manifestPath, *manifestDigest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmd: %v\n", err)
		os.Exit(1)
	}
	if *desiredPath != "" {
		fmt.Fprintln(os.Stderr, "projwmd: --desired-world is not part of production daemon startup; pre-initialize PersistentStore through the migration/admin bootstrap path")
		os.Exit(1)
	}
	if *storeDir == "" {
		fmt.Fprintf(os.Stderr, "projwmd: --store-dir is required for daemon startup\n")
		os.Exit(1)
	}
	if *privatePayloadDir == "" {
		*privatePayloadDir = defaultPrivatePayloadDir(*storeDir)
	}
	privateStore, err := browser.NewFilePrivatePayloadStore(*privatePayloadDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmd: private payload store: %v\n", err)
		os.Exit(1)
	}
	adapter, vivaldiAdapter, zedAdapter, backend, err := selectAdapter(env, privateStore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	var (
		st       store.PersistentStore
		ancestry store.GenerationAncestry
	)
	if *storeDir != "" {
		kind := store.StoreKind(*storeKind)
		if kind == store.StoreKindTest {
			fmt.Fprintf(os.Stderr, "projwmd: refusing test store for daemon startup; use production or recovery store kind\n")
			os.Exit(1)
		}
		if kind != store.StoreKindProduction {
			fmt.Fprintf(os.Stderr, "projwmd: refusing non-production store kind for daemon startup: %s\n", kind)
			os.Exit(1)
		}
		fileStore, err := store.OpenExistingFileStore(context.Background(), *storeDir, kind)
		if err != nil {
			fmt.Fprintf(os.Stderr, "projwmd: open store: %v\n", err)
			os.Exit(1)
		}
		ancestry, err = fileStore.LoadGenerationAncestry(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "projwmd: load store ancestry: %v\n", err)
			os.Exit(1)
		}
		st = fileStore
	}
	current, err := st.LoadCurrentGeneration(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmd: load current store generation: %v\n", err)
		os.Exit(1)
	}
	runtimeProof, launchdProof, err := verifyLaunchdRuntimeProof(context.Background(), env, *launchdLabel, *requireLaunchdRuntimeProof)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if ancestry.Current.ID != "" {
		current = ancestry.Current
	}
	provenance, err := buildStartupProvenance(env, *manifestPath, actualManifestDigest, *storeDir, *privatePayloadDir, store.StoreKind(*storeKind), current, ancestry.Root, *socketPath, backend, *launchdLabel, runtimeProof, launchdProof)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	ctrl := controller.NewFromGeneration(env, current, adapter, st)
	if vivaldiAdapter != nil {
		// Wire the Vivaldi browser-window-close lifecycle path. The Executor
		// uses this only when an op's lifecycleRemoval method is
		// browser-window-close, and even then only after collect-evidence +
		// contract validation succeed.
		ctrl.Executor.Vivaldi = vivaldiAdapter
	}
	if zedAdapter != nil {
		// Wire the Zed project-scoped-app removal lifecycle path. The
		// Executor uses this only when an op's lifecycleRemoval method is
		// project-scoped-app for the Zed bundle, and even then only after
		// collect-evidence + contract validation succeed.
		ctrl.Executor.Zed = zedAdapter
	}

	// Subscribe broadcast hook (design v3 §3.3): forward card-added /
	// card-removed / generation-committed signals from the controller
	// to the process-wide ConnHub so MsgSubscribe streams see them.
	ctrl.OnBroadcast = func(kind string, payload any, generation string) {
		connHub.Broadcast(kind, payload, generation)
	}

	if *socketPath == "" {
		fmt.Fprintf(os.Stderr, "projwmd: ipc protocol=%s store-schema=%s backend=%s; no --socket-path, exiting\n",
			ipc.ProtocolVersion, ipc.StoreSchemaVersion, backend)
		return
	}
	if strings.Contains(*socketPath, "/tmp/") {
		fmt.Fprintf(os.Stderr, "projwmd: refusing /tmp socket for production daemon startup: %s\n", *socketPath)
		os.Exit(1)
	}
	if err := prepareUnixSocketPath(*socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create the Unix socket BEFORE startup reconcile so that the IPC
	// handshake (used by tests and healthchecks) can succeed immediately,
	// even while the startup lifecycle transaction is still running.
	l, err := net.Listen("unix", *socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmd: listen unix %s: %v\n", *socketPath, err)
		os.Exit(1)
	}
	defer l.Close()
	defer os.Remove(*socketPath)

	acceptErrCh := make(chan error, 1)
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					acceptErrCh <- err
					return
				}
			}
			go handleConn(ctx, conn, ctrl, actualManifestDigest)
		}
	}()

	// Requirement §8.1 / §8.8 — reap stale cockpit artifacts BEFORE the
	// startup reconcile so the planner's SpawnCockpit op fires against a
	// clean slate. This kills orphaned cockpit binaries (from earlier
	// daemon crashes or `projwm tui` direct invocations) and any
	// duplicate ghostty/tmux artifacts that would otherwise be considered
	// "already present" by the idempotent SpawnCockpit pre-check and
	// silently leave the multi-instance state in place.
	if reaper, ok := adapter.(wm.CockpitReaper); ok {
		reaper.ReapStaleCockpit(ctx)
		// Defense-in-depth: also collapse duplicates immediately if a
		// prior daemon run left more than one ghostty cockpit alive
		// (omniwm ghost reference, AppKit race, manual `open -na`).
		reaper.ReapDuplicateCockpits(ctx)
	}

	// Requirement v2.8 §8.9 — Omniwm self-heal: probe omniwm health and
	// apply recovery ladder Lv1 (omniwm-deploy re-push) + Lv2 (managed
	// app relaunch) before the startup reconcile. Lv3-Lv4 (omniwm
	// restart) are staged later. Best-effort: failures are logged but do
	// not block startup.
	if healer, ok := adapter.(wm.OmniwmSelfHealer); ok {
		runOmniwmRecovery(ctx, healer, env)
	}

	startupResult, startupErr := ctrl.ApplyEvent(ctx, event.Event{Source: event.SourceSystem, Kind: event.KindStartup})
	if startupErr != nil {
		if !durableNoCommitTrace(startupResult, startupErr) {
			fmt.Fprintf(os.Stderr, "projwmd: startup lifecycle: %v\n", startupErr)
			os.Exit(1)
		}
		provenance.StartupLifecycleStatus = "blocked"
		provenance.StartupLifecycleBlockedReason = startupResult.Trace.NoCommitReason + ": " + startupErr.Error()
		provenance.StartupLifecycleTransaction = startupResult.TransactionID
		fmt.Fprintf(os.Stderr, "projwmd: startup lifecycle blocked; serving degraded IPC: %v\n", startupErr)
	} else {
		provenance.StartupLifecycleStatus = "committed"
		provenance.StartupLifecycleTransaction = startupResult.TransactionID
	}
	current, err = st.LoadCurrentGeneration(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmd: load post-startup store generation: %v\n", err)
		os.Exit(1)
	}
	provenance.CurrentGeneration = current.ID
	if err := writeStartupProvenance(*provenancePath, provenance); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "projwmd: listening on %s backend=%s storeKind=%s mode=production\n", *socketPath, backend, *storeKind)

	// v2.8 §8.9 continuous self-heal ticker: re-probe omniwm every 30s
	// and re-apply the recovery ladder if drift is detected. Also collapses
	// duplicate cockpit ghostty processes (§8.10) on each tick.
	if healer, ok := adapter.(wm.OmniwmSelfHealer); ok {
		var reaper wm.CockpitReaper
		if r, ok2 := adapter.(wm.CockpitReaper); ok2 {
			reaper = r
		}
		go runOmniwmRecoveryTicker(ctx, healer, env, reaper)
	}

	// Tier 1 5-second grace ticker (design v3 §3.6): polls PendingOrphans
	// once a second; entries older than 5s without a MatchedTo are
	// promoted into [NEW] cards. Cheap O(N) walk.
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ctrl.PromoteOrphans(5 * time.Second)
			}
		}
	}()

	// E2.1 manifest digest mismatch watchdog: re-hash the manifest every
	// 60 seconds and surface a [MANIFEST] cockpit card if it has drifted
	// from the digest we booted with. The daemon stays running because
	// the in-memory manifest is still self-consistent — the user just
	// gets nudged to re-load.
	go runManifestWatchdog(ctx, ctrl, *manifestPath, actualManifestDigest)

	// Tier 3 BrowserTabsSync: poll Vivaldi automation-profile tab URLs
	// every 5 seconds; submit SyncBrowserTabs intent when the URL set
	// changes (requirements §3.3 / §4.2.3 / §15.2).
	if bts := newBrowserTabsObserver(ctrl, vivaldiAdapter, privateStore); bts != nil {
		go bts.Run(ctx)
	}

	// Cockpit lifecycle is now driven by the projwm-next planner/executor
	// pipeline via SystemWindows + scratchpad ops (unified design v1 §6).
	// CFG_COCKPIT_BIN feeds into sigwm.ensureCockpitBaseSession when the
	// first SpawnCockpit op fires.
	if bin := getEnvOrDefault("PROJWM_NEXT_COCKPIT_BIN", ""); bin != "" {
		_ = os.Setenv("CFG_COCKPIT_BIN", bin)
	}

	select {
	case <-ctx.Done():
	case err := <-acceptErrCh:
		fmt.Fprintf(os.Stderr, "projwmd: accept: %v\n", err)
		os.Exit(1)
	}

	// Requirement §8.8 row "projwmd 停止/再起動 → cockpit プロセスも停止" —
	// kill all cockpit artifacts before exiting so a follow-up start
	// (launchd KeepAlive) re-spawns from a clean slate. Use a fresh
	// context with a tight budget because the original ctx is already
	// canceled by the time we reach here.
	if reaper, ok := adapter.(wm.CockpitReaper); ok {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		reaper.ShutdownCockpit(shutCtx)
		cancel()
	}
}

// runOmniwmRecovery probes omniwm health and applies the §8.9 recovery
// ladder (Lv1: rule redeploy, Lv2: managed app relaunch). Lv3-Lv4
// (omniwm restart) are not yet wired — they require warning card +
// user grace which the next implementation stage adds.
//
// Best-effort: each step swallows its error after logging to stderr so
// daemon startup is never blocked by a recovery failure. Self-heal is
// supplementary, not gating.
// lv2RelaunchEnabled controls whether the runOmniwmRecovery may issue
// Lv2 (managed app quit+relaunch). It is true only on the very first
// call (startup) — subsequent ticker calls keep it false to avoid the
// observed loop where every 30s probe sees ghostty briefly absent from
// TrackedApps right after omniwm restart, triggers Lv2 relaunch, which
// causes the next probe to see it absent again. Mid-run drift is
// handled by reaper.ReapDuplicateCockpits + planner SpawnCockpit, not
// by app-level relaunch.
var lv2RelaunchEnabled = true

func runOmniwmRecovery(ctx context.Context, healer wm.OmniwmSelfHealer, env w.ManagedEnvironment) {
	probe := healer.ProbeOmniwmHealth(ctx)
	if !probe.Reachable {
		// Lv3: omniwm unreachable — kickstart -k restart. This has side
		// effects (all app workspace assignments re-apply, column order
		// not preserved), so we emit a [OMNIWM-RECOVERY] warning log and
		// sleep 5s as a "user might Esc" grace before committing. A
		// future stage adds an interactive Esc via cockpit modal; for
		// now the grace is non-cancellable but documented.
		fmt.Fprintln(os.Stderr, "projwmd: [OMNIWM-RECOVERY] Lv3: omniwmctl unreachable, restarting omniwm in 5 seconds (side effect: all app workspace assignments reset)")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
		if err := healer.RestartOmniwm(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "projwmd: [OMNIWM-RECOVERY-FAILED] Lv3 restart failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "projwmd: [OMNIWM-RECOVERY-FAILED] please check macOS System Settings > Privacy > Accessibility for omniwm permission, or run `sudo darwin-rebuild switch --flake .#yuta` to reinstall")
			return
		}
		fmt.Fprintln(os.Stderr, "projwmd: [OMNIWM-RECOVERY] Lv3 succeeded, omniwm is back; re-probing")
		probe = healer.ProbeOmniwmHealth(ctx)
		if !probe.Reachable {
			fmt.Fprintln(os.Stderr, "projwmd: [OMNIWM-RECOVERY-FAILED] omniwm restart returned but ping still fails — manual intervention required")
			return
		}
	}
	// Lv1: rule count threshold. omniwm-deploy is the authority on the
	// expected number; we use a conservative floor here (Ghostty + Vivaldi
	// + Zed app-rules alone need ≥5 entries). Below this we always re-push.
	const minExpectedRules = 5
	if probe.RuleCount < minExpectedRules {
		fmt.Fprintf(os.Stderr, "projwmd: omniwm-recovery Lv1: rule count %d < %d, re-pushing\n", probe.RuleCount, minExpectedRules)
		if err := healer.RedeployOmniwmRules(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "projwmd: omniwm-recovery Lv1 failed: %v\n", err)
		} else {
			// Re-probe after rule push.
			probe = healer.ProbeOmniwmHealth(ctx)
		}
	}
	// Lv2: managed apps from manifest must appear in TrackedApps.
	// Bootstrap-only — after the first call, lv2RelaunchEnabled is set
	// to false to avoid the 30s probe / quit / relaunch loop observed
	// 2026-05-18 (omniwm momentarily drops ghostty from TrackedApps right
	// after restart, ticker treats it as missing, relaunches, next tick
	// repeats forever). Mid-run cockpit drift is handled by the
	// cockpit reaper + planner.SpawnCockpit, not by relaunch.
	if lv2RelaunchEnabled {
		for _, app := range env.Apps.ManagedApps {
			if probe.TrackedApps[app.BundleID] {
				continue
			}
			if app.AppPath == "" {
				fmt.Fprintf(os.Stderr, "projwmd: omniwm-recovery Lv2: %s missing from tracking but appPath unknown\n", app.BundleID)
				continue
			}
			appName := strings.TrimSuffix(filepath.Base(app.AppPath), ".app")
			fmt.Fprintf(os.Stderr, "projwmd: omniwm-recovery Lv2: relaunching %s (%s) to re-register with omniwm\n", appName, app.BundleID)
			if err := healer.RelaunchManagedApp(ctx, appName, app.AppPath); err != nil {
				fmt.Fprintf(os.Stderr, "projwmd: omniwm-recovery Lv2 failed for %s: %v\n", app.BundleID, err)
			}
		}
		lv2RelaunchEnabled = false
	}
}

// runOmniwmRecoveryTicker runs runOmniwmRecovery every 30 seconds for
// the lifetime of ctx. Realises requirements v2.8 §8.9 continuous
// monitoring: omniwm crashes / tracking drops / rule list resets that
// happen after daemon startup are caught and remediated automatically.
func runOmniwmRecoveryTicker(ctx context.Context, healer wm.OmniwmSelfHealer, env w.ManagedEnvironment, reaper wm.CockpitReaper) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			runOmniwmRecovery(ctx, healer, env)
			// v2.8 §8.10: collapse any duplicate cockpit ghostty
			// processes that appeared since the last tick. Cheap (one
			// pgrep + per-pid pgrep + selective kill), and the only
			// real-time guarantee that "exactly one cockpit" holds
			// while the daemon is running.
			if reaper != nil {
				reaper.ReapDuplicateCockpits(ctx)
			}
		}
	}
}

// getEnvOrDefault returns os.Getenv(key) or fallback if unset.
func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durableNoCommitTrace(result controller.TransactionResult, err error) bool {
	if err == nil {
		return false
	}
	if result.Trace.NoCommitReason == "" || result.TransactionID == "" {
		return false
	}
	return !strings.Contains(err.Error(), "additionally failed to record no-commit trace")
}
