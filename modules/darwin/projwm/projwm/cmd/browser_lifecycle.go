package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/naming"
	"github.com/yuu-th/projwm/internal/reconcile"
	"github.com/yuu-th/projwm/internal/state"
)

// browser ライフサイクル helper（v12 paradigm C, queue/projwm-spec.md FR-29）。
//
// reconcile.Run は browser に **触らない** 設計なので、cmd 層の明示イベントから
// これらを呼ぶ。launchd auto-reconcile では絶対に発火しない。

// spawnBrowserWindowsForProject は project の browser kind windows を全て spawn する。
// add-browser / profile switch active 復帰 / unarchive 等から呼ぶ。
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
	acts := r.SpawnAllBrowserWindowsInProject(ctx, projectName, p)
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
		return fmt.Errorf("%d spawn-browser action(s) failed", errs)
	}
	return nil
}

// snapshotAndCloseBrowserWindowsForProject は project の browser kind windows を全て
// snapshot + close する。profile switch active 外 / archive / remove --window=browser
// 等から呼ぶ。Vivaldi 未起動 / 該当 window 不在 → no-op。
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
		return fmt.Errorf("%d close-browser action(s) failed", errs)
	}
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
