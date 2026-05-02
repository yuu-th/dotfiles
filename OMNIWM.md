# OMNIWM.md — OmniWM 取説（移植検討用ナレッジベース）

このプロジェクトで AeroSpace から OmniWM への移行を検討するにあたり、
**「OmniWM で何ができて、何ができないか」を網羅した事実カタログ**。
詳細仕様（出力フォーマット等）は割愛し、各機能の「存在する／しない」を確実に押さえることに徹する。

調査時点バージョン: **v0.4.7.4**（2026-05、pre-1.0、活発開発中）
ライセンス: GPL-2.0
情報源: 公式リポジトリ `BarutSRB/OmniWM` の `docs/` と Swift ソース直読み

---

## 0. 採用判断の前提

| 項目 | 値 |
|---|---|
| 必要 OS | macOS 15+ (Sequoia 以降)。本機は 26.3 なので OK |
| アーキ | Apple Silicon / Intel 両対応 |
| 必須権限 | Accessibility |
| Mission Control 設定 | "Displays have separate spaces" を **OFF** 必須 |
| SIP | 無効化不要（Apple notarized） |
| 配布 | Homebrew tap (`BarutSRB/tap`) または公式 zip |

---

## 1. 設定モデル

### 1.1 設定ファイル
- パス: `~/.config/omniwm/settings.toml`
- 形式: TOML（**唯一の宣言的設定ソース**）
- ランタイム状態は別途 UserDefaults に保存（GUI トグル等）
- **ホットリロード無し**：設定変更は OmniWM プロセス再起動が必要

### 1.2 設定ファイルのトップレベルキー一覧（canonical-settings.toml ベース）

```
[appearance]              ダークモード等
[borders]                 内蔵ボーダー (色・幅・有効化)
[dwindle]                 Dwindle (BSP) レイアウトのデフォルト
[focus]                   フォーカス挙動 (followsMouse 等)
[gaps]                    余白
[gaps.outer]              外側余白
[general]                 アニメ・hotkey・IPC・スリープ防止・更新確認
[gestures]                トラックパッドジェスチャ
[mouseWarp]               マウスをモニタ間で warp
[niri]                    Niri (Scrolling) レイアウトのデフォルト
[quakeTerminal]           Quake terminal
[state]                   ランタイム状態（手書き不要）
[statusBar]               ステータスバー
[workspaceBar]            ワークスペースバー
[[appRules]]              アプリ別ルール（後述）
[[hotkeys]]               キーバインド（action ID 列挙）
[[workspaces]]            ワークスペース定義
monitorBarOverrides       モニタ別オーバーライド
monitorDwindleOverrides
monitorNiriOverrides
monitorOrientationOverrides
```

### 1.3 declarative 化（Nix からの生成）の可否
- `[[appRules]]` / `[[hotkeys]]` / `[[workspaces]]` の `id` フィールドは UUID だが、デコーダが `?? UUID()` で**省略可能**。Nix で UUID を書かなくても良い。
- `pkgs.formats.toml { }` で AeroSpace と同型に生成できる。

---

## 2. レイアウトエンジン

### 2.1 二択（ワークスペース単位で切替可能）

| Layout | 概要 | 特徴 |
|---|---|---|
| **Niri** | スクロール式 column タイル | 横方向の column 列、column 内に縦スタック、tabbed 表示可 |
| **Dwindle** | BSP（Hyprland 流） | 二分木で再帰的に分割 |

`set-workspace-layout` / `toggle-workspace-layout` でワークスペースごとに切替。
`general.defaultLayoutType` で新規 WS のデフォルトを指定。

### 2.2 Niri 固有の設定（[niri]）
- `alwaysCenterSingleColumn`
- `centerFocusedColumn`（never / always / on-overflow 等）
- `columnWidthPresets`（width プリセット配列）
- `infiniteLoop`
- `maxVisibleColumns`
- `maxWindowsPerColumn`
- `singleWindowAspectRatio`

### 2.3 Dwindle 固有の設定（[dwindle]）
- `defaultSplitRatio`
- `moveToRootStable`
- `singleWindowAspectRatio`
- `smartSplit`
- `splitWidthMultiplier`
- `useGlobalGaps`

### 2.4 モニタ別オーバーライド
`monitorNiriOverrides` / `monitorDwindleOverrides` / `monitorOrientationOverrides` でモニタ毎に上書き可能。

---

## 3. ワークスペース

### 3.1 構造（[[workspaces]]）
```toml
[[workspaces]]
id = "<UUID>"           # 省略可
name = "1"              # 任意文字列（"M", "B", "E" など可）
displayName = "❤️"      # 任意（絵文字 OK）
layoutType = "niri"     # default | niri | dwindle

[workspaces.monitorAssignment]
type = "main"           # main | secondary | specificDisplay
# specificDisplay の場合は output = "<OutputId>" を併記
```

### 3.2 モニタ割当の表現力
- **`main`**：メインモニタに固定
- **`secondary`**：「メイン以外」に置く
- **`specificDisplay`**：特定の OutputId（モニタ名）に固定
- ❌ AeroSpace の「モニタ番号 2」みたいな**インデックス指定は無い**（OutputId / 名前で指定）

### 3.3 ワークスペース操作
- WS 切替（番号 / 隣接 / back-and-forth / 全モニタ横断）
- ウィンドウを WS に送る（番号指定 / 隣接 WS / 隣接モニタ上の WS）
- WS 全体を別モニタに swap

---

## 4. モニタ・ディスプレイ

### 4.1 検出・操作
- `query displays` で全モニタの geometry / name / ID を取得可能
- `command focus-monitor prev|next|last`（**順序ベースのみ**、方向ベースなし）
- `command swap-workspace-with-monitor <left|right|up|down>` は**方向ベース**で動く（focus-monitor は不可）
- `mouseWarp.axis` でモニタ間でのマウス warp 方向、`mouseWarp.monitorOrder` で順序を制御
- `focus.followsMouse` / `focus.followsWindowToMonitor` / `focus.moveMouseToFocusedWindow` 各種

### 4.2 多モニタ運用の実装方針
- 「モニタ枚数別プロファイル」は OmniWM 自体に概念無し
- 自前スクリプトで **TOML 差替 → OmniWM プロセス再起動** することで実現（reload API なし）
- `query displays` で枚数判定 → `cp` → `launchctl kickstart`

---

## 5. アプリルール（[[appRules]]）

### 5.1 マッチャー（複合可、specificity でソート）
| フィールド | 内容 |
|---|---|
| `bundleId` | 必須キー |
| `appNameSubstring` | アプリ名部分一致 |
| `titleSubstring` | ウィンドウタイトル部分一致 |
| `titleRegex` | タイトル正規表現 |
| `axRole` | アクセシビリティ role |
| `axSubrole` | アクセシビリティ subrole |

### 5.2 アクション
| フィールド | 内容 |
|---|---|
| `layout` | `auto` / `tile` / `float` |
| `assignToWorkspace` | **そのアプリのウィンドウを開いた時に投げ込む WS 名** ← AeroSpace の `on-window-detected → move-node-to-workspace` と等価 |
| `minWidth` / `minHeight` | 最小サイズ強制 |

### 5.3 動的ルール管理
`omniwmctl rule add/list/remove/replace/reapply` で実行時にも操作可能。

---

## 6. ホットキー（[[hotkeys]]）

### 6.1 モデル
**事前定義された action ID にバインドを当てる**方式。
任意キーに任意コマンドを当てる自由なバインドではない。

```toml
[[hotkeys]]
binding = "Option+1"    # または "Unassigned"
id = "switchWorkspace.0"
```

### 6.2 action ID カテゴリ全網羅

#### Workspace
- `switchWorkspace.0` 〜 `.8`（数字 1〜9）
- `switchWorkspace.next` / `.previous`
- `workspaceBackAndForth`
- `moveToWorkspace.0` 〜 `.8`
- `moveWindowToWorkspaceUp` / `.Down`
- `moveColumnToWorkspaceUp` / `.Down`
- `moveColumnToWorkspace.0` 〜 `.8`

#### Focus（同 WS 内）
- `focus.left` / `.right` / `.up` / `.down`
- `focusPrevious`
- `focus.downOrLeft` / `.upOrRight`（niri 専用、column を跨ぐ走査）

#### Move（ウィンドウ）
- `move.left` / `.right` / `.up` / `.down`
- → **左右 move は隣 column への consume 効果を持つ**（niri）
- → **上下 move は column 内スタック移動**（niri）
- `toggleFocusedWindowFloating`

#### Column（Niri）
- `moveColumn.left` / `.right`
- `toggleColumnTabbed`（**column 内ウィンドウをタブ表示化** = AeroSpace の `layout accordion` 相当）
- `toggleColumnFullWidth`
- `cycleColumnWidthForward` / `Backward`
- `focusColumnFirst` / `Last`
- `focusColumn.0` 〜 `.8`

#### Dwindle
- `moveToRoot`
- `toggleSplit` / `swapSplit`
- `resizeGrow.left/right/up/down`
- `resizeShrink.left/right/up/down`
- `preselect.left/right/up/down`
- `preselectClear`

#### Monitor
- `focusMonitor.prev` / `.next` / `.last`
- `swapWorkspaceWithMonitor` 系

#### Layout
- `toggleFullscreen` / `toggleNativeFullscreen`
- `toggleWorkspaceLayout`（niri ⇄ dwindle）
- `balanceSizes`

#### Window 特殊操作
- `raiseAllFloatingWindows`
- `rescueOffscreenWindows`
- `assignFocusedWindowToScratchpad`
- `toggleScratchpadWindow`

#### UI
- `openCommandPalette`
- `openMenuAnywhere`
- `toggleWorkspaceBarVisibility`
- `toggleHiddenBar`
- `toggleQuakeTerminal`
- `toggleOverview`

### 6.3 重要な「無いもの」
- ❌ **シェル実行 action**（`exec-and-forget` 相当が無い）
- ❌ **複数 action の連鎖**（`[ "workspace M" "exec ..." ]` のようなリスト）
- ❌ **モード切替**（resize mode / service mode のような modal バインド）
- ❌ **任意キー → 任意 action**（カタログ外の挙動を増やせない）

→ これらは **Karabiner-Elements で外部から実装する**しかない（後述）。

---

## 7. omniwmctl CLI / IPC

### 7.1 接続
- ソケット: `~/Library/Caches/com.barut.OmniWM/ipc.sock`
- 認可: `~/Library/Caches/com.barut.OmniWM/ipc.sock.secret`
- **IPC はデフォルト無効**。メニューから有効化が必要（または settings の `general.ipcEnabled = true`）
- 環境変数 `OMNIWM_SOCKET` でソケットパス上書き可

### 7.2 トップレベルコマンド
| コマンド | 用途 |
|---|---|
| `ping` / `version` | 疎通確認 |
| `command <...>` | WM コマンド実行（後述） |
| `query <...>` | 状態取得 |
| `rule add/list/remove/replace/reapply` | ルール管理 |
| `workspace <...>` | ワークスペース操作 |
| `window <...>` | ウィンドウ操作 |
| `subscribe <channel>` | イベントストリーム |
| `watch <channel> -- <child>` | イベント発生時に子プロセスを起動 |
| `completion zsh\|bash\|fish` | 補完スクリプト出力 |

### 7.3 グローバルフラグ
- `--format json|table|tsv|text` / `--json`
- 既定: query/subscribe は json、command/ping/version は text

### 7.4 `command` サブカテゴリ（CLI 経由で叩ける WM 操作）

ホットキー action とほぼ同じ集合。重要なのは：

- **Focus**: `focus <dir>`、`focus previous`、`focus down-or-left`、`focus up-or-right`、`focus-column <n|first|last>`
- **Move**: `move <dir>`（**consume 含む**）
- **Workspace**: `switch-workspace <n|next|prev|back-and-forth|anywhere>`、`move-to-workspace <n|up|down|on-monitor <n> <dir>>`
- **Monitor**: `focus-monitor <prev|next|last>`、`swap-workspace-with-monitor <dir>`
- **Niri Column**: `move-column <dir>`、`move-column-to-workspace <n|up|down>`、`toggle-column-tabbed`、`toggle-column-full-width`、`cycle-column-width <forward|backward>`
- **Dwindle**: `move-to-root`、`toggle-split`、`swap-split`、`resize <dir> <grow|shrink>`、`preselect <dir|clear>`
- **Layout**: `balance-sizes`、`toggle-workspace-layout`、`set-workspace-layout <default|niri|dwindle>`、`toggle-fullscreen`、`toggle-native-fullscreen`
- **Window**: `toggle-focused-window-floating`、`raise-all-floating-windows`、`rescue-offscreen-windows`、`scratchpad assign|toggle`
- **UI**: `open-command-palette`、`open-menu-anywhere`、`toggle-workspace-bar`、`toggle-hidden-bar`、`toggle-quake-terminal`、`toggle-overview`

### 7.5 `query` 一覧（情報取得）
| query | 取れるもの |
|---|---|
| `workspace-bar` | バー projection（モニタ毎） |
| `active-workspace` | 現在の interaction monitor + active WS |
| `focused-monitor` | フォーカスモニタ |
| `focused-window` | フォーカスウィンドウ snapshot |
| `focused-window-decision` | ルール適用デバッグ |
| `apps` | 管理アプリ summary |
| `windows` | 管理ウィンドウ（フィルタ豊富） |
| `workspaces` | WS 一覧（占有状態付き） |
| `displays` | 接続ディスプレイ（geometry 込み） |
| `rules` | 永続化ルール |
| `rule-actions` / `commands` / `queries` / `subscriptions` / `capabilities` | レジストリ自己記述 |

### 7.6 query セレクタ
- `--window <id>` / `--workspace <name>` / `--display <name>` / `--app <name>` / `--bundle-id <id>`
- フラグ: `--focused` / `--visible` / `--floating` / `--scratchpad` / `--current` / `--main`

### 7.7 `window` / `workspace` 専用アクション
- `window focus <id>` / `window navigate <id>` / `window summon-right <id>`
- `workspace focus-name <name>`

### 7.8 サブスクリプションチャネル
| channel | 内容 |
|---|---|
| `focus` | フォーカスウィンドウ変化 |
| `workspace-bar` | バー更新 |
| `active-workspace` | 現在 WS 変化 |
| `focused-monitor` | フォーカスモニタ変化 |
| `windows-changed` | ウィンドウ inventory 変化 |
| `display-changed` | ディスプレイ抜き差し |
| `layout-changed` | WS レイアウト変化 |

`subscribe` は coalesced state stream（lossless ではない、最新値のみ届く）。
`watch` は各イベントごとに子プロセスを 1 個起動し、stdin に NDJSON を流す。
子プロセスは `OMNIWM_EVENT_CHANNEL` / `OMNIWM_EVENT_KIND` / `OMNIWM_EVENT_ID` を環境変数で受け取れる。

### 7.9 エラーコード（自動化スクリプトが見るやつ）
- `0 success` / `1 rejected` / `2 transportFailure` / `3 invalidArguments` / `4 internalError`
- ペイロード上のエラー: `invalid_request` / `invalid_arguments` / `protocol_mismatch` / `ignored_disabled` / `ignored_overview` / `layout_mismatch` / `unauthorized` / `stale_window_id` / `not_found` / `internal_error`

---

## 8. 内蔵機能（外部ツールを置換できるもの）

| 内蔵機能 | これにより不要になる外部ツール |
|---|---|
| `[borders]` ボーダー描画 | **JankyBorders 不要** |
| Quake terminal（libghostty 使用） | 別途 hammerspoon 等で組む必要なし |
| アニメーション（60/120/144Hz pacing） | — |
| Overview surface | — |
| Command Palette | — |
| Workspace Bar | — |
| Scratchpad | — |
| Mouse warp（モニタ間） | — |
| Trackpad ジェスチャ | swipe で WS / column 切替 |

---

## 9. 拡張ポイント — シェル + IPC で組めるもの

OmniWM 単体では足りない部分は、以下の組み合わせで実装する設計になる：

```
Karabiner ──(任意キー)──> shell script ──> omniwmctl
                                        ──> open -a, AppleScript 等
omniwmctl watch ──(イベント)──> shell script   （イベント駆動の自動化）
launchd ──(タイマ/起動)──> shell script        （定期実行）
```

### できること（実装例）
- **キー → 任意シェル実行**：Karabiner で alt-X → `omniwmctl command switch-workspace M; open -a Spotify`
- **モーダルキーバインド**：Karabiner の `set_variable` + `variable_if` で resize mode 等を再現
- **方向ベース focus-monitor**：`omniwmctl query displays` の geometry を読んで「左隣のモニタ」を計算 → `focus-monitor` を必要回数発行
- **アプリレイアウト自動構築**：`open` で起動 → `query windows` で polling → `window focus <id>` + `command move <dir>`（consume）+ `toggle-column-tabbed` で配置
- **イベント駆動アクション**：`omniwmctl watch display-changed -- <script>` でモニタ変化時に TOML 差替 + プロセス再起動
- **launchd KeepAlive** で OmniWM 本体を起動（AeroSpace と同型）

---

## 10. AeroSpace 既存機能 → OmniWM 移植マップ（確定版）

| AeroSpace の機能 | OmniWM 側の救済 | 実現度 |
|---|---|---|
| 数字 WS 切替（alt-N） | `switchWorkspace.N` | ◎ ネイティブ |
| WS への送り＋ジャンプ | `moveToWorkspace.N` | ◎ |
| `workspace-back-and-forth` | `workspaceBackAndForth` | ◎ |
| 名前付き WS（M / B / E） | `[[workspaces]] name = "M"` | ◎ |
| `on-window-detected → move-node-to-workspace` | `[[appRules]] assignToWorkspace` | ◎ |
| `floating` ルール | `[[appRules]] layout = "float"` | ◎ |
| `focus h/j/k/l` | `focus.left/right/up/down` | ◎ |
| `move h/j/k/l` | `move.left/right/up/down`（consume 込み） | ◎ |
| `join-with` でグループ化 | `command move` の consume 効果 | ◎ |
| `layout accordion` | `toggleColumnTabbed`（列内タブ表示） | ◎ |
| `fullscreen` | `toggleFullscreen` | ◎ |
| floating⇔tiling | `toggleFocusedWindowFloating` | ◎ |
| gaps | `[gaps]` | ◎ |
| マウス追従 | `[focus]` 各種 | ◎ |
| JankyBorders 連携 | 内蔵 `[borders]` | ◎（統合） |
| `focus-monitor h/j/k/l`（方向） | shell + `query displays` で自前実装 | ○ |
| WS→モニタ強制割当 | `monitorAssignment` (main/secondary/specificDisplay) | ○（番号指定不可） |
| モニタ枚数別プロファイル | TOML 差替 + プロセス再起動スクリプト | ○（チラつき許容） |
| `mode resize` モーダル | Karabiner variable で再現 | ○ |
| `cmd-h = []` 無効化 | Karabiner で空マップ | ○ |
| `alt-s/c/a` の WS切替+起動マクロ | Karabiner → shell + `omniwmctl` | ○ |
| `setupMediaWorkspace` 全工程 | shell + `query windows` + `move`（consume）+ `toggle-column-tabbed` | ○ |
| `after-startup-command` | 内蔵 borders で代替 | ◎ |
| **無停止 reload-config** | ❌ **存在しない**（プロセス再起動） | × |

---

## 11. 既知の制約・注意点

- **設定ホットリロード無し**：TOML を書き換えても再起動するまで反映されない
- **IPC はデフォルト OFF**：`general.ipcEnabled = true` または GUI から有効化必須
- **subscribe は最新値のみ届く**：lossless ではないので「全イベントを記録したい」用途には不向き
- **`stale_window_id` エラー**：window opaque-id はセッションスコープ。OmniWM 再起動を跨ぐと無効化される
- **pre-1.0 の仕様変動リスク**：IPC 互換性は v3 → v4 → v5 のように既に破壊的変更履歴あり
- **Trackpad ジェスチャ**：作者本人が実機テストできていないと明記

---

## 12. このリポジトリでの実装方針（叩き台）

このセクションは取説ではなく、移植議論の出発点メモ。

```
modules/darwin/omniwm/
├── default.nix               # myConfig.darwin.omniwm.enable で点く
├── common.nix                # gaps/borders/focus/niri/dwindle 等の共通 TOML
├── profiles/
│   ├── two-monitor.nix       # 2枚向け [[workspaces]] + monitorAssignment
│   ├── triple-monitor.nix    # 3枚向け
│   └── quad-monitor.nix      # 4枚向け
├── scripts/
│   ├── switch-profile.sh     # モニタ枚数検出 → TOML 差替 → OmniWM 再起動
│   ├── focus-monitor-dir.sh  # 方向ベース focus-monitor の自前実装
│   └── setup-media-workspace.sh  # AeroSpace のメディアレイアウト構築の OmniWM 版
└── README.md
```

`profiles/darwin.nix` で `myConfig.darwin.aerospace.enable` と `myConfig.darwin.omniwm.enable` を**排他**に切り替える。
（必要なら nix の `assertion` でどちらか1つだけ許可するガードを入れる）

Karabiner 側で OmniWM 用のマクロキー（alt-s/c/a 等）を定義する追加レイヤを `modules/darwin/karabiner/` に足す。

---

## 13. 参照

- リポジトリ: <https://github.com/BarutSRB/OmniWM>
- IPC-CLI ドキュメント: `docs/IPC-CLI.md`
- アーキテクチャドキュメント: `docs/ARCHITECTURE.md`
- 設定スキーマ実装: `Sources/OmniWM/Core/Config/`
- アクションカタログ: `Sources/OmniWM/Core/Input/ActionCatalog.swift`
- Niri エンジン: `Sources/OmniWM/Core/Layout/Niri/`
- 公式リリース: <https://github.com/BarutSRB/OmniWM/releases>
- macOS WM ディレクトリ: <https://macoswm.com/wm/omniwm>
