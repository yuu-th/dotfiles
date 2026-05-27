package manifest

import (
	"strings"
	"testing"
)

func validManifestJSON() string {
	return `{
		"schemaVersion": 1,
		"source": "nix",
		"environment": {
			"wmBackend": "omniwm",
			"viewerWorkspace": "A",
			"slotWorkspaces": ["Q", "W", "E"]
		},
		"authorities": [
			{"resource": "environment", "owner": "nix"},
			{"resource": "desiredWorld", "owner": "persistent-store"},
			{"resource": "observedWorld", "owner": "observer"}
		],
		"writers": [
			{"name": "projwmd", "kind": "normal-mutator", "resources": ["gui", "desiredWorld", "browser"]}
		]
	}`
}

func TestValidMinimalManifest(t *testing.T) {
	if _, err := Decode(strings.NewReader(validManifestJSON())); err != nil {
		t.Fatal(err)
	}
}

func TestMissingRequiredFieldRejected(t *testing.T) {
	raw := strings.Replace(validManifestJSON(), `"schemaVersion": 1,`, "", 1)
	if _, err := Decode(strings.NewReader(raw)); err == nil {
		t.Fatal("expected missing schemaVersion to be rejected")
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	raw := strings.Replace(validManifestJSON(), `"source": "nix",`, `"source": "nix", "futureField": true,`, 1)
	if _, err := Decode(strings.NewReader(raw)); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func TestDesiredWorldInManifestRejected(t *testing.T) {
	raw := strings.Replace(validManifestJSON(), `"source": "nix",`, `"source": "nix", "desiredWorld": {},`, 1)
	if _, err := Decode(strings.NewReader(raw)); err == nil {
		t.Fatal("manifest must not contain DesiredWorld")
	}
}

func TestLiveStateInManifestRejected(t *testing.T) {
	raw := strings.Replace(validManifestJSON(), `"source": "nix",`, `"source": "nix", "liveWindowID": "ow_1",`, 1)
	if _, err := Decode(strings.NewReader(raw)); err == nil {
		t.Fatal("manifest must not contain live state")
	}
}

func TestDuplicateAuthorityOwnerRejected(t *testing.T) {
	raw := strings.Replace(validManifestJSON(),
		`{"resource": "desiredWorld", "owner": "persistent-store"}`,
		`{"resource": "desiredWorld", "owner": "persistent-store"}, {"resource": "desiredWorld", "owner": "nix"}`,
		1,
	)
	if _, err := Decode(strings.NewReader(raw)); err == nil {
		t.Fatal("expected duplicate authority owner to be rejected")
	}
}

func TestConflictingNormalWriterRejected(t *testing.T) {
	raw := strings.Replace(validManifestJSON(),
		`{"name": "projwmd", "kind": "normal-mutator", "resources": ["gui", "desiredWorld", "browser"]}`,
		`{"name": "projwmd", "kind": "normal-mutator", "resources": ["gui"]}, {"name": "projwmctl", "kind": "normal-mutator", "resources": ["gui"]}`,
		1,
	)
	if _, err := Decode(strings.NewReader(raw)); err == nil {
		t.Fatal("expected conflicting normal writer to be rejected")
	}
}

func TestViewerWorkspaceCannotBeSlot(t *testing.T) {
	raw := strings.Replace(validManifestJSON(), `"slotWorkspaces": ["Q", "W", "E"]`, `"slotWorkspaces": ["A", "Q"]`, 1)
	if _, err := Decode(strings.NewReader(raw)); err == nil {
		t.Fatal("expected viewer/slot collision to be rejected")
	}
}
