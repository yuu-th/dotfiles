# projwm-next reconcile / cockpit lifecycle ギャップ分析 (改訂版 v2)

> 「勝手な操作 / cockpit が変な位置 / space+f 効かない / ghostty 多重起動」の根本原因究明。
> 4 並列 read-only sub-agent の結果 + 実機状態確認 + 要件 v2.6 / 設計層 1 を統合。
>
> 2026-05-18 改訂 (v2)

---

## 1. 観測された症状 (実機 verify 済)

| # | 症状 | 確認方法 |
|---|---|---|
| S1 | viewer A の reorder op が got=N+1 want=N で繰り返し失敗 | log `sigwm.ReorderColumns[A]: order did not settle` |
| S2 | 直近 transaction が 1.5 分 runtime + executed=0 + commit 拒否 | `projwm trace --last` |
| S3 | TUI で workspace 選択 → cockpit が CP1 でない場所に移動 | ユーザ実機報告 |
| S4 | space+f が効かない | ユーザ実機報告 |
| S5 | **ghostty 多重起動 (34 個)** | `pgrep -f 'ghostty.*projwm-cockpit-D' \| wc -l` = 34 |
| S6 | DesiredWorld.SystemWindows は正しく 1 個 | desired_world.json verify |
| S7 | DesiredWorld.Visibility = shown のまま、PriorWorkspace=B (前回 jump 残骸) | desired_world.json verify |
| S8 | Generation G000001 → G000174 (173 commit) | CURRENT verify |

---

## 2. ギャップ表 (4 agent 結果統合)

### バグ A: viewer identity stale (P1 で deploy 済 ✅)
- **根本**: `ObservedWindow.MatchedTo` が stale な DesiredWindowID を保持、planner が validate せず信用
- **症状**: S1, S2
- **修正**: P1 deploy 済 — `viewerMatchedDesired` で MatchedTo を desired set で validate + `desiredAIWindowExists` ヘルパー
- **状態**: ✅ deploy 済、idle で再発無し

### バグ B: TUI jump auto-hide intent 欠落 ⚠️
- **根本**: TUI の `jumpToSlot()` (update.go:315-337) は `omniwmctl workspace focus-name` を直叩き + `quitMsg{}` のみ。**`SetCockpitVisibility{Hidden}` intent を送らない**
- **影響**: ユーザ視点では cockpit hide した気でも、daemon の Visibility=Shown のまま。planner が「Shown だが active≠CP1」を見て ShowCockpit op を再発火 → display CP1 戻り → 「勝手な操作」
- **症状**: S3, S4 の主要要因
- **修正**: `jumpToSlot()` で `quitMsg` の前に `setVisibilityCmd(Hidden)` を tea.Batch で発火、または cleaner には daemon 側で observed→desired 同期 (バグ E で吸収)

### バグ C: profile switch も auto-hide 欠落 ⚠️
- **根本**: TUI Tab / Enter on profile (update.go:239-242) も `submitIntentCmd(SwitchProfile)` のみ、auto-hide なし
- **影響**: バグ B と同様
- **修正**: バグ B と同じパターン or バグ E で吸収

### バグ D: observed→desired Visibility 同期欠落 (= 仕様追加 (A) と同根) ⭐
- **根本**: 要件 v2.6 §8.3 の自然な解釈「active workspace と Visibility は双方向同期」が **未実装**。reducer.ReactToEvent は `KindWindowsChanged` / `KindDisplayChanged` で Visibility を更新しない (設計層 1 通り、Tier-aware 更新無し)
- **影響**: 「ユーザの workspace 切替が DesiredWorld に反映されない」 = ユーザ直感と乖離。バグ B/C を吸収する根本対応
- **修正案** (Tier 2 / Tier 3 パターン準拠、~20 行):
  1. `reducer.ReactToEvent` で `KindWindowsChanged` 受信時、cockpit display の active=parkWs (CP1) かを check して mismatch なら `cockpit-visibility-sync` DirtyScope 作成
  2. `controller` に `applyCockpitVisibilitySync()` を追加、DirtyScope を消費して内部で `SetCockpitVisibility` intent を発火
  3. 既存 `applyCockpitSync()` / `applyTier2AutoSyncLayout()` と同レベルで実行

### バグ E: HideCockpitOnDisplay の back-and-forth race ⚠️ (バグ D で間接解消)
- **根本**: sigwm.go:2286-2294 で `queryDisplays → focus-name → sleep 100ms → back-and-forth` の race window。TUI の連続操作で omniwm 側 back-and-forth 履歴が汚染される
- **修正**: バグ D が解決すれば呼ばれる経路自体が無くなる (HideCockpit op が emit されない)

### バグ F: sigwm.SpawnCockpit の idempotence check が omniwm 応答無し時に空集合判定 🔴 (S5 の原因)
- **根本**: sigwm.SpawnCockpit (sigwm.go:2079-2098) の pre-check は `queryWindows` で title マッチを見る。omniwm が degraded で空応答時 → 「ghostty 存在しない」と判定 → 新規 spawn → 結果として 34 個多重起動
- **症状**: S5 (現在 34 個 ghostty)
- **修正**:
  1. `queryWindows` が error / empty を返した場合は spawn を **skip** (defensive)
  2. プロセスベースの追加 check: `pgrep -f 'ghostty.*projwm-cockpit-D0'` で既存があれば skip
  3. spawn の rate-limit (e.g., 同じ title へ 5 秒間 1 回まで)

### バグ G: cockpit window が物理移動する可能性 (focus.followsWindowToMonitor) ⚠️
- **根本**: manifest `focus.followsWindowToMonitor=true` (default.nix:73) は意味的に「focused window が別 monitor に移ったら focus も追従」だが、omniwm の実装によっては「focused-monitor 切替で window が物理移動」する race があるかも
- **状態**: agent 4 が指摘した仮説、まだ実機 verify せず (現在 cockpit は全部 kill 状態)。バグ D 解消後の再現状況で判定

---

## 3. 「ghostty 34 個」発生の原因連鎖

```
[ユーザ操作] space+f で cockpit show → TUI jump で別 workspace 切替
   ↓
[バグ B] jumpToSlot は SetCockpitVisibility(Hidden) を送らず、daemon の
        Visibility=Shown のまま、PriorWorkspace=B (前 jump 残骸)
   ↓
[バグ D] observed→desired 同期無いので daemon は気付かない
   ↓
[planner.planCockpitOps] Visibility=Shown && observed≠CP1 → ShowCockpit op
[planner.planCockpitOps] さらに observedCockpit map が空 (omniwm 応答無し時)
                       → spawn-cockpit op も同時 emit (idempotence 失敗)
   ↓
[バグ F] sigwm.SpawnCockpit の pre-check が空集合を信用 → 新規 spawn 実行
[各 ghostty] tmux 内 fish が projwm-cockpit binary 起動試行
   ↓
[event 駆動再 reconcile] windows-changed event でまた同じ planner 判定
   ↓ × N 回
[結果] 34 個 ghostty + DesiredWorld は正常 1 個
```

---

## 4. 修正方針 (優先順位)

### Priority 0 (即時、現在 ghostty 全 kill 済) — クリーン状態に戻す
- ✅ ghostty 全 kill 済 (count=0)
- DesiredWorld の Visibility=shown を Hidden にリセット (= `projwm cockpit hide` で済む) — daemon 復活後

### Priority 1 (再発防止のコア) — バグ D の実装 ⭐
**= 仕様追加 (A) = ユーザ直感「世界の変化に追従」の実装**

reducer / controller に observed→desired Visibility 同期を追加。Tier 2 パターン準拠で ~20-30 行:

```go
// reducer.ReactToEvent:
case event.KindWindowsChanged, event.KindDisplayChanged:
    // cockpit の DisplayIdx の active workspace を check
    // Visibility と乖離があれば "cockpit-visibility-sync" DirtyScope を作る

// controller:
func (c *Controller) applyCockpitVisibilitySync(ctx) {
    for _, ds := range c.state.Meta.DirtyScopes {
        if ds.Kind != "cockpit-visibility-sync" { continue }
        // 内部で SetCockpitVisibility intent を発火
    }
}
```

これでバグ B/C/E が **自動的に解決** (jumpToSlot が hide intent を送らなくても daemon が observed を見て自動 hide)。

### Priority 2 (Safety net) — バグ F の実装
sigwm.SpawnCockpit の idempotence check 強化 (~30 行):
- omniwm query が error / empty を返した場合は spawn skip
- pgrep ベースの process check
- 5 秒 cooldown

### Priority 3 (将来) — Karabiner 側で追加 binding
ユーザ提案の「workspace 移動コマンド = cockpit toggle と同じ意味」は P1 で daemon 側に実装すれば karabiner 側は何も変えずに自動達成。明示 binding は不要。

### Priority 4 (検証) — バグ G の確認
バグ D 実装後の実機で cockpit window が物理移動するか観察。問題があれば対処。

---

## 5. 合意して進めたい点

1. ギャップ A〜G の理解は合っているか
2. 修正順 P0 完了 → P1 (バグ D = 仕様 A) → P2 (バグ F) → P3 不要 → P4 検証 で OK か
3. P1 (バグ D) は要件 v2.6 §8.3 の自然な拡張だが、**要件文書 v2.7 として明文化** したいか?
   - 候補追記: 「§8.3 ユーザの手動 workspace 切替 (active=ParkWs から離脱) は暗黙的に Visibility=Hidden への flip と等価」
4. P0 reset 完了状態 (ghostty 0, daemon 0) を維持して、P1 実装後に daemon 復活 + 実機 verify という流れで進める
