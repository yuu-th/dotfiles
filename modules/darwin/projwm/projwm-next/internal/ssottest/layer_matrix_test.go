package ssottest

import (
	"sort"
	"strings"
	"testing"
)

type layerMatrixItem struct {
	ID       string
	Section  string
	Subject  string
	Layers   []string
	Conflict string
}

var ssotRequiredLayerMatrix = []layerMatrixItem{
	{ID: "OP-01", Section: "§10.6", Subject: "shell jump", Layers: layers("L1", "L2", "L3", "L4")},
	{ID: "OP-02", Section: "§10.6", Subject: "editor jump", Layers: layers("L1", "L2", "L3", "L4")},
	{ID: "OP-03", Section: "§10.6", Subject: "browser jump", Layers: layers("L1", "L2", "L3", "L4")},
	{ID: "OP-04", Section: "§10.6", Subject: "project switch", Layers: layers("L1", "L4")},
	{ID: "OP-05", Section: "§10.6", Subject: "same-slot window switch", Layers: layers("L1", "L2", "L4")},
	{ID: "OP-06", Section: "§10.6", Subject: "viewer jump", Layers: layers("L1", "L3", "L4")},
	{ID: "OP-07", Section: "§10.6", Subject: "cockpit show/hide", Layers: layers("L1", "L3", "L4")},
	{ID: "OP-08", Section: "§10.6", Subject: "profile switch", Layers: layers("L0", "L1", "L4")},
	{ID: "OP-09", Section: "§10.6", Subject: "project add", Layers: layers("L0", "L1", "L4")},
	{ID: "OP-10", Section: "§10.6", Subject: "project archive/unarchive", Layers: layers("L0", "L1", "L4")},
	{ID: "OP-11", Section: "§10.6", Subject: "scratch shell", Layers: layers("L1", "L3", "L4")},
	{ID: "OP-12", Section: "§10.6", Subject: "add window", Layers: layers("L0", "L1", "L2", "L3", "L4")},
	{ID: "OP-13", Section: "§10.6", Subject: "remove window", Layers: layers("L0", "L1", "L2", "L3", "L4")},
	{ID: "OP-14", Section: "§10.6", Subject: "browser add-tab", Layers: layers("L1", "L4")},
	{ID: "OP-15", Section: "§10.6", Subject: "browser remove-tab", Layers: layers("L1", "L4")},
	{ID: "OP-16", Section: "§10.6", Subject: "browser tab URL change", Layers: layers("L1", "L4")},
	{ID: "OP-17", Section: "§10.6", Subject: "browser tab reorder", Layers: layers("L1", "L4")},
	{ID: "SYS-ALL", Section: "§10.6", Subject: "§4.2 system operations", Layers: layers("L1", "L2", "L3", "L4")},
}

// This is topology inventory only. It checks which layers SSOT §10.6 requires;
// it does not prove that any layer has behavior-level coverage.
func TestSSOTRequiredLayerMatrixIsExactForUserAndSystemOperations(t *testing.T) {
	if len(ssotRequiredLayerMatrix) != 18 {
		t.Fatalf("SSOT §10.6 requires 17 user operation rows plus §4.2 system operations, got %d", len(ssotRequiredLayerMatrix))
	}
	ledgerByID := map[string]ledgerItem{}
	for _, item := range ssotLedger {
		ledgerByID[item.ID] = item
	}

	for _, req := range ssotRequiredLayerMatrix {
		item, ok := ledgerByID[req.ID]
		if !ok {
			t.Fatalf("%s (%s) has no ledger item", req.ID, req.Subject)
		}
		if req.Conflict != "" {
			if item.Status != statusRed {
				t.Fatalf("%s documents an SSOT contradiction and must remain red until the SSOT is amended; got %s", req.ID, item.Status)
			}
			continue
		}
		got := splitLayers(item.Layer)
		if strings.Join(got, "/") != strings.Join(req.Layers, "/") {
			t.Fatalf("%s layer mismatch: got %v, want %v", req.ID, got, req.Layers)
		}
	}
}

func layers(xs ...string) []string {
	out := append([]string(nil), xs...)
	sort.Strings(out)
	return out
}

func splitLayers(s string) []string {
	if s == "" {
		return nil
	}
	out := strings.Split(s, "/")
	sort.Strings(out)
	return out
}
