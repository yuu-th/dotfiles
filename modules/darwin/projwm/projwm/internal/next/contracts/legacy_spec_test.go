package contracts

import "testing"

func TestAllLegacyIntegrationValuesAreMappedToNextLayers(t *testing.T) {
	required := map[string]bool{
		"T01":               false,
		"T02":               false,
		"T03-T05":           false,
		"T06":               false,
		"T07-T08":           false,
		"T09":               false,
		"T10":               false,
		"T11":               false,
		"T12":               false,
		"T13-T17":           false,
		"T18":               false,
		"T19":               false,
		"T20":               false,
		"T21":               false,
		"TF-Snapshot":       false,
		"TF-ViewerOrder":    false,
		"TF-StatusZeroDiff": false,
	}
	for _, spec := range LegacySpecs {
		if _, ok := required[spec.ID]; !ok {
			t.Fatalf("unexpected legacy spec ID %q", spec.ID)
		}
		required[spec.ID] = true
		if spec.Value == "" {
			t.Fatalf("%s has empty user-visible value", spec.ID)
		}
		if len(spec.Layers) == 0 {
			t.Fatalf("%s has no next-layer contract mapping", spec.ID)
		}
	}
	for id, seen := range required {
		if !seen {
			t.Fatalf("legacy integration value %s is not mapped", id)
		}
	}
}

func TestEveryLegacySpecHasIntegrationOrExplicitNonGUIContract(t *testing.T) {
	for _, spec := range LegacySpecs {
		if hasLayer(spec, LayerIntegration) {
			continue
		}
		switch spec.ID {
		case "T09", "T12":
			// T09 is explicitly state-only. T12's old destructive live close is
			// replaced by DesiredWorld removal through single-writer/store contracts.
		default:
			t.Fatalf("%s lacks real integration coverage", spec.ID)
		}
	}
}

func TestMutationRelatedLegacySpecsRequireMutationSafetyLayer(t *testing.T) {
	for _, id := range []string{
		"T01", "T03-T05", "T06", "T07-T08", "T10", "T11", "T12",
		"T18", "T19", "T20", "T21", "TF-Snapshot",
	} {
		spec := findSpec(t, id)
		if !hasLayer(spec, LayerMutation) {
			t.Fatalf("%s must map to mutation-safety layer", id)
		}
	}
}

func TestBrowserPrivacyIsPartOfZeroDiffDiagnosticsContract(t *testing.T) {
	spec := findSpec(t, "TF-StatusZeroDiff")
	if !hasLayer(spec, LayerBrowserPrivacy) {
		t.Fatal("status/diagnostics zero-diff contract must include browser privacy")
	}
}

func findSpec(t *testing.T, id string) LegacySpec {
	t.Helper()
	for _, spec := range LegacySpecs {
		if spec.ID == id {
			return spec
		}
	}
	t.Fatalf("missing spec %s", id)
	return LegacySpec{}
}

func hasLayer(spec LegacySpec, layer Layer) bool {
	for _, candidate := range spec.Layers {
		if candidate == layer {
			return true
		}
	}
	return false
}
