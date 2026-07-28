# projwm 試行錯誤 history

> projwm の v0.1 開発（2026-05）における試行錯誤を時系列で archive する。
> 現状の確定仕様は `projwm-spec.md`、未着手は `projwm-roadmap.md`。
> 本書は **読まなくても projwm を運用できる**。気になった経緯がある時だけ参照。

---

## タイムライン概要

| 期間 | 大事件 |
|---|---|
| 2026-05-03 朝 | 設計書 v11.1 確定 → 実装開始 |
| 同日 | Phase 0 POC 群、20 項目の検証 |
| 同日 | Phase 1〜6 を高速実装 → 初回 dogfood で複数の致命傷発覚 |
| 同日 | UX 失敗の根本修正（AI 自動起動・viewer・windows 配置）|
| 同日 | terminal driver 紆余曲折: Ghostty → kitty → Ghostty |
| 同日 | TUI cockpit 本実装、alt+\` 配線 |
| 同日 | OmniWM-Ghostty 不可視問題を実装レベルで原因特定、titleRegex で解決 |
| 同日 | kitty 完全廃止、純正 Ghostty に統一 |

---

## 主要な決定経緯

### Zed の Nix 経由インストール方式

- 候補: (a) `pkgs.zed-editor`、(b) homebrew cask `zed`、(c) 公式 dmg ラッピング
- nixpkgs `zed-editor` は **aarch64-darwin で CLI (`zeditor`) が壊れている既知バグ**（NixOS/nixpkgs#365465）。`--help/--version` 以外で失敗
- → **homebrew cask を採用**（`/opt/homebrew/bin/zed` shim が自動配置）

### tmux viewer session 名の `:v` → `_v`

- 設計書 v11.1 では viewer 用 grouped clone の名前を `<kind>-<id>/<proj>:v` と規定
- POC-13 で **tmux が session 名内の `:` を `_` に silently 置換**することが判明
- → `_v` 末尾に変更（v11.2）

### Zed CLI の `-n` フラグ必須

- POC-17 で `zed <path>` だけだと **既存 Zed workspace を再利用**して新 window が立たないことが判明
- `-n` (`--new`) フラグで強制的に新 window を作る
- → projwm の zedwrap は常に `zed -n <cwd>` を発行

### OmniWM Quake は command 上書き不可

- POC-07 で `[quakeTerminal]` 設定に `command` フィールドが **存在しない**ことを `Sources/OmniWM/Core/Config/CanonicalTOMLConfig.swift` で確定
- Quake は libghostty 固定 fish のみ
- → **設計書 B 案（alt+space で cockpit）は実現不可能**、A 案（cockpit を別キーで）に倒した
- 最終的に `alt+\`` で Ghostty + projwm tui を spawn する形に着地

### workspace name → number 解決

- `omniwmctl move-to-workspace` が **number 引数のみ** で name 不可
- `query workspaces` で name → number map を動的解決して使う

### terminal driver の紆余曲折

#### v11.3: ghostty → kitty user-space copy（誤った診断）

- 初回 dogfood で OmniWM が Ghostty.app の window を全く認識しないと判明
- 原因不明のまま「OmniWM が SwiftUI WindowGroup app を見えないバグ」と早合点
- workaround として kitty を user-space コピー（`~/Applications/kitty-projwm.app`）して NSPrincipalClass=NSApplication 注入 + ad-hoc 再署名する方式に逃避
- 動作するが、ユーザの希望（Ghostty 統一）に反する

#### v11.6: 真の原因判明、純正 Ghostty 復帰

- ユーザの「諦めるな、自分だけのバグはありえない」という後押しで深掘り
- `Sources/OmniWM/Core/Reconcile/WMEvent.swift` / `WindowAdmissionRuntime.swift` / `Controller/AXEventHandler.swift` / `Rules/WindowRuleEngine.swift` を実装レベルで読解
- 真の原因: **Ghostty (SwiftUI WindowGroup) は起動時に hidden helper windows を多数（5+）作る**。OmniWM の rule engine は user rule 未指定だと hidden helper と main window を区別できず、disposition=`.unmanaged` → `trackedMode=nil` → admit されない
- 解決: `app-rules.nix` に `titleRegex = "^(ai|shell|ai-view)-[0-9]+:"` rule を追加することで rule engine が `.managed` 判定 → admit 成功
- これは **OmniWM のバグではなく、user rule の不足**だった
- 関連 OmniWM Issue: #243 Notion で OWNER が同様 workaround を提示

### AI 自動起動の実装漏れ（緊急修正）

- 2 度目の dogfood で「terminal は出るがすべて空 fish プロンプト、claude が起動していない」とユーザ指摘
- `internal/ghosttywrap` は `tmux new-session -A` だけで AI コマンドを起動していなかった
- 設計書 §5.1 に「AI 本体（claude or copilot）が tmux session で走る」と明記されていたのに、**完全な実装漏れ**
- 修正:
  - `internal/naming.AICommand(AI) string` 追加
  - `internal/tmuxwrap.SendKeys(target, keys...)` 追加
  - `internal/reconcile.ensureProjectInSlot` で tmux 新規作成時に send-keys
- 副次バグ: 当初 `tmux send-keys -t '=ai-1/dotfiles'` と書いて `can't find pane: =name` エラー。`=` 構文は pane 検索用なので外す

### profile switch が reconcile を呼んでいなかった

- general-purpose agent のレビューで判明
- `cmd/profile.go` の switch コマンドが state mutate のみで終わっていた
- 修正: `cmd/reconcile_helper.go` 新設、`runReconcileOnce()` を共通呼出

### Zed 無限 spawn ループ（致命的）

- launchd watch の windows-changed event 頻発 → reconcile が「Zed window 見えない → spawn-zed」を繰り返す → Zed `-n` で新 workspace 追加 → **Zed が永久に増殖**
- 修正:
  - polling 待ち（4 秒間 OmniWM での出現を待つ）
  - `internal/reconcile/zedlock.go` 新設、flock(2) ベースの spawn lock
  - `~/.cache/projwm/.locks/zed-spawn-<sha1>.lock` で並走 reconcile からの重複 spawn を抑止

### bubbletea cockpit の本実装

- Phase 4 commit 時に「最小実装」と書いて 5 操作のみ（filter / 移動 / jump / Tab cycle / esc）に留めた
- ユーザから「bubbletea でちゃんと cockpit を実装する話どうした」と指摘
- 設計書 §8.2.3 で要求された 9 操作のうち 8 操作を実装:
  - windows 詳細展開（kind/AI/tmux/window status を色付き表示）
  - empty slots 表示
  - viewer (WS A) summary
  - n / d / a / u 操作（new / unassign / archive / unarchive）
  - filter モードと prompt モードの分離
  - fsnotify reactive 更新 + 2 秒定期 probe
  - section 見出し付き描画
  - lipgloss styling

---

## Ghostty 認識問題のロングストーリー

これは projwm 開発で最も時間を使った調査。**諦めずに続けたら原因が判明した**好例。

### 段階 1: 「OmniWM が Ghostty を見えない」発見

- `omniwmctl query windows --bundle-id com.mitchellh.ghostty` が **0 件**
- でも AppleScript `count windows` は 1 を返す
- → AX 自体は動いているが OmniWM が見えない

### 段階 2: 早合点（v11.3）

- 「OmniWM 0.4.8 は SwiftUI WindowGroup を見えない上流バグ」と推測
- kitty を user-space copy で逃げる workaround 採用
- → ユーザの「kitty 嫌、Ghostty 統一したい」要請で再開

### 段階 3: 実装読解

- ユーザの「諦めるな、ちゃんと実装読め」で深掘り
- AXManager.swift / AppAXContext.swift / AXWindow.swift / WindowAdmissionRuntime.swift / AXEventHandler.swift / WindowRuleEngine.swift / EventNormalizer.swift を順次読み解く
- 「2 秒タイムアウト race」「shouldTrack」「\_AXUIElementGetWindow 不可視」「NSRunningApplication で隠れる」等の仮説を順次検証して全部否定
- → 最終的に WindowRuleEngine の **disposition** 判定で `.unmanaged` になっていることが論理的に確定

### 段階 4: 実機検証で確証

- CGS で Ghostty の window 列挙: **5 hidden helper windows + 1 main**（layer=0、title="?"、size 1440x30 が helper、800x707 が main）
- AX で window 取れるが、複数 helper の流入で OmniWM の rule engine が disposition 判定を通せない

### 段階 5: 関連 issue で workaround を発見

- OmniWM Issue #243（Notion app）で OWNER が「custom rule で tile 強制」を推奨
- → app-rules.nix に **`titleRegex = "^(ai|shell|ai-view)-[0-9]+:"` + `layout = "tile"` rule** を追加
- 即座に OmniWM が ai/shell/viewer の Ghostty window を認識（count=3）
- 完全勝利

### 学び

- **「上流バグだ」と早合点しない**。ユーザが正しく「他の人も同じ問題に当たってるはず」と指摘
- 関連 issue を真面目に検索することの重要性
- 実装読解は時間かかるが、確実に答えに辿り着く

---

## POC 結果（Phase 0、condensed）

実装着手前に行った 20 項目の致命傷チェック。基本全部解決済み。

| ID | 内容 | 結果 |
|---|---|---|
| 01〜04 | tmux session 名 / window 識別 / move-to-workspace | ✅ 解決（一部は `_v` suffix で workaround）|
| 05 | tmux grouped session 双方向同期 | ✅ |
| 06 | omniwmctl subscribe windows-changed | ✅ |
| 07 | OmniWM Quake 起動 command | ❌ 不可 → A 案（別キー）に倒す |
| 08 | ghostty quick-terminal 共存 | ✅ |
| 09 | Karabiner alt+letter 全アプリ機能 | ✅ |
| 10 | buildGoModule で bubbletea ビルド | ✅ |
| 11 | profile 切替フリッカー | 観測のみ、運用判断 |
| 12 | tmux session kill せず再 attach の表示維持 | ✅ |
| 13 | tmux session 名の `:` 不可 → `_v` | ⚠️ workaround |
| 14 | ghostty title 長さ上限 | 実用範囲で問題なし |
| 15 | Zed window title が basename で安定 | ✅ |
| 16 | omniwmctl query で Zed window 列挙 | ✅ |
| 17 | `zed -n <path>` で新 window | ⚠️ `-n` 必須 |
| 18 | Zed close 時の dirty 保存ダイアログ | 運用上 OK |
| 19 | Zed session restore | ✅ |
| 20 | Zed 起動レイテンシ | ✅ |

---

## 設計書の改版履歴

設計書 (queue/projwm-design.md) は v11.1 から v11.6 まで進化:

| 版 | 主な変更 |
|---|---|
| v11.1 | 設計書の初版確定、実装開始 |
| v11.2 | viewer tmux session 名 `:v` → `_v`（POC-13 反映）|
| v11.3 | terminal driver を kitty に切替（誤った診断）|
| v11.4 | bubbletea cockpit 本実装 + alt+\` 配線 |
| v11.5 | Zed 無限 spawn ループの致命的バグ修正 |
| v11.6 | terminal driver を純正 Ghostty に戻す（titleRegex で正しく解決）|

v11.6 時点で `projwm-spec.md` に整理。改版履歴の機能はここ（history.md）と decision log（spec.md §10）に集約。

v12 で Vivaldi browser 統合を追加（Zen は v13 deferred）。

---

## v12 paradigm 変遷の総まとめ — 2026-05-03

v12（browser 統合）は **3 つの paradigm** を経て C で確定した。

### Paradigm A — profile-level browser_workspace（撤回）

- 各 project が `BrowserWorkspace { Browser, Name }` を持ち、profile 切替で **slot Q,W,E,R,T,Y,U,I,O,P 順で最初の binding** を持つ project の workspace に切替
- Vivaldi は 1 instance で内部 active workspace を切替えるだけ
- **撤回理由**: projwm の paradigm は **複数 project 同時表示**（active profile が複数 slot を持つ）。1 project だけ追従するのは projwm 全体と整合しない

### Paradigm B — per-project window + Vivaldi Workspaces を AX で切替（撤回）

- 各 project に独立 Vivaldi window、内部で workspace を分けて tab セット保持
- 識別: `file://...html` の marker tab を 1st tab として開き、Window menu の tab list を AX で scan
- 切替: `Window menu → その他のワークスペースとタブ → <name>` を System Events で click
- close: `Cmd+Shift+W` を keystroke
- focus 退避: `tell process "Finder" to set frontmost to true` で menu cascade を強制 dismiss
- **撤回理由**: AX 操作が **観測のたびに focus を奪う**。launchd auto-reconcile が頻発 → user が何もしてないのに Vivaldi が暴れる致命的 UX。Chromium の menu auto-close 不備 + AX で active workspace が見えない構造的問題 + AppleScript dictionary に Workspaces 機能がない

### Paradigm C — chrome-cli + Chromium user profile + close 主義 + frontmost 復帰（採用）

着想の連鎖：
1. ユーザの根本疑問「Vivaldi は CLI 制御に向いてないのでは？」
2. 調査で `chrome-cli` が Vivaldi/Brave/Edge/Arc 全てで動作することが判明
3. chrome-cli は AppleScript event ベース → **read 系で focus を奪わない**
4. ただし Vivaldi の AppleScript dictionary は **Workspaces 機能を露出していない**
5. → Workspaces 機能を捨て、**Chromium user profile（`--profile-directory`）で login 分離** に切替
6. → tab セット保持は **projwm が URL list を SavedURLs に snapshot** して再現する形に
7. ユーザの提案「閉じる時も focus を元のアプリに戻せばいい」→ destructive 操作の前後で frontmost を保存・復帰する wrapper

POC で確認した事実：
- `chrome-cli list windows/tabs` は frontmost を維持（read 系 non-intrusive）
- `chrome-cli open / close` は Vivaldi に focus を奪う（write 系）
- `tell application "Vivaldi" to set minimized of every window to true` は focus 奪わない（参考、採用せず）
- `open -na Vivaldi --args --profile-directory=NAME --new-window URL` で profile 別 window 起動
- 同 Vivaldi instance 内で複数 profile の window が共存可能
- 第二 launch で `~/Library/Application Support/Vivaldi/<profile>/` が自動生成

採用の決め手：
- 通常運用で focus 完全不動（reconcile no-op）
- profile 切替時のみ flicker（ai/shell/editor の close/spawn と同等感覚）
- 状態保持: cookies / login は完璧、scroll 位置は失う（許容）
- chrome-cli が Chromium 系全部で動くので future-proof（browser 切替自由）

詳細は `projwm-roadmap.md` v12 section と `projwm-spec.md` D-46〜D-51。

---

## v12 (Vivaldi paradigm B) POC narrative — 2026-05-03

### 出発点

ユーザの要望: 「project を切り替えると browser の tab セットも自動で切り替わってほしい」。Zen は通常運用 browser として残し、projwm 連動には別の browser を使う方針。候補は Zen / Vivaldi / Arc。ユーザ判断で **Vivaldi 単独** に絞る (Zen は v13 deferred)。

### Phase 1: 起動と AX

`tell application "Vivaldi" to activate` は **AppleEvent timeout** で失敗 (Chromium 系の既知制約)。代替: `open -a /Applications/Vivaldi.app` で起動・前面化を確認。

### Phase 2: workspace 創出

最初は Quick Commands (Cmd+E → "New Workspace") を試したが Japanese locale で command 名が一致せず生成されない。突破口は **Preferences JSON 直接編集**。
- パス: `~/Library/Application Support/Vivaldi/Default/Preferences`
- キー: `vivaldi.workspaces.list = [{id, name, icon}]`
- Vivaldi 終了中に書き込み → 起動 → Window menu に出現

### Phase 3: 切替方式の検証

3 通り試した:

1. `Cmd+Shift+1..9`: menu の `AXMenuItemCmdChar` が missing で default 未設定。動作せず。
2. `Cmd+E + 名前タイプ + Enter` (Quick Commands): osascript からは web UI に keystroke が透けず AX 検証不可。背面時に keystroke が他 process に流れる事故あり。
3. **Window menu click**: 「ウィンドウ → その他のワークスペースとタブ → `<name>`」を System Events で click。**screencapture で active marker（チェック印 + 「有効」ラベル）が移動することを確認**。決定的な方式。

### Phase 4: active workspace の検出

`AXMenuItemMarkChar` / `AXSelected` 等を全ダンプしたが、`AXSelected=true` は「最後にハイライトされた item」のみで active workspace ではない。「有効」ラベルは menu subview として描画され AX 標準属性で取れない。

結論: **Vivaldi の active workspace は AX 不可視**。projwm 側の intent (state.json) を source of truth とする。

### Phase 5: menu cascade が閉じない quirk

menu click 後、Vivaldi のメニュー階層が画面に残り続ける (Chromium の menu 実装が macOS の auto-close を尊重しない)。回避策: click 後に `tell process "Finder" to set frontmost to true` を末尾に入れる。Finder へ一瞬 focus を渡すと menu が dismiss される。3 秒後の screencapture で closed 状態確認。

### 確定設計

- driver: `internal/browserwrap/vivaldi/vivaldi.go`
- state schema: `Project.BrowserWorkspace { Browser, Name }`
- CLI: `projwm browser set/unset/switch/ensure/list`
- profile switch hook: slot Q,W,E,R,T,Y,U,I,O,P 順で最初の binding
- cockpit: dim 行で `browser  vivaldi:<name>`

### 残置課題

- Vivaldi 言語 ja/en/de/zh 以外: `buildSwitchScript` の menu 名 candidate 要追加
- 同名 workspace 複数: 仕様上 undefined、未対策
- 起動が遅い環境での switch race: Activate 2s wait のみ（実害が出れば retry 追加）

---

## コミット履歴（projwm 関連、v0.1 全て）

```
606daaa fix(projwm): v11.6 — 純正 Ghostty に統一、kitty 完全廃止
463b1ac fix(projwm): Zed 無限 spawn ループの致命的バグ修正
16a0069 feat(projwm): v11.4 — bubbletea cockpit 本実装 + alt+` で cockpit 起動
8a8b4b3 feat(projwm): AI 自動起動 + profile switch reconcile + ネスト署名対応
9ab894c fix(projwm): v11.3 — terminal driver kitty user-space copy
b4d054a feat(projwm): Phase 4 — bubbletea cockpit (最小、後の v11.4 で本実装)
de85f6c feat(projwm): Phase 6 — launchd auto-reconcile (3 agents)
66e2e31 feat(projwm): Phase 5 — OmniWM 22 workspace + Karabiner hotkeys
f9c20f5 feat(projwm): Phase 2 + Phase 3 — reconcile + window 管理コマンド
9c9e4c4 feat(projwm): Phase 1 — Go binary 骨格
4b7abd3 feat(projwm): tmux と Zed を Nix で導入
```

---

_本書は projwm 開発の **archive**。読まなくても運用できる。気になる時だけ参照。_
