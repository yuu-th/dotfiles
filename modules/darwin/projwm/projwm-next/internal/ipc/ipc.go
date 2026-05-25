// Package ipc declares the projwmd ↔ projwmctl IPC contract.
//
// Wire contract:
//
//   - protocol version / store schema version / manifest digest を交換する
//     handshake のフォーマット
//   - IntentRequest / EventHint の message envelope
//   - error taxonomy（impl-design.md §5 IPC transport の表に対応）
//   - protocol mismatch を弾く検証ロジック (CheckHandshake)
//
// projwmd/projwmctl/projwmevent use this contract for the Unix socket protocol.
//
// 参照:
//   - design.md §3.7 (lifecycle transaction kinds)
//   - implementation-design.md §5 IPC transport
//   - specs.md §2 (single writer / final-focus invariants)
package ipc

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// ProtocolVersion is the wire-protocol version exchanged in handshake.
// Bumped whenever Envelope / Hello / Welcome / IntentRequest / IntentResponse
// shapes change in a non-additive way.
const ProtocolVersion = "projwm-ipc/1"

// StoreSchemaVersion mirrors store.CommittedGeneration.StoreVersion at the
// schema level. handshake で client/daemon 双方が一致する必要がある。
// StoreSchemaVersion is validated during daemon/client handshake.
const StoreSchemaVersion = "1"

// MessageType discriminates Envelope payloads.
type MessageType string

const (
	MsgHello          MessageType = "hello"           // client → daemon
	MsgWelcome        MessageType = "welcome"         // daemon → client
	MsgReject         MessageType = "reject"          // daemon → client (handshake failure)
	MsgIntentRequest  MessageType = "intent-request"  // client → daemon
	MsgIntentResponse MessageType = "intent-response" // daemon → client
	MsgEventHint      MessageType = "event-hint"      // sidecar → daemon (EventHint only; impl-design §5 sidecar limits)
	MsgEventAck       MessageType = "event-ack"       // daemon → sidecar

	// v2.3 / design v3 additions.
	MsgQueryRequest       MessageType = "query-request"       // client → daemon (read-only)
	MsgQueryResponse      MessageType = "query-response"      // daemon → client
	MsgSubscribe          MessageType = "subscribe"           // client → daemon (open push stream)
	MsgSubscribeAck       MessageType = "subscribe-ack"       // daemon → client
	MsgSubscriptionPush   MessageType = "subscription-push"   // daemon → client (async)
	MsgSubscriptionCancel MessageType = "subscription-cancel" // client → daemon (close stream)
)

// ErrorCode mirrors implementation-design.md §5 error taxonomy.
type ErrorCode string

const (
	ErrSocketAbsent      ErrorCode = "socket-absent"
	ErrConnectionRefused ErrorCode = "connection-refused"
	ErrTimeout           ErrorCode = "timeout"
	ErrDaemonBusy        ErrorCode = "daemon-busy"
	ErrProtocolMismatch  ErrorCode = "protocol-mismatch"
	ErrIntentRejected    ErrorCode = "intent-rejected"
	ErrTransactionFailed ErrorCode = "transaction-failed"
	ErrUnsupported       ErrorCode = "unsupported"
)

// Error is the typed wire error.
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("ipc: %s: %s", e.Code, e.Message)
}

// Envelope wraps every message on the socket.
//
// Wire format is newline-delimited JSON:
//
//	{"type":"hello","payload":{...}}\n
//
// Newline framing keeps the contract simple to mock; binary framing is
// considered for Phase D if profiling requires it.
type Envelope struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Hello is sent by client immediately after connecting.
type Hello struct {
	ProtocolVersion    string `json:"protocolVersion"`
	StoreSchemaVersion string `json:"storeSchemaVersion"`
	// ManifestDigest is a stable hash of the ManagedEnvironment manifest the
	// client believes is authoritative. daemon rejects on mismatch so that an
	// out-of-date `projwmctl` cannot drive a daemon launched from a different
	// nix generation.
	ManifestDigest string `json:"manifestDigest"`
	// ClientName is purely diagnostic ("projwmctl", "sidecar:wake", ...).
	ClientName string `json:"clientName"`
	// ClientVersion is diagnostic only; not used for compatibility checks.
	ClientVersion string `json:"clientVersion,omitempty"`
}

// Welcome is the daemon's accept response.
type Welcome struct {
	ProtocolVersion    string         `json:"protocolVersion"`
	StoreSchemaVersion string         `json:"storeSchemaVersion"`
	ManifestDigest     string         `json:"manifestDigest"`
	DaemonVersion      string         `json:"daemonVersion"`
	CurrentGeneration  w.GenerationID `json:"currentGeneration"`
	Epoch              w.Epoch        `json:"epoch"`
}

// Reject is the daemon's typed rejection of a handshake.
type Reject struct {
	Error Error `json:"error"`
	// Expected and Got carry the daemon-side values that the client failed to
	// match, so error messages can be precise without leaking secrets.
	Expected map[string]string `json:"expected,omitempty"`
	Got      map[string]string `json:"got,omitempty"`
}

// IntentRequest carries one user intent. design.md §9.1.
//
// We carry both an intent.Kind and the typed payload (per-kind) because the
// intent.Intent interface is not directly JSON-serializable.
type IntentRequest struct {
	RequestID string      `json:"requestId"`
	Kind      intent.Kind `json:"kind"`
	// Payload is the kind-specific body. Concrete shape is one of the
	// IntentPayload* structs below; daemon decodes by Kind.
	Payload json.RawMessage `json:"payload"`
}

// IntentResponse is returned after the controller transaction settles.
type IntentResponse struct {
	RequestID           string           `json:"requestId"`
	AcceptedTransaction *w.TransactionID `json:"acceptedTransaction,omitempty"`
	CommittedGeneration *w.GenerationID  `json:"committedGeneration,omitempty"`
	FinalFocusWorkspace *w.WorkspaceID   `json:"finalFocusWorkspace,omitempty"`
	Error               *Error           `json:"error,omitempty"`
}

// EventHint is what a sidecar sends to the daemon. impl-design.md §5 sidecar limits.
// sidecar は EventHint だけを送れる。store mutate / adapter call は禁止。
type EventHint struct {
	HintID string       `json:"hintId"`
	Source event.Source `json:"source"`
	Kind   event.Kind   `json:"kind"`
	// Epoch lets production sidecars preserve the epoch attached to the
	// observed event so the daemon can discard stale hints.
	Epoch w.Epoch `json:"epoch,omitempty"`
	// Body is opaque kind-specific data; daemon (not sidecar) decides authority.
	Body json.RawMessage `json:"body,omitempty"`
}

// EventAck confirms the daemon accepted (or dropped) an EventHint.
// EventHint は authority=hint なので daemon が evidence へ昇格するかは
// reducer 側で判定する。
type EventAck struct {
	HintID              string           `json:"hintId"`
	AcceptedTransaction *w.TransactionID `json:"acceptedTransaction,omitempty"`
	CommittedGeneration *w.GenerationID  `json:"committedGeneration,omitempty"`
	Dropped             bool             `json:"dropped,omitempty"`
	Error               *Error           `json:"error,omitempty"`
}

// QueryKind discriminates QueryRequest payloads.
type QueryKind string

const (
	QueryWorld       QueryKind = "world"        // current generation snapshot
	QueryProfiles    QueryKind = "profiles"     // profile list
	QueryArchive     QueryKind = "archive"      // archived projects
	QueryCards       QueryKind = "cards"        // active cards
	QueryTrace       QueryKind = "trace"        // latest trace or named trace
	QueryDoctor      QueryKind = "doctor"       // serverside doctor (Phase 4)
	QueryPlanPreview QueryKind = "plan-preview" // planner-only run for `projwm reconcile --dry-run` (§5.9)
)

// QueryRequest is the client's read-only request.
type QueryRequest struct {
	RequestID string    `json:"requestId"`
	Kind      QueryKind `json:"kind"`
	// TraceID is meaningful for QueryTrace; empty means "latest".
	TraceID string `json:"traceId,omitempty"`
}

// QueryResponse is the daemon's read-only response.
//
// Body is QueryKind-specific JSON. Schemas live in the cockpit/CLI
// rendering code; daemon uses pass-through JSON to keep handshake free
// of huge generated structs.
type QueryResponse struct {
	RequestID string          `json:"requestId"`
	Body      json.RawMessage `json:"body,omitempty"`
	Error     *Error          `json:"error,omitempty"`
}

// SubscribeRequest opens a push stream. The daemon emits SubscriptionPush
// envelopes for each event in Kinds until the client sends
// MsgSubscriptionCancel or closes the connection.
type SubscribeRequest struct {
	RequestID string   `json:"requestId"`
	Kinds     []string `json:"kinds"` // e.g. ["card-added","card-removed","generation-committed"]
}

// SubscribeAck confirms the subscribe (or returns an Error).
type SubscribeAck struct {
	RequestID string `json:"requestId"`
	Error     *Error `json:"error,omitempty"`
}

// SubscriptionPush is one event delivered to the subscriber.
type SubscriptionPush struct {
	Kind string          `json:"kind"`
	Body json.RawMessage `json:"body,omitempty"`
	// Generation lets the client correlate the push to a committed
	// generation when the event was triggered by an intent / lifecycle.
	Generation string `json:"generation,omitempty"`
}

// SubscriptionCancel asks the daemon to stop pushing.
type SubscriptionCancel struct {
	RequestID string `json:"requestId"`
}

// CheckHandshake validates a Hello against daemon-side expected values and
// returns a Reject (or nil if compatible).
//
// daemonManifestDigest is the digest the daemon computed at startup from the
// manifest it loaded; client must match exactly. specs.md §2 single-writer
// requires the daemon and CLI to agree on environment.
func CheckHandshake(h Hello, daemonManifestDigest string) *Reject {
	if h.ProtocolVersion != ProtocolVersion {
		return &Reject{
			Error: Error{
				Code:    ErrProtocolMismatch,
				Message: "protocol version mismatch",
			},
			Expected: map[string]string{"protocolVersion": ProtocolVersion},
			Got:      map[string]string{"protocolVersion": h.ProtocolVersion},
		}
	}
	if h.StoreSchemaVersion != StoreSchemaVersion {
		return &Reject{
			Error: Error{
				Code:    ErrProtocolMismatch,
				Message: "store schema version mismatch",
			},
			Expected: map[string]string{"storeSchemaVersion": StoreSchemaVersion},
			Got:      map[string]string{"storeSchemaVersion": h.StoreSchemaVersion},
		}
	}
	if daemonManifestDigest == "" {
		return &Reject{
			Error: Error{
				Code:    ErrProtocolMismatch,
				Message: "daemon manifest digest missing",
			},
			Expected: map[string]string{"manifestDigest": "non-empty"},
			Got:      map[string]string{"manifestDigest": daemonManifestDigest},
		}
	}
	if h.ManifestDigest == "" {
		return &Reject{
			Error: Error{
				Code:    ErrProtocolMismatch,
				Message: "client manifest digest missing",
			},
			Expected: map[string]string{"manifestDigest": daemonManifestDigest},
			Got:      map[string]string{"manifestDigest": ""},
		}
	}
	if h.ManifestDigest != daemonManifestDigest {
		return &Reject{
			Error: Error{
				Code:    ErrProtocolMismatch,
				Message: "manifest digest mismatch (client and daemon disagree on environment)",
			},
			Expected: map[string]string{"manifestDigest": daemonManifestDigest},
			Got:      map[string]string{"manifestDigest": h.ManifestDigest},
		}
	}
	return nil
}

// NewEnvelope is a convenience constructor for typed payloads.
func NewEnvelope(t MessageType, payload any) (Envelope, error) {
	if payload == nil {
		return Envelope{Type: t}, nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("ipc: marshal %s: %w", t, err)
	}
	return Envelope{Type: t, Payload: b}, nil
}

// WriteEnvelope writes one newline-delimited JSON envelope.
func WriteEnvelope(w io.Writer, env Envelope) error {
	if err := json.NewEncoder(w).Encode(env); err != nil {
		return fmt.Errorf("ipc: write %s: %w", env.Type, err)
	}
	return nil
}

// ReadEnvelope reads one newline-delimited JSON envelope.
func ReadEnvelope(r io.Reader) (Envelope, error) {
	var env Envelope
	if err := json.NewDecoder(r).Decode(&env); err != nil {
		return Envelope{}, fmt.Errorf("ipc: read envelope: %w", err)
	}
	if env.Type == "" {
		return Envelope{}, fmt.Errorf("ipc: read envelope: missing type")
	}
	return env, nil
}

// DecodeIntent decodes the kind-specific payload carried by IntentRequest.
func DecodeIntent(req IntentRequest) (intent.Intent, error) {
	switch req.Kind {
	case intent.KindSwitchProfile:
		var v intent.SwitchProfile
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode switch-profile: %w", err)
		}
		return v, nil
	case intent.KindArchiveProject:
		var v intent.ArchiveProject
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode archive-project: %w", err)
		}
		return v, nil
	case intent.KindUnarchiveProject:
		var v intent.UnarchiveProject
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode unarchive-project: %w", err)
		}
		return v, nil
	case intent.KindAssignProject:
		var v intent.AssignProject
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode assign-project: %w", err)
		}
		return v, nil
	case intent.KindUnassignSlot:
		var v intent.UnassignSlot
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode unassign-slot: %w", err)
		}
		return v, nil
	case intent.KindReconcile:
		var v intent.Reconcile
		if len(req.Payload) > 0 {
			if err := json.Unmarshal(req.Payload, &v); err != nil {
				return nil, fmt.Errorf("ipc: decode reconcile: %w", err)
			}
		}
		return v, nil
	case intent.KindValidateEnvironment:
		var v intent.ValidateEnvironment
		if len(req.Payload) > 0 {
			if err := json.Unmarshal(req.Payload, &v); err != nil {
				return nil, fmt.Errorf("ipc: decode validate-environment: %w", err)
			}
		}
		return v, nil
	case intent.KindCreateProject:
		var v intent.CreateProject
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode create-project: %w", err)
		}
		return v, nil
	case intent.KindDeleteProject:
		var v intent.DeleteProject
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode delete-project: %w", err)
		}
		return v, nil
	case intent.KindAddWindow:
		var v intent.AddWindow
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode add-window: %w", err)
		}
		return v, nil
	case intent.KindRemoveWindow:
		var v intent.RemoveWindow
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode remove-window: %w", err)
		}
		return v, nil
	case intent.KindCreateProfile:
		var v intent.CreateProfile
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode create-profile: %w", err)
		}
		return v, nil
	case intent.KindDeleteProfile:
		var v intent.DeleteProfile
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode delete-profile: %w", err)
		}
		return v, nil
	case intent.KindRenameProfile:
		var v intent.RenameProfile
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode rename-profile: %w", err)
		}
		return v, nil
	case intent.KindAdoptOrphanWindow:
		var v intent.AdoptOrphanWindow
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode adopt-orphan-window: %w", err)
		}
		return v, nil
	case intent.KindDismissOrphanWindow:
		var v intent.DismissOrphanWindow
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode dismiss-orphan-window: %w", err)
		}
		return v, nil
	case intent.KindAutoSyncLayout:
		var v intent.AutoSyncLayout
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode auto-sync-layout: %w", err)
		}
		return v, nil
	case intent.KindDismissCard:
		var v intent.DismissCard
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode dismiss-card: %w", err)
		}
		return v, nil
	case intent.KindDismissAllCards:
		var v intent.DismissAllCards
		if len(req.Payload) > 0 {
			if err := json.Unmarshal(req.Payload, &v); err != nil {
				return nil, fmt.Errorf("ipc: decode dismiss-all-cards: %w", err)
			}
		}
		return v, nil
	case intent.KindSetCockpitVisibility:
		var v intent.SetCockpitVisibility
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode set-cockpit-visibility: %w", err)
		}
		return v, nil
	case intent.KindSyncCockpitSystemWindows:
		var v intent.SyncCockpitSystemWindows
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode sync-cockpit-system-windows: %w", err)
		}
		return v, nil
	case intent.KindBrowserAddTab:
		var v intent.BrowserAddTab
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode browser-add-tab: %w", err)
		}
		return v, nil
	case intent.KindBrowserRemoveTab:
		var v intent.BrowserRemoveTab
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode browser-remove-tab: %w", err)
		}
		return v, nil
	case intent.KindBrowserChangeTabURL:
		var v intent.BrowserChangeTabURL
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode browser-change-tab-url: %w", err)
		}
		return v, nil
	case intent.KindBrowserReorderTabs:
		var v intent.BrowserReorderTabs
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode browser-reorder-tabs: %w", err)
		}
		return v, nil
	case intent.KindCycleSlotWindow:
		var v intent.CycleSlotWindow
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode cycle-slot-window: %w", err)
		}
		return v, nil
	case intent.KindSwitchProject:
		var v intent.SwitchProject
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode switch-project: %w", err)
		}
		return v, nil
	case intent.KindSummonShell:
		var v intent.SummonShell
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode summon-shell: %w", err)
		}
		return v, nil
	case intent.KindSummonEditor:
		var v intent.SummonEditor
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode summon-editor: %w", err)
		}
		return v, nil
	case intent.KindSummonBrowser:
		var v intent.SummonBrowser
		if err := json.Unmarshal(req.Payload, &v); err != nil {
			return nil, fmt.Errorf("ipc: decode summon-browser: %w", err)
		}
		return v, nil
	case intent.KindSummonViewer:
		var v intent.SummonViewer
		if len(req.Payload) > 0 && string(req.Payload) != "null" {
			if err := json.Unmarshal(req.Payload, &v); err != nil {
				return nil, fmt.Errorf("ipc: decode summon-viewer: %w", err)
			}
		}
		return v, nil
	case intent.KindShowScratchShell:
		var v intent.ShowScratchShell
		if len(req.Payload) > 0 && string(req.Payload) != "null" {
			if err := json.Unmarshal(req.Payload, &v); err != nil {
				return nil, fmt.Errorf("ipc: decode show-scratch-shell: %w", err)
			}
		}
		return v, nil
	case intent.KindHideScratchShell:
		var v intent.HideScratchShell
		if len(req.Payload) > 0 && string(req.Payload) != "null" {
			if err := json.Unmarshal(req.Payload, &v); err != nil {
				return nil, fmt.Errorf("ipc: decode hide-scratch-shell: %w", err)
			}
		}
		return v, nil
	default:
		return nil, fmt.Errorf("ipc: unsupported intent kind %q", req.Kind)
	}
}
