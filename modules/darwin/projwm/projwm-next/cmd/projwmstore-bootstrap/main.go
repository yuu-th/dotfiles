// projwmstore-bootstrap initializes the production PersistentStore through the
// store API before projwmd starts. It is an explicit admin/migration path, not a
// daemon-side DesiredWorld injection.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/migration"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func loadDesiredWorld(path string) (w.DesiredWorld, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return w.DesiredWorld{}, fmt.Errorf("read desired world %s: %w", path, err)
	}
	var desired w.DesiredWorld
	if err := json.Unmarshal(b, &desired); err != nil {
		return w.DesiredWorld{}, fmt.Errorf("parse desired world %s: %w", path, err)
	}
	if desired.ActiveProfile == "" {
		return w.DesiredWorld{}, fmt.Errorf("desired world missing ActiveProfile")
	}
	if len(desired.Profiles) == 0 {
		return w.DesiredWorld{}, fmt.Errorf("desired world missing Profiles")
	}
	if desired.Projects == nil {
		desired.Projects = map[w.ProjectID]w.DesiredProject{}
	}
	if desired.FocusPolicy.FinalFocus == nil {
		desired.FocusPolicy.FinalFocus = map[string]w.WorkspaceID{}
	}
	if desired.AcceptedLayouts == nil {
		desired.AcceptedLayouts = map[w.ProjectID]map[w.WorkspaceID]w.DesiredLayout{}
	}
	return desired, nil
}

func main() {
	storeDir := flag.String("store-dir", "", "production PersistentStore directory")
	desiredPath := flag.String("desired-world", "", "initial DesiredWorld JSON for first production generation")
	legacyStatePath := flag.String("legacy-state", "", "legacy projwm state.json to migrate into the first production generation")
	manifestPath := flag.String("managed-environment", "", "managed environment manifest path to bind the production store bootstrap")
	manifestDigest := flag.String("manifest-digest", "", "managed environment manifest digest to bind the production store bootstrap")
	flag.Parse()

	if *storeDir == "" {
		fmt.Fprintln(os.Stderr, "projwmstore-bootstrap: --store-dir is required")
		os.Exit(2)
	}
	if (*desiredPath == "") == (*legacyStatePath == "") {
		fmt.Fprintln(os.Stderr, "projwmstore-bootstrap: exactly one of --desired-world or --legacy-state is required")
		os.Exit(2)
	}

	var (
		desired       w.DesiredWorld
		report        *migration.LegacyReport
		privateReport *migration.LegacyPrivatePayloadReport
		err           error
	)
	if *desiredPath != "" {
		desired, err = loadDesiredWorld(*desiredPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "projwmstore-bootstrap: %v\n", err)
			os.Exit(1)
		}
	} else {
		desired, err = loadLegacyDesiredWorld(context.Background(), *storeDir, *legacyStatePath, &report, &privateReport)
		if err != nil {
			fmt.Fprintf(os.Stderr, "projwmstore-bootstrap: %v\n", err)
			os.Exit(1)
		}
	}
	digest, err := bootstrapManifestDigest(*manifestPath, *manifestDigest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmstore-bootstrap: %v\n", err)
		os.Exit(1)
	}
	fs, err := store.OpenFileStoreWithBootstrapTrace(context.Background(), *storeDir, store.StoreKindProduction, desired, bootstrapTrace(*desiredPath, *legacyStatePath, digest))
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmstore-bootstrap: open store: %v\n", err)
		os.Exit(1)
	}
	current, err := fs.LoadCurrentGeneration(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "projwmstore-bootstrap: load current generation: %v\n", err)
		os.Exit(1)
	}
	if report != nil {
		fmt.Printf("ok generation=%s store-kind=%s migrated-profiles=%d migrated-projects=%d migrated-windows=%d missing-project-roots=%d browser-url-records=%d private-payload-discovered=%d private-payload-migrated=%d private-payload-skipped-invalid=%d private-payload-dropped-browser-windows=%d private-payload-committed-raw-urls=%d\n",
			current.ID, store.StoreKindProduction, report.Profiles, report.Projects, report.Windows, report.MissingProjectRoots, report.BrowserURLRecords, privateReport.Discovered, privateReport.MigratedToPrivatePayload, privateReport.SkippedInvalid, privateReport.DroppedBrowserWindowsWithoutPayload, privateReport.CommittedRawURLs)
		return
	}
	fmt.Printf("ok generation=%s store-kind=%s\n", current.ID, store.StoreKindProduction)
}

func bootstrapTrace(desiredPath, legacyStatePath, manifestDigest string) store.TransactionTrace {
	trace := store.TransactionTrace{
		Reason:                  "admin-bootstrap",
		TriggerSource:           "admin",
		TriggerKind:             "desired-world-bootstrap",
		BootstrapManifestDigest: manifestDigest,
	}
	if legacyStatePath != "" {
		trace.TriggerKind = "legacy-state-migration"
	}
	return trace
}

func bootstrapManifestDigest(manifestPath, manifestDigest string) (string, error) {
	if manifestDigest != "" {
		return manifestDigest, nil
	}
	if manifestPath == "" {
		return "", nil
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", fmt.Errorf("read managed environment manifest %s: %w", manifestPath, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func loadLegacyDesiredWorld(ctx context.Context, storeDir string, legacyStatePath string, reportOut **migration.LegacyReport, privateReportOut **migration.LegacyPrivatePayloadReport) (w.DesiredWorld, error) {
	data, err := os.ReadFile(legacyStatePath)
	if err != nil {
		return w.DesiredWorld{}, fmt.Errorf("read legacy state %s: %w", legacyStatePath, err)
	}
	privateDir := privatePayloadStoreDir(storeDir)
	privateStore, err := browser.NewFilePrivatePayloadStore(privateDir)
	if err != nil {
		return w.DesiredWorld{}, err
	}
	desired, report, privateReport, err := migration.DesiredWorldFromLegacyStateWithPrivatePayloadStore(ctx, data, privateStore)
	if err != nil {
		return w.DesiredWorld{}, err
	}
	if err := quarantineLegacyInput(storeDir, privateDir, data, report, privateReport); err != nil {
		return w.DesiredWorld{}, err
	}
	*reportOut = &report
	*privateReportOut = &privateReport
	return desired, nil
}

func privatePayloadStoreDir(storeDir string) string {
	return filepath.Join(filepath.Dir(storeDir), "private-payloads")
}

func quarantineLegacyInput(storeDir string, privateDir string, legacyState []byte, report migration.LegacyReport, privateReport migration.LegacyPrivatePayloadReport) error {
	name := "legacy-state-" + time.Now().UTC().Format("20060102T150405Z")
	dir := filepath.Join(storeDir, "quarantine", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create legacy quarantine: %w", err)
	}
	privateInputDir := filepath.Join(privateDir, "legacy-input-quarantine", name)
	if err := os.MkdirAll(privateInputDir, 0o700); err != nil {
		return fmt.Errorf("create private legacy quarantine: %w", err)
	}
	if err := os.WriteFile(filepath.Join(privateInputDir, "state.json"), legacyState, 0o600); err != nil {
		return fmt.Errorf("write private legacy quarantine state: %w", err)
	}
	reason := struct {
		Migration      migration.LegacyReport               `json:"migration"`
		PrivatePayload migration.LegacyPrivatePayloadReport `json:"privatePayload"`
		PrivateInput   string                               `json:"privateInput"`
	}{
		Migration:      report,
		PrivatePayload: privateReport,
		PrivateInput:   "private-payloads/legacy-input-quarantine/" + name + "/state.json",
	}
	reportData, err := json.MarshalIndent(reason, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal legacy migration report: %w", err)
	}
	reportData = append(reportData, '\n')
	if err := os.WriteFile(filepath.Join(dir, "reason.json"), reportData, 0o600); err != nil {
		return fmt.Errorf("write legacy migration report: %w", err)
	}
	return nil
}
