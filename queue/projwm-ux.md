# projwm 理想ユーザ体験

> projwm を使ったときに **ユーザの目に何が映り、何が起きるか** を物語形式で記述する。
> 仕様の詳細は `projwm-spec.md`、未着手の構想は `projwm-roadmap.md` を参照。
> このストーリーが満たされない実装は **不完全** とみなす。

---

## 0. 用語クイックリファレンス（読みながら戻る用）

- **slot**: AI project が乗る OmniWM workspace（Q W E R T Y U I O P の 10 個）
- **viewer**: 全 AI を read-only で並べる workspace A
- **profile**: いまアクティブな project セット（work / personal 等）
- **park**: profile に属さないが state には居る project（tmux は alive、窓は閉）
- **archive**: tmux も windows も止まった保管状態（unarchive で復活）
- **cockpit**: `alt+\`` で出る `projwm tui`、操作の中心
- **scratch**: `alt+space` で出る OmniWM Quake、ad-hoc コマンド用

---

## Story 1 — はじめての projwm

### 状態（前提）
- macOS、OmniWM 稼働、Karabiner alt+letter 配線済み、`projwm` CLI が PATH にある
- state.json はまだ無い
- ユーザは WS 1（普通の作業画面）に居る

### 操作シーケンス

| # | 操作 | 起きること |
|---|---|---|
| 1 | `projwm doctor` | `state file: not yet created` と表示。エラー無し |
| 2 | `projwm profile create work` | プロファイル `work` が新規作成、active になる |
| 3 | `cd ~/dev/dotfiles` | （shell の cd） |
| 4 | `projwm up --ai claude --slot Q` | 数秒で完了。`up: registered dotfiles, N reconcile actions` |
| 5 | **`alt+q`** 押下 | 画面が WS Q に切り替わる |

### 操作 5 直後の画面（最重要観測点）

WS Q に **3 つの window** が並んでいる:

| 位置 | window | 中身 |
|---|---|---|
| 左 | terminal `ai-1:dotfiles` | **claude が起動済み**、入力プロンプト `❯` が表示。「1 MCP server needs auth」等のメッセージが見える |
| 中 | terminal `shell-1:dotfiles` | fish shell プロンプト、cwd は `/Users/yuta/dev/dotfiles` |
| 右 | Zed `dotfiles` | dotfiles フォルダがファイルツリー、何かのファイルが開いている |

### 操作 6: `alt+a` 押下

WS A (viewer) に画面が切り替わる:
- **1 つの terminal window** `ai-view-1:dotfiles`
- 中身は **ai-1:dotfiles と完全同期** (claude のプロンプト等、同じ画面)

---

## Story 2 — 2 つ目の AI を追加

### 状態
Story 1 の操作 5 完了、WS Q に居る。

### 操作

| # | 操作 | 起きること |
|---|---|---|
| 1 | shell-1 で `projwm add-ai --ai copilot` | reconcile 走る、新 window 出現 |
| 2 | （自動）WS Q に `ai-2:dotfiles` window が出現 | copilot が起動済み |
| 3 | （自動）WS A に `ai-view-2:dotfiles` window が追加 | ai-2 と同期 |

ai-1 (claude) は触らずそのまま生きている。viewer は AI 窓単位で 2 タイルに増える。

---

## Story 3 — Profile 切替で AI セッションは生きたまま窓だけ片付く

### 状態
profile `work` で `dotfiles` (slot Q) が稼働。`personal` profile は別 project `blog` が slot Q を占有予定。

### 操作

| # | 操作 | 起きること |
|---|---|---|
| 1 | `projwm profile switch personal` | 即座に切替（5 秒以内）|
| 2 | `alt+q` で WS Q を見る | dotfiles の windows は消えて、blog の windows が現れている |
| 3 | `tmux ls` | **dotfiles の tmux session も blog の tmux session も両方生きている** |
| 4 | `projwm profile switch work` | 元に戻る、瞬時 |

### 重要

- AI の作業履歴（過去の会話）は tmux pty で保持されているので、profile 切替で **消えない**
- archive 化していない限り tmux session はすべて alive

---

## Story 4 — Project を archive

### 状態
`old-project` が登場、しばらく触らない予定。

### 操作

| # | 操作 | 起きること |
|---|---|---|
| 1 | `projwm archive-project old-project` | reconcile 走る |
| 2 | `alt+<旧 slot>` で見る | old-project の windows は全て閉じている |
| 3 | `tmux ls` | old-project の AI/shell tmux session は **kill されている** |
| 4 | `projwm archive list` | `old-project` が archived 一覧に出る |
| 5 | `projwm unarchive old-project --profile work --slot R` | 復活、WS R に展開 |

---

## Story 5 — 物理キーで素早く navigation

### 状態
複数 project が稼働。

### 操作

| キー | 起きること |
|---|---|
| `alt+1`〜`alt+9` | 日常 WS 1〜9 に jump（既存、変更なし）|
| `alt+m` `alt+b` | M (Media) / B (Browser) jump（既存、変更なし）|
| `alt+a` | **viewer (WS A) jump**（旧 Calendar 起動マクロは廃止）|
| `alt+q`〜`alt+p` | 各 AI project slot に jump |
| `alt+shift+<letter>` | 現在のフォーカス window をその WS に送る |
| `alt+space` | OmniWM Quake terminal（scratch fish）|
| `alt+\`` | projwm cockpit を Ghostty で開く |

### 体験のポイント

- 修飾キーは **alt 単独 / alt+shift のみ**。`cmd` 修飾は使わない（macOS 他アプリと衝突しない）
- chord（連打 leader）なし、modal なし。**1 動作 1 ジェスチャー** で完結
- alt+a で Calendar が立ち上がらない（旧マクロ撤去済み）

---

## Story 6 — Cockpit で全体俯瞰

### 状態
複数 project が稼働、profile が複数、archived も少しある。

### 操作

| # | 操作 | 起きること |
|---|---|---|
| 1 | `alt+\`` 押下 | Ghostty window がポップアップ、`projwm tui` が表示 |
| 2 | 画面表示 | 下記の cockpit 画面 |
| 3 | 文字を打つ | fzf 風 incremental filter で絞込 |
| 4 | ↑↓ | 行移動 |
| 5 | Enter | 選択行のアクション（slot → jump、profile → activate、archived → "u で unarchive" 案内）|
| 6 | Tab | active profile を循環切替 |
| 7 | `n` | 新規 project 作成プロンプト |
| 8 | `d` | active profile から unassign（park 化）|
| 9 | `a` | archive |
| 10 | `u` | unarchive |
| 11 | `?` | help |
| 12 | esc / ctrl+c | 終了 |

### 表示される画面例

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
       shell-1            tmux●  win●
       editor             —      win●
  [E] (empty)
  [R] (empty)
  ...

other profiles
  ● personal   2 assignments

parked (no slot, tmux alive)
  • spike-x   /Users/yuta/dev/spike-x

archived
  ▼ old-thing   /Users/yuta/dev/old-thing

viewer (WS A)
  [A] 3 ai stream(s): dotfiles ai-1, dotfiles ai-2, manaflow ai-1
```

### リアクティブ更新

別ターミナルで `projwm archive-project foo` を打つと、cockpit 画面が **即座に反映**される（fsnotify 監視 + 2 秒 probe）。

---

## Story 7 — Scratch shell（一時 shell）が常時呼べる

### 操作

| キー | 起きること |
|---|---|
| `alt+space` | OmniWM Quake terminal が画面上から 50% スライドイン、fish プロンプト |
| もう一度 `alt+space` | 隠れる（toggle）|

### 重要

- projwm の AI/shell window と独立してすぐ叩ける
- AI workspace から離れずに ad-hoc コマンドを打てる
- projwm の管理対象外、Quake 内蔵の fish が走る

---

## Story 8 — モニタ抜き差しから 1 分以内に復帰

### 状態
WS Q に dotfiles project（ai-1 + shell-1 + Zed + viewer）が稼働、Mac を外部モニタに繋いでいる。

### 操作

| # | 操作 | 起きること |
|---|---|---|
| 1 | 外部モニタの USB-C を物理的に外す | window が main display に再配置（OmniWM の標準動作）|
| 2 | 外部モニタを繋ぎ戻す | 60 秒以内に projwm が monitor profile の slot 配置を再 reconcile |
| 3 | `alt+q` で WS Q | 元通りの window 配置 |

### 重要

- launchd の `omniwmctl watch display-changed` → `projwm-reconcile-debounced` 連携
- tmux session は kill されない

---

## Story 9 — `projwm reconcile --dry-run` で差分を確認

### 操作

```bash
$ projwm reconcile --dry-run
(dry-run) planned actions:
  spawn-ghostty   ai-1:dotfiles  ws=Q
  new-tmux        shell-1/dotfiles
  ...

$ projwm reconcile
actions:
  spawn-ghostty   ai-1:dotfiles  ws=Q  (実行された)
```

### 重要

- dry-run は副作用ゼロ（tmux session 作成も window spawn もしない）
- `--verbose` で詳細
- `--gc` で orphan 一括 close（破壊的）

---

## Story 10 — `projwm status` で全体俯瞰（CLI から）

### 操作

```bash
$ projwm status
profile: work    archive: 2    parked: 1
───────────────────────────────────────────
[Q] dotfiles    /Users/yuta/dev/dotfiles
     ai-1     claude
     ai-2     copilot
     shell-1
     editor
[W] manaflow    /Users/yuta/work/manaflow
     ai-1     claude
     shell-1
     editor
───────────────────────────────────────────
```

`--json` で構造化出力。

---

## Story 11 — 同じ project が複数 profile に居る

### 状態
`shared-x` という project を `work` と `personal` の両方で使いたい。

### 操作

| # | 操作 | 起きること |
|---|---|---|
| 1 | `work` active 中に `projwm up --ai claude --slot Q` ([cwd=~/dev/shared-x]) | shared-x が work の slot Q に登録 |
| 2 | `projwm profile switch personal` | shared-x の windows close（tmux は alive）|
| 3 | `personal` active 中に `projwm profile assign R shared-x` | personal の slot R に shared-x 割当 |
| 4 | `alt+r` | shared-x の windows が WS R に出現（**同じ tmux session の再 attach**）|

### 重要

- tmux session は work と personal で **共有**（同じ project = 同じ tmux）
- profile 切替で window の slot は変わるが、AI の作業履歴は連続
- cockpit の inactive セクションで `profiles=work+personal` 表示

---

## Story 12 — viewer は active profile のみ表示

### 状態
- profile A: dotfiles (Q), blog (W) → AI 窓 合計 2 つ
- profile B: client-x (Q) → AI 窓 1 つ

### 操作

| # | 操作 | 起きること |
|---|---|---|
| 1 | profile A active、`alt+a` | viewer に dotfiles ai-1 + blog ai-1 = **2 タイル** |
| 2 | `projwm profile switch B` | viewer の dotfiles/blog 用 windows が close、client-x ai-1 だけが残る |
| 3 | `alt+a` | viewer に **1 タイル** |

### 重要

- viewer は active profile に **strictly 連動**
- profile 切替で grouped session は kill しない、**viewer 窓だけを close/spawn**

---

## Story 13 — 多くの window を同 slot に同居（自由度）

### 状態
WS Q（dotfiles project）で作業中。projwm が立てたのは ai-1 + shell-1 + editor。

### 操作

| # | 操作 | 起きること |
|---|---|---|
| 1 | Browser を `alt+shift+q` で WS Q に送る | Q に Ghostty 2 + Zed 1 + Browser 1 が並ぶ |
| 2 | 別の Zed window を `zed -n ~/dev/dotfiles/docs` で開く | OmniWM が自動で配置 |
| 3 | `alt+shift+q` で送る | Q に Ghostty 2 + Zed 2 + Browser 1 |
| 4 | `projwm reconcile` 実行 | **これらの追加 windows は触らない**（state にない orphan は尊重）|

### 重要

- projwm が管理するのは state.json に書かれた windows のみ
- ユーザが手動で送った window は **orphan として尊重**（reconcile が close しない）
- orphan を一括掃除したい時は `projwm reconcile --gc`

---

## Story 14 — Browser window を project に bind（v12, paradigm C）

### 状態（前提）

- profile = `work`、active project = dotfiles (slot Q)
- Vivaldi 起動済み or 未起動
- ユーザは「dotfiles 用に GitHub と仕様書 PR を常に開いておきたい」と思っている

### 操作

```bash
projwm add-browser --project=dotfiles --profile=work \
  --url=https://github.com/yuu-th/dotfiles \
  --url=https://example.com/spec
```

直後：
1. projwm が **直前の frontmost アプリ** を記録（仮に Ghostty）
2. `open -na Vivaldi --args --profile-directory=work --new-window <urls>` で Vivaldi window を spawn
3. 一瞬 Vivaldi が前面になる（spawn の OS 仕様）
4. projwm が記録した Ghostty を再 activate → **focus が元に戻る**
5. OmniWM が Vivaldi window を slot Q（dotfiles）の tile に admit

### user 視点

- 一瞬 Vivaldi が見えるが、すぐ Ghostty に戻る
- alt+Q で OmniWM workspace Q に切り替えた時、Vivaldi window が他 tiles と並んで見える
- Vivaldi 内では `work` profile の cookies / login state を使う

### 重要

- profile (`--profile=work`) は **Vivaldi 内 user profile**（cookies 分離）。projwm の profile とは別概念
- 同一 Vivaldi instance 内で複数 profile の window が共存できる（chromium 仕様）
- `--profile=client-x` を指定すれば、別 cookie jar の window が立ち上がる ⇒ 会社用 / 個人用 login が完全分離

---

## Story 15 — Profile 切替で browser が一緒に切り替わる（paradigm C）

### 状態

- profiles: `work = {Q: dotfiles, W: client-x}`、`personal = {Q: blog}`
- 現在 active = `work`、Vivaldi に dotfiles と client-x の window が 2 個 visible
- ユーザは Ghostty で作業中

### 操作: `projwm profile switch personal`

projwm 内部の動き：
1. frontmost を記録（Ghostty）
2. dotfiles の browser window: `chrome-cli list tabs -w <wid>` で URL 全取得 → `state.Window.SavedURLs` に保存 → `chrome-cli close -w <wid>`
3. client-x の browser window: 同上
4. blog の browser window: `state.Window.SavedURLs` の URL list を `open -na Vivaldi --profile-directory=personal --new-window <urls>` で spawn
5. Ghostty を再 activate

### user 視点

- profile 切替の瞬間、Vivaldi が一瞬チラつく（close と spawn が走るため）
- すぐ Ghostty に focus が戻る
- alt+Q で workspace Q を見ると、blog の browser window だけが見える（dotfiles と client-x の window は消えている）
- 後で `personal` から `work` に戻すと、dotfiles と client-x の URL が **保存されていた URL list 通りに再 open**

### 重要

- close 時の URL snapshot は **その瞬間に開いている全 tab**。手で開いた tab も次回復元される
- scroll 位置 / form 入力 / 動画再生位置は失われる（close するため）
- login / cookies は Vivaldi profile に残るので **再 spawn 後も login 済み**
- ai/shell/editor の close/spawn cycle と完全に同じ paradigm

---

## Story 16 — Browser の focus 強奪が起きないこと（paradigm C 観察可能）

### 状態

- Vivaldi に project window 1 個以上 spawn 済
- ユーザは Ghostty で作業中、profile 切替もしていない

### 観察

- launchd auto-reconcile が走っても **Vivaldi は何もしない**（idempotent な no-op）
- focus が Ghostty から動かない
- cockpit (`projwm tui`) で browser window の URL 一覧が表示される（`chrome-cli list tabs` は read-only で focus を奪わない）

### 重要

- paradigm B（v12 初版）で起きていた「user が触ってないのに Vivaldi が暴れる」は **完全消滅**
- focus が動くのは **destructive event**（add/archive/profile-switch）の時だけ
- 通常運用での focus 動はゼロ

---

## Story 違反の典型（避けるべきこと）

これらが発生したら **直ちに修正対象**:

1. **AI が自動起動していない**: terminal が出るが claude/copilot が走っていない
2. **viewer (WS A) が空**: active な AI 窓に対応する viewer が出ていない
3. **window が間違った WS にいる**: 起動したのに WS Q ではなく別 WS に出る
4. **コマンドが「成功」で返るのにエラー**: errCount=0 表示だが画面に何も無い
5. **profile 切替で tmux session が kill される**: 戻ってきた時に AI 履歴が消えている
6. **archive 後も tmux が alive**: archive は tmux kill が必須
7. **basename collision で `up` が黙って通る**: validate で reject すべき
8. **Ghostty window が OmniWM に見えない**: app-rules の titleRegex rule が無いと SwiftUI hidden helper が干渉
9. **Browser が user の作業中に勝手に動く**: paradigm C は read 系で focus 奪わない。reconcile no-op でも Vivaldi が動いたら直ちに修正対象
10. **profile 切替で browser の URL list が失われる**: close 前の URL snapshot 必須、`SavedURLs` field を欠かさない

---

_本書は projwm の **理想体験** の記述。仕様詳細は `projwm-spec.md`、未着手構想は `projwm-roadmap.md`。_
