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

// browser ライフサイクルの focus 切替戦略 (paradigm C, 再設計版):
//
// OmniWM の API 制約: window-id 直接で move する API なし、
// move-column-to-workspace は **focused column** 前提なので focus 切替が必須。
//
// **理想形**: spawn 前に target slot に focus 切替 → spawn → OmniWM auto-place で
// 新 window が active ws (= slot) に置かれる → defer で origWS 戻す。
//
// 過去の罠 (撤去済):
//   - reconcile.Run で focus 切替 → launchd watch が fire → 無限ループ
//   - 多段 Activate (0/0.5/1/1.5s) → 遅延 grab で cmd 完了後も focus が戻ってくる
//   - 各 spawnBrowserWindowsForProject で個別 origWS save/restore →
//     複数 project 一括処理時の race で W 移動失敗
//
// 現設計:
//   - 単一 project (add-browser / unarchive 等): spawnBrowserWindowsForProject 内で
//     1 回だけ focus 切替 → spawn → 戻す
//   - 複数 project (profile switch): withFocusBatch で全 spawn を 1 つの origWS
//     save/restore session で囲む。 各 project の slot 切替は内部で順次。

// slotProjectPair は wrapper 用の slot+project 組。
type slotProjectPair struct {
	Slot    string
	Project string
}

// findSlotInProfile は profile の Assignments から project の slot を返す (canonical 順)。
func findSlotInProfile(st *state.State, profileName, projectName string) string {
	prof, ok := st.Profiles[profileName]
	if !ok {
		return ""
	}
	for _, slot := range []string{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"} {
		if prof.Assignments[slot] == projectName {
			return slot
		}
	}
	for s, p := range prof.Assignments {
		if p == projectName {
			return s
		}
	}
	return ""
}

// withFocusBatch は origWS を保存し、 各 item の slot に順次 focus → fn を実行
// → 全完了後に origWS 戻す。 wrapper 内の fn では更なる focus 切替をしない。
//
// 単一 / 複数 project 両方対応 (items=1 でも安全)。
// items の Slot が "" の要素は focus 切替 skip。
func withFocusBatch(ctx context.Context, oc *omniwm.Client, items []slotProjectPair, fn func(slotProjectPair) error) error {
	if oc == nil || len(items) == 0 {
		// focus 管理なしで実行
		var firstErr error
		for _, it := range items {
			if err := fn(it); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
	var origWS string
	if w, e := oc.ActiveWorkspaceName(ctx); e == nil {
		origWS = w
	}
	defer func() {
		if origWS != "" {
			_ = oc.FocusWorkspaceByName(ctx, origWS)
		}
	}()
	var firstErr error
	for _, it := range items {
		if it.Slot != "" {
			_ = oc.FocusWorkspaceByName(ctx, it.Slot)
		}
		if err := fn(it); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// browser ライフサイクル helper（v12 paradigm C, queue/projwm-spec.md FR-29）。
//
// reconcile.Run は browser に **触らない** 設計なので、cmd 層の明示イベントから
// これらを呼ぶ。launchd auto-reconcile では絶対に発火しない。

// spawnBrowserWindowsForProject は project の browser kind windows を全て spawn する。
// add-browser / unarchive / profile assign 等の **単一 project** イベントから呼ぶ。
//
// 内部で 1 回だけ origWS save → slot focus → spawn → restore する。
// (複数 project 一括処理は呼出元で withFocusBatch を直接使うこと、 二重切替防止)
func spawnBrowserWindowsForProject(projectName string) error {
	cfgRes, err := config.LoadFromDefaultPath()
	if err != nil {
		return err
	}
	_, st, err := loadStore()
	if err != nil {
		return err
	}
	p, ok := st.Projects[projectName]
	if !ok {
		return fmt.Errorf("project %q not found", projectName)
	}
	hasBrowser := false
	for _, w := range p.Windows {
		if w.Kind == naming.KindBrowser {
			hasBrowser = true
			break
		}
	}
	if !hasBrowser {
		return nil
	}
	r := reconcile.New(cfgRes.Config)
	r.Chromium.Logger = os.Stderr
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	slot := findSlotInProfile(st, st.ActiveProfile, projectName)
	return withFocusBatch(ctx, r.OmniWM, []slotProjectPair{{Slot: slot, Project: projectName}}, func(_ slotProjectPair) error {
		acts := r.SpawnAllBrowserWindowsInProject(ctx, projectName, p)
		return reportActions(acts, "spawn-browser")
	})
}

// reportActions は acts を stderr に出力、 error 数を error として返す。
func reportActions(acts []reconcile.Action, label string) error {
	errs := 0
	for _, a := range acts {
		if a.OnError != nil {
			errs++
			fmt.Fprintf(os.Stderr, "  ERROR %s %s: %v\n", a.Op, a.Target, a.OnError)
		} else {
			fmt.Fprintf(os.Stderr, "  %s %s  %s\n", a.Op, a.Target, a.Detail)
		}
	}
	if errs > 0 {
		return fmt.Errorf("%d %s action(s) failed", errs, label)
	}
	return nil
}

// spawnBrowserWindowsForMultipleProjects は **複数 project の spawn を 1 つの focus
// 切替 session で囲む** (profile switch 用)。 各 project の slot に順次 focus 切替
// しつつ spawn し、 全完了後に origWS に戻す。 race / 二重切替なし。
func spawnBrowserWindowsForMultipleProjects(profileName string, projectNames []string) error {
	if len(projectNames) == 0 {
		return nil
	}
	cfgRes, err := config.LoadFromDefaultPath()
	if err != nil {
		return err
	}
	_, st, err := loadStore()
	if err != nil {
		return err
	}
	r := reconcile.New(cfgRes.Config)
	r.Chromium.Logger = os.Stderr
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	items := make([]slotProjectPair, 0, len(projectNames))
	for _, n := range projectNames {
		p, ok := st.Projects[n]
		if !ok || p.Archived {
			continue
		}
		hasBrowser := false
		for _, w := range p.Windows {
			if w.Kind == naming.KindBrowser {
				hasBrowser = true
				break
			}
		}
		if !hasBrowser {
			continue
		}
		items = append(items, slotProjectPair{Slot: findSlotInProfile(st, profileName, n), Project: n})
	}
	if len(items) == 0 {
		return nil
	}
	return withFocusBatch(ctx, r.OmniWM, items, func(it slotProjectPair) error {
		p := st.Projects[it.Project]
		acts := r.SpawnAllBrowserWindowsInProject(ctx, it.Project, p)
		return reportActions(acts, "spawn-browser")
	})
}

// snapshotAndCloseBrowserWindowsForProject は project の browser kind windows を全て
// snapshot + close する。profile switch active 外 / archive / remove --window=browser
// 等から呼ぶ。Vivaldi 未起動 / 該当 window 不在 → no-op。
//
// close は **focus 切替不要** (chrome-cli の close -w <wid> は wid 直接指定で OmniWM
// 経由しない)。 user の作業中 ws 上の Vivaldi window が短時間 close されても
// focus が変わるだけで ws は変動しない。
func snapshotAndCloseBrowserWindowsForProject(projectName string) error {
	cfgRes, err := config.LoadFromDefaultPath()
	if err != nil {
		return err
	}
	_, st, err := loadStore()
	if err != nil {
		return err
	}
	p, ok := st.Projects[projectName]
	if !ok {
		return nil // archive で project が消えた直後等。silent.
	}
	hasBrowser := false
	for _, w := range p.Windows {
		if w.Kind == naming.KindBrowser {
			hasBrowser = true
			break
		}
	}
	if !hasBrowser {
		return nil
	}
	r := reconcile.New(cfgRes.Config)
	r.Chromium.Logger = os.Stderr
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	acts := r.SnapshotAndCloseAllBrowserWindowsInProject(ctx, projectName, p)
	return reportActions(acts, "close-browser")
}

// _ omniwm import 警告抑制 (omniwm.Client は withFocusBatch の引数型として参照)。
var _ = omniwm.New

// snapshotAndCloseSingleBrowser は単一の browser window を snapshot+close する
// (remove --window=browser-N 用)。
func snapshotAndCloseSingleBrowser(projectName string, w state.Window) error {
	cfgRes, err := config.LoadFromDefaultPath()
	if err != nil {
		return err
	}
	r := reconcile.New(cfgRes.Config)
	r.Chromium.Logger = os.Stderr
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	a := r.SnapshotAndCloseBrowserWindow(ctx, projectName, w)
	if a.OnError != nil {
		fmt.Fprintf(os.Stderr, "  ERROR %s %s: %v\n", a.Op, a.Target, a.OnError)
		return a.OnError
	}
	fmt.Fprintf(os.Stderr, "  %s %s  %s\n", a.Op, a.Target, a.Detail)
	return nil
}

// activeProjectsOfProfile は profile の Assignments から非 archived project 名を返す。
func activeProjectsOfProfile(st *state.State, profileName string) []string {
	prof, ok := st.Profiles[profileName]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(prof.Assignments))
	for _, projName := range prof.Assignments {
		p, ok := st.Projects[projName]
		if !ok || p.Archived {
			continue
		}
		out = append(out, projName)
	}
	return out
}

// diffProjects は a に含まれて b に含まれない project を返す。
func diffProjects(a, b []string) []string {
	bset := map[string]bool{}
	for _, n := range b {
		bset[n] = true
	}
	var out []string
	for _, n := range a {
		if !bset[n] {
			out = append(out, n)
		}
	}
	return out
}
