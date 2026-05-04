// cmd/layout_lifecycle.go: column / stack layout の snapshot / restore を
// archive / unarchive / profile-switch から呼ぶための薄い helper 群。
//
// snapshot タイミング:
//   - archive 直前 (close 前)
//   - profile-switch で active から外れる project (close 前、 mutate 前)
//
// restore タイミング:
//   - unarchive (reconcile + browser spawn 完了後)
//   - profile-switch で active に入る project (reconcile + browser spawn 完了後)
//
// 妥協: layout snapshot は project の slot ws のみ。 viewer ws (A) や browser
// ws (B) は共有 ws のため project 単位 snapshot しない。
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/naming"
	"github.com/yuu-th/projwm/internal/omniwm"
	"github.com/yuu-th/projwm/internal/reconcile"
	"github.com/yuu-th/projwm/internal/state"
)

// snapshotLayoutForProject は project の slot ws の column/stack 配置を
// snapshot し、 state.json に persist する。 失敗は warn ログのみ。
func snapshotLayoutForProject(profileName, projectName string) {
	cfgRes, err := config.LoadFromDefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: layout snapshot %q: load config: %v\n", projectName, err)
		return
	}
	s, st, err := loadStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: layout snapshot %q: load store: %v\n", projectName, err)
		return
	}
	slot := findSlotInProfile(st, profileName, projectName)
	fmt.Fprintf(os.Stderr, "layout snapshot %q: profile=%q slot=%q\n", projectName, profileName, slot)
	if slot == "" {
		return
	}
	r := reconcile.New(cfgRes.Config)
	r.Chromium.Logger = os.Stderr
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Mutate(func(st *state.State) error {
		p, ok := st.Projects[projectName]
		if !ok {
			return nil
		}
		if err := r.SnapshotProjectLayout(ctx, projectName, &p, slot, os.Stderr); err != nil {
			return err
		}
		st.Projects[projectName] = p
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: layout snapshot %q on slot %s: %v\n", projectName, slot, err)
	} else {
		fmt.Fprintf(os.Stderr, "layout snapshot %q on slot %s: ok\n", projectName, slot)
	}
}

// restoreLayoutForProject は spawn 完了済の project に対し Layout に従う rearrange を行う。
//
// expected window 数が揃うまで短時間 polling、 揃わなくても期限で諦め (best-effort)。
func restoreLayoutForProject(profileName, projectName string) {
	cfgRes, err := config.LoadFromDefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: layout restore %q: load config: %v\n", projectName, err)
		return
	}
	_, st, err := loadStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: layout restore %q: load store: %v\n", projectName, err)
		return
	}
	slot := findSlotInProfile(st, profileName, projectName)
	if slot == "" {
		return
	}
	p, ok := st.Projects[projectName]
	if !ok {
		return
	}
	expected := 0
	for _, w := range p.Windows {
		if w.Layout != nil && w.Kind != naming.KindBrowser {
			expected++
		}
	}
	if expected == 0 {
		return
	}
	r := reconcile.New(cfgRes.Config)
	r.Chromium.Logger = os.Stderr
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Fprintf(os.Stderr, "restore: waiting for %d windows on slot %q\n", expected, slot)
	// Wait until all expected windows are present AND stable (confirmed twice).
	// Zed's placeZedAfterSpawn goroutine can cause brief workspace transitions,
	// so a single match is not reliable — require the count to hold for 2 polls.
	deadline := time.Now().Add(35 * time.Second)
	stableCount := 0
	for time.Now().Before(deadline) {
		wins, err := r.OmniWM.QueryWindows(ctx, "--workspace", slot)
		if err != nil {
			break
		}
		matched := 0
		for _, w := range p.Windows {
			if w.Layout == nil || w.Kind == naming.KindBrowser {
				continue
			}
			for _, lw := range wins {
				if matchProjectWindow(w, lw, projectName) {
					matched++
					break
				}
			}
		}
		fmt.Fprintf(os.Stderr, "restore: %d/%d windows on ws %q\n", matched, expected, slot)
		if matched >= expected {
			stableCount++
			if stableCount >= 2 {
				break // stable for 2 consecutive polls → proceed
			}
		} else {
			stableCount = 0
		}
		time.Sleep(200 * time.Millisecond)
	}

	if err := r.RestoreProjectLayout(ctx, projectName, p, slot, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: layout restore %q on slot %s: %v\n", projectName, slot, err)
	} else {
		fmt.Fprintf(os.Stderr, "layout restore %q on slot %s: ok\n", projectName, slot)
	}
}

// fixViewerOrderForProfile は viewer WS の ai-view 窓を slot_names 順に並べ直す。
//
// reconcile の viewer spawn は非同期競合のため column 順が不定になる場合がある。
// profile switch の全 restore 完了後に呼んで viewer を Q→W→E→... の順に整列する。
func fixViewerOrderForProfile(profileName string) {
	cfgRes, err := config.LoadFromDefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: viewer order %q: load config: %v\n", profileName, err)
		return
	}
	_, st, err := loadStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: viewer order %q: load store: %v\n", profileName, err)
		return
	}
	prof, ok := st.Profiles[profileName]
	if !ok {
		return
	}
	// slot_names 順で viewer title のリストを作成
	var slotOrder []string
	for _, slot := range cfgRes.Config.SlotNames {
		projName, ok := prof.Assignments[slot]
		if !ok {
			continue
		}
		p, ok := st.Projects[projName]
		if !ok {
			continue
		}
		for _, w := range p.Windows {
			if w.Kind == naming.KindAI {
				slotOrder = append(slotOrder, naming.ViewerGhosttyTitle(w.ID, projName))
			}
		}
	}
	if len(slotOrder) == 0 {
		return
	}

	r := reconcile.New(cfgRes.Config)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 全 viewer が WS A に出現するまで待つ
	viewerWS := cfgRes.Config.ViewerWorkspace
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		wins, err := r.OmniWM.QueryWindows(ctx, "--workspace", viewerWS)
		if err != nil {
			break
		}
		found := 0
		for _, title := range slotOrder {
			for _, w := range wins {
				if w.Title == title {
					found++
					break
				}
			}
		}
		fmt.Fprintf(os.Stderr, "viewer order: %d/%d viewers on ws %q\n", found, len(slotOrder), viewerWS)
		if found >= len(slotOrder) {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	if err := r.FixViewerOrder(ctx, viewerWS, slotOrder, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: viewer order fix: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "viewer order fix: ok\n")
	}
}

// matchProjectWindow は state.Window ↔ omniwm.Window の照合 (cmd 層 polling 用)。
// reconcile 内部の matchStateToLive と同等。
func matchProjectWindow(sw state.Window, lw omniwm.Window, projectName string) bool {
	switch sw.Kind {
	case naming.KindAI, naming.KindShell:
		return lw.BundleID == naming.TerminalBundleID &&
			lw.Title == naming.GhosttyTitle(sw.Kind, sw.ID, projectName)
	case naming.KindEditor:
		return lw.BundleID == naming.ZedBundleID && lw.Title == projectName
	case naming.KindBrowser:
		return sw.LiveWindowID != "" && sw.LiveWindowID == lw.ID
	}
	return false
}
