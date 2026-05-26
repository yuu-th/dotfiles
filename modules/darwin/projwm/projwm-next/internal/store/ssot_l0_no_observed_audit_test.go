package store

import (
	"reflect"
	"strings"
	"testing"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// SSOT §8.1 NO-OBSERVED + GAP-22 PersistentStore 完全性 audit.
//
// CommittedGeneration is the immutable snapshot persisted on every
// commit. SSOT §8.1: "ObservedWorld は保存しない (起動時に observer で
// 再構成する)". This L0 reflection test enforces the contract at the
// type level so future schema changes cannot accidentally introduce
// an Observed field.

func TestSSOTSection81NoObservedFieldInCommittedGeneration(t *testing.T) {
	gen := reflect.TypeOf(CommittedGeneration{})
	for i := 0; i < gen.NumField(); i++ {
		f := gen.Field(i)
		// SSOT §8.1 forbids ObservedWorld persistence — any field whose
		// type name contains "Observed" violates the contract.
		typeName := f.Type.String()
		if strings.Contains(typeName, "Observed") {
			t.Errorf("SSOT §8.1 violation: CommittedGeneration field %s has type %s containing 'Observed'", f.Name, typeName)
		}
		if strings.Contains(strings.ToLower(f.Name), "observed") {
			t.Errorf("SSOT §8.1 violation: CommittedGeneration has field named %s (Observed snapshot must not be persisted)", f.Name)
		}
	}
}

// SSOT §8.1 explicitly enumerates the artifacts that MUST be persisted:
// DesiredWorld, AcceptedLayouts, ControllerCheckpoint (BrowserSnapshots
// are embedded within DesiredWorld's per-window DesiredBrowserSession,
// so they ride along with Desired). This audit fails if any of those
// fields is dropped from CommittedGeneration.
func TestSSOTSection81RequiredArtifactsInCommittedGeneration(t *testing.T) {
	gen := reflect.TypeOf(CommittedGeneration{})
	required := map[string]bool{
		"Desired":         false,
		"AcceptedLayouts": false,
		"Checkpoint":      false,
	}
	for i := 0; i < gen.NumField(); i++ {
		if _, ok := required[gen.Field(i).Name]; ok {
			required[gen.Field(i).Name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Errorf("SSOT §8.1 required artifact missing from CommittedGeneration: %s", name)
		}
	}
}

// Embedding check: SSOT §4.4 BR-PRIV-NOSTORE says URL/cookie/session
// token are NOT in PersistentStore. DesiredBrowserSession (which IS
// persisted via Desired) only carries opaque tokens (PrivatePayloadRef)
// + URLCount. This L0 reflection test asserts the type-level shape.
func TestSSOTSection44BrowserSessionFieldsAreOpaqueRefsOnly(t *testing.T) {
	br := reflect.TypeOf(w.DesiredBrowserSession{})
	// Allowlist of fields that may exist on DesiredBrowserSession.
	allowed := map[string]bool{
		"PrivacyMode":       true,
		"URLPayloadRefs":    true,
		"URLCount":          true,
		"InvalidURLCount":   true,
		"RestoreURLs":       true,
		"RedactionPolicyID": true,
	}
	for i := 0; i < br.NumField(); i++ {
		f := br.Field(i)
		if !allowed[f.Name] {
			t.Errorf("SSOT §4.4 BR-PRIV-NOSTORE: DesiredBrowserSession has unaudited field %s (type %s). If this carries raw URL/cookie/token, move it to PrivatePayloadStore", f.Name, f.Type.String())
		}
	}
	// SSOT §4.4 BR-PRIV-NOSTORE concrete constraint: URLPayloadRefs must
	// be the only URL-bearing field. The struct must not gain a `URLs
	// []string` or similar raw-content field.
	urlPayloadRefsField, hasField := br.FieldByName("URLPayloadRefs")
	if !hasField {
		t.Fatal("URLPayloadRefs field absent — DesiredBrowserSession lost its opaque-token storage")
	}
	if !strings.Contains(urlPayloadRefsField.Type.String(), "PrivatePayloadRef") {
		t.Errorf("URLPayloadRefs is not a slice of PrivatePayloadRef; raw strings would leak URLs into the store. Got: %s", urlPayloadRefsField.Type.String())
	}
}
