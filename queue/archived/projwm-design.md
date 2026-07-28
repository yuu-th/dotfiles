# projwm 設計ドキュメント

> cmux + zellij を捨て、OmniWM ネイティブと tmux を主軸にした AI ワークスペース管理基盤 (`projwm`) の設計。本ドキュメントは要件・設計・実装方針を層別に網羅する一次資料。

---

## ステータスと引き継ぎガイド

### このドキュメントの状態

| 項目 | 値 |
|---|---|
| **フェーズ** | **設計フェーズ完了 / 実装未着手** |
| **最終改訂** | v11.1（2026-05-03） |
| **承認者** | yuta（本人） |
| **実装着手条件** | レイヤー 11.1 の POC 項目（POC-01〜20）で致命傷が出ない事を確認 |

### 同梱資料

projwm を引き継ぐ人は、**以下 2 文書だけで設計の全容を把握できる**ように作っている：

| 文書 | 役割 | 関係 |
|---|---|---|
| **本ドキュメント** (`queue/projwm-design.md`) | projwm 自体の要件・設計・実装方針 | これだけで projwm の全体像が分かる |
| **`OMNIWM.md`** （リポジトリ直下） | 基盤となる OmniWM の機能カタログ・能力境界 | projwm が依存する OmniWM の挙動を全列挙。**本書の前に一読推奨** |

旧資料 (`CMUX.md` / `CMUX-WORKFLOW.md`) は撤去対象。読み返す必要は無し。

### 推奨読み順（新メンバー向け、所要 1〜2 時間）

1. **`OMNIWM.md`** を 30 分でざっと（特に §6〜§9 の hotkeys / IPC / 拡張ポイント）
2. **本書 §0〜§2**（動機・用語・要件、20 分）で何を作るかを掴む
3. **本書 §12 確定決定一覧**（5 分）で「なぜこの設計か」の太い決定群を一気読み
4. **本書 §3〜§8**（30 分）で具体構造（アーキ・スキーマ・reconcile・UI）
5. **本書 §11 Phase 計画**（10 分）で着手順序
6. 不明点は **§1 用語** と **付録 B Open Issues** を行き来

### 30 秒で掴む TL;DR

- **何を作るか**: macOS 上で「AI コーディング project」を OmniWM の named workspace に 1:1 で割り当てて管理する Go バイナリ + 周辺設定
- **特徴**:
  - 1 project = 1 OmniWM 名前付き WS（slot Q〜P の 10 個）
  - 各 slot 内に AI（複数可、claude/copilot 自由混在）+ shell + Zed editor が同居
  - viewer WS A に全 project の AI が read-only で並ぶ
  - Profile でスロット構成セットを保存・切替（work/personal 等）
  - state.json が source of truth、ズレは `reconcile` が自動修正
  - ユーザ操作は alt + 1 文字 chord 無し、TUI launcher が cockpit
- **既存資産との関係**: cmux + zellij は完全廃止、OmniWM の上に新規 Go ツール `projwm` を被せる
- **影響範囲**: `modules/darwin/cmux.nix` 削除、`modules/darwin/projwm/` 新設、OmniWM の workspace/hotkeys 設定追記

### 引き継ぎ後にまずやること

| 優先 | アクション |
|---|---|
| 1 | §11.1 の POC-01〜20 を実機で消化。**致命傷が出れば即フィードバックして設計再考**（特に POC-01/05/15/16/18） |
| 2 | POC を全通過したら §11 Phase 1 から順に実装着手 |
| 3 | 開発中に発見した未確定事項は **付録 B Open Issues に追記**して履歴を残す |

### 版数の意味

| 版 | 主な変更 |
|---|---|
| v3〜v7 | 初期方針の収束（cmux 廃止 / tmux 採用 / WS 名 Q-P / launcher 戦略） |
| v8 | プロファイル & アーカイブ機構の導入 |
| v9 | スキーマ対称化（`primary` / `default` の概念を全廃、`(kind, id)` ペアでの identity） |
| v10 | バージョニング廃止 / down 廃止 / park first-class / config 分離 |
| v11 | Zed を first-class editor 化、`nvim` kind 廃止、basename 一意制約追加 |
| v11.1 | 意味論レビューによる軽微修正（CLI status 出力例 / 空 windows / 複数 profile 表示 / profile rename / config fallback） |
| v11.2 | POC-13 結果反映: tmux session 名 `<kind>-<id>/<proj>:v` が tmux で `:` を許容しない（自動的に `_` 置換）ため、viewer 用 grouped session 名を **`<kind>-<id>/<proj>_v`** に変更（ghostty title `<kind>-view-<id>:<project>` は変更なし、title では `:` OK） |

---

## レイヤー 0 — 動機と背景

### 0.1 現状（移行前）の構造

| 層 | 現状 |
|---|---|
| WM | OmniWM（macOS Niri/Dwindle 系タイル） |
| ターミナル | cmux（独自の縦タブ・サイドバー UI を持つネイティブ macOS app、Ghostty ベース） |
| マルチプレクサ | zellij（cmux 内の永続化のみに使用） |
| AI CLI | Claude Code / GitHub Copilot CLI |

cmux 内部では `1 project = 1 cmux ワークスペース = 左 AI ペイン + 右 ペイン (tools/nvim/browser サーフェス)` という固定レイアウトを `aidev` / `cmux-init` などの fish 関数で組み立てている。OmniWM から見ると cmux は単一アプリの単一ウィンドウ群でしかない。

### 0.2 課題

1. **レイヤーの重複**: cmux と OmniWM が両方ともワークスペース概念を持っており、操作系・状態系が二重化している。混乱と保守コストの源。
2. **project に紐付くウィンドウの限界**: cmux ws の中に押し込める構造が固定（左 AI + 右 3 surface）。同じ project 用の追加ブラウザウィンドウや 2 つ目の AI を自然に同居させにくい。
3. **zellij の操作性**: スクリプタビリティで tmux に劣る。`tmux capture-pane` や `tmux send-keys` のようなメタ操作が乏しい。
4. **OmniWM の表現力を活用できていない**: Niri 列タイル / 動的 appRule / `omniwmctl watch` などの機能が cmux の囲いに阻まれて使えていない。

### 0.3 ゴール

- cmux を完全廃止し、**OmniWM ネイティブの WS とウィンドウ**で project 管理を行う
- zellij を tmux に置き換え、**シェル状態をメタ管理可能**にする（外部から `tmux capture-pane` で観測、`tmux send-keys` で操作）
- AI プロセスは tmux で永続化し、**ウィンドウを閉じても止まらない**運用を維持
- 1 project = 1 OmniWM WS にして、**project 固有の追加ウィンドウ（追加 AI、ローカル browser など）を自由に同居**させられるようにする
- AI を **複数同時に俯瞰する viewer** を別 WS に提供（project ごとの AI を read-only で並べる）
- 人間ピア作業（検索 / ファイルツリー / git / フォルダ操作）を **GUI editor (Zed)** に集約。AI WS 内に editor を同居させ、AI と人間が同じ slot を共有
- 操作 UX は **chord なし／layer なし／単一修飾＋単一キー**を貫く（HHKB ergonomic 重視）
- **state ファイルが source of truth**、ズレは `reconcile` で自動修正される堅牢性

### 0.4 非ゴール（やらないこと）

- 既存 cmux / zellij インターフェースとの互換性維持
- 動的 appRule で project 専用 browser を自動 WS 固定（必要時はユーザが手動で送る）
- レイヤー型キーバインド（modal / leader-key 方式）
- cmux 復活させた場合の dual-write 対応

---

## レイヤー 1 — 用語

| 用語 | 定義 |
|---|---|
| **slot** | OmniWM の AI 用 named workspace。`Q W E R T Y U I O P` の 10 個＋viewer 用 `A` |
| **project** | 1 つの作業 cwd（典型的には 1 git worktree）。slot に動的に対応付けられる |
| **viewer** | WS `A` に集約される、各 project の AI を read-only で複製表示する画面 |
| **state file** | `~/.local/state/projwm/state.json`。runtime 状態（プロファイル・project・アーカイブフラグ） |
| **config file** | `~/.config/projwm/config.toml`。固定的設定（slot 名群・viewer WS 名など） |
| **reconcile** | state file（期待状態）と OmniWM/tmux/ghostty の実状態を比較し、差分を是正する処理 |
| **profile** | slot 割当の名前付きセット（例：`work` / `personal`）。同時にひとつだけ "active" |
| **active profile** | いま slot に展開されているプロファイル。launcher 上部・CLI で常時可視 |
| **archive** | project ごとの状態フラグ。`archived = true` で tmux session を kill、ウィンドウを閉じ、いずれのプロファイルからも展開されなくなる。state は完全保持され、復活 (`unarchive`) で元に戻る |
| **kind** | window の役割。`"ai"` / `"shell"` / `"editor"` の 3 種 |
| **editor** | GUI エディタ。MVP では Zed（bundleId `dev.zed.Zed`）固定。tmux ラップしない。検索・ファイルツリー・git・フォルダ操作などの "人間ピア作業" を一手に引き受ける |
| **basename uniqueness** | active な全 project（archived を除く）の cwd basename は一意でなければならない。Zed window title が basename ベースのため。`up` 時に validate |
| **id** | kind ごとの 1 始まり連番。永続採番、down で穴が空いても再利用しない |
| **`(kind, id)` ペア** | 1 つの window の grand identity。project 内で一意 |
| **windows[]** | project が持つ window の集合。順序意味なし（map 的に扱う） |
| **projwm** | 本設計の中核となる Go バイナリ。CLI と TUI launcher の両方を提供 |
| **launcher** | bubbletea ベースの TUI。OmniWM Quake terminal の中で表示される操縦席 |
| **scratch** | ghostty 純正 quick-terminal。ad-hoc コマンド実行用の汎用シェル |
| **grouped session** | tmux の `new-session -t <base>` で作る、別 client 状態を持ちつつ pty を共有するセッション。AI viewer の複製にだけ使う |
| **無所属 project**（park） | `projects` には居るが、いずれの profile の `assignments` にも入っていない project。warning ではなく first-class の状態（C-2 採用） |

---

## レイヤー 2 — 要件

### 2.1 機能要件 (FR)

| ID | 内容 |
|---|---|
| FR-01 | 任意の cwd を空きスロットに割り当て、**AI ウィンドウ + shell ウィンドウ + editor (Zed) ウィンドウ** を既定で起動できる（FR-22 と整合） |
| FR-02 | 単一修飾＋単一キー（`alt+letter`）で任意の slot に jump できる |
| FR-03 | 同様に `alt+shift+letter` で現在ウィンドウを slot に送れる |
| FR-04 | `alt+a` で viewer WS に jump できる |
| FR-05 | 全 project の AI を viewer WS で read-only で並列表示できる |
| FR-06 | TUI launcher で project 一覧の確認・jump・新規・停止・reconcile が完結する |
| FR-07 | スクラッチシェルが project 操作と独立して常時利用できる |
| FR-08 | OmniWM 再起動・モニタ抜き差し後でも 1 分以内に意図状態に自動復帰する |
| FR-09 | `projwm reconcile --dry-run` で差分のみ表示できる |
| FR-10 | `projwm status` で全 slot の整合性を可視化できる |
| FR-11 | `projwm restore` で tmux サーバから生存 project を再構築できる（cmux-init 相当） |
| FR-12 | 同一 project に 2 つ目以降の AI ウィンドウ・shell・editor を追加できる |
| FR-22 | `up` の既定起動セットは AI 1 個 + shell 1 個 + **editor 1 個**（Zed）。`--no-editor` で editor をスキップできる |
| FR-23 | editor (Zed) は project の cwd を開き、profile 切替で window が close、active 復帰で再 spawn される |
| FR-24 | 既存 OmniWM の `dev.zed.Zed → WS E` 一律ルールは廃止し、projwm が **per-project の slot に Zed window を振り分ける** |
| FR-12a | 同一 project 内で **複数の AI（claude と copilot 同時、または同種の複数並走）が first-class** に共存できる。各 AI は独立した tmux session と viewer 用 grouped clone を持つ |
| FR-12b | viewer (WS A) は **project 単位ではなく AI ウィンドウ単位**で複製を表示する。`project A` に AI が 2 つあれば viewer に 2 個のタイルが並ぶ |
| FR-13 | 任意の project を停止し、tmux セッションとウィンドウを片付け、slot を解放できる |
| FR-14 | 複数のプロファイル（例：`work` / `personal`）を保存し、いつでも切替できる |
| FR-15 | プロファイル切替時、旧プロファイルの project ウィンドウを閉じ、新プロファイルの project を slot に展開する |
| FR-16 | プロファイルが切替わっても、archived でない project の tmux session は生かしたまま（窓だけ閉じる）にすることで再切替を瞬時に行える |
| FR-17 | project をアーカイブできる（tmux kill、windows close、state は保全）。アーカイブ済みは launcher で別エリアに表示 |
| FR-18 | アーカイブ済み project を復活（unarchive）でき、以後通常のプロファイル切替で再展開できる |
| FR-19 | プロファイル間の project の移動・複数プロファイルへの所属が可能（同じ project が `work` と `personal` 両方で別 slot に居る等） |
| FR-20 | launcher TUI に現在の active profile 名と「他プロファイル一覧／アーカイブ件数」が常時表示される |
| FR-21 | viewer (WS A) は active profile の project だけを read-only で表示する（profile 切替で viewer も自動入れ替え） |

### 2.2 非機能要件 (NFR)

| ID | 内容 |
|---|---|
| NFR-01 | コマンドは冪等（同じ操作を繰り返しても破壊的にならない） |
| NFR-02 | state file 編集は `flock(2)` で排他、書き込みは tmpfile + atomic rename |
| NFR-03 | reconcile は副作用最小（state にない既存ウィンドウは保持、自動 close しない） |
| NFR-04 | 全コマンドの実行ログを `~/.local/state/projwm/logs/` に蓄積 |
| NFR-05 | キーバインドは macOS の他アプリと衝突しない（cmd 修飾の使用禁止） |
| NFR-06 | OmniWM `[[workspaces]]` 定義の追加で導入できる。既存の数値 WS 1〜9 / M / B は破壊しない（**WS E は意図的に廃止**してプロジェクト slot として再利用） |
| NFR-07 | Go バイナリ単体で動作（外部 fish 関数依存を排し、Karabiner からも直接起動可能） |
| NFR-08 | プロファイル切替は **windows 操作のみで完結し、tmux session の kill/start を伴わない**（高速切替の保証）。inactive 化された project の AI / shell tmux session は **archive を明示的に呼ばない限り破棄されない**。viewer (WS A) の表示は **active profile の slot に居る AI 窓だけ**を反映する |
| NFR-09 | アーカイブ操作は idempotent（archived 済みを再 archive しても no-op） |
| NFR-10 | state.json は **バージョニングを持たない**（pre-launch なので不要）。将来 schema を破壊的に変える必要が出たときに `version` フィールドを足す（migration コードもその時点で書く）|
| NFR-11 | 設定値（slot 名群、viewer WS 名など）は **state ではなく config (`~/.config/projwm/config.toml`)** に置く。state は runtime 状態のみ |
| NFR-12 | active な全 project（archived を除く）で **cwd basename が一意**。`projwm up` 実行時に validate、衝突したら拒否してユーザに `--as <name>` フラグまたは folder rename を促す |
| NFR-13 | GUI app（editor 等の tmux ラップしない window）は projwm の title 規約 (`<kind>-<id>:<project>`) を強制しない。各 GUI app の自然 title をそのまま使い、bundleId と title pattern の組合せで identify |

### 2.3 非要件（明示的にやらないこと）

| ID | 内容 | 理由 |
|---|---|---|
| NR-01 | 動的 appRule での project 専用 browser 自動固定 | ユーザ判断で送る派 |
| NR-02 | modal / leader-key UX | 押下数増を嫌う |
| NR-03 | cmux 復活互換 | 完全廃止前提、dual-write は壊れる |
| NR-04 | shell 永続化を tmux 以外の手段（systemd-style）で行う | tmux で十分、複雑化を避ける |
| NR-05 | nvim を kind として持つ | Zed が first-class エディタ。nvim 派は shell-N の tmux 内で起動 |
| NR-09 | GUI editor を tmux に押し込む | tmux は terminal multiplexer、GUI app は対象外。Zed は素の macOS app として動かす |
| NR-06 | state を SQLite 等の DB で持つ | JSON で十分、可搬性と人間可読性優先 |
| NR-07 | 複数プロファイルを **同時に active** 状態にする（オーバーレイ） | slot 衝突解消のロジックが複雑、UX も曖昧 |
| NR-08 | アーカイブ済み project の自動再活性（時間経過 / 利用頻度 / git 活動などのヒューリスティック） | 暗黙的な復活はバグの温床、明示的な unarchive のみ |

---

## レイヤー 3 — 全体アーキテクチャ

### 3.1 構成図

```
┌──── Karabiner ─────────────────────────────────────────────────┐
│  alt+1..9                  日常 WS（既存、手付かず）             │
│  alt+m/b                   既存 named WS（手付かず）             │
│  alt+space                 OmniWM Quake = launcher 起動         │
│  alt+`                     ghostty quick-term = scratch 起動     │
│  alt+q/w/e/r/t/y/u/i/o/p   slot へ直接 jump                     │
│  alt+shift+ 同             slot へ窓を送る                       │
│  alt+a / alt+shift+a       viewer (WS A) jump / 送る             │
└────────────────────────────────────────────────────────────────┘
                                │
                  ┌─────────────┴───────────┐
                  ▼                         ▼
         OmniWM Quake                ghostty quick-term
         （= launcher）               （= scratch）
                  │
                  ▼
       projwm (Go binary)
       ─────────────────
       cobra CLI          ──→  state.json + reconcile
       bubbletea TUI            │
                                ▼
                      omniwmctl + tmux + ghostty
                                │
          ┌─────────────────────┼─────────────────────┐
          ▼                     ▼                     ▼
   OmniWM (WS 配置)        tmux server          ghostty windows
                          ─────────────         （title 規約）
                          ai-N/<proj>           ai-N:<proj>
                          ai-N/<proj>_v         ai-view-N:<proj>
                          shell-N/<proj>        shell-N:<proj>
                                                <basename> (Zed window, GUI、tmux 無し)
```

### 3.2 主要レスポンシビリティ

| コンポーネント | 責務 |
|---|---|
| **Karabiner** | 物理キー → omniwmctl コマンド or projwm CLI 実行 |
| **OmniWM** | WS の定義、ウィンドウのタイル/移動、Quake、appRule、IPC |
| **omniwmctl** | OmniWM 操作の唯一のインターフェース |
| **tmux server** | AI / shell の永続セッション、grouped session による viewer 複製 |
| **ghostty** | tmux client を表示する単一窓（kind = ai / shell） |
| **Zed** | GUI editor。kind = editor の窓を担当。bundleId `dev.zed.Zed` |
| **state.json** | 「あるべき姿」を保存する source of truth |
| **projwm CLI** | state を編集、reconcile を駆動 |
| **projwm TUI (bubbletea)** | launcher 操縦席、人間からの入力 |
| **reconcile** | 期待状態と実状態の差分検出と修正 |
| **launchd** | reconcile を `omniwmctl watch` 駆動 + 60s 定期で常駐 |

---

## レイヤー 4 — ワークスペース層

### 4.1 OmniWM ワークスペース構成

| 種別 | 名前 | 用途 | layoutType | キー |
|---|---|---|---|---|
| 既存・手付かず | `1`〜`9` | 日常用（メール、調べもの、Slack 等何でも） | niri | `alt+1..9` |
| 既存・手付かず | `M` | Media（Spotify / Discord / Calendar / Music） | niri | `alt+m` |
| 既存・手付かず | `B` | Browser（Chrome / Firefox / Safari / Dia / Zen） | niri | `alt+b` |
| **新設** | **`A`** | **AI Viewer（read-only grid、active profile の project を表示）** | niri | `alt+a`（既存 Calendar 起動から振替） |
| **新設** | **`Q`** | **AI project slot 1** | niri | `alt+q` |
| 新設 | `W` | AI project slot 2 | niri | `alt+w` |
| 新設 | `E` | AI project slot 3（既存 WS E は廃止、解放） | niri | `alt+e` |
| 新設 | `R` | AI project slot 4 | niri | `alt+r` |
| 新設 | `T` | AI project slot 5 | niri | `alt+t` |
| 新設 | `Y` | AI project slot 6 | niri | `alt+y` |
| 新設 | `U` | AI project slot 7 | niri | `alt+u` |
| 新設 | `I` | AI project slot 8 | niri | `alt+i` |
| 新設 | `O` | AI project slot 9 | niri | `alt+o` |
| 新設 | `P` | AI project slot 10 | niri | `alt+p` |

### 4.2 命名規則と固定割当

- **物理キーが WS 名そのもの**。`alt+q` → `omniwmctl workspace focus-name Q`。間接化なし。
- 文字 slot はすべて空のとき `hideEmptyWorkspaces=true`（既存設定）で workspace bar から自動で隠れる
- WS `E` は既存「Editor」用途を廃止。エディタは **Zed として project の AI slot に直接配置**（projwm が per-project に振り分け）
- monitorAssignment は全 slot `main` 固定（マルチモニタプロファイル時の挙動は移行期に再検討）

### 4.3 廃止する WS

| WS 名 | 廃止理由 |
|---|---|
| `E` | エディタ (Zed) は project AI slot に projwm が振り分けるため WS E は不要 |

### 4.4 廃止／振替する既存キーバインド

| キー | 旧動作 | 新動作 | 備考 |
|---|---|---|---|
| `alt+a` | `WS M + Calendar` 起動マクロ（`karabiner-rules.nix` の `wsLaunch a Calendar`） | **viewer (WS A) jump** | Calendar は手動 / Spotlight / Dock 起動に切替 |
| `alt+shift+a` | （未割当） | **viewer (WS A) へ窓を送る** | 新規 |
| `alt+e` | `WS E` jump（既存） | **AI slot E へ jump**（WS E 廃止に伴う） | エディタ (Zed) は各 AI slot に同居 |
| `alt+shift+e` | `WS E へ窓を送る` | **AI slot E へ窓を送る** | 同上 |

---

## レイヤー 5 — プロセス・セッション層

### 5.1 tmux セッション構成（1 project あたり）

| セッション名 | 役割 | 永続化 | 備考 |
|---|---|---|---|
| `ai-N/<proj>`（N=1,2,...） | AI 本体（Claude or Copilot） | 必須 | 各 AI 窓から `tmux attach -t`。N は連番採番、永続 |
| `ai-N/<proj>_v` | AI N の viewer 用 grouped clone | 必須 | viewer 窓から `tmux attach -r -t`（read-only）。tmux は session 名内の `:` を許容しないため `_v` 末尾（v11.2） |
| `shell-N/<proj>`（N=1,2,...） | 自由シェル | 必須 | メタ管理（`capture-pane` / `send-keys`）で観測・操作可能 |

すべての AI / shell が 1 から始まる連番付き名前を持つ。「最初の 1 個だけ無連番」のような特例は無い。

**運用上の既定は AI 1 個／project** だが、設計は多 AI を first-class にサポートする。AI 1 個を **強制する制約はどこにも置かない**。

容量は事実上無制限：Niri の `maxWindowsPerColumn = 4` は **「1 column に縦スタックできる最大窓数」**であって workspace 全体の上限ではない。5 窓以降は **新しい column が右に自動生成**され、`alt+h/l` で巡回する。1 slot WS に AI 10 個でも収納可能（視認性は別問題）。

#### 5.1.1 「すべての AI は完全に対等」

スキーマにも命名規則にも **primary / default / main の概念を持たない**。全 AI は `(kind="ai", id=N)` で識別され、N は単なる連番。

| 項目 | AI 1 (`id=1`) | AI N (`id=N`) | 差 |
|---|---|---|---|
| schema 表現 | `windows[]` の 1 要素 | 同 | 無 |
| `ai` フィールド | 持つ | 持つ | 無 |
| viewer 窓（grouped clone） | 持つ | 持つ | 無 |
| viewer (WS A) に並ぶ | する | する | 無 |
| reconcile 対象 | 同等 | 同等 | 無 |
| archive で kill | される | される | 無 |
| profile 切替で hibernate | される | される | 無 |
| `down` 個別停止 | できる | できる | 無 |
| 名前（title・tmux） | `ai-1:<proj>` 等 | `ai-N:<proj>` 等 | **N 値のみ**（生成時刻順序） |

`add-ai` で新規追加時の id 採番は **現存する最大 id + 1**。`down` で穴が空いても再利用しない（連番永続）。例：

```
状態: windows=[ai-1, ai-2, ai-3]
$ projwm down ai-2 dotfiles
状態: windows=[ai-1, ai-3]
$ projwm add-ai --ai claude dotfiles
状態: windows=[ai-1, ai-3, ai-4]   ← ai-2 は再利用しない
```

これにより tmux session 名（`ai-1/dotfiles`, `ai-3/dotfiles`, ...）と viewer 窓 title（`ai-view-1:dotfiles`, ...）の同一性が永続的に保たれ、reconnect 時の混乱を防ぐ。

#### 5.1.2 grouped session の生成方法

grouped session は **AI window の viewer 複製にだけ使う**（shell では使わない、editor は tmux 無し）：

```sh
# 例: ai-1/<proj> 本体と viewer 用 grouped clone
tmux new-session -d -s ai-1/<proj>                       # 本体作成
tmux new-session -d -t ai-1/<proj> -s ai-1/<proj>_v      # 同じ pty を共有する別 client
                                                          # （`:v` は tmux で `:` 不可のため `_v`、v11.2）
```

#### 5.1.3 リサイズ衝突回避

複数 client の最小サイズに従うデフォルト挙動を回避するため、tmux サーバ設定に：

```tmux
set -g window-size latest
set -g aggressive-resize on
```

を入れる。最後にフォーカスした client に追従。

### 5.2 ghostty ウィンドウ構成

| ウィンドウ title | App | 中身 | tmux | 配置 WS |
|---|---|---|---|---|
| `ai-N:<proj>` | ghostty | tmux client → `ai-N/<proj>` | yes | project slot |
| `ai-view-N:<proj>` | ghostty | tmux client → `ai-N/<proj>_v`（read-only） | yes | viewer (`A`) |
| `shell-N:<proj>` | ghostty | tmux client → `shell-N/<proj>` | yes | project slot |
| `<basename of cwd>` | **Zed** | GUI editor (cwd を開く、`dev.zed.Zed`) | **no** | project slot |

ghostty 系は tmux ラップで永続化。Zed は GUI app なので tmux 外、Zed 自身の session restore に頼る。

### 5.3 ghostty title 規約

OmniWM の appRules マッチャは title 文字列で識別する以外に ghostty を区別する手段がない（bundleId 一意）。ゆえに：

title / tmux / viewer は **state に保存せず、`(kind, id, project)` から projwm が決定的に算出**する：

| kind | App | title | tmux session | viewer title | viewer tmux |
|---|---|---|---|---|---|
| `ai` | ghostty | `ai-N:<project>`（projwm 規約） | `ai-N/<project>` | `ai-view-N:<project>` | `ai-N/<project>_v` |
| `shell` | ghostty | `shell-N:<project>`（projwm 規約） | `shell-N/<project>` | — | — |
| `editor` | **Zed** | `<basename(cwd)>`（**Zed の自然 title**、projwm 規約使わず） | — | — | — |

**ghostty 窓の N 採番規則**: kind ごとに現存する最大 id + 1。down で穴が空いても再利用しない（永続）。最初の窓も `N=1` 付き — 「最初だけ無連番」のような特例は無し。

**`editor` kind の採番**: 通常は id=1 のみ（1 project に 1 Zed window が標準）。`add-editor` で id=2,3 と増やすことは可能だが、Zed の window 識別は basename ベースなので **多 editor 並走時は POC で挙動確認**（OI-15）。

**`term` / `nvim` kind は廃止**：
- `term` は旧設計の "plain ghostty" 用途。全 ghostty が tmux ラップになり `shell` と機能重複
- `nvim` は Zed 導入で人間ピア作業の主役交代。nvim を使いたい場面は `shell-N` の tmux 内で起動

ghostty 起動時に `--title` で固定し、tmux 内部からの title 上書きを防ぐ：

```tmux
set -g set-titles off
set -g allow-rename off
```

ghostty 設定でも `title = ""` を空にして外部 `--title` を尊重させる（POC で確認）。

### 5.4 ghostty 起動コマンド規約

```sh
# kind="ai", id=N, project=<proj> の場合
ghostty \
  --title="ai-N:<proj>" \
  --working-directory=<cwd> \
  -e tmux new-session -A -s ai-N/<proj>

# kind="shell", id=N の場合
ghostty \
  --title="shell-N:<proj>" \
  --working-directory=<cwd> \
  -e tmux new-session -A -s shell-N/<proj>

# kind="editor"（Zed）の場合
zed -n <cwd>
# `-n` (--new) は **必須**: フラグ無しの `zed <cwd>` は既存 Zed workspace を再利用してしまう（POC-17、v11.2）
# spawn 後、basename(cwd) で omniwmctl query して window ID を取り、project slot に move-to-workspace
```

`-A` は「セッションがあれば attach、無ければ作成」。冪等性確保。

projwm 内部で `(kind, id, project)` から title・tmux session 名を算出するヘルパ `naming.Resolve(kind, id, project)` を 1 か所に置き、他の全コードはそれを呼び出すだけにする（不整合バグ排除）。

---

## レイヤー 6 — 状態管理層

### 6.1 state / config の場所

state（runtime 変動）と config（固定設定）は分離する：

```
~/.local/state/projwm/        ← XDG_STATE_HOME 配下、runtime のみ
├── state.json                ← source of truth（profiles + projects）
├── state.json.bak            ← 直前のバックアップ
├── lock                      ← flock(2) ファイル
└── logs/
    ├── reconcile.log
    └── commands.log

~/.config/projwm/             ← XDG_CONFIG_HOME 配下、固定設定
└── config.toml               ← slot 名群、viewer WS 名 等
```

state は Nix 管理外。config は projwm 自身がデフォルト埋め込み（手動編集も可）。

### 6.2 config.toml

```toml
# ~/.config/projwm/config.toml

viewer_workspace = "A"
slot_names = ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"]

# 将来の拡張用（現状ではどれも空でよい）
# default_ai は意図的に持たない
```

projwm 起動時に読む。値の変更は projwm の再起動で反映（reconcile 経由）。

#### 6.2.1 config.toml が無い／壊れている時の fallback

- **ファイル不在**: 上記の値群を **デフォルトとして埋め込み**で使う。エラーにしない。`projwm doctor` だけが INFO で「config.toml 未配置、デフォルト動作中」を通知
- **TOML パース失敗**: エラー終了し、ユーザに `~/.config/projwm/config.toml` の修正または削除を促す（壊れた config の上から推測動作させない）
- **不明なフィールドが含まれる**: 警告のみ（forward compat、将来の field を考慮）
- **必須フィールド欠落**（例：`slot_names` が空配列）: エラー終了

これにより **初回起動時でも projwm は動く**（Nix 配布前の手動インストールでも問題なし）。

### 6.3 state.json スキーマ

**バージョニングフィールドは持たない**。pre-launch かつ schema 進化の必要が今ないため YAGNI。将来必要になった時点で `version` フィールドを足す（古い state は version 無し = 1 とみなす）。

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
      "assignments": {
        "Q": "blog"
      }
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
        { "id": 1, "kind": "editor"                 }
      ]
    },
    "manaflow":  {
      "cwd": "/Users/yuta/work/manaflow",
      "archived": false,
      "windows": [
        { "id": 1, "kind": "ai",     "ai": "copilot" },
        { "id": 2, "kind": "ai",     "ai": "copilot" },   // 同種 2 並走
        { "id": 1, "kind": "shell"                    },
        { "id": 1, "kind": "editor"                   }
      ]
    },
    "client-x":  { "cwd": "...", "archived": false, "windows": [
        { "id": 1, "kind": "ai", "ai": "claude" },
        { "id": 1, "kind": "shell" }                       // editor 未起動の例
    ]},
    "blog":      { "cwd": "...", "archived": false, "windows": [
        { "id": 1, "kind": "ai", "ai": "claude" },
        { "id": 1, "kind": "shell" },
        { "id": 1, "kind": "editor" }
    ]},
    "park-1":    { "cwd": "...", "archived": false, "windows": [
        { "id": 1, "kind": "ai", "ai": "claude" }          // 無所属（park）の例
    ]},
    "old-thing": { "cwd": "...", "archived": true,  "windows": [...] }
  }
}
```

**無所属 project**（park）の例：上記 `park-1` は `archived: false` だが、どの profile の `assignments` にも入っていない。これは **first-class な状態**（warning ではない、C-2 採用）。tmux session は alive のまま、ghostty 窓は閉じている。launcher の "parked projects" セクションに表示され、ユーザは任意のタイミングで profile に再 assign できる。

#### 6.3.1 フィールド

| フィールド | 型 | 説明 |
|---|---|---|
| `active_profile` | string | 現在 active なプロファイル名。`profiles` のキーを指す |
| `profiles` | map<string, Profile> | プロファイル名→定義 |
| `profiles[name].description` | string | 人間可読な説明（任意） |
| `profiles[name].assignments` | map<slot, project> | slot 名 → project 名のマップ。重複不可（1 slot に 1 project） |
| `projects` | map<string, Project> | project 名 → 定義（master pool） |
| `projects[name].cwd` | string | 絶対パス |
| `projects[name].archived` | bool | `true` なら tmux kill / windows close、いずれの profile からも展開しない |
| `projects[name].windows[]` | array | 意図ウィンドウ群（AI 複数・shell・editor をフラットに並べる） |
| `projects[name].windows[].id` | int | `kind` ごとの連番（1 始まり、永続採番、穴は埋めない） |
| `projects[name].windows[].kind` | enum | `"ai"` \| `"shell"` \| `"editor"` |
| `projects[name].windows[].ai` | enum? | `kind="ai"` のときのみ必須。その窓個別の `"claude"` \| `"copilot"` |

**スキーマには "primary" / "default" / "main" のような階層的概念を一切持たせない**。すべての AI 窓は完全に対等。`(kind, id)` のペアが window の identity。

#### 6.3.2 算出フィールド

`title` / `tmux session 名` / `viewer 窓` / `viewer tmux` は state に保存せず、projwm が `(kind, id, project)` から決定的に算出する：

| 算出物 | 規則 | 例 |
|---|---|---|
| ghostty title (kind = ai / shell) | `<kind>-<id>:<project>` | `ai-1:dotfiles` `ai-2:dotfiles` `shell-1:dotfiles` |
| tmux session (kind = ai / shell) | `<kind>-<id>/<project>` | `ai-1/dotfiles` `shell-1/dotfiles` |
| viewer title (kind = ai のみ) | `<kind>-view-<id>:<project>` | `ai-view-1:dotfiles` |
| viewer tmux (kind = ai のみ) | `<kind>-<id>/<project>_v` | `ai-1/dotfiles_v` |
| Zed window title (kind = editor) | `basename(<cwd>)` （**Zed が決める**、projwm は識別子として使うだけ） | `dotfiles`（cwd = `/Users/yuta/dev/dotfiles` のとき） |
| Zed identifier (kind = editor) | bundleId `dev.zed.Zed` + title 完全一致 | (上記 title で query) |

これにより state は **identity のみ**を保ち、表現に関するすべての文字列はプログラム側の唯一の真実関数で生成される（不整合バグを排除）。

#### 6.3.3 不変条件 (invariant)

- `active_profile` は必ず `profiles` の既存キー
- `profiles[*].assignments` の値は必ず `projects` の既存キー
- archived = true な project は **active profile の assignments に存在してはならない**（archive 操作時に該当 profile の assignment は自動解除）
- 同一 profile 内では同一 slot に複数 project をマップしない（map 構造で物理的に保証）
- 同一 profile 内では同一 project を複数 slot にマップしない（reconcile 前に projwm が validate）
- 同一 `(kind, id)` ペアは project 内で一意。重複不可
- `id` は kind ごとに 1 始まり、最大値+1 で採番。down で穴が空いても **再利用しない**（連番永続）
- `kind="ai"` の窓は **必ず `ai` フィールドを持つ**。それ以外の kind は持たない
- `kind="editor"` の窓は **tmux session を持たない**（GUI app）。viewer も持たない
- `kind="editor"` は project あたり **id=1 が標準**（1 project = 1 Zed window）。多 editor は将来検討（OI-15）
- **basename uniqueness**: active な全 project（archived を除く）で `path.basename(cwd)` が一意。`up` 時に validate（NFR-12）
- **空 `windows[]` の許容**: project の `windows[]` が空であっても **state 上は valid**（cwd だけ覚えている "metadata-only project"）。reconcile は何も spawn しない（archive と区別される：archived=false、tmux 不在、windows 不在）。launcher / status は "no windows" と表示
- **最後の window 削除時の挙動**: `remove --window` で project の最後の window が消える時、デフォルトでは **空 windows[] を許容して project を残す**。`--purge-if-empty` 付きなら project ごと削除（unrecoverable、確認プロンプト）
- `windows[]` の並び順に意味は無い（schema 上はセット扱い、表示順は projwm が `(kind, id)` でソート）

### 6.4 source of truth ポリシー

- state.json に**書かれているものだけ**が「あるべき姿」
- 実状態に存在しても state にない窓は **orphan として尊重**（自動 close しない、NR 参照）
- state にあって実状態に無いものは reconcile が**作りに行く**
- 直接の手編集も許容。`projwm reconcile` で意図状態に揃う

### 6.5 排他制御

- 全書き込みは `flock(2)` で `lock` ファイルを排他取得
- 書き込みは `state.json.tmp` に書いて `rename(2)` で atomic 差替
- 読み込みは lock 不要（atomic rename により部分書込み状態は読まれない）

### 6.6 プロファイル＆アーカイブのライフサイクル

#### 6.6.1 プロファイル切替のセマンティクス

`projwm profile switch <new>` 実行時の挙動：

```
current = state.profiles[state.active_profile].assignments
target  = state.profiles[<new>].assignments

closing_projects = { p for slot,p in current if (slot,p) not in target.items() }
opening_projects = { p for slot,p in target  if (slot,p) not in current.items() }
moving_projects  = { (p, old_slot, new_slot) for slot,p in target.items()
                                              if p in current.values() and current_slot_of(p) != slot }

for p in closing_projects:
    # p.windows[] の全ウィンドウ（AI 全数 + shell 全数 + editor 全数）を close
    # 各 AI 窓に紐づく viewer (WS A) 側の窓も close
    # tmux session は touch しない（NFR-08）

state.active_profile = <new>

for p in opening_projects:
    spawn all of p.windows in target slot (reconcile が穴埋め)
    spawn all viewer windows for p's AI windows in WS A

for (p, old_slot, new_slot) in moving_projects:
    # p の **全 windows を** new_slot に move-to-workspace
    # viewer 窓は WS A のままなので touch しない
    for w in p.windows:
        omniwmctl move-to-workspace --window <id-of-w> --workspace <new_slot>

projwm reconcile  # 整合性最終確認
```

**保証**:
- tmux session の kill / start は伴わない（NFR-08）
- moving_projects では **project の全 window が一括で新 slot へ移動**（half-moved 状態を作らない）
- プロファイル切替コストは「ウィンドウ数 × spawn/close/move レイテンシ」のみ

#### 6.6.2 アーカイブのセマンティクス

`projwm archive <project>` 実行時：

1. `state.projects[<project>].archived = true`
2. 全 profile の `assignments` から `<project>` を **値として** 持つ slot を削除
3. project の全 ghostty windows を close（title 一致 で omniwmctl から削除）
4. project の **全 tmux session を kill**：
   - 全 AI 窓ぶん `ai-N/<project>` と `ai-N/<project>_v`
   - 全 shell 窓ぶん `shell-N/<project>`
   - editor 窓は tmux 無し → window close のみ（Zed app 自身は kill しない、他 project の Zed window が生きていれば共存）
5. state を保存
6. reconcile（残骸の掃除確認）

`projwm unarchive <project>` 実行時：

1. `state.projects[<project>].archived = false`
2. （**自動再展開はしない**）
3. unarchive 後の状態は **無所属 project (park)** になる（first-class）。ユーザは launcher / CLI で任意のタイミングで profile に assign する

#### 6.6.3 viewer (WS A) の更新ルール

- viewer は **active profile の assignments に含まれる project だけ**を表示
- profile 切替時：旧プロファイル only の project の viewer 窓を閉じ、新プロファイル only の project の viewer 窓を spawn
- archived project の viewer 窓は自動閉鎖、grouped session も kill
- **プロファイルに属さない project**（`projects` には居るが任意 profile の assignments に居ない）は viewer に出さない

#### 6.6.4 プロファイル削除の安全性

`projwm profile delete <name>` 実行時：

- `<name>` が active なら拒否（`switch` を要求）
- `<name>` の `assignments` を解除しても、**`projects` 側のエントリは保持**（他プロファイルが参照中の可能性、または無所属 project として park 状態に）
- どのプロファイルからも参照されなくなった project は **無所属 project (park)** として first-class に存在する。**warning は出さない**（C-2 採用）

#### 6.6.5 初期 state

初期 `state.json` は **空のプロファイル群** からスタート（既定プロファイル無し）：

```json
{
  "active_profile": "",
  "profiles": {},
  "projects": {}
}
```

最初の `projwm up` 実行時、TUI または CLI prompt で：
1. プロファイル名を入力（"work" など）→ プロファイル新規作成、active 化
2. cwd を確認
3. `--ai` を選択（必須）
4. 空き slot を自動割当または明示

これにより「使われない既定プロファイル」が state に残らない。

#### 6.6.6 無所属 project (park) のライフサイクル

park 状態の project は：

| 観点 | 状態 |
|---|---|
| state | `projects[name]` に存在、`archived: false` |
| 任意 profile への assignment | **無し** |
| tmux session | **alive のまま**（archive と区別） |
| ghostty 窓 | **closed**（active profile に居ないので） |
| viewer (WS A) | **表示されない** |
| launcher 表示 | "parked projects" セクションに表示 |
| 復活方法 | profile に assign するだけ（unarchive 不要） |
| 完全停止方法 | `projwm archive <project>` で archive へ |

park になる経路は 3 通り：
- `projwm down <project>`（と思いきや `down` は廃止：代わりに `profile unassign <slot>`）
- `profile unassign <slot>` 実行
- 所属していた profile が `delete` された
- `unarchive <project>` で archived から復活した直後

---

## レイヤー 7 — Reconcile

### 7.1 期待状態 vs 実状態の差分

`projwm reconcile` は以下のループを 1 回実行。`naming.Resolve(kind, id, project)` が title・tmux session 名を算出するヘルパ：

```
active = state.profiles[state.active_profile].assignments  # slot → project name
non_active_projects = state.projects のうち、active.values() に含まれず、archived=false なもの
                       (= park + 他 profile に居る project)

# 1) active profile の slot 配置
for slot, project_name in active.items():
    p = state.projects[project_name]
    if p.archived: continue                  # 不変条件違反、ログして skip
    for w in p.windows:
        if w.kind in ("ai", "shell"):
            title, tmux_session = naming.Resolve(w.kind, w.id, project_name)
            ensure_ghostty_window_at_slot(title, tmux_session, p.cwd, slot)
            if w.kind == "ai":
                v_title, v_tmux = naming.ResolveViewer(w.id, project_name)
                ensure_grouped_session(tmux_session, v_tmux)
                ensure_ghostty_window_at_slot(v_title, v_tmux, p.cwd, viewer_workspace)
        elif w.kind == "editor":
            zed_title = path.basename(p.cwd)
            ensure_zed_window_at_slot(zed_title, p.cwd, slot)
            # ↑ omniwmctl query --bundle-id dev.zed.Zed --title=zed_title で識別、
            #   無ければ `zed <cwd>` で spawn → 出現を polling → move-to-workspace

# 2) viewer (WS A) の orphan を掃除
expected_viewer_titles = {
    naming.ResolveViewer(w.id, name)[0]
    for slot, name in active.items()
    for w in state.projects[name].windows
    if w.kind == "ai" and not state.projects[name].archived
}
for w_in_A in omniwmctl_query_windows(workspace="A"):
    if w_in_A.title not in expected_viewer_titles and matches_pattern(w_in_A.title):
        close_window(w_in_A)

# 3) park / 他 profile の project: windows close、tmux は alive 維持
for p in non_active_projects:
    for w in p.windows:
        if w.kind in ("ai", "shell"):
            title, _ = naming.Resolve(w.kind, w.id, p_name(p))
            close_ghostty_window_if_exist(title)
            if w.kind == "ai":
                v_title, _ = naming.ResolveViewer(w.id, p_name(p))
                close_ghostty_window_if_exist(v_title)
        elif w.kind == "editor":
            zed_title = path.basename(p.cwd)
            close_zed_window_if_exist(zed_title)  # Zed app 自身は kill しない
    # tmux session は touch しない（editor は tmux 無いので無関係）

# 4) archived project の完全片付け
for name, p in state.projects.items():
    if not p.archived: continue
    for w in p.windows:
        if w.kind in ("ai", "shell"):
            title, tmux_session = naming.Resolve(w.kind, w.id, name)
            close_ghostty_window_if_exist(title)
            kill_tmux_session_if_exist(tmux_session)
            if w.kind == "ai":
                v_title, v_tmux = naming.ResolveViewer(w.id, name)
                close_ghostty_window_if_exist(v_title)
                kill_tmux_session_if_exist(v_tmux)
        elif w.kind == "editor":
            zed_title = path.basename(p.cwd)
            close_zed_window_if_exist(zed_title)
            # tmux 無し
```

state には title / tmux session 名は **保存しない**。常に `naming.Resolve` で算出。これによりリネーム時の不整合バグが構造的に発生しない。

### 7.2 修正アクション一覧

| 差分 | アクション | 冪等性 |
|---|---|---|
| active project の窓が無い | `ghostty --title=<t> --working-directory=<cwd> -e tmux new-session -A -s <session>` | ✓ |
| active project の窓が間違った slot | `omniwmctl command move-to-workspace --window <id> --workspace <slot>` | ✓ |
| active project の tmux セッションが無い | `tmux new-session -d -s <session>` | ✓ |
| viewer 用 grouped clone が切れている | `tmux new-session -d -t <ai-session> -s <ai-session>_v` | ✓ |
| viewer 窓が viewer WS にいない | `omniwmctl command move-to-workspace --window <id> --workspace A` | ✓ |
| AI セッションに対し viewer clone が無い（active のみ） | 上記 grouped session 作成 | ✓ |
| **inactive profile** の project の窓がまだ開いている | windows close、**tmux は touch しない** | ✓ |
| **archived** project の窓・tmux がまだ生きている | windows close、tmux kill | ✓ |
| state の `active_profile` が `profiles` に存在しない | エラー、ユーザに通知。デフォルトには戻さない | — |
| state の `assignments` 値が `projects` に存在しない | エラー、reconcile 中断 | — |
| state.json が破損 | `state.json.bak` から復旧、無ければ操作中断・ユーザに通知 | — |

### 7.3 orphan の扱い（明示）

| 状況 | 扱い |
|---|---|
| ghostty 窓 title が `(ai\|shell)-N:*` パターンに一致するが state に対応 project 無し | **触らない**。`projwm reconcile --verbose` で INFO（warning ではない、無所属 project の存在は first-class） |
| Zed window (bundleId `dev.zed.Zed`) の title が state の project basename に一致しない | **触らない**。手動で開いた Zed window は projwm 管理外として尊重 |
| project slot WS 内に title 規約外のウィンドウ（例: 手動で送った Chrome） | **触らない**。ユーザの意図物として尊重 |
| tmux サーバに `(ai\|shell)-N/*` 系セッションが残っているが state に無い | **触らない**。`projwm restore` で取り込みを促す |
| ghostty 窓が title 規約に一致するが規約外 WS にいる（例: ai-1:dotfiles が WS B にいる） | **state にあれば正しい slot へ修正、無ければ触らない** |

`--gc` オプション付きで実行した時のみ orphan を一括 close（明示的・破壊的、ユーザ判断）。

### 7.4 起動トリガ

| トリガ | 用途 | 実装 |
|---|---|---|
| 手動 `projwm reconcile` | ユーザが明示修正 | CLI |
| OmniWM `windows-changed` イベント | ウィンドウ閉じ等の即応 | launchd `omniwmctl watch windows-changed -- projwm-reconcile-debounced` |
| OmniWM `display-changed` イベント | モニタ抜き差し対応 | 同上、別チャネル |
| 60s 定期 | watch 取りこぼし backstop | launchd timer |
| `projwm up` / `down` 等の最後 | コマンド完了直後の整合確認 | CLI 内部呼び出し |

### 7.5 debounce 戦略

`omniwmctl watch` は短時間に大量イベントを発火し得る。reconcile 駆動では 500ms の debounce を挟む（`projwm-reconcile-debounced` ラッパスクリプトで実装）。

### 7.6 多 AI 並走時の挙動

| 項目 | 挙動 |
|---|---|
| 同 slot に複数 AI 窓 | Niri が自動タイル。1 column 4 窓まで縦スタック、5 窓目以降は **新しい column が右に生え**、`alt+h/l` で巡回。事実上 **1 slot WS の窓数は無制限**。AI 3, 4, ... N 個が並んでも破綻しない |
| viewer (WS A) | 各 AI 窓に **1 対 1 で対応する viewer 窓** が並ぶ。AI 3 つの project なら viewer に 3 タイル |
| プロファイル切替 | inactive 化 → 全 AI 窓 + 全 viewer 窓を close、tmux は全 AI 分 alive |
| アーカイブ | 全 AI セッション + 全 viewer clone を kill |
| `add-ai --ai claude` | **現存最大 id + 1** で採番（最初の `add-ai` でも `ai-2` から、`up` で作られた `ai-1` が居るため）。tmux session 作成、viewer clone も自動作成 |
| 連番の永続性 | `ai-2` を停止してから `add-ai` した場合、新規は `ai-3` ではなく **現存最大+1** = `ai-N+1`（`ai-2` の番号を再利用しない）。これは tmux session 名・viewer title の同一性を維持し、reconnect 時の混乱を防ぐため |

---

## レイヤー 8 — ユーザインタフェース層

### 8.1 キーバインド完全一覧

| キー | 動作 | 実装 |
|---|---|---|
| `alt+1..9` | 日常 WS jump | OmniWM hotkeys（既存） |
| `alt+shift+1..9` | 日常 WS へ窓を送る | OmniWM hotkeys（既存） |
| `alt+m / alt+b` | 既存 M/B（手付かず） | OmniWM hotkeys |
| `alt+shift+m / alt+shift+b` | 同上 move | Karabiner（既存 `karabiner-rules.nix`） |
| `alt+ctrl+h/j/k/l` | focus-monitor 方向（既存） | Karabiner（既存） |
| `alt+ctrl+m` | setup-media（既存） | Karabiner（既存） |
| `alt+s/c/a` | アプリ起動マクロ（既存） | Karabiner（既存） |
| `alt+a` | viewer WS A jump | OmniWM hotkeys 新規 |
| `alt+shift+a` | viewer WS A へ窓を送る | OmniWM hotkeys 新規 |
| **`alt+q/w/e/r/t/y/u/i/o/p`** | **AI slot jump** | **OmniWM hotkeys 新規（10 本）** |
| **`alt+shift+q/w/e/r/t/y/u/i/o/p`** | **AI slot へ窓を送る** | **OmniWM hotkeys 新規（10 本）** |
| `alt+space` | **OmniWM Quake = launcher**（既存トリガを launcher に転用） | OmniWM 内蔵 |
| `alt+\`` | **ghostty quick-term = scratch**（新規有効化） | ghostty config |

### 8.2 launcher（bubbletea TUI）

#### 8.2.1 起動経路

`alt+space` → OmniWM Quake terminal が出現 → 内部で `projwm` バイナリが TUI モードで起動。

#### 8.2.2 画面構成

```
┌─ projwm cockpit ────────────────────────────────────────────────┐
│  profile: ● work    ○ personal    ○ default   |   archive: 3   │
│                                                                 │
│  > _                                                            │
│                                                                 │
│   active slots (profile=work)                                   │
│    [Q] dotfiles                                                 │
│         ai-1     claude    tmux●  win●                          │
│         ai-2     copilot   tmux●  win●                          │
│         shell-1            tmux●  win●                          │
│         editor                    win●                          │
│    [W] manaflow                                                 │
│         ai-1     copilot   tmux●  win●                          │
│         shell-1            tmux●  win●                          │
│         editor                    win●                          │
│    [E] client-x                                                 │
│         ai-1     claude    tmux●  win✗                          │
│         shell-1            tmux●  win●                          │
│         editor                    win●                          │
│   viewer                                                        │
│    [A] 4 ai streams (read-only): dotfiles ai-1, dotfiles ai-2,  │
│        manaflow ai-1, client-x ai-1                             │
│   empty slots                                                   │
│    [R] [T] [Y] [U] [I] [O] [P]                                  │
│                                                                 │
│   inactive (in other profiles)                                  │
│    blog          1 ai (claude)    tmux● (profile=personal)      │
│    side-app      1 ai (claude)    tmux● (profile=personal)      │
│    shared-x      2 ai (claude×2)  tmux● (profiles=work+personal)│ ← 複数所属
│                                                                 │
│                                                                 │
│   parked (no profile, tmux alive)                               │
│    spike-x       2 ai (claude×2)  tmux●                         │
│                                                                 │
│   archived (3)                                                  │
│    old-thing  /  abandoned-poc  /  spike-2025                   │
│                                                                 │
│  ───────────────────────────────────────────────────────────     │
│   ↵ jump   n new   d down   p profile   a archive   r reconcile │
│   tab cycle profile                                             │
└─────────────────────────────────────────────────────────────────┘
```

#### 8.2.3 操作

| キー | 動作 |
|---|---|
| 文字入力 | fzf 風の絞り込み（全 project + プロファイル名 + slot 名にマッチ） |
| `↑` / `↓` / `Ctrl-J` / `Ctrl-K` | 行移動 |
| `Enter` | 選択行のアクション実行（jump / unarchive / activate profile 等、コンテキスト依存） |
| `Tab` | active profile を循環切替（work → personal → default → ...） |
| `p` | プロファイル選択ダイアログ |
| `n` | 新規 project 作成（cwd 入力プロンプト + 配属 profile + slot を順次選択） |
| `d` | 選択 project を active profile から外す（park 状態へ。archived にはしない） |
| `a` | 選択 project を archive |
| `u` | 選択 archived を unarchive（profile + slot を要求） |
| `m` | 選択 project を別 profile / 別 slot に move |
| `r` | reconcile 実行 |
| `s` | status 詳細表示 |
| `q` / `Esc` | 終了（Quake が hide） |

#### 8.2.4 複数 profile に所属する project の表示

同一 project が active profile **以外の** 2 つ以上の profile に居る場合、`inactive` セクションに 1 行で集約表示し、`profiles=...+...` 形式で全所属プロファイルを列挙する（前章の `shared-x` 行）。

active profile に居る場合は通常通り `active slots` セクションに展開表示し、他 profile への所属は **note 行** として project エントリの下に小さく付記：

```
[Q] dotfiles                                           (also in: personal)
     ai-1     claude    tmux●  win●
     ...
```

これにより「この project は work から外しても personal 経由でアクセスできる」が一目でわかる。

#### 8.2.5 リアルタイム更新

bubbletea の `tea.Cmd` で state.json を fsnotify 監視し、外部 reconcile 実行時に画面が即更新される。

### 8.3 Quake と quick-terminal の役割分担

| | OmniWM Quake | ghostty quick-terminal |
|---|---|---|
| トリガ | `alt+space` | `alt+\`` |
| 起動コマンド | `projwm`（TUI 直起動） | `fish -l`（標準シェル） |
| 用途 | **launcher 専用**（cockpit） | **スクラッチ**（ad-hoc） |
| 永続性 | OmniWM 管理、選択後 auto-hide | ghostty 管理、手動で閉じる |
| OmniWM 連携 | `windows` query で見える / `command palette` 連携可 | OmniWM 管理外（フロート overlay） |

#### 8.3.1 OmniWM Quake の起動コマンド設定

`[quakeTerminal]` セクションに `command` フィールドが存在するかは POC で要検証。3 段階フォールバック：

| 優先度 | 方法 | 条件 |
|---|---|---|
| a | `quakeTerminal.command = "projwm"` | OmniWM がフィールド対応 |
| b | fish 起動 hook で env 検知し `exec projwm` | OmniWM が env を立てる |
| c | Karabiner で `toggle-quake-terminal` 後にキー注入 | 上記いずれも不可 |

c に倒れる場合、Quake の汎用性が失われるので **ロール反転（A 案: Quake = scratch / ghostty quick = launcher）** に切り替える。

### 8.4 CLI

```
projwm [global flags] <subcommand>

project lifecycle:
  up --ai <claude|copilot> [--cwd <path>] [--profile <name>] [--slot <X>]
     [--as <name>] [--no-editor]
                                            cwd を project として登録、active profile（または指定 profile）の
                                            空き slot に割当。**ai-1 + shell-1 + editor-1（Zed）を既定起動**。
                                            --ai は **必須**（暗黙のデフォルトを持たない）。
                                            --as 指定で project 名を basename と切り離せる
                                            （basename 衝突時の回避策、内部名のみ変更）。
                                            --no-editor で Zed の起動を抑止。
                                            basename uniqueness を validate（衝突時は --as を促す）
  jump <slot|name>                           slot 名 (Q/W/...) or project 名 or profile 名で jump
  add-ai --ai <claude|copilot>               カレント slot に追加 AI 窓（id は最大+1）
                                            --ai は明示必須。引数省略時は既存 AI 群から推論
                                            （複数あれば fzf で選択）
  add-shell                                  カレント slot に追加 shell 窓（tmux ラップ）
  add-editor                                 カレント slot に追加 editor 窓（Zed、id 最大+1）
                                            既に id=1 が居るなら id=2 で起動（実用上稀）
  remove --window <kind-N> [--project <p>]   特定の窓を 1 つ閉じる（state からも削除）。
                                            tmux session も kill（editor は GUI close のみ）。
                                            例: remove --window ai-2 / remove --window editor-1

profile management:
  profile list                               全プロファイル一覧
  profile show [<name>]                      指定（無引数=active）プロファイル詳細
  profile create <name> [--description=...]  新規プロファイル
  profile delete <name>                      削除（active は拒否）
  profile switch <name>                      active プロファイル切替（高速、tmux kill 無し）
  profile assign <slot> <project>            active プロファイルに割当追加
  profile unassign <slot|project>            active プロファイルから外す
  profile rename <old> <new>                 改名。**`active_profile` が `<old>` を指していれば自動で `<new>` に追従**。
                                            `assignments` の値（project 名）は影響なし。
                                            `<new>` が既存プロファイル名と衝突したら拒否

archive management:
  archive <project>                          tmux kill、windows close、state は保全
  unarchive <project> --profile <name> --slot <X>
                                             archived を解除、指定の場所に復活
  archive list                               archived な project 一覧
  archive purge <project>                    state からも完全削除（unrecoverable、--yes 必須）

state & 整合性:
  reconcile [--dry-run] [--verbose] [--gc]   差分修正
  status [--json]                            active profile の全 slot 整合性表示
                                            （project ごとに **全 windows を kind/id 別に展開**して列挙、
                                             ai-1 / ai-2 / shell-1 / editor を個別表示）
                                            出力例（テキスト形式）：

                                            profile: work    archive: 3    parked: 1
                                            ───────────────────────────────────────────
                                            [Q] dotfiles    /Users/yuta/dev/dotfiles
                                                 ai-1     claude    tmux ✓  window ✓
                                                 ai-2     copilot   tmux ✓  window ✓
                                                 shell-1            tmux ✓  window ✓
                                                 editor             —       window ✓
                                            [W] manaflow    /Users/yuta/work/manaflow
                                                 ai-1     copilot   tmux ✓  window ✗  ← 修正対象
                                                 shell-1            tmux ✓  window ✓
                                                 editor             —       window ✓
                                            [E] (empty)
                                            ───────────────────────────────────────────
                                            viewer (A): 3 streams: dotfiles ai-1, dotfiles ai-2,
                                                                   manaflow ai-1
                                            ───────────────────────────────────────────
                                            inactive: blog (personal), side-app (personal)
                                            parked:   spike-x
                                            archived: old-thing, abandoned-poc, spike-2025

                                            --json 指定時は構造化 JSON
  restore                                    tmux サーバ走査 → state 再構築
  state {show|edit|repair|migrate}           state.json 直接操作
  logs [--tail N]                            実行ログ表示
  doctor                                     環境健全性チェック

global flags:
  --state-dir <path>   既定 ~/.local/state/projwm
  --no-reconcile       コマンド後の自動 reconcile を抑止
  --profile <name>     コマンドを active 以外の profile に対して実行（一部のみ対応）
```

#### 8.4.1 冪等性保証

| コマンド | 冪等動作 |
|---|---|
| `up` | 既に active profile に cwd が登録 → focus only（再 spawn しない） |
| `up` | slot あり、窓欠損あり → reconcile 経由で穴埋め |
| `jump` | 既にその WS にいる → no-op |
| `reconcile` | 何度実行しても結果が同じ |
| `profile switch` | 既に active → no-op |
| `profile unassign` | 既に外れている → no-op |
| `archive` | 既に archived → no-op |
| `unarchive` | 既に非 archived → no-op |
| `remove --window` | 該当窓が既に無ければ no-op |

---

## レイヤー 9 — 実装層

### 9.1 Nix モジュール構成

```
modules/darwin/projwm/
├── default.nix              # myConfig.darwin.projwm.enable で点く
├── omniwm-workspaces.nix    # WS A / Q-P を [[workspaces]] に追加
├── hotkeys.nix              # alt+letter / alt+shift+letter / alt+a を hotkeys に追加
├── ghostty.nix              # quick-terminal 設定の追加
├── karabiner.nix            # 既存ルール群の差分（cmux ルール削除）
├── launchd.nix              # reconcile-watcher と periodic
├── projwm/                  # Go ソース
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── cmd/                 # cobra subcommand 群
│   │   ├── root.go
│   │   ├── up.go
│   │   ├── down.go
│   │   ├── jump.go
│   │   ├── reconcile.go
│   │   ├── status.go
│   │   ├── restore.go
│   │   ├── add.go             # add-ai / add-shell / add-editor
│   │   ├── state.go
│   │   ├── doctor.go
│   │   └── tui.go           # 引数なし起動時に TUI へ
│   ├── internal/
│   │   ├── state/           # JSON 読み書き、flock、migrate
│   │   ├── config/          # config.toml 読み込み
│   │   ├── reconcile/       # 差分検出と修正
│   │   ├── omniwm/          # omniwmctl ラッパ
│   │   ├── tmux/            # tmux ラッパ
│   │   ├── ghostty/         # ghostty 起動ラッパ
│   │   ├── zed/             # zed CLI 起動 + bundleId/title での識別
│   │   └── tui/             # bubbletea アプリ
│   └── README.md
├── scripts/
│   ├── reconcile-watcher.sh # omniwmctl watch → debounced projwm reconcile
│   └── projwm-reconcile-debounced
└── README.md
```

### 9.2 Go プロジェクト方針

- Go 1.22+ / `cobra` (CLI) / `bubbletea` + `bubbles` + `lipgloss` (TUI) / `fsnotify` (state watch) / `gofrs/flock` (排他)
- `omniwmctl` / `tmux` / `ghostty` は外部 binary。`os/exec` で薄くラップ
- 全主要関数を表テストでカバー（reconcile の差分テーブル特に）

### 9.3 ビルド・配布

- `pkgs.buildGoModule` で `projwm` バイナリを生成
- nix-darwin の `environment.systemPackages` に追加
- バージョン情報は `-ldflags "-X main.version=$(git describe)"` で埋め込む

### 9.4 既存モジュールの変更

| ファイル | 変更内容 |
|---|---|
| `modules/darwin/cmux.nix` | **削除** |
| `modules/common/zellij.nix` | **削除** |
| `modules/darwin/omniwm/common.nix` | `quakeTerminal.command` 追加（POC 結果次第）、`workspaces` に A/Q-P 追加 |
| `modules/darwin/omniwm/hotkeys.nix` | alt+letter / alt+shift+letter / alt+a 系 21 本追加 |
| `modules/darwin/omniwm/karabiner-rules.nix` | **`alt+a` Calendar 起動マクロを削除**（viewer に振替） |
| `modules/darwin/omniwm/workspace-assignment.nix` | **`dev.zed.Zed → "12"` のエントリを削除**。Zed routing は projwm が per-project に行う |
| `modules/darwin/cmux.nix` 由来の Karabiner ルール | 削除（cmux.nix と一緒に消える） |
| `modules/darwin/ghostty.nix` | quick-terminal キーバインド `alt+\`` 追加 |
| `profiles/darwin.nix` | `myConfig.darwin.projwm.enable = true` 追加 |
| `CMUX.md` / `CMUX-WORKFLOW.md` | **削除** |
| `OMNIWM.md` | projwm 統合の節を追記 |
| 新規 `PROJWM.md` | エンドユーザ向け取説（本設計書とは別物、簡潔版） |

---

## レイヤー 10 — 移行・撤去計画

### 10.1 既存ユーザ資産

- 既存の zellij セッション → 移行スクリプト `projwm-migrate-from-cmux` で tmux に移し替える、または捨てて作り直す（推奨）
- 既存 cmux workspace → 全廃。再 attach は `projwm restore` か `projwm up` で再構築

### 10.2 撤去手順

1. 新ブランチ `migration/projwm` を切る
2. Phase 0〜5 までを各 PR で main にマージ（cmux はまだ生きている）
3. Phase 6 PR で：
   - `myConfig.darwin.cmux.enable = false`
   - `myConfig.zellij.enable = false`
   - `cmux.nix` `zellij.nix` の物理削除
   - 旧ドキュメント削除、新 PROJWM.md 投入
4. nix-darwin `darwin-rebuild switch` 後、cmux app は `brew uninstall --cask cmux` で物理削除（オプション）

### 10.3 ロールバック

- 全変更は git 管理下なので `git revert` で復帰可能
- state.json は完全独立、cmux 復帰時に問題なし
- tmux サーバは projwm 廃止後も残るので再 attach 可能

---

## レイヤー 11 — Phase 計画

| Phase | 内容 | 工数 | 完了条件 |
|---|---|---|---|
| **0** | POC（致命的穴の検証） | 半日〜1 日 | 後述 11.1 の全項目クリア |
| **1** | Go binary 骨格 + state / config パッケージ（profile/archive/park 含む） | 2 日 | `projwm state show` `profile list` `archive list` が動く、Nix で build できる、config.toml 読み込み動作 |
| **2** | reconcile 実装（profile-aware） | 1.5 日 | active/inactive/archived 全パターンの単体テストが green、`--dry-run` 動作 |
| **3** | profile / archive コマンド | 1 日 | `switch` `archive` `unarchive` が冪等動作、windows 操作のみで切替 |
| **4** | TUI（bubbletea） | 1.5 日 | launcher が profile 切替・archive 操作・新規作成全てを 1 画面で完結 |
| **5** | Karabiner + OmniWM hotkeys + ghostty config | 半日 | alt+letter で全 slot に jump、scratch / launcher 起動分離、alt+a 振替 |
| **6** | 自動 reconcile 常駐 | 半日 | launchd 2 系統が稼働、windows-changed で 1 秒以内に追従 |
| **7** | 撤去 + 移行ドキュメント | 1 日 | cmux/zellij が消え、`darwin-rebuild` 通る、PROJWM.md 完成 |

合計約 8 日。各 Phase 独立 PR、Phase 0 通過後に Phase 1〜7 を並走可能。

### 11.1 Phase 0 で検証する open questions（致命傷チェック）

| ID | 検証項目 | 失敗時の影響 | 失敗時の対処 |
|---|---|---|---|
| POC-01 | ghostty `--title=` が tmux `set-titles off` で踏まれず保持される | title-based routing 全壊 | ghostty 設定で title 上書き禁止オプション探す、無ければ ghostty fork 検討 |
| POC-02 | `omniwmctl query windows --json` で title 一致 ID が安定取得 | reconcile 全壊 | bundleId + 起動順 / pid 等の代替識別 |
| POC-03 | `omniwmctl command move-to-workspace` が新規 ghostty に効く（タイミング含む） | slot 配置できない | spawn 後 polling + retry |
| POC-04 | `omniwmctl move-to-workspace` の冪等性（同 ID 反復で副作用無し） | reconcile が破壊的 | スクリプト側で「既に在 WS」を事前 check |
| POC-05 | tmux grouped session で本体／viewer 同時アタッチ画面崩れず | viewer 全壊 | `window-size latest` の効き確認、不可なら read-only 別 client 戦略 |
| POC-06 | `omniwmctl watch windows-changed` が実機で安定発火 | 自動 reconcile 全壊 | `subscribe` の `--json` 解析直書き、または 60s 定期だけで運用 |
| POC-07 | OmniWM Quake terminal が `command` 設定または env で外部バイナリを直接起動できる | launcher 起動経路の見直し | A 案（Quake = scratch / ghostty quick = launcher）にロール反転 |
| POC-08 | ghostty quick-terminal の有効化が OmniWM Quake と共存（キー競合なし、両方 toggle 可能） | scratch 起動できない | ghostty 純正側を諦め、Karabiner で別 ghostty 起動に切り替え |
| POC-09 | Karabiner で `frontmost_application_unless` を使った除外なしで `alt+letter` 群が全アプリで機能 | キーバインド衝突 | 必要アプリのみ条件除外 |
| POC-10 | `buildGoModule` 内で bubbletea アプリがビルドできる、xterm-256color で動く | TUI 採用見直し | charm の Wish（SSH-tui ベース）等の代替検討 |
| POC-11 | プロファイル切替時、ghostty 窓 close → 別 project 窓 spawn が見た目スムーズか（フリッカー測定） | 切替体験悪化 | アニメーション無効化、または「窓を hide / show する」OmniWM 機能の活用検討 |
| POC-12 | tmux session を kill せずウィンドウ close した場合、再 attach で表示状態（カーソル位置・色 buffer）が保たれるか | 切替後 AI 表示崩れ | tmux の `set-option default-terminal screen-256color` 等の調整 |
| POC-13 | tmux session 名に **`/` と `:` を含めて** 動作する（`ai-1/<proj>` / `ai-1/<proj>_v`） | 命名規約全壊 | 区切り文字を `__` 等に変える設計に倒す |
| POC-14 | ghostty title 規約 `<kind>-<id>:<project>` の長さ上限 | OmniWM 側で title が truncate されると識別不能 | プロジェクト名短縮 / ハッシュ化等の fallback、要件側でユーザに「短い project 名」を促す |
| **POC-15** | `zed <path>` 起動時の **window title が cwd basename で安定**（編集前後・dirty 状態でも変わらない） | editor 識別不能 | symlink トリック（一意名のシンボリックリンクを作って `zed` に渡す）／別 ID 戦略 |
| **POC-16** | `omniwmctl query windows --bundle-id dev.zed.Zed` で **個別 Zed window が title 込みで列挙**できる | identify 不能 | OmniWM 側で `axRole` 等の代替 matcher を試す |
| **POC-17** | 起動済み Zed がある状態で `zed <別 path>` を打つと **新しいウィンドウが立つ**（既存に上書きや tab 化しない） | project 切替が壊れる | Zed CLI flags 確認（`--new` 等の有無）／AppleScript で強制別窓 |
| **POC-18** | Zed window を `omniwmctl close-window` で閉じた時、**未保存変更があると保存ダイアログが出る**かどうか／profile 切替が止まらないか | 自動 reconcile が止まる | profile 切替前に "Zed が dirty なら手動保存を促す" UX、または force-close |
| **POC-19** | Zed の **session restore**（再起動時に開いていたファイル・カーソル復元）が profile 切替後の reopen でも効く | 体験劣化 | Zed 設定で session restore 有効化を確認、不可なら「閉じない、hide だけ」戦略を検討 |
| **POC-20** | Zed 起動レイテンシが profile 切替に耐えられるか（< 500ms 目標） | UX が遅い | Zed を kill せず hide する戦略に切替（OmniWM の hide-window 機能要確認） |

---

## レイヤー 12 — 確定した決定一覧（再掲）

| # | 決定 | 確定 ver |
|---|---|---|
| 1 | cmux 完全廃止 | v3 |
| 2 | zellij → tmux | v3 |
| 3 | **ghostty 系の非 viewer 窓（ai / shell）は全て tmux 化**、各窓が個別 tmux session を持つ。**editor (Zed) は GUI app なので tmux ラップしない** | v11 |
| 4 | 1 project = 1 OmniWM 名前付き WS | v4 |
| 5 | AI Viewer = WS A、read-only grouped attach | v4 |
| 6 | 数値 WS 1〜9 / M / B は手付かず（**E は廃止**） | v7 |
| 7 | キーバインド：`alt+q/w/e/r/t/y/u/i/o/p`（top row 10 letters）で直接 jump | v7 |
| 8 | キーバインド：`alt+shift+letter` で窓を slot に送る | v7 |
| 9 | キーバインド：`alt+a` / `alt+shift+a` で viewer | v6 |
| 10 | `alt+space` = OmniWM Quake = projwm launcher（B 案） | v7 |
| 11 | `alt+\`` = ghostty quick-terminal = scratch（B 案） | v7 |
| 12 | launcher 実装 = Go + bubbletea 単一バイナリ `projwm` | v7 |
| 13 | state.json が source of truth、reconcile が一手に修正 | v4 |
| 14 | reconcile orphan ポリシー：触らない（`--gc` で明示一括 close） | v4 |
| 15 | 動的 appRule は使わない、ブラウザは手動運用 | v4 |
| 16 | reconcile 駆動：`omniwmctl watch` + 60s 定期 + コマンド末尾 | v4 |
| 17 | state file 排他：flock + atomic rename | v4 |
| 18 | ghostty title 規約：`<kind>-<id>:<project>` | v9 |
| 19 | プロファイル機構：state.json で `profiles` を導入、active/inactive を区別 | v8 |
| 20 | アーカイブ機構：project ごとの `archived` フラグ。tmux kill + windows close、state は保全 | v8 |
| 21 | プロファイル切替は **windows 操作のみ**で完結、tmux session kill/start は伴わない | v8 |
| 22 | viewer (WS A) は active profile の project だけ表示、profile 切替で viewer も入れ替わる | v8 |
| 23 | 既存 `alt+a` Calendar 起動マクロは廃止、viewer (WS A) jump に振替 | v8 |
| 24 | 既定プロファイルは持たない、初期 state は **空のプロファイル群**。最初の `up` でユーザが命名・作成 | v10 |
| 25 | 同時 active なプロファイルは 1 つだけ（オーバーレイ非対応） | v8 |
| 26 | **多 AI / project は first-class**。各 AI 窓は `(kind="ai", id=N)` で識別、`ai` と viewer を個別に持つ | v8 |
| 27 | viewer (WS A) は **AI 窓単位**で複製。1 project に AI が N 個あれば viewer に N タイルが並ぶ | v8 |
| 28 | 全 kind が連番 id（`ai-1`, `shell-1`, `editor-1`, ...）。down で穴が空いても再利用しない | v9 |
| 29 | **schema に primary / default / main の概念を持たない**。`default_ai` フィールドは廃止 | v9 |
| 30 | **title / tmux session 名は state に保存せず、`(kind, id, project)` から projwm が算出** | v9 |
| 31 | `projwm up` の `--ai` は **必須**。暗黙のデフォルトを持たない（TUI 経由は選択ダイアログ） | v9 |
| 32 | **state.json はバージョニングを持たない**（pre-launch、YAGNI）。schema 進化が必要になった時点で `version` フィールドを足す | v10 |
| 33 | **state と config の分離**：state.json は runtime のみ、`~/.config/projwm/config.toml` に固定値（slot 名、viewer WS 名） | v10 |
| 34 | **`down` コマンド廃止**。「profile から外す」は `profile unassign`、「完全停止」は `archive` で代替 | v10 |
| 35 | **`term` kind 廃止**。tmux ラップで `shell` と機能重複 | v10 |
| 36 | **無所属 project (park) を first-class** とする（warning 出さず、launcher で別エリア表示）。`down` の代替経路 | v10 |
| 37 | プロファイル切替時、`moving_projects` は **project の全 windows を一括で新 slot に move**（half-moved 状態を作らない） | v10 |
| 38 | **Zed を first-class エディタとして導入**。kind に `editor` を追加、`nvim` kind は廃止 | v11 |
| 39 | `editor` は GUI app（Zed、bundleId `dev.zed.Zed`）。**tmux ラップしない**、bundleId + cwd basename で identify | v11 |
| 40 | `up` の既定起動セットは **ai-1 + shell-1 + editor-1**。`--no-editor` で editor 抑止可 | v11 |
| 41 | active 全 project（archived 除く）の **cwd basename は一意**。`up` 時に validate、衝突は `--as <name>` で内部名分離 or rename 要求 | v11 |
| 42 | OmniWM 既存の `dev.zed.Zed → WS E` 一律 appRule は **削除**。Zed routing は projwm が per-project に実施 | v11 |
| 43 | nvim を使いたいユーザは shell-N の tmux 内で起動（kind としては持たない） | v11 |

---

## レイヤー 13 — 参照

### 13.1 リポジトリ内ドキュメント

| 文書 | 引き継ぎ時の役割 |
|---|---|
| `OMNIWM.md`（必読） | **本書と並ぶ一次資料**。OmniWM の機能・hotkeys・IPC・appRule の境界を網羅。本書の前提知識 |
| `CMUX.md` / `CMUX-WORKFLOW.md` | **撤去対象**。移行前の現状記録、Phase 7 で削除予定。読まなくて良い |
| `AGENTS.md` | エージェント（Claude / Copilot CLI 等）運用方針。projwm の AI 種別選定の参考 |
| 既存 nix モジュール群 (`modules/darwin/`) | 実装時に触る対象。本書 §9.4 が変更箇所をリストアップ |

### 13.2 外部参照

- OmniWM IPC: <https://github.com/BarutSRB/OmniWM/blob/main/docs/IPC-CLI.md>
- bubbletea: <https://github.com/charmbracelet/bubbletea>
- bubbles: <https://github.com/charmbracelet/bubbles>
- lipgloss: <https://github.com/charmbracelet/lipgloss>
- cobra: <https://github.com/spf13/cobra>
- fsnotify: <https://github.com/fsnotify/fsnotify>
- gofrs/flock: <https://github.com/gofrs/flock>
- ghostty: <https://ghostty.org>
- tmux grouped sessions: `man tmux`「GROUPED SESSIONS」節

---

## 付録 A — 既存配置の差分要約（実装前イメージ）

```diff
- modules/darwin/cmux.nix
- modules/common/zellij.nix
+ modules/darwin/projwm/
+   default.nix
+   omniwm-workspaces.nix
+   hotkeys.nix
+   ghostty.nix
+   launchd.nix
+   projwm/                  (Go module)
+   scripts/
+   README.md

  modules/darwin/omniwm/common.nix          # workspaces / quakeTerminal 設定追加
  modules/darwin/omniwm/hotkeys.nix         # alt+letter 系追加
  modules/darwin/ghostty.nix                # quick-terminal 設定追加

- CMUX.md
- CMUX-WORKFLOW.md
+ PROJWM.md
  OMNIWM.md                                  # projwm 連携節を追記
```

---

## 付録 B — Open issues（POC 後に判断する）

| ID | 議題 | 候補 |
|---|---|---|
| OI-01 | viewer の grouped session を `attach -r`（read-only）にするか、通常 attach にするか | 推奨：`-r`（誤入力防止）、ただし操作したい場合は通常 attach も並列に対応 |
| OI-02 | maxVisibleColumns / defaultColumnWidth を AI slot 専用にチューニングするか | project slot で AI 単独表示前提なら 1 column フル幅推奨 |
<!-- OI-03 (multi-AI tmux naming): closed in v9 → ai-N/<proj> 採用、コロンは grouped clone のみ -->
| OI-04 | state.json の version 移行ロジックの ergonomics | `projwm state migrate` を独立コマンド化 |
| OI-05 | 多モニタプロファイル時の slot WS の monitorAssignment 挙動 | profile ごとに override セクション追加するか、main 固定で運用するか |
| OI-06 | viewer の grid 自動配置（Niri 列タイル任せか、initial sizing を強制するか） | デフォルトの Niri に任せて十分か POC で観察 |
| OI-07 | プロファイル切替に専用キーバインドを与えるか（例：`alt+1..9` を「日常 WS」と「プロファイル切替」で動的切替するなど） | MVP は launcher 経由のみ、運用後に判断 |
| OI-08 | inactive profile の tmux session の長時間生存ポリシー（例：N 日触られなければ自動 archive） | MVP では自動 archive しない、手動 only |
| OI-09 | プロファイル切替時の cwd 維持（同 project が複数 profile に居る場合、別 profile では別 worktree を指したい等） | project は cwd を 1 つだけ持つ前提に固定。複数 worktree なら別 project として登録する設計 |
<!-- OI-10 (default_ai per profile): closed in v9 → default_ai 概念を全廃。将来必要なら config.toml 拡張で対応 -->
| OI-11 | 多 AI 並走時の viewer タイル順序（追加順？AI 種別グルーピング？） | Niri の Niri 列順（生成順）に任せ、必要なら `move-column` で並び替え。専用 sort 機能は出さない |
| OI-12 | 同 slot 内に窓が増えすぎたとき（例 AI 5 個 + shell + editor）の視認性 | 1 slot に列何個でも生やせるので機能上は破綻しないが、視認性は落ちる。判断: **MVP は何も制限しない**（Niri に任せる）。運用後に「同一 project を 2 slot に分ける」「`maxVisibleColumns` を slot ごとに override」等を判断 |
| OI-13 | tmux session 名に `/` `:` を含めることが安全か（POC で要検証、§5.1 参照） | tmux target 構文と衝突する可能性。POC で `ai-1/<proj>_v` 形式が確実に使えることを確認。問題があれば区切り文字を `__` 等に変更 |
| OI-14 | parked project の長期生存ポリシー | MVP では park 状態を無期限に保つ。ユーザが明示的に archive するまで tmux session は alive のまま。運用後に「N 日触られなければ通知」等を検討 |
| OI-15 | 多 editor 並走時の挙動（同 project に Zed window 2 つ等） | Zed は同じ folder を再度開くと既存 window を focus する仕様の可能性。`add-editor` が "新 window" にならない場合は MVP では id=1 のみ運用、将来 Zed の split workspace 機能等で代替検討 |
| OI-16 | 他の GUI editor（VSCode / Cursor / IntelliJ 等）への汎用化 | MVP は Zed 決め打ち。需要が出たら config.toml に `editor.command` `editor.bundleId` `editor.title_pattern` を持たせて差替可能に拡張 |
| OI-17 | basename 衝突時の `--as <name>` 運用 | 内部名は projwm 側で別、Zed window title は basename のまま。**衝突時はそもそも 2 つの project を同時 active にしない**運用が現実解。`--as` は park 中に内部名だけ変えたい時の補助 |

---

## 付録 C — 引き継ぎチェックリスト

新メンバーが「読んだ → 着手準備完了」と言える時点で、以下が全部 yes になっているはず：

### 理解の確認

- [ ] `OMNIWM.md` の hotkeys / IPC / appRule の節を読んだ
- [ ] 本書 §0〜§2（動機・用語・要件）を読んだ
- [ ] 本書 §12 確定決定一覧を 1 通り読んで、なぜそうなっているかを質問できる
- [ ] cmux + zellij が完全廃止されることを把握した
- [ ] 「1 project = 1 名前付き WS」「viewer は WS A」「`(kind, id)` で window 識別」を空で説明できる
- [ ] profile / archive / park の 3 状態の違いを言える
- [ ] state.json と config.toml の責務分離を理解した

### 着手前の手元確認

- [ ] `OMNIWM.md` 記載のバージョン（pre-1.0）と手元 OmniWM のバージョンを確認した
- [ ] `omniwmctl` IPC が手元で有効化されている（`general.ipcEnabled = true`）
- [ ] Karabiner-Elements が手元にある
- [ ] Go 1.22+ / `pkgs.buildGoModule` が手元の Nix で通る
- [ ] tmux が `set-titles off` 動作可能（POC-01 用）
- [ ] Zed (`dev.zed.Zed`) が手元にあり、`zed <path>` CLI が通る

### Phase 0 開始準備

- [ ] §11.1 の POC-01〜20 をテスト計画として印刷／メモした
- [ ] POC は 1 project（例: 自分の dotfiles）で進めることを了解した
- [ ] POC で詰まったら **設計を見直す（Open Issue 追加）** スタンスで、無理に押し通さないことを了解した

### 開始してから

- [ ] 各 Phase は **独立 PR** として出す
- [ ] 設計外の判断が必要になったら **付録 B Open Issues に追記**してから決める
- [ ] 文書のバージョンを更新するときは **§ステータスとガイド** の版数表に履歴を残す

---

_End of document._
