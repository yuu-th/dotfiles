# projwm 仕様書

> macOS 上で「AI コーディング project」を OmniWM の named workspace に 1:1 で割り当てて管理する基盤。
> 本文書は projwm を **これから知る人** が読む現状仕様。過去の改版経緯や試行錯誤は `projwm-history.md` を参照。

---

## 0. 概要

### 0.1 これは何か

projwm は macOS 上で:

- 1 つの **project**（典型的には 1 つの git worktree）を OmniWM の **named workspace = "slot"** に 1:1 で対応付ける
- 各 slot に `AI ターミナル + shell ターミナル + Zed editor` を自動配置する
- 複数 project の AI 出力を 1 つの **viewer workspace** で並列俯瞰する
- 複数の **profile** で「いまアクティブな project セット」を保存・切替する
- 状態は `state.json` に集約、実画面とのズレは **reconcile** で自動修正する

技術スタック: **Go バイナリ `projwm` + OmniWM (window manager) + tmux (永続化) + Ghostty (terminal) + Zed (editor) + bubbletea (TUI) + Karabiner (キーバインド)**。

### 0.2 30 秒 TL;DR

| 概念 | 値 |
|---|---|
| 1 project = 1 slot | OmniWM 名前付き WS (`Q W E R T Y U I O P`)、合計 10 個 |
| viewer | WS `A` に全 AI 窓を read-only で複製 |
| key | `alt+letter` で slot jump、`alt+shift+letter` で送る、`alt+a` で viewer |
| cockpit | `alt+\`` で `projwm tui`（bubbletea） |
| state of truth | `~/.local/state/projwm/state.json` |
| 自動同期 | launchd で `omniwmctl watch windows-changed` → reconcile |
| terminal | 純正 Ghostty (`com.mitchellh.ghostty`)、ai/shell は tmux ラップ |
| editor | 純正 Zed (`dev.zed.Zed`)、tmux ラップしない |

### 0.3 関連文書

| 文書 | 役割 |
|---|---|
| **`projwm-spec.md`** (本書) | 現状仕様 |
| **`projwm-ux.md`** | 理想のユーザ体験ストーリー |
| **`projwm-roadmap.md`** | 未着手の計画（v12 browser 統合など）|
| **`projwm-history.md`** | 過去の試行錯誤の archive |
| **`projwm-decisions.md`** | 今後の意思決定記録 |
| **`OMNIWM.md`** (リポジトリ直下) | OmniWM 自体の機能カタログ |

---

## 1. 用語

| 用語 | 定義 |
|---|---|
| **slot** | OmniWM の AI 用 named workspace。`Q W E R T Y U I O P` の 10 個。`alt+<letter>` で jump |
| **viewer** | WS `A`。全 project の AI を read-only で複製表示する画面 |
| **layout** | slot WS 上の column / stack 配置。`state.Window.Layout {Column, Stack, Tabbed}` に保存。archive / profile-switch 前に自動 snapshot、spawn 完了後に自動 restore |
| **project** | 1 つの作業 cwd（典型は 1 git worktree）。slot に動的に対応付けられる |
| **profile** | slot 割当の名前付きセット（例 `work` / `personal`）。同時に 1 つだけ active |
| **active profile** | 現在 slot に展開されているプロファイル |
| **archive** | project ごとの状態フラグ。`archived = true` で tmux kill + windows close、いずれの profile からも展開されない |
| **park (無所属 project)** | `projects` に居るがどの profile の `assignments` にも入っていない project。tmux は alive、windows は closed |
| **kind** | window の役割。`"ai"` / `"shell"` / `"editor"` の 3 種 |
| **id** | kind ごとの 1 始まり連番。永続採番、削除で穴が空いても再利用しない |
| **`(kind, id)` ペア** | 1 つの window の identity（project 内で一意）|
| **state file** | `~/.local/state/projwm/state.json`。runtime 状態 |
| **config file** | `~/.config/projwm/config.toml`。固定設定（slot 名群、viewer WS 名）|
| **reconcile** | state（期待状態）と OmniWM/tmux/ghostty/Zed の実状態を比較し、差分を是正する処理 |
| **basename uniqueness** | active な全 project（archived 除く）の `cwd basename` は一意でなければならない |
| **scratch** | OmniWM Quake terminal（libghostty 内蔵 fish）。ad-hoc コマンド実行用 |
| **cockpit** | `projwm tui` を Ghostty で起動した cockpit window（title=`projwm-cockpit`）|

---

## 2. 要件

### 2.1 機能要件 (FR)

| ID | 内容 |
|---|---|
| FR-01 | 任意の cwd を空き slot に割り当て、AI ウィンドウ + shell ウィンドウ + editor (Zed) ウィンドウを既定で起動できる |
| FR-02 | `alt+<letter>`（単一修飾＋単一キー）で任意の slot に jump できる |
| FR-03 | `alt+shift+<letter>` で現在ウィンドウを slot に送れる |
| FR-04 | `alt+a` で viewer (WS A) に jump できる |
| FR-05 | 全 project の AI を viewer で read-only に並列表示できる |
| FR-06 | TUI cockpit (`projwm tui`) で project 一覧の確認・jump・新規・archive・unarchive・unassign・remove が完結する |
| FR-07 | scratch shell が project 操作と独立して常時利用できる（OmniWM Quake terminal）|
| FR-08 | OmniWM 再起動・モニタ抜き差し後でも 1 分以内に意図状態に自動復帰する |
| FR-09 | `projwm reconcile --dry-run` で差分のみ表示できる |
| FR-10 | `projwm status` で全 slot の整合性を可視化できる |
| FR-11 | `projwm restore` で tmux サーバから生存 project を再構築できる（**未実装、roadmap**）|
| FR-12 | 同一 project に 2 つ目以降の AI ウィンドウ・shell・editor を追加できる |
| FR-13 | project を archive でき、tmux session と windows を片付け、slot を解放できる |
| FR-14 | 複数のプロファイル（`work` / `personal` 等）を保存し、いつでも切替できる |
| FR-15 | プロファイル切替時、旧プロファイルの project ウィンドウを閉じ、新プロファイルの project を slot に展開する |
| FR-16 | プロファイル切替で archived でない project の tmux session は生かしたまま（窓だけ閉じる）→ 再切替が瞬時 |
| FR-17 | unarchive で archived project を復活でき、profile + slot を指定すれば再展開される |
| FR-18 | プロファイル間の project の移動・複数プロファイルへの所属が可能（同 project が `work` と `personal` 両方で別 slot に居る等）|
| FR-19 | TUI cockpit に active profile / 他プロファイル / parked / archived の各セクションが常時表示される |
| FR-20 | viewer は active profile の project だけを表示する（profile 切替で viewer も自動入れ替え）|
| FR-21 | AI コマンド（`claude` / `copilot`）が tmux session 作成と同時に自動起動する |
| FR-22 | `up` の既定起動セットは ai-1 + shell-1 + editor-1。`--no-editor` で editor をスキップ可能 |
| FR-23 | editor (Zed) は project の cwd を開き、profile 切替で window が close、active 復帰で再 spawn される |
| FR-24 | 同一 project 内で複数の AI（claude と copilot 同時、または同種の複数並走）が first-class に共存可能 |
| FR-25 | viewer は project 単位ではなく **AI ウィンドウ単位** で複製を表示（AI が 2 個なら viewer に 2 タイル）|
| FR-26 | project ごとに browser window を windows[] に追加可能（kind=browser、v12 paradigm C）。state.Window は `browser_profile` / `saved_urls` / `live_window_id` を持つ |
| FR-27 | browser の **spawn / close は明示イベントでのみ** 発火: `add-browser` / `profile switch` / `profile assign|unassign` / `archive-project` / `unarchive` / `remove --window=browser-N` / TUI の a/u/d。reconcile は browser に絶対触らない |
| FR-28 | profile 切替・assign/unassign で active 外になった project の browser window は **URL snapshot → close**、active 復帰した project は **`SavedURLs` から再 spawn** |
| FR-29 | browser の destructive 操作（spawn / close）の前後で **frontmost app を保存 → 復帰**。Vivaldi の遅延 focus 強奪に対抗するため多段 Activate |
| FR-30 | TUI の destructive 操作（a archive, u unarchive, d unassign）は **外部 projwm cmd を exec** で呼ぶ。paradigm C ライフサイクル hook の単一実装を共有 |
| FR-31 | archive / profile-switch 前に slot WS 上の column / stack 配置を snapshot し、unarchive / profile-switch 後の spawn 完了時に自動 restore する。browser kind は対象外 |
| FR-32 | ユーザがスロット WS 内で手動にコラム順やスタック順を変更した場合、その変更を state.json の Layout フィールドに反映する（ユーザ意図を尊重）|
| FR-33 | ユーザが規約 title ウィンドウを誤った WS に移動した場合、reconcile が正しい slot へ戻す（§7.3 の自動修正ポリシーに基づく）|
| FR-34 | ユーザが projwm 管理ウィンドウを直接閉じた場合（kill/close）、reconcile が元の column 位置で再 spawn する。`projwm remove` によるのみが意図的削除として扱われ、再 spawn しない |

### 2.2 非機能要件 (NFR)

| ID | 内容 |
|---|---|
| NFR-01 | コマンドは冪等（同操作の繰り返しで破壊的にならない）|
| NFR-02 | state file 編集は `flock(2)` で排他、書き込みは tmpfile + atomic rename |
| NFR-03 | reconcile は副作用最小（state にない既存ウィンドウは保持、自動 close しない）|
| NFR-04 | 全コマンドの実行ログを `~/.local/state/projwm/logs/` に蓄積 |
| NFR-05 | キーバインドは macOS 他アプリと衝突しない（`cmd` 修飾の使用禁止）|
| NFR-06 | OmniWM `[[workspaces]]` 定義の追加で導入できる。既存の数値 WS 1〜9 / M / B は破壊しない |
| NFR-07 | Go バイナリ単体で動作（外部 fish 関数依存なし、Karabiner からも直接起動可能）|
| NFR-08 | プロファイル切替は **windows 操作のみ** で完結し、tmux session の kill/start を伴わない（高速保証）|
| NFR-09 | アーカイブ操作は idempotent（archived 済みを再 archive しても no-op）|
| NFR-10 | state.json はバージョニングを持たない（schema 進化が必要になった時点で `version` フィールドを足す）|
| NFR-11 | 設定値（slot 名群、viewer WS 名等）は **state ではなく config (`config.toml`)** に置く |
| NFR-12 | active な全 project（archived 除く）で `cwd basename` が一意。`up` 時に validate、衝突は `--as <name>` で内部名分離 or rename を促す |
| NFR-13 | GUI app（editor 等の tmux ラップしない window）は projwm の title 規約 (`<kind>-<id>:<project>`) を強制しない。bundleId と title pattern の組合せで identify |
| NFR-14 | reconcile の Zed spawn は flock ベースの spawn lock で並走重複を防ぐ |
| NFR-15 | 全 projwm 操作はフォーカス状態に依存しない。どの WS・どのウィンドウにフォーカスがあっても操作結果は同一でなければならない。操作内部で一時的な WS focus が必要な場合は操作後に元の WS を復元する |

### 2.3 非要件 (NR、明示的にやらないこと)

| ID | 内容 | 理由 |
|---|---|---|
| NR-01 | 動的 appRule での project 専用 browser 自動固定 | ユーザ判断で送る派 |
| NR-02 | modal / leader-key UX | 押下数増を嫌う |
| NR-03 | shell 永続化を tmux 以外の手段で行う | tmux で十分 |
| NR-04 | nvim を kind として持つ | Zed が first-class editor。nvim 派は shell-N の tmux 内で起動 |
| NR-05 | GUI editor を tmux に押し込む | tmux は terminal multiplexer、GUI app は対象外 |
| NR-06 | state を SQLite 等の DB で持つ | JSON で十分、可搬性と人間可読性優先 |
| NR-07 | 複数プロファイルを **同時に active** 状態にする | slot 衝突解消ロジックが複雑、UX も曖昧 |
| NR-08 | アーカイブ済 project の自動再活性（時間経過 / 利用頻度ヒューリスティック）| 暗黙的復活はバグの温床、明示的 unarchive のみ |

---

## 3. アーキテクチャ

### 3.1 全体構成図

```
┌──── Karabiner ─────────────────────────────────────────────────┐
│  alt+1..9                  日常 WS                             │
│  alt+m/b                   既存 named WS（M/B）                 │
│  alt+a / alt+shift+a       viewer (WS A) jump / 送る            │
│  alt+q/w/e/r/t/y/u/i/o/p   slot へ直接 jump                     │
│  alt+shift+ 同             slot へ窓を送る                       │
│  alt+space                 OmniWM Quake terminal = scratch      │
│  alt+`                     ghostty で `projwm tui` = cockpit    │
└────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                      ┌─ OmniWM (window manager) ─┐
                      │   workspaces / hotkeys     │
                      │   appRules（titleRegex）   │
                      │   omniwmctl IPC            │
                      └────────────────────────────┘
                                │
                                ▼
                      ┌─ projwm (Go binary) ──────┐
                      │   cobra CLI                │
                      │   bubbletea TUI            │
                      │   reconcile                │
                      │   state.json (flock)       │
                      └────────────────────────────┘
                                │
                  ┌─────────────┼─────────────────┐
                  ▼             ▼                 ▼
            tmux server    Ghostty windows    Zed windows
            ai-N/<proj>    ai-N:<project>     <basename>
            shell-N/<proj> shell-N:<project>
            ai-N/<proj>_v  ai-view-N:<project>
```

### 3.2 主要レスポンシビリティ

| コンポーネント | 責務 |
|---|---|
| Karabiner | 物理キー → omniwmctl コマンド or projwm CLI 実行 |
| OmniWM | WS の定義、ウィンドウのタイル / 移動 / Quake / appRule / IPC |
| omniwmctl | OmniWM 操作の唯一のインターフェース |
| tmux server | AI / shell の永続セッション、grouped session による viewer 複製 |
| Ghostty | tmux client を表示する terminal window（kind = ai / shell）|
| Zed | GUI editor（kind = editor）、bundleId `dev.zed.Zed` |
| state.json | 「あるべき姿」の source of truth |
| projwm CLI | state を編集、reconcile を駆動 |
| projwm TUI (bubbletea) | cockpit 操作画面 |
| reconcile | 期待状態と実状態の差分検出と修正 |
| launchd | reconcile を `omniwmctl watch` 駆動 + 60s 定期で常駐 |

---

## 4. ワークスペース層

### 4.1 OmniWM workspace 構成

| 種別 | 名前 | 用途 | 番号 (rawName) | キー |
|---|---|---|---|---|
| 既存（手付かず）| `1`〜`9` | 日常用 | 1〜9 | `alt+1..9` |
| 既存 | `M` | Media | 10 | `alt+m` |
| 既存 | `B` | Browser | 11 | `alt+b` |
| **新設（projwm）**| `A` | **AI Viewer** | 12 | `alt+a` |
| 新設 | `Q` | AI project slot 1 | 13 | `alt+q` |
| 新設 | `W` | slot 2 | 14 | `alt+w` |
| 新設 | `E` | slot 3 | 15 | `alt+e` |
| 新設 | `R` 〜 `P` | slot 4 〜 10 | 16〜22 | `alt+r` 〜 `alt+p` |

合計 22 workspace。すべて OmniWM の scrolling-columns レイアウト。

### 4.2 命名規則

- 物理キーが WS 名そのもの。`alt+q` → `omniwmctl workspace focus-name Q`
- 文字 slot は空のとき `hideEmptyWorkspaces=true` で workspace bar から自動非表示
- monitorAssignment は全 slot `main` 固定（マルチモニタ運用時は monitor-profile で override）

### 4.3 廃止された WS と振替

| 旧 | 新 |
|---|---|
| 旧 `WS E` Editor 用途 | **AI slot 3**（rawName 15）に再利用。Zed は projwm が per-project に slot 配置するため一律集約は不要 |
| `alt+a` Calendar 起動マクロ | viewer (WS A) jump に振替。Calendar は手動 / Spotlight / Dock 起動 |
| 旧 `dev.zed.Zed` startup-sort entry | 削除。projwm が per-project に slot 配置するため不要 |

---

## 5. プロセス・セッション層

### 5.1 tmux session 構成（1 project あたり）

| セッション名 | 役割 | 永続化 | 備考 |
|---|---|---|---|
| `ai-N/<proj>` | AI 本体（claude or copilot）| 必須 | `tmux attach -t` で各 AI 窓から接続 |
| `ai-N/<proj>_v` | AI N の viewer 用 grouped clone | 必須 | viewer 窓から `tmux attach -r -t`（read-only）|
| `shell-N/<proj>` | 自由シェル | 必須 | 外部から `capture-pane` / `send-keys` で観測・操作可能 |

すべて `1` 始まり連番。「最初の 1 個だけ無連番」のような特例なし。**多 AI / project は first-class** で、各 AI は独立 tmux session + viewer clone を持つ。

#### 5.1.1 AI 自動起動

`ai-N/<proj>` session を新規作成した直後、projwm が以下を発行:

```
tmux send-keys -t <session> '<aiCommand>' Enter
```

`<aiCommand>` は `naming.AICommand(ai)`：
- `claude` → `"claude"`
- `copilot` → `"copilot"`

これにより shell prompt 表示後に AI コマンドが打鍵される（300ms wait で shell の起動を待つ）。

#### 5.1.2 grouped session の生成

```sh
tmux new-session -d -s ai-1/<proj>                       # 本体
tmux new-session -d -t ai-1/<proj> -s ai-1/<proj>_v      # 同 pty 共有 clone（read-only）
```

注: tmux は session 名内の `:` を許容しないため、viewer は **`_v` 末尾**を使う。

#### 5.1.3 リサイズ衝突回避

`~/.tmux.conf`:

```
set -g window-size latest
set -g aggressive-resize on
```

最後にフォーカスした client に追従。

#### 5.1.4 多 AI 並走

| 項目 | 挙動 |
|---|---|
| 同 slot に複数 AI 窓 | OmniWM が自動タイル。1 column 4 窓まで縦スタック、5 窓目以降は新 column |
| viewer | 各 AI 窓と 1 対 1 対応の viewer 窓が並ぶ |
| profile 切替 | inactive 化 → 全 AI 窓 + 全 viewer 窓を close、tmux は alive |
| archive | 全 AI session + 全 viewer clone を kill |
| `add-ai` | id = 現存最大 + 1（穴は埋めない、永続採番）|

### 5.2 Ghostty / Zed window 構成

| ウィンドウ title | App | tmux | 配置 WS |
|---|---|---|---|
| `ai-N:<proj>` | Ghostty | yes (`ai-N/<proj>`) | project slot |
| `ai-view-N:<proj>` | Ghostty | yes (`ai-N/<proj>_v` read-only) | viewer (`A`) |
| `shell-N:<proj>` | Ghostty | yes (`shell-N/<proj>`) | project slot |
| `<basename(cwd)>` | Zed | no | project slot |
| `projwm-cockpit` | Ghostty | no | 自由配置（floating 推奨）|

### 5.3 title 規約と naming 関数

projwm 内部の `internal/naming` パッケージ:

| 関数 | 入力 | 出力例 |
|---|---|---|
| `GhosttyTitle(kind, id, project)` | (ai, 1, dotfiles) | `ai-1:dotfiles` |
| `TmuxSession(kind, id, project)` | (ai, 1, dotfiles) | `ai-1/dotfiles` |
| `ViewerGhosttyTitle(id, project)` | (1, dotfiles) | `ai-view-1:dotfiles` |
| `ViewerTmuxSession(id, project)` | (1, dotfiles) | `ai-1/dotfiles_v` |
| `ZedTitle(cwd)` | /Users/yuta/dev/dotfiles | `dotfiles` |

定数:
- `TerminalBundleID = "com.mitchellh.ghostty"`（純正 Ghostty）
- `ZedBundleID = "dev.zed.Zed"`

state.json には title / tmux session 名は **保存しない**。`(kind, id, project)` から決定的に算出する。

### 5.4 起動コマンド規約

```sh
# kind=ai / shell の場合
open -na /Applications/Ghostty.app --args \
     --title=ai-1:dotfiles \
     --working-directory=/Users/yuta/dev/dotfiles \
     -e tmux new-session -A -s ai-1/dotfiles

# kind=editor (Zed) の場合
zed -n /Users/yuta/dev/dotfiles
# `-n` (--new) は必須。素の `zed <path>` は既存 workspace を再利用してしまう
```

`-A` フラグ: tmux は session があれば attach、無ければ作成。冪等性確保。

### 5.5 OmniWM app-rule（titleRegex で SwiftUI 対応）

`modules/darwin/omniwm/app-rules.nix`:

```nix
# projwm 規約 title の Ghostty window のみ tile 強制管理
{ bundleId   = "com.mitchellh.ghostty";
  titleRegex = "^(ai|shell|ai-view)-[0-9]+:";
  layout     = "tile";
  minWidth   = 480.0; minHeight = 240.0; }

# Ghostty 一般（projwm 管理外）
{ bundleId  = "com.mitchellh.ghostty";
  minWidth  = 480.0; minHeight = 240.0; }
```

**重要**: Ghostty (SwiftUI WindowGroup app) は起動時に hidden helper windows を複数作る。OmniWM の rule engine は `titleRegex` で projwm 規約 title のみ matched user rule と認識し、disposition=`.managed` 判定 → admit 成功する。この rule が無いと hidden helpers と main を区別できず admit されない。

---

## 6. 状態管理層

### 6.1 ファイル配置

```
~/.local/state/projwm/        ← XDG_STATE_HOME 配下、runtime のみ
├── state.json                ← source of truth
├── state.json.bak            ← 直前のバックアップ
├── lock                      ← flock(2) ファイル
└── logs/
    ├── reconcile.log
    └── commands.log

~/.config/projwm/             ← XDG_CONFIG_HOME 配下、固定設定
└── config.toml
```

state は Nix 管理外、config は projwm のデフォルトから Nix 経由で配置（手動編集も可）。

### 6.2 config.toml

```toml
viewer_workspace = "A"
slot_names = ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"]
terminal_app_path = "/Applications/Ghostty.app"
terminal_bundle_id = "com.mitchellh.ghostty"
```

#### 6.2.1 fallback ポリシー

| 状況 | 挙動 |
|---|---|
| ファイル不在 | デフォルト埋め込みで動作。`projwm doctor` が INFO 通知 |
| TOML パース失敗 | エラー終了、修正 or 削除を促す |
| 未知フィールド | 警告のみ（forward compat）|
| 必須フィールド欠落（`slot_names` 空配列など） | エラー終了 |

### 6.3 state.json schema

```json
{
  "active_profile": "work",
  "profiles": {
    "work": {
      "description": "Work AI sessions",
      "assignments": {
        "Q": "dotfiles",
        "W": "manaflow",
        "E": "client-x"
      }
    },
    "personal": {
      "description": "Personal experiments",
      "assignments": {"Q": "blog"}
    }
  },
  "projects": {
    "dotfiles": {
      "cwd": "/Users/yuta/dev/dotfiles",
      "archived": false,
      "windows": [
        { "id": 1, "kind": "ai",    "ai": "claude"  },
        { "id": 2, "kind": "ai",    "ai": "copilot" },
        { "id": 1, "kind": "shell"                  },
        { "id": 1, "kind": "editor"                 },
        { "id": 1, "kind": "browser",
          "browser_profile": "work",
          "saved_urls": ["https://github.com/...", "https://example.com/..."] }
      ]
    },
    "park-1":    { "cwd": "...", "archived": false, "windows": [
        { "id": 1, "kind": "ai", "ai": "claude" }
    ]},
    "old-thing": { "cwd": "...", "archived": true,  "windows": [...] }
  }
}
```

#### 6.3.1 フィールド

| フィールド | 型 | 説明 |
|---|---|---|
| `active_profile` | string | 現在 active なプロファイル名 |
| `profiles` | map<string, Profile> | プロファイル名 → 定義 |
| `profiles[name].description` | string | 人間可読な説明（任意）|
| `profiles[name].assignments` | map<slot, project> | slot 名 → project 名 |
| `projects` | map<string, Project> | project 名 → 定義 |
| `projects[name].cwd` | string | 絶対パス |
| `projects[name].archived` | bool | archived か |
| `projects[name].windows[]` | array | 意図ウィンドウ群 |
| `projects[name].windows[].id` | int | kind ごとの連番（1 始まり）|
| `projects[name].windows[].kind` | enum | `"ai"` / `"shell"` / `"editor"` |
| `projects[name].windows[].ai` | enum? | `kind="ai"` のみ必須。`"claude"` / `"copilot"` |
| `projects[name].windows[].browser_profile` | string? | `kind="browser"` のみ必須。Chromium の `--profile-directory` 引数で指定する user profile 名 |
| `projects[name].windows[].saved_urls` | string[]? | `kind="browser"` のみ。close 直前に snapshot された tab URL 一覧。再 spawn 時に復元する |
| `projects[name].windows[].live_window_id` | string? | `kind="browser"` のみ。chrome-cli の window-id（spawn で設定、close で空）。再起動で stale になっても close は no-op で安全 |
| `projects[name].windows[].layout` | object? | column/stack 配置 snapshot。`{column, stack, tabbed}` 形式。archive / profile-switch 前に自動保存、spawn 完了後に restore。browser には適用しない |

スキーマには「primary」「default」「main」の階層的概念を持たない。すべての AI 窓は完全に対等。`(kind, id)` ペアが identity。

#### 6.3.2 不変条件

- `active_profile` は必ず `profiles` の既存キー（空文字は初期状態として許容）
- `profiles[*].assignments` の値は必ず `projects` の既存キー
- archived = true な project は active profile の assignments に居てはいけない（archive 操作時に自動解除）
- 同一 profile 内で同一 slot に複数 project を map しない（map 構造で物理保証）
- 同一 profile 内で同一 project を複数 slot に map しない
- `(kind, id)` ペアは project 内で一意
- `id` は kind ごとに 1 始まり、最大値 + 1 で採番。**穴を再利用しない**
- `kind="ai"` の窓は必ず `ai` フィールドを持つ。それ以外は持たない
- `kind="editor"` は project あたり id=1 が標準（多 editor 並走は実装可だが Zed の挙動に依存）
- 空 `windows[]` を許容（cwd だけ覚えている "metadata-only project"、archive とは別）
- active 全 project（archived 除く）で `path.basename(cwd)` が一意

### 6.4 排他制御

- 全書き込みは `flock(2)` で `lock` ファイルを排他取得
- 書き込みは `state.json.tmp` に書いて `rename(2)` で atomic 差替
- 読み込みは lock 不要（atomic rename により部分書込状態は読まれない）
- 破損時は `state.json.bak` から復旧、無ければ操作中断

### 6.5 profile / archive / park ライフサイクル

#### 6.5.1 profile switch

```
current = state.profiles[state.active_profile].assignments
target  = state.profiles[<new>].assignments

closing  = { p for slot,p in current if (slot,p) not in target }
opening  = { p for slot,p in target  if (slot,p) not in current }
moving   = { (p, old_slot, new_slot) }   # slot だけ違う

for p in closing:
    p の全 windows を close（ai + viewer + shell + editor）
    tmux session は touch しない
state.active_profile = <new>
for p in opening:
    spawn all of p.windows in target slot（reconcile が穴埋め）
    spawn all viewer windows for p's AI windows in WS A
for (p, old_slot, new_slot) in moving:
    p の全 windows を一括で new_slot へ move-to-workspace
projwm reconcile  # 整合性最終確認
```

保証:
- tmux session の kill / start は伴わない（NFR-08）
- moving は project の全 window が一括で新 slot へ移動（half-moved 状態を作らない）

#### 6.5.2 archive

`projwm archive-project <project>` 実行時:
1. `projects[<project>].archived = true`
2. 全 profile の `assignments` から該当 project を値として持つ slot を削除
3. project の全 ghostty windows を close
4. project の全 tmux session を kill（ai-N/<proj>, ai-N/<proj>_v, shell-N/<proj>）
5. editor は window close のみ（Zed 自体は kill しない、他 project の Zed window があれば共存）
6. state save → reconcile

#### 6.5.3 unarchive

`projwm unarchive <project> [--profile X --slot Y]`:
1. `archived = false`
2. `--profile + --slot` 指定で active 化、無指定なら **park 状態**（無所属）
3. park は first-class（warning 出さない、launcher で別エリア表示）

#### 6.5.4 park

`projects` に居るがどの profile の `assignments` にも入っていない project の状態:

| 観点 | 状態 |
|---|---|
| state | `projects[name]` に存在、`archived: false` |
| profile assignment | 無し |
| tmux session | **alive のまま**（archive と区別）|
| ghostty 窓 | **closed** |
| viewer | 表示されない |
| TUI 表示 | "parked projects" セクション |
| 復活 | profile に assign するだけ |

park になる経路:
- `profile unassign <slot>` で外す
- 所属していた profile が `delete` された
- `unarchive <project>` で archived から復活した直後（profile 未指定時）

---

## 7. Reconcile

### 7.1 期待 vs 実

`projwm reconcile` は以下のループを 1 回実行:

```
1) active profile の slot 配置
   for slot, project in active.assignments:
       for w in project.windows:
           ensure ghostty window (kind=ai/shell) at slot
              + tmux session
              + AI 自動起動 (kind=ai 新規 session 時)
           ensure grouped session (kind=ai)
           ensure viewer ghostty window in WS A (kind=ai)
           ensure Zed window at slot (kind=editor)

2) viewer (WS A) orphan 掃除
   active な AI 窓に対応しない viewer 窓を close

3) inactive (park or 他 profile) project の windows close
   tmux は alive を維持

4) archived project の完全片付け
   全 windows close + 全 tmux kill + viewer kill

5) --gc 指定時のみ orphan ghostty 窓を一括 close
```

### 7.2 修正アクション一覧

| Op | 内容 | 冪等 |
|---|---|---|
| `new-tmux` | tmux session 作成 | ✓ |
| `new-grouped` | viewer 用 grouped clone 作成 | ✓ |
| `spawn-ghostty` | Ghostty window を `open -na` で起動 | ✓ |
| `spawn-zed` | Zed を `zed -n <cwd>` で起動 | ✓ |
| `move-to-ws` | window を WS に移動（focus → move-column-to-workspace）| ✓ |
| `close-window` | window を close（PID kill）| ✓ |
| `kill-tmux` | tmux session を kill | ✓ |
| `ai-launch` | tmux send-keys で AI コマンド打鍵 | new session 時のみ |
| `skip-zed-spawn` | flock で並走 spawn を回避 | 観測用 |

### 7.3 orphan の扱い

| 状況 | 扱い |
|---|---|
| 規約 title だが state に対応 project 無い ghostty 窓 | **触らない**（無所属 project の存在は first-class）|
| Zed window の title が state の basename に一致しない | **触らない**（手動 open は管理外）|
| project slot 内に title 規約外の窓（手動で送った Chrome 等）| **触らない** |
| tmux に `<kind>-<id>/*` 系 session が残っているが state に無し | **触らない**（`projwm restore` 待ち）|
| 規約 title だが規約外 WS にいる | state にあれば正しい slot へ修正、無ければ触らない |
| cmuxterm (title="shell", bundleId=`com.cmuxterm.app`) on WS 3 | **触らない**（state 管理外、Phase 7 撤去対象だが当面は existing window として保持。NFR-03 の原則適用）|

`--gc` オプション付きで実行した時のみ orphan を一括 close。

### 7.4 起動トリガと debounce

| トリガ | 用途 | 実装 |
|---|---|---|
| 手動 `projwm reconcile` | ユーザ明示修正 | CLI |
| `omniwmctl watch windows-changed` | ウィンドウ閉じ等の即応 | launchd `org.nixos.projwm-reconcile-watch` |
| `omniwmctl watch display-changed` | モニタ抜き差し対応 | launchd `org.nixos.projwm-reconcile-display` |
| 60s 定期 | watch 取りこぼし backstop | launchd `org.nixos.projwm-reconcile-periodic` |
| `up` / `add-ai` / `archive` 等の最後 | コマンド完了直後の整合確認 | CLI 内部呼び出し |

`omniwmctl watch` は短時間に大量 event を発火しうる。**500ms debounce** を `projwm-reconcile-debounced` ラッパで挟む（`flock` + marker file）。

### 7.5 多 AI / Zed loop 対策

#### Zed spawn lock

Zed は `-n` で起動するたびに新 workspace を追加するため、reconcile が「Zed window 見えない → spawn」を頻発すると **無限増殖**する。対策:

1. **polling 待ち**: Zed window 不在時、4 秒間 polling で出現を待つ（直前 spawn の認識遅延を吸収）
2. **flock spawn lock**: `~/.cache/projwm/.locks/zed-spawn-<sha1prefix>.lock` で並走 reconcile からの重複 spawn を抑止
3. 並走時は `skip-zed-spawn` action で skip 観測可能

#### Ghostty タイミング対策

Ghostty 起動は数秒かかるため、reconcile が "spawn 直後に未認識 → 再 spawn" しないよう、tmux 既存時は 3 秒 polling 待ちを挟む。

---

## 8. ユーザインタフェース

### 8.1 キーバインド完全一覧

| キー | 動作 | 実装 |
|---|---|---|
| `alt+1..9` | 日常 WS jump | OmniWM hotkeys |
| `alt+shift+1..9` | 日常 WS へ窓を送る | OmniWM hotkeys |
| `alt+m / alt+b` | 既存 M/B WS jump | Karabiner shell |
| `alt+shift+m / alt+shift+b` | 同 move | Karabiner shell |
| `alt+ctrl+h/j/k/l` | focus-monitor 方向 | Karabiner |
| `alt+ctrl+m` | setup-media-workspace | Karabiner |
| `alt+s/c` | アプリ起動マクロ（Spotify/Discord）| Karabiner |
| **`alt+a`** | viewer (WS A) jump | Karabiner shell |
| **`alt+shift+a`** | viewer へ窓を送る | Karabiner shell |
| **`alt+q/w/e/r/t/y/u/i/o/p`** | AI slot jump | Karabiner shell |
| **`alt+shift+q/.../p`** | AI slot へ窓を送る | Karabiner shell |
| **`alt+space`** | OmniWM Quake terminal = scratch | Karabiner shell |
| **`alt+\``** | Ghostty で `projwm tui` 起動 = cockpit | Karabiner shell |
| `alt+shift+\`` | OmniWM Quake terminal toggle（旧 alt+`）| OmniWM hotkeys |

### 8.2 cockpit (`alt+\``)

`alt+\`` 押下で `open -na /Applications/Ghostty.app --args --title=projwm-cockpit -e projwm tui` を発火。新規 Ghostty window として `projwm tui`（bubbletea TUI）が起動。

#### TUI 画面構成（v11.6 実装）

```
projwm cockpit
profile: work    parked: 1    archived: 3
> _

active slots
  [Q] dotfiles    /Users/yuta/dev/dotfiles
       ai-1     claude    tmux●  win●
       ai-2     copilot   tmux●  win●
       shell-1            tmux●  win●
       editor             —      win●
  [W] manaflow    /Users/yuta/work/manaflow
       ai-1     claude    tmux●  win●
       ...
  [E] (empty)
  [R] (empty)
  ...

other profiles
  ● personal   2 assignments — Personal experiments

parked (no slot, tmux alive)
  • spike-x   /Users/yuta/dev/spike-x

archived
  ▼ old-thing   /Users/yuta/dev/old-thing

viewer (WS A)
  [A] 3 ai stream(s): dotfiles ai-1, dotfiles ai-2, manaflow ai-1

↵ jump  tab profile  n new  d unassign  a archive  u unarchive  r remove  ? help  esc quit
```

#### 操作

| キー | 動作 |
|---|---|
| 文字入力 | fzf 風 incremental filter |
| ↑↓ / ctrl+jk | 行移動 |
| Enter | 選択行のアクション（slot → jump、profile → activate）|
| Tab | active profile を循環切替 |
| `n` | 新規 project 作成プロンプト（cwd 名入力）|
| `d` | active profile から unassign（park 化）|
| `a` | archive |
| `u` | unarchive |
| `r` | remove window（`projwm remove --window <kind-N>` 相当、意図的削除 = 再 spawn しない）|
| `?` | help |
| esc / ctrl+c | 終了 |

リアクティブ更新: `fsnotify` で state.json の変化を即反映 + 2 秒定期 probe で tmux/window 状態追従。

### 8.3 scratch (`alt+space`)

OmniWM 内蔵 Quake terminal（libghostty + 固定 fish）。画面 50% で down からスライドイン。**projwm 管理外**、ad-hoc コマンド用。

### 8.4 CLI コマンド完全リスト

```
projwm [global flags] <subcommand>

== project lifecycle ==
up --ai <claude|copilot> [--cwd <path>] [--profile <name>] [--slot <X>]
   [--as <name>] [--no-editor]
        cwd を project として登録、active profile（または指定 profile）の
        空き slot に割当。ai-1 + shell-1 + editor-1 を既定起動。
        --ai は必須。--as で project 名を basename と切り離せる。
        basename uniqueness を validate。
jump <slot|name>            slot 名 / project 名 / profile 名で jump
add-ai --ai <claude|copilot> [--project <p>]
add-shell  [--project <p>]
add-editor [--project <p>]
remove --window <kind-N> [--project <p>]
        例: remove --window ai-2 / remove --window editor-1

== profile management ==
profile list
profile show [<name>]
profile create <name> [--description=...]
profile delete <name>                 # active は拒否
profile switch <name>                 # 切替時に reconcile 自動発火
profile assign <slot> <project>
profile unassign <slot|project>
profile rename <old> <new>            # active なら自動追従

== archive ==
archive-project <project>             # tmux kill + windows close + state 保全
unarchive <project> [--profile <name> --slot <X>]
archive list
archive purge <project> --yes         # state 完全削除（unrecoverable）

== state & 整合性 ==
reconcile [--dry-run] [--verbose] [--gc]
status [--json]
restore                               # 未実装、roadmap
state {show|edit|repair}
doctor

== UI ==
tui                                   # bubbletea cockpit を起動

== global flags ==
--state-dir <path>     既定 ~/.local/state/projwm
--no-reconcile         コマンド後の自動 reconcile を抑止
--profile <name>       active 以外の profile に対して実行（一部のみ対応）
```

#### 8.4.1 冪等性

| コマンド | 冪等動作 |
|---|---|
| `up` | 既登録 → focus only（再 spawn しない）|
| `jump` | 既にその WS にいる → no-op |
| `reconcile` | 何度実行しても結果が同じ |
| `profile switch` | 既に active → no-op |
| `profile unassign` | 既に外れている → no-op |
| `archive-project` | 既に archived → no-op |
| `unarchive` | 既に非 archived → no-op |
| `remove --window` | 該当窓が無ければ no-op |

---

## 9. 実装層

### 9.1 Nix モジュール構成

```
modules/darwin/projwm/
├── default.nix              # myConfig.darwin.projwm.enable で点く
├── sudoers.nix              # 開発・運用用の sudo nopass 設定（一時的）
├── projwm/                  # Go ソース
│   ├── go.mod / go.sum
│   ├── main.go
│   ├── cmd/                 # cobra subcommand 群
│   │   ├── root.go
│   │   ├── up.go
│   │   ├── add.go           # add-ai / add-shell / add-editor
│   │   ├── archive.go / archive_top.go
│   │   ├── jump.go (in up.go)
│   │   ├── layout_lifecycle.go      # layout snapshot/restore の cmd 層ヘルパ
│   │   ├── layout_integration_test.go  # build:integration T01-T21 統合テスト
│   │   ├── profile.go
│   │   ├── reconcile.go / reconcile_helper.go
│   │   ├── state.go
│   │   ├── status.go
│   │   ├── tui.go
│   │   └── doctor.go
│   └── internal/
│       ├── config/          # config.toml ロード
│       ├── ghosttywrap/     # Ghostty 起動ラッパ
│       ├── naming/          # title/tmux 名の決定的算出
│       ├── omniwm/          # omniwmctl ラッパ
│       ├── reconcile/       # 差分検出と修正、zedlock、layout.go（column/stack snapshot/restore）
│       ├── state/           # state.json (flock + atomic rename)
│       ├── terminalsetup/   # ghostty/kitty driver 抽象（現在 unused、将来用）
│       ├── tmuxwrap/        # tmux ラッパ
│       ├── tui/             # bubbletea cockpit
│       └── zedwrap/         # zed CLI 起動
└── README.md                # （未作成、将来）

modules/common/tmux.nix      # tmux + projwm 用 tmux.conf
modules/darwin/zed.nix       # Zed (homebrew cask)
modules/darwin/omniwm/
├── app-rules.nix            # titleRegex で Ghostty SwiftUI 対応
├── hotkeys.nix              # OmniWM 内蔵キーバインド
├── karabiner-rules.nix      # alt+letter / alt+shift / alt+space / alt+`
├── workspace-builder.nix    # 22 workspace 定義
└── workspace-assignment.nix # bundleId → 番号 (startup-sort)
```

### 9.2 Go バイナリ構成

- Go 1.22+ / `cobra` (CLI) / `bubbletea` + `lipgloss` (TUI) / `fsnotify` (state watch) / `gofrs/flock` (排他) / `BurntSushi/toml` (config)
- `omniwmctl` / `tmux` / `Ghostty.app` / `zed` は外部 binary。`os/exec` で薄くラップ
- 表テストで主要関数をカバー（reconcile / naming / state validation / config）

### 9.3 統合テスト

`cmd/layout_integration_test.go`（build tag `integration`）に T01〜T21 の E2E テストを定義。実機 macOS + 実 OmniWM 環境でのみ動作する。

```sh
# 実行方法
go test -tags integration -v -timeout 300s ./cmd/...
```

#### フォーカス独立性テスト（NFR-15）

各 T シナリオは複数の「開始フォーカス位置」から繰り返す。projwm 操作はどの WS にいても同一結果を保証しなければならない（NFR-15）。

```
開始 WS バリエーション:
  - "A"  (viewer)
  - "Q"  (active slot — 操作対象 WS と同じ)
  - "W"  (active slot — 操作対象とは別)
  - "E"  (active slot)
  - "M"  (isolated, managed 外)
  - "1"  (numeric, Discord)
  - "B"  (Browser WS)
  - "3"  (cmuxterm WS)
```

テストは `for _, startWS := range focusVariants { focusWS(startWS); runScenario() }` 形式で網羅する。

#### テスト項目

| ID | 内容 | 状態 |
|---|---|---|
| T01 | profile switch で全 window が理想レイアウトに戻る | ❌ FAILING (OI-9 のバグ) |
| T03 | archive → unarchive で同スロット・同レイアウト復元 | ❌ 未実装 |
| T04 | archive → unarchive を別 slot で復元 | ❌ 未実装 |
| T05 | 複数 project の同時 archive/unarchive | ❌ 未実装 |
| T06 | profile 切替 → 旧 profile 戻し（往復）| ❌ 未実装（T01 バグに依存）|
| T07 | add-ai で viewer に新 tile 追加 | ❌ 未実装 |
| T08 | add-shell でスタック追加 | ❌ 未実装 |
| T09 | `projwm status` でゼロ差分を返す | ✅ PASSING |
| T10 | `projwm reconcile` 冪等性（2 回実行で変化なし）| ✅ PASSING |
| T11a | shell window kill → reconcile で再 spawn | ❌ 未実装 |
| T11b | ai window kill → reconcile で再 spawn + column 位置復元 | ❌ 未実装 |
| T11c | editor window kill → reconcile で再 spawn | ❌ 未実装 |
| T12 | `projwm remove --window shell-2` で意図的削除（再 spawn なし）| ❌ 未実装 |
| T13 | `darwin-rebuild switch` 後の自動復帰 | ❌ 未実装（手動確認）|
| T14 | OmniWM 再起動後の自動復帰（FR-08）| ❌ 未実装 |
| T15 | モニタ抜き差し後の自動復帰 | ❌ 未実装 |
| T16 | launchd ペリオディック（60s）による自動修正 | ❌ 未実装 |
| T17 | cmuxterm/Spotify/Discord/Zen は一切触られない | ❌ 未実装 |
| T18 | 手動レイアウト変更が state に反映される（FR-32）| ❌ 未実装 |
| T19 | 誤 WS への手動移動が reconcile で revert される（FR-33）| ❌ 未実装 |
| T20 | 手動コラム順変更が state に保存される（FR-32 の subset）| ❌ 未実装 |
| T21 | isolated WS（M/1/B/3）は操作後も同一 WS に留まる | ✅ PASSING |

#### 理想状態定義（テスト基準）

```
WS A: col[ai-view-1:dotfiles]  col[ai-view-1:projwm-jtest]  col[ai-view-1:MyEmmoWorld]
WS Q: col[dotfiles(Zed)]  col[ai-1:dotfiles]  col[shell-1:dotfiles / shell-2:dotfiles (stacked)]  col[Vivaldi]
WS W: col[projwm-jtest(Zed)]  col[ai-1:projwm-jtest]  col[shell-1:projwm-jtest]
WS E: col[ai-1:MyEmmoWorld]
WS 3: col[shell (com.cmuxterm.app)]  ← projwm 管理外、不変
WS M: col[Spotify]  WS 1: col[Discord]  WS B: col[Zen]  ← 管理外、不変
```

### 9.3 ビルド・配布

- `pkgs.buildGoModule` で `projwm` バイナリを生成
- HM 経由で `~/.nix-profile/bin/projwm` に配置
- バージョン情報は `-ldflags "-X main.version=$(git describe)"` で埋込
- launchd agents:
  - `org.nixos.projwm-reconcile-watch` (windows-changed listener)
  - `org.nixos.projwm-reconcile-display` (display-changed listener)
  - `org.nixos.projwm-reconcile-periodic` (60s timer)
- `projwm-reconcile-debounced` ラッパスクリプトが flock + marker で 500ms debounce

---

## 10. 確定決定一覧

v11.6 までに確定した全決定。番号は通し番号（D-XX）。

| # | 決定 |
|---|---|
| D-1 | 1 project = 1 OmniWM 名前付き WS |
| D-2 | AI Viewer = WS A、read-only grouped attach |
| D-3 | 数値 WS 1〜9 / M / B は手付かず（E は projwm 用 slot に再利用）|
| D-4 | キーバインド: `alt+letter`（Q/W/E/R/T/Y/U/I/O/P）で slot jump、`alt+shift+letter` で送る |
| D-5 | `alt+a / alt+shift+a` で viewer |
| D-6 | `alt+space` = OmniWM Quake = scratch（libghostty fish） |
| D-7 | `alt+\`` = Ghostty で `projwm tui` = cockpit |
| D-8 | cockpit 実装 = Go + bubbletea 単一バイナリ `projwm` |
| D-9 | state.json が source of truth、reconcile が一手に修正 |
| D-10 | reconcile orphan ポリシー: 触らない（`--gc` で明示一括 close） |
| D-11 | reconcile 駆動: omniwmctl watch + 60s 定期 + コマンド末尾 |
| D-12 | state file 排他: flock + atomic rename |
| D-13 | プロファイル機構: state.json で `profiles` を導入、active/inactive を区別 |
| D-14 | アーカイブ機構: project ごとの `archived` フラグ |
| D-15 | プロファイル切替は windows 操作のみで完結、tmux session kill/start を伴わない |
| D-16 | viewer は active profile の project だけ表示 |
| D-17 | 既定プロファイルは持たない、初期 state は空のプロファイル群 |
| D-18 | 同時 active profile は 1 つだけ（オーバーレイ非対応）|
| D-19 | 多 AI / project は first-class、各 AI 窓は `(kind="ai", id=N)` で識別 |
| D-20 | viewer は AI 窓単位で複製（1 project に AI N 個 → viewer に N タイル）|
| D-21 | 全 kind が連番 id、削除で穴が空いても再利用しない |
| D-22 | schema に primary / default / main 概念なし、すべての AI 窓は対等 |
| D-23 | title / tmux session 名は state に保存せず、`(kind, id, project)` から算出 |
| D-24 | `up` の `--ai` は必須、暗黙のデフォルトを持たない |
| D-25 | state.json はバージョニングを持たない |
| D-26 | state と config を分離（state は runtime のみ、config は固定値）|
| D-27 | 無所属 project (park) を first-class として扱う |
| D-28 | プロファイル切替時の moving は project の全 windows を一括で move |
| D-29 | Zed を first-class editor として導入、kind=editor |
| D-30 | editor (Zed) は GUI app、tmux ラップしない、bundleId + cwd basename で identify |
| D-31 | `up` 既定: ai-1 + shell-1 + editor-1。`--no-editor` で editor 抑止可 |
| D-32 | active 全 project の cwd basename は一意 |
| D-33 | OmniWM の `dev.zed.Zed → WS E` 一律 appRule は削除、projwm が per-project に Zed 配置 |
| D-34 | nvim を kind として持たない（shell-N の tmux 内で起動）|
| D-35 | Zed は homebrew cask 経由で導入（nixpkgs zed-editor は darwin で CLI 壊れている）|
| D-36 | viewer 用 grouped session 名は `<kind>-<id>/<proj>_v`（tmux は session 名内の `:` を許容しないため）|
| D-37 | Zed は `zed -n <cwd>` で起動（素の `zed <path>` は既存 workspace を再利用）|
| D-38 | OmniWM Quake terminal は command 上書き不可、libghostty 固定 fish。cockpit は別キー（`alt+\``）でGhostty + projwm tui |
| D-39 | workspace name → number 解決は projwm 内部で動的（`omniwmctl query workspaces` 経由）|
| D-40 | AI 自動起動: tmux session 新規作成時に send-keys で AI コマンド打鍵（300ms 待機後）|
| D-41 | profile switch は state mutate 後に reconcile を必ず呼ぶ（共通ヘルパ `runReconcileOnce`）|
| D-42 | terminal driver は **純正 Ghostty.app**。OmniWM app-rules に titleRegex rule (`^(ai\|shell\|ai-view)-[0-9]+:`) + `layout = "tile"` を追加することで SwiftUI hidden helper windows と区別して admit 成功 |
| D-43 | bubbletea cockpit は windows 詳細展開 + empty slots + n/d/a/u 操作 + fsnotify reactive 更新 |
| D-44 | reconcile の Zed spawn は flock ベースの spawn lock で並走重複を防ぐ（`~/.cache/projwm/.locks/zed-spawn-<sha1>.lock`）|
| D-45 | Ghostty SwiftUI app の hidden helper windows 流入問題は、user rule の不足が原因（OmniWM のバグではなく設定の問題）|
| D-46 | v12 browser 統合は **Chromium 系 + chrome-cli 経由**。Vivaldi Workspaces を AX で操作する paradigm B (focus 強奪) は撤回。ユーザは通常運用 browser として Zen を別途使い、projwm 連動は Vivaldi 専用 |
| D-47 | browser 識別は **state.Window.LiveWindowID** に chrome-cli の window-id を保存して引き回す。spawn 直後に pre/post window-id 集合 diff で取得。**marker tab は使わない**（user の tab strip を汚さない）|
| D-48 | profile 切替で active 外になった browser window は **URL snapshot → close**、active 復帰で **`SavedURLs` から再 spawn**。ai/shell/editor の close/spawn cycle と完全同 paradigm |
| D-49 | destructive 操作（spawn / close）の前後で **frontmost app を osascript で保存 → 復帰** する。Vivaldi の遅延 focus 強奪に対抗するため defer 内で 0/0.5/1/1.5 秒の多段 Activate |
| D-50 | reconcile は browser に **絶対触らない**（reconcile.Run の switch case は no-op）。launchd auto-reconcile / display change / periodic が発火しても browser は動かない。**spawn / close は cmd 層の明示イベントのみ** (add-browser / profile switch / profile assign / profile unassign / archive-project / unarchive / remove --window=browser-N / TUI a/u/d) |
| D-51 | browser kind は ai/shell/editor と並ぶ第 4 の window kind として state.Window に統合。`browser_workspace` 等の project レベルフィールドは持たない（per-window が paradigm の核）|
| D-52 | TUI の destructive 操作（a/u/d）は内部で `exec.Command("projwm", ...)` 経由で cmd 層を呼ぶ。重複実装を避け、paradigm C ライフサイクル hook を一箇所に集約 |
| D-53 | layout snapshot/restore: archive / profile-switch 直前に `SnapshotProjectLayout`（slot WS に一時 focus + 150ms 待機 → QueryWindows → frame.x 昇順で column 順確定、frame.y 降順で stack 順確定）を呼び `state.Window.Layout` に保存。unarchive / profile-switch 後の spawn 完了（stable×2 polling）後に `RestoreProjectLayout` を呼ぶ。colMap（liveID → colIndex）を初回 QueryWindows で構築、以後 MoveColumnDirection / MoveDirection のたびに in-memory 更新（再 QueryWindows 不要、OmniWM 伝播遅延に左右されない）|
| D-54 | 手動操作ポリシー（3 種に分類）: (a) 手動コラム順・スタック順変更 → state.json の Layout を更新（ユーザ意図を尊重、FR-32）; (b) 規約 title 窓を誤った WS へ手動移動 → reconcile が正しい slot へ revert（FR-33、§7.3 に既記載）; (c) 規約 title 窓を直接 kill/close → reconcile が再 spawn（元 column 位置含む、FR-34）。唯一の意図的削除インターフェースは `projwm remove --window <kind-N>`（TUI でも同等操作提供）|
| D-55 | stack 検出は `query windows` のフレーム座標で行う。`workspace-bar` はスタックされた窓を別 column として誤報告するため不可。正しい手順: WS に focus → 150ms 待機 → `query windows --workspace <name>` → frame.x の ±5px 範囲で同一列グループ化 → y 降順（y-up 座標系、高 y が視覚的上）でスタック順確定 |

---

## 11. 既知の制約と open issues

### 11.1 解決済み（Decision に記載）

すべての過去 issue は D-XX で解決。詳細経緯は `projwm-history.md`。

### 11.2 まだ open

| ID | 内容 | 移行先 |
|---|---|---|
| OI-1 | Browser 統合（project 切替で browser tab セットも自動切替）| roadmap.md v12 |
| OI-2 | Phase 7（cmux / zellij 撤去）| roadmap.md（ユーザ確認後実行）|
| OI-3 | `projwm restore`（tmux server から state を再構築）| roadmap.md |
| OI-4 | 多 editor 並走時の挙動（Zed 同 folder の 2 window 制御）| 観察継続、当面 id=1 のみ運用 |
| OI-5 | parked project の長期生存ポリシー | MVP では無期限保持、運用後判断 |
| OI-6 | プロファイル切替時のフリッカー（窓 close/spawn/move のレイテンシ）| 実機運用で判断、必要なら hide/show 戦略 |
| OI-7 | profile 別 monitor 配置の override | MVP では main 固定、運用後検討 |
| OI-8 | FR-08 未実装: OmniWM 再起動・モニタ抜き差し後の自動復帰。launchd watch は存在するが layout/column 復元まで含めた完全な "1 分以内復帰" は未検証。T14-T16 が FAILING | reconcile + layout restore の統合 |
| OI-9 | profile switch に既知バグ（T01 で検出）: (a) 重複 spawn（shell×2, ai×2）; (b) viewer 窓が slot WS に誤配置; (c) browser window が誤 slot へ; (d) viewer が他 profile の slot に誤配置。根本原因調査中 | cmd/profile.go switch ロジック見直し |
| OI-10 | reconcile が直接 kill された窓を再 spawn する際、元の column 位置への restore が未実装（T11 FAILING）| FR-34 実装 |
| OI-11 | 手動レイアウト変更を state へ反映するイベント駆動機構（FR-32 / T18）が未実装。現在は snapshot タイミング（archive / switch 直前）のみ | layout-changed subscription |

---

## 付録 A: 改版履歴（condensed）

過去の改版経緯は `projwm-history.md` に詳細あり。本書は v12 の確定状態。

---

## 付録 B: 関連 OmniWM Issue

| Issue | 内容 | 関連 |
|---|---|---|
| #128 Apps not recognized | EndNote / Adobe Illustrator の認識問題 | 解決策の参考にした |
| #197 Emacs window gets lost | Emacs window が tile から消える | 同様の symptom 系列 |
| #243 Notion app breaks out | Notion の管理外れ | OWNER の workaround「custom rule で tile 強制」を採用 → D-42 |
| #263 Ghostty focus ring | Ghostty の child window で focus ring が rectangular | 別 issue、参考程度 |

---

_本書は projwm の **v12 確定仕様**（browser 統合 + layout snapshot/restore 含む）。最新の追記・open issue は `projwm-decisions.md` / `projwm-roadmap.md` を参照。_
