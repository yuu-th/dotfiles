# projwm-next Cockpit & CLI 統合要件 v2.9

> **要件のみ**を記述する。実装方針・コード設計・Phase 計画は別ドキュメント。  
> UI 仕様は**機能と情報**で記述し、**視覚レイアウト（配色・配置・装飾）は記述しない**。実装側に委ねる。

---

## 0. 改版履歴と変更点

- **v1**: 初版
- **v2**: macOS notification 廃止 / Hyper modifier を karabiner variable 方式に / Tier 4 強制復元追加 / Ghostty Cmd+N 挙動明示 / スコープ判断分離
- **v2.9**: §9 TUI 構造を全面再設計。Top tabs (`Slots / Cards / Archived / Profiles / Trace`) + main content + context-aware bottom menu の 3 層構造。発見性最優先、その時可能な action を bottom menu で全列挙。新規 project 作成は B2 フォーム wizard (defaults prefill、Tab field 切替、Enter で submit)。Command palette (`Ctrl-P`) で fuzzy search 全 action。`o` キー + `1-5` キーで tab/modal 行き来。要件 §10 カードモーダルは Cards tab に統合 (auto-pop on card-added は維持)。要件 §9.5 keybind を context-aware に再整理。
- **v2.8**: §8.9「自己修復契約」を新設。projwmd 起動時と 30 秒間隔の継続監視で omniwm health probe + recovery ladder (Lv1 omniwm-deploy 再 push / Lv2 個別 app quit+relaunch / Lv3 omniwm restart / Lv4 bootout+bootstrap) を実行し、AX permission 喪失等 macOS layer の問題を除く全崩壊パターンから projwmd 単独で復旧する。Lv3-Lv4 は副作用ありなので [OMNIWM-RECOVERY] warning card + 数秒猶予 + ユーザ Esc 可。§8.10「cockpit invariant」も新設 (cockpit window は常に ParkWorkspace=CP1 に居る、別 workspace 移動禁止、違反検出時は Lv1 omniwmctl で focus+move、Lv2 kill+respawn の自己修復 ladder で確実に矯正)。
- **v2.7**: §8.3 に「双方向同期」の項を新設。ユーザの手動 workspace 切替 (active=ParkWorkspace から離脱) は暗黙の `Visibility=Hidden` flip と等価、逆に active=ParkWorkspace に戻ったら `Visibility=Shown` と等価。これにより workspace 移動コマンド (space+1/q/m 等) が自動的に cockpit toggle 相当の意味を持ち、専用 toggle binding (space+f) と等価な体験になる。実装は reducer / controller の Tier-aware observed→desired 同期で達成 (Tier 2 column ordering の auto-overwrite と同パターン)。
- **v2.6**: §11.3 / §11.6 を「位置/focus/move 系 → space-leader、size/構造/UI 系 → option 維持」の体系に全面再編。OmniWM 全 144 hotkey の binding 割当てを spec table として明示。karabiner-rules.nix に focus hjkl/;, workspace next/prev/back-and-forth, monitor-next, focus-column 0-8/first/last, window move hjkl, window→ws up/down, column move h/l, column→ws 0-8/up/down の新規 space binding を追加。hotkeys.nix の位置/move 系を全 Unassigned 化し、balanceSizes → Option+/、toggleWorkspaceLayout → Option+L に変更。
- **v2.1**: `projwm restore` を不要として確定
- **v2.4**: §8 を「全モニタ同時表示」から「projwm-managed モニタに 1 つだけ」に縮退（UX 再検討の結果、cockpit は workspace A / Q-P が住むモニタに 1 つあれば十分と確認、他モニタには配置しない）/ §11.1 を `to_if_alone` 方式から `simultaneous + key_down_order=strict` 方式に変更（実機検証で `to_if_alone` の release-time emit が単独 space タップを体感的に遅延、また letter-first ロール打鍵時に space バッファ問題が残ることが判明）
- **v2.3**: §11.1 に keybind 出力方式の決定根拠を追記（hyper modifier emit / F-key 方式を不採用とした理由、shell_command 統一の妥当性が現状の `alt+letter` 実装と同一であることを確認）
- **v2.2**: 以下を反映
  - UI 仕様方針を「視覚記述しない、機能と情報を言葉で」に統一
  - TUI 内部 keybind, fzf-style 検索, doctor/dry-run 出力, profile switch 付随挙動を明示
  - Zed 別 path から `[i]gnore` 排除
  - 管理外 workspace 上の手動追加は Tier 1 発火しない
  - 管理外 → managed への window 移動を Tier 4 禁止行為に追加
  - Workspace E の管理内訂正
  - `WindowExternal` origin (b) 廃止、Ghostty/Vivaldi/Zed は External 化させない
  - cockpit は workspace 越え overlay と明示
  - karabiner `variable_if` パターンを明示、f13/f14 等の特殊キー不要を明記

---

## 1. 全体ビジョン

projwm-next に対して旧 projwm が持っていたユーザ向け CLI / TUI を再構築する。projwm-next 特有の概念（generation, epoch, lifecycle contract, private payload, manifest digest, inactive policy, invariants 等）もユーザに surface する。

レイヤー構成:

- **Layer 1**: `projwmctl-next` — 既存の低レベル IPC クライアント、intent 送信のみ。debug 用に残す（deprecate しない）
- **Layer 2**: `projwm` — 新規ユーザ向け CLI（旧 projwm 互換 + projwm-next 拡張）
- **Layer 3**: `projwm tui` — 常駐 TUI cockpit、projwm-managed モニタ (workspace A / Q-P が住む) に 1 つ常駐、提案表示と全操作

通常ユーザは Layer 2 と Layer 3 のみ使う。

---

## 2. 管理対象 workspace の定義

managed workspace と管理外 workspace を明確に分ける。

- **管理対象 workspace**（projwm の管理下、Tier 1〜4 が適用される）:
  - `A` — viewer workspace
  - `Q W E R T Y U I O P` — 10 個の project slot workspace
- **管理外 workspace**（projwm は一切干渉しない）:
  - `1`〜`9` — 通常作業用
  - `M` — Media
  - `B` — Browser
  - その他 OmniWM が定義するすべての非 managed workspace

実装側（`classifyLiveWindow`、planner、invariant 等）でも上記定義が一貫していることを Phase 1 序盤で検証する（§15 参照）。

---

## 3. 状態変更の 4-tier モデル

| Tier | 名称 | 例 | 挙動 |
|---|---|---|---|
| **1** | 構造変更 | managed workspace 上で新 shell/editor/browser window 手動追加 | **明示** — cockpit カード提案 |
| **2** | 配置変更（同一 workspace 内） | 同一 workspace 内の column 並び替え | **自動上書き** — DesiredWorld.Layouts 更新 |
| **3** | 内部状態 | Vivaldi タブ URL、タブ追加・並び替え | **自動 observe + persist** |
| **4** | 強制復元（禁止行為） | 認めない操作 | observed と desired が乖離 → 自動で desired へ revert + カード通知 |

### 3.1 Tier 1 発火条件

managed workspace 上で managed 候補 app（Ghostty, Vivaldi automation profile, Zed）の手動 window が出現した場合のみ発火する。

**Tier 1 を発火させない条件**:
- 管理外 workspace 上の window 出現（無関係なので干渉しない）
- spawn token と一致する window（projwm 自身の spawn）
- manifest に bundle ID が無いアプリ（自動的に origin (a) External 扱い、§6 参照）

### 3.2 Tier 4 該当操作（禁止行為、自動 revert）

以下はすべて確認なしに自動で desired 状態へ戻す:

- **Workspace 間 window 移動**: managed window が手動で別 workspace に移されたら、元の desired workspace へ強制 revert
- **管理外 → managed への window 移動**: 管理外 workspace で生まれた orphan window が managed workspace に手動移動された場合、元の管理外 workspace へ強制 revert（または close）
- **Managed → 管理外への window 移動**: 上記同様、元の managed workspace に強制 revert
- **Managed window のユーザ閉じ**: 自動で respawn（grace period 60s 内 2 回で復元停止）

すべての Tier 4 発火は cockpit カード（`[CLOSED]` / `[MOVED]`）で事後通知する。

### 3.3 旧仕様との差分

- 旧 `intent.AcceptManualLayout` は **廃止**（Tier 2 自動上書きに変更）
- `ManualLayoutCandidate` 構造体削除

---

## 4. アプリ別ルール

### 4.1 Ghostty

**基本管理**:
- title prefix `ai-N:` / `shell-N:` / `ai-view-N:` で managed と判定
- 各 managed window は対応する tmux session（`ai-N/proj`, `shell-N/proj`, `ai-N/proj_v`）を backing にもつ

**Cmd+N / Cmd+T で手動 Ghostty が managed workspace に出現したとき**:

projwm 経由で spawn した window は `-e tmux new-session -A -s ...` で起動するため tmux backing を持つ。Cmd+N はそれを経由しないので **adopt しても tmux session が無い不整合状態になる**。

→ Tier 1 提案カードで以下の選択肢を提示:

- **`[Enter]`**: close orphan + spawn proper（projwm 正規ルートで shell-N として再 spawn）
- **`[c]`**: close orphan
- **`[t]`**: cockpit TUI に carry over して詳細操作

`[i]gnore`（External 永続化）は **提供しない**（§6 origin (b) 廃止）。

### 4.2 Vivaldi

- **user profile (デフォルト profile)**: 完全に無視。`classifyLiveWindow` を修正し、argv の `--profile-directory` を判定して `projwm-next` 以外は origin (a) External 扱い（§6 参照）。
- **automation profile (`projwm-next`) の手動 window 追加**: Tier 1 提案カード発火、選択肢は `[c]lose` / `[Enter]` adopt / `[t]` TUI carry over の 3 択
- **automation profile のタブ追加 / URL 変更**: Tier 3 自動観測。PrivatePayloadStore に persist。ユーザ確認不要。
- **managed window の column 移動**: Tier 2 自動上書き
- **managed window のユーザ閉じ**: Tier 4 自動 respawn

### 4.3 Zed

- **基本管理**: bundle ID `dev.zed.Zed` + title が project path basename
- **同一 project に 2 つ目の Zed**: contract 違反、Tier 1 提案カード、選択肢は `[c]lose` / `[t]` TUI の 2 択（`[Enter]` のデフォルトは `[c]lose`）
- **別 path で Zed を開く**: Tier 1 提案カード、選択肢は `[Enter]` 新規 project として登録 + slot 選択 prompt / `[c]lose` / `[t]` TUI の 3 択。**`[i]gnore` は提供しない**。

### 4.4 「間違った操作」への能動的反応

ユーザが意図せず変な操作をしたとき、内部的に `External` として silent に黙殺しない。必ず cockpit カードで通知 + 操作提案を出す（Tier 1）または自動 revert する（Tier 4）。

### 4.5 Workspace × アプリ → 候補 project 推論ロジック

Tier 1 提案カードで「どの project の何として adopt するか」を推論するルール:

- **手動 Ghostty が managed workspace (slot X に project P が assign 済み) に出現**:
  - 候補: P の次空き番号で `shell-N:P` として adopt
- **手動 Ghostty が managed workspace (空き slot) に出現**:
  - 候補なし。カードは「どこに adopt するか? 候補 project 入力 prompt」を出す
- **手動 Vivaldi が automation profile で出現** (managed workspace 内):
  - workspace 不問。「どの project の browser として adopt するか?」prompt（候補は active profile の全 project）
- **手動 Zed が別 path で出現** (managed workspace 内):
  - cwd / project path から判定。`~/dev/foo` の path → project `foo` を新規作成提案

管理外 workspace 上の手動追加は §3.1 により Tier 1 を発火しないため、ここに該当ケースは存在しない。

---

## 5. CLI コマンドカタログ — `projwm`

### 5.1 診断 / 情報表示

- `projwm status [--json]`
- `projwm doctor`
- `projwm trace [--last | <txid>]`

### 5.2 プロジェクト操作

- `projwm up --ai <name> --slot <SLOT> [--cwd <PATH>] [--as <NAME>]`
- `projwm add-ai --ai <name> [--project <P>]`
- `projwm add-shell [--project <P>]`
- `projwm add-editor [--project <P>]`
- `projwm remove --window <KIND-N> [--project <P>]`

### 5.3 プロファイル操作

- `projwm profile create <NAME> [--description <TEXT>] [--inactive-policy remove|keep]`
- `projwm profile switch <NAME>`
- `projwm profile assign <SLOT> <PROJECT>`
- `projwm profile unassign <SLOT>`
- `projwm profile delete <NAME>`
- `projwm profile rename <OLD> <NEW>`
- `projwm profile list`
- `projwm profile show [<NAME>]`

### 5.4 アーカイブ操作

- `projwm archive <PROJECT>`
- `projwm unarchive <PROJECT> [--profile <X> --slot <Y>]`
- `projwm archive list`
- `projwm archive purge <PROJECT> --yes`

### 5.5 ナビゲーション

- `projwm jump <SLOT | PROJECT | PROFILE>`

### 5.6 同期 / メンテナンス

- `projwm reconcile [--dry-run] [--verbose]`
- `projwm validate-environment`

### 5.7 TUI 起動

- `projwm tui` — cockpit が落ちているときの手動 spawn / fallback show トリガー用。通常は常駐しているので普段使わない。

### 5.8 旧仕様との対応

- `projwm up --ai <name> --slot Q` は内部的に「project 不存在なら default 構成（ai-1+shell-1+editor）で自動生成 → assign」と展開される。`CreateProject` + `AssignProject` 合成。

### 5.9 各コマンドの**出力情報仕様**（視覚は実装側に委ねる）

#### `projwm status`

ユーザが「今 projwm は何をどう保持しているか」を一目で把握できる情報を含む:

- Generation ID（現在 committed の世代）
- Epoch（現在の controller epoch）
- Active profile name + description
- 全 profile の slot → project 割り当て一覧（active profile は強調表示可、強調方式は実装側）
- 各 active project の windows 状態（kind, index, tmux session の生死, live window の生死）
- viewer workspace A 上の AI stream 一覧
- 各 profile の inactive policy
- park 状態の project 一覧（archive されていないが assignments に無いもの）
- archived project 一覧
- manifest digest と検証状態（mismatch なら警告フラグ）
- convergence status（CONVERGED / CONVERGING / REPLAN_FAILED）

`--json` 指定時は機械可読 JSON。スキーマは実装側で固定し、後方互換を維持。

#### `projwm doctor`

projwm が動作するための前提条件と現状の健全性を検査・報告:

- `projwmd` プロセスの存在と launchd 経由起動の確認
- PersistentStore のファイル存在と読み取り可否
- managed-environment manifest のファイル存在と digest 検証
- IPC socket の到達性
- 必要な外部依存アプリ（Ghostty, Vivaldi, Zed, tmux, omniwmctl）の存在
- Vivaldi automation profile (`projwm-next`) の存在
- 各 active project の tmux session の到達性
- 各 managed window の live 状態
- invariant INV.1〜13 の現状チェック
- spawn token の orphan 残存（古い未照合 token）
- transaction trace の最新エラーがあれば表示

各検査項目は **PASS / WARN / FAIL** のいずれかで報告。FAIL があれば exit code 非 0。

#### `projwm trace`

- `--last`: 最新 transaction を 1 件表示
- `<txid>`: 指定 transaction を表示

表示する情報:
- TransactionID, generation before/after, epoch
- 発火 trigger（intent or event）
- 各 iteration（plan→execute→settle→verify）の summary
- 実行された operations 一覧（kind, target, success/fail）
- verifier diff（predicted vs observed、差分があれば）
- invariant チェック結果
- 関連 dirty scopes
- discard された stale-epoch events（あれば）
- 完了か abort か、abort なら理由

private payload (URL) は redact 表示。

#### `projwm reconcile --dry-run`

planner のみ実行、commit しない。表示する情報:

- 計算された operations 一覧（kind, target, parameters）
- 期待される removal / spawn / move / layout 変更の概要
- predicted な世界差分（現在 observed → 期待 desired）
- 0 ops なら "Already converged" メッセージ

実際の WM mutation は一切起こさない。

---

## 6. Intent と CLI のマッピング

### 6.1 新規追加 Intent

projwm-next の既存 8 intent に加え、以下を追加:

- `intent.CreateProject { ID, Path, Windows []DesiredWindow }`
- `intent.DeleteProject { ID, Purge bool }`
- `intent.AddWindow { Project, Kind, Index }`
- `intent.RemoveWindow { Project, WindowID }`
- `intent.CreateProfile { ID, Description, InactivePolicy }`
- `intent.DeleteProfile { ID }`
- `intent.RenameProfile { Old, New }`
- `intent.AdoptOrphanWindow { LiveID, AsProject, AsKind }`
- `intent.DismissOrphanWindow { LiveID, Action: close }` — `ignore` action は origin (b) 廃止により削除
- `intent.RespawnOrphanGhostty { LiveID, AsProject, AsKind }`（§4.1 `[Enter]` 用、close + respawn 合成）

### 6.2 マッピング表

| CLI | 呼び出す Intent / 内部処理 |
|---|---|
| `projwm up` | `CreateProject` (if not exists) + `AssignProject` |
| `projwm add-ai` | `AddWindow{Kind=ai}` |
| `projwm add-shell` | `AddWindow{Kind=shell}` |
| `projwm add-editor` | `AddWindow{Kind=editor}` |
| `projwm remove` | `RemoveWindow` |
| `projwm profile create` | `CreateProfile` |
| `projwm profile switch` | `SwitchProfile`（既存） |
| `projwm profile assign` | `AssignProject`（既存） |
| `projwm profile unassign` | `UnassignSlot`（既存） |
| `projwm profile delete` | `DeleteProfile` |
| `projwm profile rename` | `RenameProfile` |
| `projwm profile list` | IPC `MsgQueryRequest{Kind: "profiles"}` or ストア直読み |
| `projwm profile show` | 同上 |
| `projwm archive` | `ArchiveProject`（既存） |
| `projwm unarchive` | `UnarchiveProject`（既存） |
| `projwm archive list` | ストア直読み |
| `projwm archive purge` | `DeleteProject{Purge: true}` |
| `projwm jump` | IPC を経由せず `omniwmctl workspace focus-name X` 直接呼び出し |
| `projwm reconcile` | `Reconcile`（既存）。`--dry-run` は planner のみ実行、commit せず |
| `projwm validate-environment` | `ValidateEnvironment`（既存） |
| `projwm status` | IPC `MsgQueryRequest{Kind: "world"}` first、ストア直読み fallback |
| `projwm doctor` | ストア直読み + 環境チェック（§5.9） |
| `projwm trace` | IPC `MsgQueryRequest{Kind: "trace"}` or ストア直読み |

Cockpit 内部カードのアクション ↔ Intent:

| Card Action | Intent |
|---|---|
| `[NEW]` Enter (adopt for Vivaldi/Zed) | `AdoptOrphanWindow` |
| `[NEW]` Ghostty Enter (respawn) | `RespawnOrphanGhostty` |
| `[NEW]` close | `DismissOrphanWindow{Action: close}` |

### 6.3 Profile switch の付随挙動（要件）

`intent.SwitchProfile` 実行時、controller は以下を一括で行う:

- 旧 active profile に assign されていた project のうち、新 active profile に無い project の managed window を close（inactive policy が `keep` の場合は維持）
- 旧 active profile に assign されていなかったが新 active profile に assign された project の managed window を spawn
- 旧 active profile に居た AI window の viewer (workspace A 上) を close、新 active の AI window の viewer を spawn
- Vivaldi automation profile の各 project の DesiredBrowserSession に従って URL を復元
- tmux session は inactive policy に関わらず常に維持（windows-only semantics）
- focus policy table の `intent:switch-profile` に従って final focus 設定

---

## 7. Spawn token と `WindowExternal` の意味再定義

### 7.1 Spawn token（誤検知防止）

projwm 自身が spawn 中の transient window を「手動追加」と誤判定しない仕組み:

```go
type SpawnInFlight struct {
    Token     string
    Kind      WindowKind
    Project   ProjectID
    Workspace WorkspaceID
    Expiry    time.Time
}
```

- planner spawn 時に token 発行・登録
- observer 新 window 検出時、token 照合:
  - 一致 → silent adopt（projwm 起源）
  - 不一致 → Tier 1 提案カード発火条件評価（§3.1）

### 7.2 `WindowExternal` の単一 origin 定義

v2.2 で origin (b) を**完全廃止**。`WindowExternal` に分類されるのは以下のみ:

- **Origin (a) のみ**: 明示的に管理対象外と判定される window
  - 例: Vivaldi の user profile window（`classifyLiveWindow` で `--profile-directory != projwm-next` を検知）
  - その他、bundle ID が manifest の `apps` に含まれない window（Calculator, Slack, Discord 等）

Ghostty / Vivaldi automation profile / Zed の手動追加 window は **External 永続化させない**。必ず Tier 1 カードで `[Enter]` adopt or `[c]lose` のどちらかへ収束する。

triage 前の未判定 orphan は `WindowExternal` 化せず、Tier 1 candidate として保留される。

planner は origin (a) `WindowExternal` を完全スキップする。

---

## 8. Cockpit TUI — 常駐・projwm-managed モニタに 1 つ

### 8.1 起動・常駐

- `projwmd` 起動時に **projwm-managed モニタ (workspace A / Q-P が住むディスプレイ) に 1 つだけ** Ghostty window を spawn
- tmux session `projwm-cockpit` を backing にもつ
- 他モニタには cockpit を spawn しない（UX 再評価の結果、複数モニタ表示は不要と確定）

**v2.4 補足**: v2.3 までは「全モニタに同時表示」を要件としていたが、実運用上ユーザは workspace A / Q-P を見ているモニタ (= projwm-managed モニタ) でしか cockpit を使わないことが確認された。他モニタの cockpit は冗長で、操作と spawn コストの両方を増やすだけだったため 1 個に縮退する。

### 8.2 表示制御

- 平時: 隠れている、または明示表示中
- ユーザ召喚: `space+f` で対象モニタ上で show / hide toggle
- システム強制表示: 提案カード発生時、対象モニタで強制 show + focus 移動

### 8.3 Workspace 越え overlay 要件（対象モニタ内）

- cockpit はその対象モニタの**どの workspace 表示中でも** overlay として呼び出せる
- show 時、対象モニタの active workspace が CP1 に切り替わる（cockpit が前面化）
- hide 時、対象モニタは元の workspace に戻る
- 他モニタは一切影響を受けない

park-workspace `CP1` は projwm-managed モニタ専用に 1 つだけ確保する（CP2-CP6 は廃止）。実装方式詳細は §15 技術判断。

### 8.3.1 active workspace と Visibility の双方向同期 (v2.7)

cockpit の `Visibility` 状態 (`Shown` / `Hidden`) は projwm-managed モニタの **observed active workspace** と双方向に同期する:

| observed.activeWorkspace | DesiredWorld.Visibility | 取扱い |
|---|---|---|
| `CP1` (= ParkWorkspace) | `Shown` | 整合済、planner は noop |
| `CP1` 以外 | `Hidden` | 整合済、planner は noop |
| `CP1` だが `Hidden` | (observed→desired 同期で `Shown` へ flip) | reducer が自動同期 |
| `CP1` 以外だが `Shown` | (observed→desired 同期で `Hidden` へ flip) | reducer が自動同期 |

これにより:
- ユーザが `space+1` / `space+q` 等で別 workspace に移動 → observed が `CP1` から離脱 → reducer が自動的に Visibility=Hidden に flip → planner は ShowCockpit op を出さない (= 「勝手な CP1 戻り」が消える)
- ユーザが `space+f` で cockpit を表示 → SetCockpitVisibility(Shown) intent → planner が ShowCockpit op → observed が CP1 に → 同期は整合
- ユーザが `space+f` で再度 cockpit を隠す → SetCockpitVisibility(Hidden) → planner が HideCockpit op → observed が PriorWorkspace に戻る

**設計上の位置付け**: 設計層 1 (design.md §13) の「external event は desired を直接変えない」原則の **cockpit visibility に対する例外**。Tier 2 (column ordering の auto-overwrite) と同じパターン (event → DirtyScope → 内部 intent 発火 → DesiredWorld 更新)。

これにより workspace 移動コマンド (位置系 space-leader binding) は自動的に cockpit toggle 相当の意味を持ち、専用 `space+f` (= ToggleCockpit intent) と並存できる。

### 8.4 表示状態モデル — 3 モード

| Mode | 入口 | 出口 | 該当操作 |
|---|---|---|---|
| **Mode 1: Proposal**（強制表示） | システムが提案カードを push | 応答 (Enter/Esc/英字) 後、**元の visibility 状態へ復帰** | system 起源イベント |
| **Mode 2: Navigation** | `space+f` で開く | 操作後 **自動 hide** | jump, profile switch, focus window |
| **Mode 3: Management** | `space+f` で開く | 操作後も **stay**、`space+f` で hide | assign, archive, add-*, profile create/delete, status, list, adopt 等 |

### 8.5 操作別 hide 動作

| 操作 | Mode | 自動 hide |
|---|---|---|
| jump (slot/project/workspace) | 2 | ✅ |
| profile switch | 2 | ✅ |
| focus window | 2 | ✅ |
| assign / unassign | 3 | ❌ |
| archive / unarchive | 3 | ❌ |
| archive purge | 3 | ❌ |
| add-ai / add-shell / add-editor / remove | 3 | ❌ |
| profile create / delete / rename | 3 | ❌ |
| status / list / show / trace | 3 | ❌ |
| adopt / dismiss orphan | 3 | ❌ |
| reconcile | 3 | ❌ |
| validate-environment | 3 | ❌ |
| 提案カードへの応答 | 1 | 元状態へ復帰 |

### 8.6 強制表示時のフォーカス挙動

1. projwm-managed モニタ (cockpit 設置先) で cockpit を show
2. そのモニタの cockpit に focus 移動
3. IME 入力中なら一旦解除
4. プロンプトに直接キー入力可能な状態
5. アクション完了 / Esc → **元の focus window へ復帰、元 visibility 状態へ**

注: v2.3 までは「マウスカーソルがあるモニタへ強制移動」としていたが、cockpit が 1 モニタ専用化されたため「対象モニタへの focus 移動」に簡素化。

macOS notification は**一切使わない**（ユーザ要望により完全排除）。通知・気付き・操作はすべて cockpit に集約。

### 8.7 復帰のループ防止（managed window user-close）

- 自動復元の grace period: **同一 DesiredWindow** に対して 60 秒以内に 2 回閉じられたら復元 retry を停止、warning カードで通知
- ユーザは warning カードから `[k]eep removed` で desired から削除、または `[u]ndo` で再復元 retry を選択

### 8.9 自己修復契約 (v2.8)

**原則**: projwmd を起動するだけで、AX permission 喪失等 macOS layer の問題を除く全崩壊パターンから自律的に理想状態に収束する。手段は問わない (強制的でも良い)。

#### 8.9.1 監視タイミング
- **Bootstrap event 時** (`projwmd` 起動直後): 必ず実行
- **継続監視**: 30 秒間隔の goroutine が omniwm health を probe

#### 8.9.2 Health probe で確認する項目
- `omniwmctl ping` 応答
- `omniwmctl query apps` に manifest 列挙の全 managed app (Ghostty/Vivaldi/Zed) が居る
- `omniwmctl query rules` の rule 数が omniwm-deploy 期待値と一致
- `omniwmctl query windows` で managed window が見える
- cockpit window が `ParkWorkspace=CP1` に居る (§8.10 cockpit invariant 参照)

#### 8.9.3 Recovery ladder

| Lv | 操作 | 副作用 | UX |
|---|---|---|---|
| Lv1 | `omniwm-deploy` バイナリ実行 (rules 再 push) | ほぼ無 | 自動発火、無通知 |
| Lv2 | 個別 managed app を quit + relaunch | tmux re-attach race の可能性 | `[OMNIWM-RECOVERY]` カード + 3 秒猶予 |
| Lv3 | `launchctl kickstart -k gui/<uid>/org.nixos.omniwm` | 全 app の workspace 配置リセット (app-rule で再 bind されるが column 順序乱れ) | `[OMNIWM-RECOVERY]` カード + **5 秒猶予 + ユーザ Esc 可** |
| Lv4 | `launchctl bootout` → `bootstrap` | Lv3 と同等 | Lv3 が成功しなかった時のみ、**10 秒猶予 + Esc 可** |

全 step を `[OMNIWM-RECOVERY]` カードに記録し、透明性を保つ。すべて失敗で `[OMNIWM-RECOVERY-FAILED]` warning card を提示し、ユーザに「macOS System Settings で AX permission を確認してください」等の guidance を出す。

#### 8.9.4 限界 (どんな設計でも届かない)
- AX permission が macOS で revoke された → System Settings での手動許可必須
- omniwm.app の binary が壊れた → `darwin-rebuild` が必要
- macOS 自体の bug

これらは `[OMNIWM-RECOVERY-FAILED]` カードで明示し、ユーザに guidance を出す。

### 8.10 Cockpit invariant (v2.8)

**cockpit は他の system window と異なり、特例として「強制的に必ず ParkWorkspace=CP1 に居る」ことを保証する**。

#### 8.10.1 不変条件
- `Observed.cockpit.workspace == "CP1"` を常時保持
- ユーザの手動移動 (drag, omniwmctl 直叩き等) は **禁止行為** = Tier 4 強制復元
- §8.3.1 の Visibility 同期と区別: 「display.activeWorkspace の切替 = Visibility flip」は Tier 2/3 相当の auto-sync、「cockpit window 自身の workspace」は Tier 4 強制 revert

#### 8.10.2 検出・矯正 ladder

| Lv | 操作 | 内容 |
|---|---|---|
| Lv1 | `omniwmctl window focus <id>` + `command move-to-workspace <CP1番号>` | gentle move、focused state で workspace 移動 |
| Lv2 | cockpit ghostty kill + daemon の planner が再 spawn (app-rule 期待) | kill+respawn で app-rule 再発火を狙う |
| Lv3' | Lv1+Lv2 を N 回試して失敗 → `[COCKPIT-AUTOHEAL-FAILED]` warning card | ユーザ判断 (omniwm restart 等) を促す |

実装上は planner.planCockpitOps で「observed.cockpit.workspace ≠ ParkWorkspace」を検出して MoveCockpitToParkWorkspace op を emit、N 回失敗で warning card。

#### 8.10.3 cockpit が「強制的に CP1」である根拠
- 要件 §8.1: cockpit は projwm-managed monitor の CP1 に 1 個常駐
- 要件 §8.3: cockpit は workspace 越え overlay として CP1 から呼び出せる
- ユーザ意図: 「cockpit は別 workspace に移動はできないよう禁止」(設計議論 2026-05-18)

### 8.8 Cockpit のライフサイクル

| イベント | 挙動 |
|---|---|
| `projwmd` 起動 | projwm-managed モニタに cockpit を 1 個 spawn |
| `projwmd` 停止 / 再起動 | cockpit プロセスも停止 → 再起動で再 spawn |
| macOS sleep | cockpit プロセス維持（tmux session 維持） |
| macOS wake | tmux session reconnect、表示状態リセット（hidden に戻る） |
| monitor 接続 | 何もしない (cockpit は projwm-managed モニタ専用のため新規モニタ追加は無関係) |
| monitor 切断 | projwm-managed モニタ自体が切断された場合: cockpit のみ近接する残存モニタへ再配置（実装側判断）、他モニタ切断は無関係 |
| cockpit プロセス異常終了 | projwmd が検知して再 spawn（5 秒以内に最大 3 回 retry） |
| ユーザが `projwm tui` 実行 | プロセスが存在しなければ手動 spawn、存在すれば show |

---

## 9. Cockpit TUI — 情報要素・機能・UI 構造 (v2.9 再設計)

### 9.0 全体構造 (v2.9 新設)

3 層構造:

```
┌─ projwm-cockpit ────────────────────────────────────────────────────┐
│ topbar: gen / epoch / profile / convergence / digest / cards count  │  §9.1
├─[ Slots ]─ Cards (N) ─ Archived ─ Profiles ─ Trace ─────────────────┤  §9.4 top tabs
│                                                                     │
│  /filter (fzf)                                                      │  §9.4.6
│  main content (depends on active tab)                               │  §9.2 / §9.3
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│ Navigate: <keys for navigation>                                     │  §9.9 bottom menu
│ Actions:  <keys for actions possible RIGHT NOW>                     │  (context-aware,
│ Help:     [?] help  [Ctrl-P] palette  [Esc] hide                    │   全可能 action 列挙)
└─────────────────────────────────────────────────────────────────────┘
```

設計原則:
- **発見性最優先**: その時可能な操作はすべて bottom menu に列挙される
- **行き来は自由**: tab は `1-5` キー or `Shift+[/]` で切替、`o` で modal/list 切替
- **新規操作は wizard 形式 (B2 フォーム)**: 全項目同時表示、defaults prefill、Tab field 切替

### 9.1 常時見えるべき情報

- **Generation ID**（現在 committed 世代）
- **Epoch**
- **Active profile name**
- **Convergence status**（CONVERGED / CONVERGING(N/M) / REPLAN_FAILED）
- **Manifest digest 検証状態**（OK か mismatch 警告）
- **未対応カード件数**

### 9.2 一覧として見えるべき情報

- **Active profile の slot → project 割り当て**: 各 slot に何の project が居て、その project の各 window (ai-N / shell-N / editor / browser) の状態（tmux session 生死, live window 生死, focused フラグ）
- **Active profile 以外の profile**: 名前 + slot 数（折り畳み or 概要）
- **Park 状態の project**: profile に未 assign かつ archive されていない project
- **Archived project 一覧**
- **Viewer (workspace A) の AI stream 一覧**: 表示順は現在の slot order に従う

### 9.3 提案カード（最優先表示）

未対応カードがあれば最上部に表示。カード自体の情報要素は §10。

### 9.4 タブ構造 (v2.9)

5 つの top tab を持つ。`1-5` キー or `Shift+]` / `Shift+[` で切替。新カード到着時、ユーザ idle なら `Cards` tab に自動 hop (= proposal mode K1.5)。

| Tab | キー | 内容 |
|---|---|---|
| **Slots** | `1` | active profile の slot Q-P assignment、viewer (A) AI stream 一覧、park 一覧 (= §9.2 主要部) |
| **Cards (N)** | `2` | カードモーダル (full-screen 2 column: 左 detail + 右 workspace zoom-out)。N=未対応カード件数 |
| **Archived** | `3` | archived project 一覧 (unarchive 操作) |
| **Profiles** | `4` | 全 profile + assignments (active 強調)、profile create / delete / rename |
| **Trace** | `5` | 最近の transaction trace (`projwm trace` 相当) |

カード来た時の挙動 (§8.4 Mode 1):
- ユーザが何も操作中でない (idle) → 自動的に Cards tab に hop + modal 表示
- 操作中 (filter 入力 / wizard 中 / palette open) → cards カウント増やすだけ、自動 hop しない
- `o` キーで Cards tab ↔ 直前 tab を toggle 切替

### 9.4.6 検索 / 絞り込み機能

- **fzf-style インクリメンタル絞り込み**: 文字入力で slot / project / profile / viewer 項目をリアルタイムにフィルタ
- マッチ箇所の強調（実装側の表現）
- 上下移動で候補選択

### 9.5 内部 keybinding (v2.9 context-aware に再整理)

cockpit window が focus 状態にある時の操作:

| キー | 機能 |
|---|---|
| `↑` / `↓` / `Ctrl+J` / `Ctrl+K` | 項目間カーソル移動 |
| 任意文字 | fzf 検索バーへの入力（絞り込み開始） |
| `Enter` | 選択中項目に対する推奨アクション（slot/project なら jump、カードなら推奨 action） |
| `Tab` | active profile を循環切替 |
| `n` | 新規 project 作成 prompt（modal、cwd 入力） |
| `d` | 選択 project を unassign（park 化） |
| `a` | 選択 project を archive |
| `u` | 選択 archived project を unarchive prompt（profile / slot 選択） |
| `r` | 選択 window 定義を remove |
| `?` | help 表示 |
| `Esc` | filter クリア / modal を閉じる / cockpit を hide |
| `Ctrl+C` | cockpit を hide |
| `Ctrl+L` | 全カード一括 dismiss（確認 prompt あり） |
| `t`（カード上で） | カードを cockpit TUI 内 detail view に carry over |

### 9.7 Wizard (v2.9 新設) — 新規 project / profile 作成 UI

`n` 押下 (Slots tab) or palette `new project` で wizard overlay を開く:

```
┌─ Create new project ───────────────────────────────────┐
│   ID       ▶ foo▌                                     │
│   Path     $HOME/dev/foo               ✓ exists        │
│   Slot     ▾ Q   (currently unassigned, recommended)   │
│   Windows  ☑ ai-1   ☐ shell-1   ☑ editor               │
│                                                        │
│   Tab: next field   Enter: create   Esc: cancel        │
└────────────────────────────────────────────────────────┘
```

**仕様**:
- 全項目同時表示 (B2 form)
- defaults prefill: ID 入力後に path / slot を auto-fill (path = `$HOME/dev/<ID>`、slot = cursor 位置 or 空き最初)
- `Tab` で field 移動、`Shift+Tab` 逆順、 `Enter` で submit (default 値で確定)
- 編集自由、いつでも戻れる
- submit 後: `CreateProject` + `AssignProject` + `AddWindow` を順次発火、Cards tab に [NEW] orphan があれば候補表示

Profile 作成も同パターンで Profiles tab から `n`:
```
┌─ Create new profile ───────────────────────────────────┐
│   ID            ▶ work▌                               │
│   Description   weekday tasks                          │
│   Inactive policy  ▾ remove                            │
└────────────────────────────────────────────────────────┘
```

### 9.8 Command palette (v2.9 新設) — `Ctrl-P`

```
┌─ Command palette ──────────────────────────────────────┐
│   ▶ new pro█                                          │
│                                                        │
│   ★ new project              create + assign + window  │
│     new profile                                        │
│     archive (selected)       foo → archived            │
│     unarchive ...                                      │
│     reconcile                                          │
│     status / doctor / trace                            │
│     hide cockpit                                       │
└────────────────────────────────────────────────────────┘
```

**仕様**:
- 任意の tab / wizard 中から `Ctrl-P` で開く
- fuzzy 検索、Enter で実行 (= 即発火 or wizard 起動)
- 全 action を 1 つの list として網羅 (TUI で行えるすべての操作)

### 9.9 Bottom menu (v2.9 新設) — 発見性最優先、context-aware

その時可能な操作を**全列挙**。 3 行構造、context で内容変化:

```
─────────────────────────────────────────────────────────
 Navigate:  [↑↓] cursor  [1-5] tab  [Tab] profile  [/] filter
 Actions:   <キー (context-dependent, ALL possible right now)>
 Help:      [?] help  [Ctrl-P] palette  [Esc] hide cockpit
─────────────────────────────────────────────────────────
```

例 (slot Q with project foo assigned):
```
 Actions:   [Enter] jump to Q  [n] new project  [a] archive foo
            [d] unassign Q     [r] remove window from foo  [+] add window
```

例 (Cards tab, [NEW] orphan ghostty):
```
 Actions:   [Enter] adopt to existing  [n] new project + adopt
            [c] close orphan  [t] carry over  [←/→] prev/next  [o] back to Slots
```

例 (空 slot):
```
 Actions:   [Enter] focus workspace (empty)  [n] new project here
            [u] unarchive into this slot
```

**実装責務**: `Bottom menu` は cursor 位置と現 tab に応じて「daemon に問い合わせ可能 action」を計算して表示。新規 action を増やすときも必ず ここに現れるべき。

### 9.6 表示しないこと

- 私的情報の生 URL（必ず redact 表示）
- PrivatePayloadRef の中身（不透明 token のまま）
- 内部 spawn token

---

## 10. Cockpit カード — 情報要素と機能

### 10.1 カードに必須の情報要素

各カードは以下を含む:

- **カードタイプ**（`[NEW]` / `[CLOSED]` / `[MOVED]` / `[REPLAN]` / `[INVARIANT]` / `[MANIFEST]` / `[ORPHAN]`）
- **発生時刻**（相対 or 絶対、実装側選択）
- **要約 1 行**（人間可読、ユーザに何が起きたか即座に分かる）
- **コンテキスト**（関連 project / workspace / window id 等）
- **問題の説明**（必要なら、例: Ghostty Cmd+N の "no tmux backing"）
- **可能なアクション一覧**: 各アクションに対して
  - キー（`Enter`, 英字 1 文字, `t`, `Esc`）
  - 簡潔な説明
  - そのアクションを選んだ場合の結果（破壊的か可逆か）

### 10.2 キー規約

- **`Enter`** = 推奨アクション。常に最も安全 / 便利な default
- **`Esc`** = 何もしない（= ignore once）。自動 hide が走った場合は元 focus へ復帰
- **英字キー** = alternate アクション
- **`t`** = cockpit TUI 内に carry over して詳細操作（カードは dismiss しない）

### 10.3 カードタイプ別 `Enter` の意味

| カードタイプ | 発生条件 | Enter の意味 |
|---|---|---|
| `[NEW]` Ghostty | Tier 1: Ghostty 手動追加 | respawn properly (close + 正規 spawn) |
| `[NEW]` Vivaldi/Zed | Tier 1: Vivaldi/Zed 手動追加 | adopt (project 選択 prompt あり) |
| `[CLOSED]` | Tier 4: managed window のユーザ閉じ → 自動復元済 | keep restored（誤閉じ前提） |
| `[MOVED]` | Tier 4: window が別 workspace に手動移動 → 自動 revert 済 | acknowledge revert |
| `[REPLAN]` | Verifier replan 失敗 | retry once |
| `[INVARIANT]` | INV.1-13 違反 | show details |
| `[MANIFEST]` | manifest digest mismatch | open validation report |
| `[ORPHAN]` | adopt も close もされず長時間放置された orphan | propose adoption again |

### 10.4 複数カード処理

- 最新カードが上、古いものはスクロール
- `Enter` / 英字キーは最上カードに作用
- 個別カードを `Esc` で dismiss
- `Ctrl+L` で全カード一括 dismiss（確認 prompt あり）

### 10.5 提供しない選択肢

- **`[i]gnore` (External 永続化)**: §7 origin (b) 廃止により Ghostty/Vivaldi/Zed カードから完全排除

---

## 11. キーバインド再構成

### 11.1 Space as combo trigger — `simultaneous` + `key_down_order=strict`

**物理 modifier キー（cmd/ctrl/opt/shift）は一切使わない**。`space + letter` を karabiner の `simultaneous` でマッチさせ、`key_down_order=strict` で「space が先押下」順序のみ発火するよう制約する。これにより:

- **letter→space ロール打鍵での暴発がゼロ** (順序違反として karabiner が即素通し)
- **既存物理 modifier との衝突なし** (modifier は使っていない)
- **追加の特殊キー (f13/f14) 不要** (space + 通常英数字キーのみで完結)

#### v2.3 提案 (`to_if_alone` + variable) からの変更理由

v2.3 仕様は `variable_if` + `to_if_alone` の組み合わせを提案していた:

1. space 押下時に `set_variable space_held=1` (OS には何も流さない)
2. release 時に `to_if_alone` が発火、通常 space を emit
3. 他キー併用時は `to_if_alone` キャンセル

**実機検証の結果、この方式は単独 space タップに体感的に遅延が出る** ことが判明した。理由:

- `to_if_alone` の発火タイミングは **必ず key_up イベント受信時**
- 一般的なキーイベントは **key_down 時** に OS に文字が届く
- ユーザの打鍵時間 (30〜150ms) がそのまま遅延として認識される

加えて、letter→space のロール打鍵で letter が space_held=1 を立てた後の release race も無視できなかった (letter は OS に直接届いていても、変数管理上は letter→space combo として処理される時間窓があった)。

#### 採用方式 (v2.4): simultaneous + strict

```json
{
  "from": {
    "simultaneous": [
      { "key_code": "spacebar" },
      { "key_code": "q" }
    ],
    "simultaneous_options": {
      "key_down_order": "strict",
      "key_up_order": "insensitive",
      "detect_key_down_uninterruptedly": false
    },
    "modifiers": { "optional": ["any"] }
  },
  "parameters": {
    "basic.simultaneous_threshold_milliseconds": 80
  },
  "to": [{ "shell_command": "omniwmctl workspace focus-name Q" }],
  "type": "basic"
}
```

#### 各シナリオの挙動

| シナリオ | 挙動 |
|---|---|
| 単独 space タップ (< 80ms hold) | space を 1 文字 emit (release 時) |
| 単独 space ホールド (> 80ms) | threshold 経過時点で space を emit |
| space → letter (space 先、80ms 以内に letter) | combo 発火、space は emit されない |
| letter → space (letter 先) | `strict` 違反、karabiner は match しない → space は即 OS へ素通し (**ロール打鍵安全**) |
| ctrl / opt / cmd + space (modifier-first) | `modifiers.optional = []` のため simultaneous マッチ対象外 → karabiner バッファに入らず即素通し (modifier ごと OS へ) |
| space + shift + letter | 別 rule (`modifiers.mandatory = ["shift"]`) が match |
| space + ctrl + letter | 別 rule (`modifiers.mandatory = ["control"]`) が match |

#### 採用しない方式と理由

- **Hyper modifier emit 方式 (karabiner が cmd+ctrl+opt 等を出力)**: 採用しない。
  理由: space hold 中に cmd / ctrl / opt のショートカット (cmd+S, cmd+Q 等) が完全に奪われる。
- **F-key 経由で OmniWM 内蔵 hotkey に乗せる**: 採用しない。
  理由: OmniWM の内蔵 hotkey ID 144 個には named workspace jump (Q-P/A/M/B) が存在しない。
- **v2.3 の `to_if_alone` + variable 方式**: 上述の遅延問題により不採用。

#### 出力方式: shell_command 統一

すべての projwm/omniwm bindings は karabiner → `shell_command` → omniwmctl / projwm CLI 経由で動作する。

**Latency**: 〜15-25ms（fork+exec + IPC）+ simultaneous threshold (max 80ms)。実用上は問題なし。

### 11.2 タイピング誤発火の例外措置

`simultaneous` 方式では combo 登録されていない letter は自動的に素通しになるため、v2.3 の `variable_if` + 例外 rule のような措置は不要。

実用上 typing と関連する `comma`, `period` 等のキーは combo に登録しないことで自然に素通しを実現する (要件は variable_if 方式のためのものだったが、simultaneous strict では自動解決)。

### 11.3 keybind 一覧 (v2.6)

体系: **位置/focus/move 系 → space-leader (karabiner)、size/構造/UI 系 → option (OmniWM 内蔵)**

#### SPACE-base (karabiner spaceBind で実装、OmniWM 内蔵は Unassigned)

```
space alone (tap)     → space (normal typing)

# Workspace 切替
space + 1..9                          → omniwmctl workspace focus-name 1..9
space + q,w,e,r,t,y,u,i,o,p          → omniwmctl workspace focus-name Q..P (projwm slot)
space + a                             → omniwmctl workspace focus-name A
space + m                             → omniwmctl workspace focus-name M
space + b                             → omniwmctl workspace focus-name B

# Workspace ナビゲーション
space + tab                           → omniwmctl command switch-workspace back-and-forth
space + ]                             → omniwmctl command switch-workspace next
space + [                             → omniwmctl command switch-workspace prev

# Window → Workspace 移動
space + shift + 1..9                  → omniwm-move-window-to-named-ws 1..9
space + shift + q..p                  → omniwm-move-window-to-named-ws Q..P
space + shift + a/m/b                 → omniwm-move-window-to-named-ws A/M/B
space + shift + ]                     → omniwmctl command move-to-workspace down
space + shift + [                     → omniwmctl command move-to-workspace up

# Focus 方向 (within workspace) — vim hjkl
space + h                             → omniwmctl command focus left
space + j                             → omniwmctl command focus down
space + k                             → omniwmctl command focus up
space + l                             → omniwmctl command focus right
space + ;                             → omniwmctl command focus previous

# Focus column
space + ctrl + 1..9                   → omniwmctl command focus-column 0..8 (0-based)
space + ctrl + [                      → omniwmctl command focus-column first
space + ctrl + ]                      → omniwmctl command focus-column last

# Focus monitor
space + ctrl + h/j/k/l                → omniwm-focus-monitor-dir left/down/up/right
space + ctrl + tab                    → omniwmctl command focus-monitor next

# Window 方向 move (within workspace) — hjkl
space + shift + h/j/k/l               → omniwmctl command move left/down/up/right

# Column 方向 move
space + ctrl + shift + h              → omniwmctl command move-column left
space + ctrl + shift + l              → omniwmctl command move-column right

# Column → workspace 並び替え
space + ctrl + shift + 1..9           → omniwmctl command move-column-to-workspace 0..8 (0-based)
space + ctrl + shift + ]              → omniwmctl command move-column-to-workspace down
space + ctrl + shift + [              → omniwmctl command move-column-to-workspace up

# 機能呼び出し
space + f                             → projwm cockpit toggle
space + ctrl + m                      → omniwm-setup-media-workspace
space + s                             → omniwm-ws-launch M Spotify
space + c                             → omniwm-ws-launch M Discord
```

注: **`space + e` は workspace E (slot E) への jump**。E は管理対象 workspace。

#### OPTION-base 維持 (OmniWM 内蔵で直接ハンドル、karabiner 経由なし)

```
Option+. / Option+,                   → cycleColumnWidthForward / Backward
Option+= / Option+-                   → setColumnWidth.increase10Percent / decrease
Option+Shift+= / Option+Shift+-       → setWindowHeight.increase10Percent / decrease
Control+Option+F                      → expandColumnToAvailableWidth
Option+Shift+F                        → toggleColumnFullWidth
Option+T                              → toggleColumnTabbed
Option+Return                         → toggleFullscreen
Option+Shift+Space                    → toggleFocusedWindowFloating
Option+/                              → balanceSizes
Option+L                              → toggleWorkspaceLayout
Option+Shift+O                        → toggleOverview
Option+Shift+R                        → raiseAllFloatingWindows
Control+Option+Shift+R                → rescueOffscreenWindows
Control+Option+R                      → resetWindowHeight
Control+Option+Space                  → openCommandPalette
```

### 11.4 廃止するもの (v2.6)

- **OmniWM 内蔵 Quake terminal**（`toggleQuakeTerminal`）: 完全廃止、Unassigned。cockpit が代替。
- **旧 `opt+letter` 系の位置/focus/move binding**（projwm/omniwm 両方）: 全廃止、`space+letter` に置換。
- **OmniWM 内蔵の位置/focus/move 系 hotkey**: Unassigned（space-leader 経由 shell_command で代替）。具体リスト:
  - focus.{left,right,up,down}, focusPrevious
  - move.{left,right,up,down}
  - focusColumn.0-8, focusColumnFirst, focusColumnLast
  - moveColumn.{left,right}
  - moveColumnToWorkspace.0-8, moveColumnToWorkspaceDown/Up
  - switchWorkspace.0-8, switchWorkspace.next/previous, workspaceBackAndForth
  - moveToWorkspace.0-8, moveWindowToWorkspaceDown/Up
  - focusMonitorLast, focusMonitorNext, focusMonitorPrevious
- 一気切替（移行期間なし）

### 11.5 opt の解放

opt+ 系の projwm/omniwm bindings は完全に外す。opt は他用途用に解放（用途は今回スコープ外）。

### 11.6 OmniWM 内蔵 keybind の置換範囲 (v2.6)

体系原則: **位置/focus/move 系 → space-leader (karabiner shell_command)、size/構造/UI 系 → option 維持 (OmniWM 内蔵)**

置換済み mapping:

| 旧 OmniWM binding | 新 space-leader binding |
|---|---|
| Option+1..9 (switchWorkspace.0-8) | space+1..9 |
| Option+Shift+1..9 (moveToWorkspace.0-8) | space+shift+1..9 |
| Option+H/J/K/L (focus.left/down/up/right) | space+h/j/k/l |
| Option+P (focusPrevious) | space+; |
| Option+Shift+H/J/K/L (move.left/down/up/right) | space+shift+h/j/k/l |
| Control+Option+1-9 (focusColumn.0-8) | space+ctrl+1..9 |
| Option+Home/End (focusColumnFirst/Last) | space+ctrl+[/] |
| Control+Option+Shift+H/L (moveColumn.left/right) | space+ctrl+shift+h/l |
| Control+Option+Tab (workspaceBackAndForth) | space+tab |
| Control+Option+M (openMenuAnywhere) | Unassigned (space+ctrl+m と衝突) |
| Control+Command+Tab (focusMonitorNext) | space+ctrl+tab |
| (新規) switch-workspace next/prev | space+]/[ |
| (新規) move-to-workspace down/up | space+shift+]/[ |
| (新規) move-column-to-workspace 0-8/down/up | space+ctrl+shift+1..9/]/[ |

OmniWM 内蔵で維持するもの (OPTION-base): §11.3 OPTION-base 維持リスト参照。

実装済み: `karabiner-rules.nix` に全 space binding を追加、`hotkeys.nix` の overrides で位置/move 系を全 Unassigned 化。

---

## 12. IPC 拡張要件

cockpit が常駐 TUI として状態購読・操作するため、IPC に以下を追加:

- `MsgQueryRequest { Kind: "world" | "trace" | "cards" | "profiles" | ... }`
- `MsgQueryResponse { Body: json.RawMessage }`
- `MsgSubscribe { Kinds: []string }` — cockpit が event stream を購読
- `MsgEvent { Kind, Body }` — daemon → client への非同期 push（カード発生通知等）

### 12.1 ストア直読みフォールバック

`projwm status` / `projwm doctor` / `projwm profile list` / `projwm archive list` / `projwm trace` 等の read-only 操作は、daemon 停止時にも見えるよう、ストア JSON 直読み実装も提供する（IPC 経由を first、ファイル直読みを fallback）。

書き込み系（intent submission）は daemon 経由必須。

---

## 13. surface すべき projwm-next 特有概念

cockpit / CLI で以下を見せる責務:

### 13.1 常時情報

- Generation ID
- Epoch
- Active profile name
- Convergence status

### 13.2 状態に応じて見せる情報

- Manifest digest 状態（mismatch 時警告）
- 各 profile の inactive policy
- Invariant violation（カード `[INVARIANT]`）
- Reconcile-zero-mutation 通知（`Already converged (0 ops)`）

### 13.3 redacted 情報

- Browser URL は必ず redact 表示
- PrivatePayloadRef は不透明 token のまま表示

### 13.4 trace 表示専用

- Transaction trace 全文（`projwm trace`）
- Dirty scopes
- Stale-epoch event discard ログ
- Lifecycle removal contract evidence

---

## 14. 「park」概念の扱い

旧 projwm の「park」（profile 未割当だが archive されていない project）は、projwm-next の DesiredWorld 上は表現可能だが意味付けがない。

決定: **表示ロジックのみで park を扱う**。
- `Archived: false` かつ どの Profile の Assignments にも含まれない project = park として cockpit / CLI に表示
- DesiredWorld の構造変更は不要

---

## 15. スコープ判断

スコープ判断を **「不要」** と **「今回スコープ外」** に分離。

### 15.1 不要（実装しない、将来も実装する予定なし）

- **`projwm add-browser` 系 intent / CLI**: Vivaldi タブは Tier 3 で自動観測。明示的 URL 追加 CLI は不要
- **macOS notification 連携**: 完全排除、すべて cockpit カード経由
- **opt+ binding の並行サポート期間**: 一気切替
- **user profile Vivaldi の管理**: 完全無視（External 再分類）
- **`projwm restore` コマンド**: 不要（理由は §15.4）
- **`[i]gnore` 選択肢 / `WindowExternal` origin (b)**: 完全廃止

### 15.2 Vivaldi タブの責務（不要じゃない、Tier 3 として実装する）

- **初回起動時のタブ復元**: `intent.SwitchProfile` 等で project が active になる際、`DesiredBrowserSession.URLPayloadRefs` の URL を Vivaldi window に復元
- **手動操作の自動観測**: ユーザがタブを追加・閉じ・並び替え・URL 変更したら、自動で `PrivatePayloadStore` に反映（Tier 3）

### 15.3 今回スコープ外（要件議論未了、別途判断）

（現在該当項目なし）

### 15.4 `projwm restore` 不要の根拠

「restore」には目的が 2 種類あり、projwm-next ではいずれも明示コマンドが不要:

**(B) macOS 起動時の tmux / window 復活**:
- `projwmd` は launchd 経由で macOS 起動時に起動
- PersistentStore から DesiredWorld を読み込み、reconcile が走る
- planner が「DesiredWindow に対応する live window が無い」と判定し spawn 操作を生成
- executor が Ghostty + tmux session を作成
- → **自動。明示コマンド不要。** `TestHumanE2ERestartVisiblePersistenceSteps` で検証済み

**(A) PersistentStore 損失からの災害復旧**:
- 想定シナリオ: ディスク破壊、ユーザ誤削除、データ破損
- 発生確率は極めて低い
- 仮に発生しても tmux 側に「どの project がどの slot に assign されていたか」の情報は無い
- → 自動推論より「ユーザが `projwm up` を打ち直す」方が安全
- 残存 tmux session は Tier 1 カードによる orphan adopt 機構で部分的に救済

結論: 明示的な `restore` コマンドを提供しない。

---

## 16. 受け入れ基準

### 16.1 CLI

- 旧 projwm の全 subcommand（§5）が `projwm` 経由で動作する
- `projwm status` / `doctor` / `trace` / `profile list` / `archive list` は daemon 停止時もストア直読みで動作する
- §6.2 のマッピング表通りに intent / 内部処理が呼ばれる
- §5.9 の出力情報仕様を満たす
- §5.9 `projwm doctor` の検査項目をすべて実施
- §5.9 `projwm reconcile --dry-run` で実 mutation が一切起こらない

### 16.2 Cockpit

- macOS 起動時から（projwmd 経由で）cockpit プロセスが常駐
- projwm-managed モニタに 1 つだけ常駐（`space+f` で対象モニタ上 toggle）
- システム提案カード発生時、対象モニタで強制表示 + フォーカス移動
- workspace 切り替えに関わらず visibility 状態維持（§8.3）
- §8.5 ごとに hide 動作が決定表通り
- macOS notification を一切使用しない
- §8.8 ライフサイクル表通りに動作
- §9.1〜9.2 の情報要素がすべて見える
- §9.4 fzf-style 検索が機能
- §9.5 TUI 内部 keybind すべて機能
- §10 カード仕様準拠

### 16.3 状態変更モデル

- Tier 1（managed workspace 上の手動 window 追加）: 全アプリで cockpit カード発火
- Tier 1 は管理外 workspace 上の手動追加では発火しない
- Tier 2（同一 workspace 内 column 並び替え）: 自動上書き、`accept-manual-layout` 廃止
- Tier 3（Vivaldi タブ URL 等）: 自動 observe + persist
- Tier 4: workspace 間 move / 管理外↔managed 間 move / managed window 閉じ いずれも自動 revert + カード通知
- Spawn token により projwm 自身の spawn は誤検知されない
- managed window user-close は 60s 内 2 回で復元停止

### 16.4 キーバインド

- `space` を karabiner variable (`space_held`) で virtual hyper として動作、tap で通常 space
- 物理 modifier キー（cmd/ctrl/opt/shift）と衝突しない設計
- f13/f14 等の特殊キー不要
- comma, period 等の例外措置が機能（typing 中に space+comma が space-binding 発火しない）
- 旧 opt+ 系の projwm/omniwm bindings はすべて space+ に移行
- Quake terminal 廃止
- opt は他用途用に解放
- `space+f` で cockpit toggle
- `space+e` で workspace E (slot) へ jump

### 16.5 アプリ別

- Ghostty Cmd+N: managed workspace 上では Tier 1 カード発火、`[Enter]` で respawn properly。`[i]gnore` 選択肢は無い。
- Vivaldi user profile: 完全無視（origin (a) External 再分類）
- Vivaldi automation profile タブ追加: Tier 3 自動 persist
- Vivaldi automation profile 手動 window 追加: Tier 1 カード発火、`[i]gnore` 無し
- Zed 同一 project 2 つ目: Tier 1 カード発火、デフォルト close、`[i]gnore` 無し
- Zed 別 path: Tier 1 カード発火、`[Enter]` で新規 project 作成、`[i]gnore` 無し
- 管理外 workspace 上の手動追加: 一切干渉しない（Tier 1 発火しない）

### 16.6 Workspace 定義

- 管理対象 = A + Q W E R T Y U I O P
- 管理外 = 1-9, M, B, その他
- 実装側（classifyLiveWindow, planner, invariant）でこの定義が一貫している

---

## 17. 開いている技術判断（実装 Phase で確定）

要件としては合意済みだが実装時に最終確認すべき事項:

- karabiner `to_if_alone_timeout` の最適値（200ms 初期値、実運用で tuning）
- 例外措置の対象キー（comma/period 以外も必要か実運用で判断）
- omniwmctl scratchpad / sticky / always-on-top のどれで workspace 越え overlay を実現するか
- IPC subscribe の wire format（既存 EventHint/EventAck パターン拡張 or 別 stream）
- TUI 実装ライブラリ（bubbletea or tview, etc.）
- monitor 接続変化の検知タイミング（OmniWM display-changed event 経由 or 独自監視）

これらは実装 Phase で判断するため、要件として固定しない。

---

## 18. 後続検証タスク（Phase 末尾でやる）

実装の最初では入れず、メイン要件が一通り動いた後にサブとして検証・必要なら修正する項目:

### 18.1 管理対象外アプリが managed workspace に居座る場合の planner 挙動

**懸念**: origin (a) External 分類された外部アプリ（Calculator 等）が managed workspace に手動で置かれた場合、planner の column 並び替え（Tier 2 自動上書き）や spawn / move 操作で **整合性が崩れる可能性**がある。

**対応**:
- メイン要件実装完了後、シナリオテストで再現を試みる
- 問題が出れば、ルール追加「管理外アプリが managed workspace に来たら Tier 4 で外に出す（または cockpit カード通知）」を実装
- planner が origin (a) External windows を明示的にハンドルできることを確認

### 18.2 Workspace E の管理内化が最新実装で完全に反映されているか検証

`classifyLiveWindow`, planner, invariant など、workspace E を管理内 slot として正しく扱っているかを確認。  
誤って管理外扱いされているコード箇所が見つかれば修正する。

### 18.3 grouped tmux session for cockpit の同期挙動

cockpit を grouped session で複数モニタ表示する際の入力衝突 / 表示遅延 / リサイズ挙動を実機で確認。問題があれば代替方式（同一 session への独立 attach、または同期 broadcast 機構）を検討。

---

## 19. 確認すべきオープン項目

（現在該当項目なし。すべて §1-§18 で確定済み）
