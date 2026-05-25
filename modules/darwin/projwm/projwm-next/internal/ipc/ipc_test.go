package ipc

import "testing"

func TestCheckHandshake_OK(t *testing.T) {
	h := Hello{
		ProtocolVersion:    ProtocolVersion,
		StoreSchemaVersion: StoreSchemaVersion,
		ManifestDigest:     "abc123",
		ClientName:         "projwmctl",
	}
	if r := CheckHandshake(h, "abc123"); r != nil {
		t.Fatalf("expected accept, got reject: %+v", r)
	}
}

func TestCheckHandshake_ProtocolMismatch(t *testing.T) {
	h := Hello{ProtocolVersion: "projwm-ipc/0", StoreSchemaVersion: StoreSchemaVersion}
	r := CheckHandshake(h, "")
	if r == nil || r.Error.Code != ErrProtocolMismatch {
		t.Fatalf("expected protocol-mismatch reject, got %+v", r)
	}
}

func TestCheckHandshake_StoreSchemaMismatch(t *testing.T) {
	h := Hello{ProtocolVersion: ProtocolVersion, StoreSchemaVersion: "9"}
	r := CheckHandshake(h, "")
	if r == nil || r.Error.Code != ErrProtocolMismatch {
		t.Fatalf("expected protocol-mismatch reject, got %+v", r)
	}
}

func TestCheckHandshake_ManifestDigestMismatch(t *testing.T) {
	h := Hello{
		ProtocolVersion:    ProtocolVersion,
		StoreSchemaVersion: StoreSchemaVersion,
		ManifestDigest:     "client-digest",
	}
	r := CheckHandshake(h, "daemon-digest")
	if r == nil || r.Error.Code != ErrProtocolMismatch {
		t.Fatalf("expected protocol-mismatch reject, got %+v", r)
	}
}

func TestCheckHandshake_ManifestDigestRequired(t *testing.T) {
	h := Hello{
		ProtocolVersion:    ProtocolVersion,
		StoreSchemaVersion: StoreSchemaVersion,
		ManifestDigest:     "client-digest",
	}
	if r := CheckHandshake(h, ""); r == nil || r.Error.Code != ErrProtocolMismatch {
		t.Fatalf("expected missing daemon digest reject, got %+v", r)
	}
	h.ManifestDigest = ""
	if r := CheckHandshake(h, "daemon-digest"); r == nil || r.Error.Code != ErrProtocolMismatch {
		t.Fatalf("expected missing client digest reject, got %+v", r)
	}
}

func TestNewEnvelope_RoundTrip(t *testing.T) {
	hello := Hello{ProtocolVersion: ProtocolVersion, StoreSchemaVersion: StoreSchemaVersion}
	env, err := NewEnvelope(MsgHello, hello)
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}
	if env.Type != MsgHello {
		t.Fatalf("type: got %q want %q", env.Type, MsgHello)
	}
	if len(env.Payload) == 0 {
		t.Fatalf("payload empty")
	}
}
