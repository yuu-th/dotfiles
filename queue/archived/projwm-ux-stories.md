# projwm ユーザ体験ストーリー集（v1）

> 「ユーザが何を期待し、何が起きるべきか」を **画面状態の遷移** として言語化する。  
> queue/projwm-design.md（仕様）と queue/projwm-report.md（実装ログ）と並ぶ第 3 の文書。  
> このストーリーが満たされない実装は不完全。

---

## メタ情報

| 項目 | 値 |
|---|---|
| 用途 | エージェントによる客観評価のための **観測可能な期待動作** の定義 |
| 対象 | projwm を **初めて触るユーザ** が日常運用に至るまでの主要シナリオ |
| 評価方法 | 各ストーリーの「期待」と「観測」を別エージェントが照合、乖離を Issue 化 |

---

## 用語

- **WS**: workspace。OmniWM 上の仮想デスクトップ。`alt+q`〜`alt+p` で 10 個、`alt+a` で AI viewer
- **slot**: AI project を 1 つ載せる WS。Q〜P の 10 個
- **kitty/ghostty window**: AI または shell が走る terminal window
- **Zed**: editor
- **AI**: claude (Anthropic CLI) または copilot (GitHub Copilot CLI)

---

## Story 1 — はじめての projwm 起動（最重要）

### 状態（前提）

- macOS 26.x、OmniWM 0.4.8 稼働、Karabiner alt+letter 配線済み
- `projwm` CLI が PATH にある
- state.json はまだ無い（初回）
- WS 1 にユーザは居る、画面は通常の作業状態

### 操作シーケンス

| # | ユーザ操作 | 期待動作 |
|---|---|---|
| 1 | `projwm doctor` | `state file: not yet created` と表示。エラーは出ない |
| 2 | `projwm profile create work` | プロファイル `work` 新規作成、active になる。出力は静かでも OK |
| 3 | `cd ~/dev/dotfiles` | （shell の cd） |
| 4 | `projwm up --ai claude --slot Q` | 数秒で完了。`up: registered dotfiles, N reconcile actions` 等のログ。**ERROR 行が無い** |
| 5 | `alt+q` 物理キー押下 | OmniWM が WS Q にフォーカスを移す。画面が切り替わる |

### 操作 5 直後の画面（**最重要観測点**）

WS Q に **3 つの window** が並んでいる:

| 位置 | window | 中身 |
|---|---|---|
| 左 | terminal `ai-1:dotfiles` | **claude が起動済み**、入力プロンプト `>` が出ている。tmux 背後で動作 |
| 中 | terminal `shell-1:dotfiles` | fish shell プロンプト、cwd は `/Users/yuta/dev/dotfiles` |
| 右 | Zed `dotfiles` | dotfiles フォルダがファイルツリーに見えている、何かのファイルが開いている |

❌ **NG なケース** (現状の実装漏れ):
- terminal が「ただの shell」(claude が起動していない)
- Zed しか見えない
- terminal は出るが title が違う

### 操作 6: `alt+a` 押下

| 期待 |
|---|
| WS A (viewer) に画面が切り替わる |
| **1 つの terminal window** が見える: `ai-view-1:dotfiles` |
| 中身は ai-1:dotfiles と **同じ画面**（grouped session で同期、claude のプロンプト表示） |

❌ **NG**: WS A が空、または別のもの

---

## Story 2 — もう 1 つ AI を追加

### 状態（前提）

Story 1 の操作 5 が完了、WS Q にいる。

### 操作

| # | 操作 | 期待 |
|---|---|---|
| 1 | shell-1:dotfiles で `projwm add-ai --ai copilot` | reconcile 走る、新 window が立つ |
| 2 | （自動的に）WS Q に新規 `ai-2:dotfiles` window が**出現** | copilot が起動済み |
| 3 | （自動的に）WS A に新規 `ai-view-2:dotfiles` window が出現 | ai-2 と同期 |

### 重要

- ai-1 (claude) は**そのまま生きている**。新 ai-2 が**追加**される
- viewer (WS A) は AI window 単位で 2 つに増える

---

## Story 3 — Profile 切替で AI セッションは生きたまま窓だけ片付く

### 状態（前提）

profile `work` で `dotfiles` (slot Q) が稼働中。`personal` profile は別 project `blog` が slot Q を占有予定。

### 操作

| # | 操作 | 期待 |
|---|---|---|
| 1 | `projwm profile switch personal` | 即座に切替（5 秒以内） |
| 2 | WS Q に `alt+q` で行く | dotfiles の window 群は消えて、blog の window 群が現れている |
| 3 | `tmux ls` | **dotfiles の tmux session も blog の tmux session も両方生きている** |
| 4 | `projwm profile switch work` | 元に戻る、瞬時 |

### 重要

- AI の状態（過去の会話履歴）は tmux pty で保持されているので、profile 切替で**消えない**
- archived 化していない限り tmux session は全て alive

---

## Story 4 — Project を archive

### 状態（前提）

`old-project` が登場、しばらく触らない予定。

### 操作

| # | 操作 | 期待 |
|---|---|---|
| 1 | `projwm archive-project old-project` | reconcile 走る |
| 2 | WS Q（旧 slot）を見る | old-project の window は全て閉じている |
| 3 | `tmux ls` | old-project の AI/shell tmux session は **kill されている** |
| 4 | `projwm archive list` | `old-project` が archived として一覧に出る |
| 5 | `projwm unarchive old-project --profile work --slot R` | 復活、WS R に展開 |

---

## Story 5 — 物理キーで素早く navigation

### 状態（前提）

複数 project が稼働。

### 操作

| キー | 期待 |
|---|---|
| `alt+1`〜`alt+9` | 日常 WS 1〜9 jump（既存、変更なし） |
| `alt+m` `alt+b` | M (Media) / B (Browser) jump（既存、変更なし） |
| `alt+a` | **viewer (WS A) jump**（旧 Calendar 起動マクロ廃止）|
| `alt+q`〜`alt+p` | 各 AI project slot に jump |
| `alt+shift+<letter>` | 現在のフォーカス window をその WS に送る |

### 重要

- **alt+a が AI Viewer 起動**になっていることがユーザの最大の体感差
- Calendar は手動 / Spotlight 等で起動

---

## Story 6 — TUI cockpit で全体俯瞰

### 操作

| # | 操作 | 期待 |
|---|---|---|
| 1 | `projwm tui` を terminal で実行 | bubbletea TUI が起動、altscreen |
| 2 | 画面表示 | active profile + slot 一覧 + parked + archived の俯瞰 |
| 3 | 文字入力 | fzf 風 incremental filter |
| 4 | ↑↓ + Enter | slot に jump（OmniWM workspace focus） |
| 5 | Tab | profile 循環切替 |
| 6 | esc | 終了 |

---

## 重要な観測点リスト（評価エージェント向け）

各エージェントは以下の項目を **可能なら CLI コマンドで直接確認** し、ストーリーとの乖離を報告:

```bash
# 1) projwm CLI 配置確認
which projwm projwm-reconcile-debounced projwm-setup-kitty

# 2) 初期化フロー
projwm doctor                            # state 不在
projwm profile create test
projwm profile list                       # test が active

# 3) up コマンド
cd ~/dev/dotfiles
projwm up --ai claude --slot Q

# 4) tmux 状態
tmux ls                                  # ai-1/dotfiles, ai-1/dotfiles_v, shell-1/dotfiles の 3 つ

# 5) AI 自動起動の確認（最重要）
tmux capture-pane -t ai-1/dotfiles -p | tail -10
# 期待: claude のプロンプト or 起動メッセージが見える
# NG  : fish のプロンプト ($, ❯, > 等) のみ

# 6) OmniWM 視認性
omniwmctl query windows --workspace Q --json    # kitty 2 + Zed 1 = 3 件
omniwmctl query windows --workspace A --json    # kitty 1 (viewer)

# 7) state.json 整合性
cat ~/.local/state/projwm/state.json | python3 -m json.tool

# 8) 物理キー jump（実機操作のみ確認可能、API 等価コマンド）
omniwmctl workspace focus-name Q
omniwmctl workspace focus-name A

# 9) profile 切替
projwm profile create alt
projwm profile switch alt
projwm profile switch test
```

---

---

## Story 7 — Scratch shell（一時 shell）が常時呼べる（FR-07）

### 状態

projwm が動いている、ユーザは色々 WS を行き来している。

### 操作

| キー | 期待 |
|---|---|
| `alt+\`` | OmniWM Quake terminal (libghostty 内蔵) が画面 50% で出現、fish プロンプト |
| 同キー再押下 | 隠れる（toggle）|

### 重要

- **projwm の AI/shell window と独立**してすぐ叩ける
- AI workspace から離れずに ad-hoc コマンドを打てる
- 設計書 §8.3 の「Quake = scratch」役割

❌ **NG**: alt+` で何も起きない、OmniWM Quake が projwm 関連の何かを開く

---

## Story 8 — モニタ抜き差しから 1 分以内に復帰（FR-08）

### 状態

WS Q に dotfiles project（ai-1 + shell-1 + Zed + viewer）が稼働、Mac を外部モニタに繋いでいる。

### 操作

| # | 操作 | 期待 |
|---|---|---|
| 1 | 外部モニタの USB-C を物理的に外す | window が main display に再配置される（OmniWM の標準動作） |
| 2 | 外部モニタを繋ぎ戻す | (60 秒以内に) projwm が monitor profile の slot 配置を再 reconcile |
| 3 | `alt+q` で WS Q | 元通りの window 配置 |

### 重要

- launchd の `omniwmctl watch display-changed` → `projwm-reconcile-debounced` 連携が機能
- tmux session は kill されない（NFR-08）

---

## Story 9 — `projwm reconcile --dry-run` で差分を確認（FR-09）

### 操作

```bash
$ projwm reconcile --dry-run
(dry-run) planned actions:
  spawn-ghostty   ai-1:dotfiles  ws=Q
  ...

$ projwm reconcile
actions:
  spawn-ghostty   ai-1:dotfiles  ws=Q
  (実行された)
```

### 重要

- dry-run は **副作用ゼロ**: tmux session 作成も window spawn もしない
- 出力は人間が読める単純行
- `--verbose` で詳細

---

## Story 10 — `projwm status` で全体俯瞰（FR-10）

### 操作

```bash
$ projwm status
profile: work    archive: 2    parked: 1
───────────────────────────────────────────
[Q] dotfiles    /Users/yuta/dev/dotfiles
     ai-1     claude
     shell-1
     editor
[W] manaflow    /Users/yuta/work/manaflow
     ...
───────────────────────────────────────────
```

### 重要

- 1 行 1 windows、kind/id ソート（ai → shell → editor の順）
- archived / parked / inactive の数だけ summary
- `--json` で構造化出力

---

## Story 11 — 既存 tmux サーバから state を再構築（FR-11）

### 状態（前提）

projwm が一度動いていた → state.json を間違って消した。tmux サーバには `ai-1/dotfiles`, `shell-1/blog` 等のセッションがまだ生きている。

### 操作

```bash
$ projwm restore
discovered: dotfiles (ai-1, shell-1)
discovered: blog (ai-1, shell-1, shell-2)
state.json reconstructed.
```

### 重要

- tmux ls から projwm 命名規則 `<kind>-<id>/<project>` を逆解析
- cwd は project 名 → 該当 path の guess（`~/dev/<project>` を最初に試す、見つからなければ user prompt）
- archived 化されていた過去の project は復元できない（tmux 不在のため）

⚠️ **MVP 未実装**（report.md で言及）— Story 11 は将来実装目標

---

## Story 12 — 同じ project が複数 profile に居る（FR-19）

### 状態

`shared-x` という project がある。`work` と `personal` 両プロファイルで使いたい。

### 操作

| # | 操作 | 期待 |
|---|---|---|
| 1 | `work` で `projwm up --ai claude --slot Q` ([cwd=~/dev/shared-x]) | shared-x が work の slot Q に登録 |
| 2 | `personal` に switch | shared-x の windows close（tmux は alive） |
| 3 | `personal` で `projwm profile assign R shared-x` | personal の slot R に shared-x 割当 |
| 4 | `alt+r` | shared-x の windows が WS R に出現（同じ tmux session の再 attach） |

### 重要

- tmux session は work と personal で**共有**（同じ project = 同じ tmux）
- profile 切替で window の slot は変わるが、AI の作業履歴（tmux pane）は連続
- launcher の inactive セクションで `profiles=work+personal` 表示

---

## Story 13 — viewer (WS A) は active profile のみ表示（FR-21）

### 状態

profile A: dotfiles (Q), blog (W) → 2 AI windows
profile B: client-x (Q) → 1 AI window

### 操作

| # | 操作 | 期待 |
|---|---|---|
| 1 | profile A active、`alt+a` | viewer に dotfiles ai-view-1 + blog ai-view-1 = **2 タイル** |
| 2 | `projwm profile switch B` | viewer の dotfiles/blog ai-view が close、client-x ai-view-1 だけが残る |
| 3 | `alt+a` | viewer に **1 タイル** |

### 重要

- viewer は active profile に **strictly 連動**
- profile 切替で grouped session も作り直しではなく、**viewer 窓だけを close/spawn**

---

## Story 違反の典型

これらは **直ちに修正対象**:

1. **AI が自動起動していない** ← Story 1-5 違反、最優先
2. **terminal が ghostty じゃない**（ユーザの好み）← 設計外だが優先 高
3. **viewer (WS A) が空** ← Story 1-6 違反
4. **window が間違った WS にいる** ← Story 1, 2 違反
5. **コマンドが「成功」で返るのにエラーが起きている** ← yuu agent の指摘
6. **profile 切替で tmux session が kill される** ← Story 3 違反

---

_このファイルは生きたドキュメント。実装が進む過程でストーリーは更新される。_
