# projwm-next + Cockpit 統合設計 (v3)

> projwm 元仕様 (`projwm-spec.md`) と cockpit 拡張要件
> (`projwm-cockpit-requirements.md` v2.4) を **重ね合わせて1枚の絵**
> として捉え、それを既存 projwm-next 基盤 (DesiredWorld → Reducer →
> Planner → Executor + IPC + Store) の単一パターンで実装する設計。
>
> パッチ的な修正の積み重ねを止め、根本的に正しい構造でやり直す。
>
> **v3 (2026-05-17)**: cockpit を「全モニタ同時」から「projwm-managed モニタに 1 つだけ」に縮退。要件 v2.4 §8 の改訂に対応。SystemWindows は固定 1 件、ParkWorkspace=CP1、CP2-CP6 は廃止。Reducer の DisplayChanged ハンドラは cockpit 設置先モニタ消失時の再配置のみ扱う。
>
> **v2**: omniwm scratchpad pool が single-window 専用と判明し、park-workspace 方式 (CP1-CP6 + display-specific switch) に転換。同時に SpawnCockpit pre-focus、 viewer-revert planner 拡張、display-mapping を WorkspaceToDisplay map 経由に統一。
>
> **v1**: scratchpad pool 想定での初版。

---

## 0. 前提: なぜ既存設計が綻んだか

これまで cockpit を **projwm 基盤の外側** に作っていた:
- shell scripts (cockpit-show/hide/toggle.sh) が omniwmctl を直叩き
- CockpitManager (cmd/projwmd/cockpit_manager.go) が独自に process と tmux を管理
- state.json (cockpit-state.json) が別途
- CP1-CP6 park workspaces が manifest に存在するが planner / reducer 経由でなく shell で切替
- app-rules.nix の titleRegex workaround
- race condition + 状態の二重管理

結果:
- daemon の startup lifecycle と cockpit lifecycle が分離していて整合性なし
- ある瞬間に「shell が CP1 へ切替」「daemon の planner が別の workspace へ動かす」の競合
- 識別がうまくいかず planner が動かない / cockpit が 1 つしか出ない / window が M に集まる

要件 §8.3 が明示している:
> cockpit はどの workspace の上でも **overlay** として表示される
> 既存の workspace A (viewer) や他の slot workspace を **占有しない**

私が採用してきた park-workspace 方式はこの要件に **違反**。cockpit が CPn を占有し、他 workspace との重なりを「workspace 切替」で表現していた。

要件 §15 が「実装方式 (scratchpad / sticky / always-on-top のどれを使うか) は技術判断」と書いていた。OmniWM API を改めて読むと **`omniwmctl command scratchpad assign / toggle`** が真の overlay 機構。これを使えば cockpit windows は workspace を占有せず、どの workspace 上にも overlay として現れる。

これが正しい根本構造。

---

## 1. 中心概念: 「世界モデル一本化」

projwm-next の核は:
- **DesiredWorld** が「あるべき世界」の唯一の SoT
- **Reducer** が intent / event を受けて DesiredWorld を変える純粋関数
- **Planner** が DesiredWorld と Observed の差分から operations を計算
- **Executor** が operation を WM に投げる
- **PersistentStore** が DesiredWorld を世代ごとに永続化

これに **cockpit を一級市民として組み込む**。cockpit window は managed window の一種であり、それ以上でも以下でもない。

具体的には:
- **WindowKind = cockpit** (新規) を追加 (既存 ai / shell / editor / browser / viewer / external と並ぶ)
- 各 cockpit window は DesiredWorld 上の `SystemWindows[]` として表現 (`Projects` は profile/project に紐付き、cockpit は project 不問なので別フィールド)
- 1 connected display = 1 cockpit `DesiredWindow`
- cockpit window の "表示状態" は `Visibility ∈ {shown, hidden}` という属性で表現
- shown = scratchpad から外して floating overlay として表示
- hidden = scratchpad へ assign

---

## 2. アーキテクチャ図 (cockpit 統合後)

```
                     ┌──────────────────────────────┐
                     │  Karabiner (simultaneous)    │
                     │  space+letter → projwm CLI   │
                     │  space+f → projwm cockpit *  │
                     └────────────┬─────────────────┘
                                  │
                                  ▼
                     ┌──────────────────────────────┐
                     │  Layer 2: projwm CLI         │
                     │  projwm status/profile/...   │
                     │  projwm cockpit show/hide/   │
                     │           toggle/focus       │
                     └────────────┬─────────────────┘
                                  │ IPC (intent)
                                  ▼
   ┌──────────────────────────────────────────────────────────┐
   │                    projwmd (Layer 1)                     │
   │  ┌──────────┐  ┌────────┐  ┌─────────┐  ┌────────────┐   │
   │  │ Reducer  │→ │Planner │→ │Executor │→ │   sigwm    │   │
   │  └──────────┘  └────────┘  └─────────┘  │ (omniwm IF)│   │
   │       ▲                                  └────────────┘   │
   │       │ intents / events / display-change                 │
   │  ┌──────────┐                                             │
   │  │  Store   │ (CommittedGeneration 永続化)                │
   │  └──────────┘                                             │
   │                                                            │
   │  ConnHub → cockpit subscribers (push card events)         │
   └──────────────────────────────────────────────────────────┘
                                  │ spawn / move / scratchpad
                                  ▼
                     ┌──────────────────────────────┐
                     │      OmniWM (omniwmctl)      │
                     │  - workspaces                │
                     │  - scratchpad pool ← cockpit │
                     │  - app-rules                 │
                     └────────────┬─────────────────┘
                                  │
                                  ▼
                     ┌──────────────────────────────┐
                     │  ai/shell/editor/viewer/     │
                     │  cockpit windows             │
                     └──────────────────────────────┘
```

cockpit window の生死とライフサイクルは **完全に DesiredWorld → Planner → Executor 経路で扱う**。CockpitManager (独自 lifecycle 管理) は **削除**。shell scripts も **削除**。

---

## 3. データモデル拡張

### 3.1 WindowKind: cockpit を追加

```go
const (
    WindowAI       WindowKind = "ai"
    WindowShell    WindowKind = "shell"
    WindowEditor   WindowKind = "editor"
    WindowBrowser  WindowKind = "browser"
    WindowViewer   WindowKind = "viewer"
    WindowExternal WindowKind = "external"
    WindowCockpit  WindowKind = "cockpit"   // 新規
)
```

### 3.2 DesiredWorld.SystemWindows

```go
type DesiredWorld struct {
    ActiveProfile ProfileID
    Profiles      map[ProfileID]DesiredProfile
    Projects      map[ProjectID]DesiredProject
    FocusPolicy   FocusPolicySet
    AcceptedLayouts ...
    SystemWindows []SystemWindow   // 新規
}

// SystemWindow は profile/project に紐付かない常駐 window。
// 現状 cockpit のみだが、将来 status bar / dashboard も同枠で扱える。
type SystemWindow struct {
    ID         SystemWindowID    // e.g. {Kind:cockpit, Index:0}
    Kind       WindowKind
    DisplayIdx int               // 0-based display index
    Title      string            // controller-owned: "projwm-cockpit-D{Index}"
    Visibility CockpitVisibility // shown | hidden
}

type SystemWindowID struct {
    Kind  WindowKind
    Index int        // display index 順
}

type CockpitVisibility string
const (
    CockpitShown  CockpitVisibility = "shown"
    CockpitHidden CockpitVisibility = "hidden"
)
```

### 3.3 ObservedWorld.Windows: 既存のまま

cockpit window が新規 spawn された後、`omniwmctl query windows` から戻ってくる ObservedWindow は `App.BundleID=com.mitchellh.ghostty`, `Title=projwm-cockpit-Dn`, `IsScratchpad` フラグを持つ。

`classifyLiveWindow` は title が `projwm-cockpit-D` で始まる Ghostty を `WindowCockpit` と判定。

---

## 4. Reducer 拡張

### 4.1 Bootstrap event ハンドラ

`reducer.ReactToEvent(event.KindStartup)`:
1. omniwmctl query displays で現 display 一覧を取得 (これは Adapter 経由で Observed に既に入っているはず)
2. Display 数 N に対して、`SystemWindows = [N 個の cockpit SystemWindow]` を構築
3. 各 SystemWindow.DisplayIdx = 0..N-1
4. 各 SystemWindow.Title = `projwm-cockpit-D<idx>`
5. 各 SystemWindow.Visibility = `hidden` (起動時は隠れている — 要件 §8.2 "平時は隠れている")

ただし reducer は pure 関数なので Observed への access は state.Observed 経由のみ可。Bootstrap event 時点で Observed.Displays が populated していれば OK。

### 4.2 DisplayChanged event ハンドラ

`reducer.ReactToEvent(event.KindDisplayChanged)`:
1. 現 SystemWindows.length と Observed.Displays.length を比較
2. 増えていれば末尾に新 cockpit SystemWindow を追加 (Visibility = current 既存 cockpit の状態を継承)
3. 減っていれば末尾から削除 (planner が close op を生成する)

### 4.3 intent 拡張

```go
// 新 intent
type SetCockpitVisibility struct {
    Visibility CockpitVisibility   // shown / hidden
}
type ToggleCockpit struct{}        // 現在の Visibility を反転
type FocusCockpit struct{}         // 現マウスがある display の cockpit に focus
```

Reducer 実装:
- `SetCockpitVisibility{shown}`: 全 SystemWindows.Visibility = shown
- `SetCockpitVisibility{hidden}`: 全 SystemWindows.Visibility = hidden
- `ToggleCockpit`: 1 つでも shown があれば hidden へ、全 hidden なら shown へ
- `FocusCockpit`: visibility 変更しない、planner に "focus cockpit on display X" hint を渡す (新 op kind)

### 4.4 SwitchProfile 等の Navigation intent との連動

要件 §8.4 / §8.5 から:
- Mode 2 (Navigation: jump/profile switch/focus window): 操作後 auto-hide

これは CLI 側で実装:
- `projwm jump ...` → omniwmctl 直叩き + その後 `SetCockpitVisibility{hidden}` intent 送信
- `projwm profile switch ...` → SwitchProfile intent + その後 hidden

CLI が intent を 2 連で送ることで Mode 2 を実現。reducer は intent ベースで純粋に動く。

---

## 5. Planner 拡張

### 5.1 cockpit window の spawn / move / close

既存 planner は Project Windows を扱うのと同じパターンで SystemWindows を扱う:

```go
// 既存パターン (擬似コード)
for project := range desired.Projects:
    for window := range project.Windows:
        if not observed.matches(window):
            plan.add(SpawnOp{...})

// 拡張パターン
for systemWindow := range desired.SystemWindows:
    if not observed.matches(systemWindow):
        plan.add(SpawnSystemWindow{...})
    elif observed.position(systemWindow) != desired.position(systemWindow):
        plan.add(MoveSystemWindow{...})
```

ただし cockpit window の "position" は workspace ではなく **scratchpad 状態**:

| Visibility | scratchpad 状態 | OmniWM 操作 |
|---|---|---|
| `shown`  | scratchpad pool **に居ない** (visible overlay) | `scratchpad toggle` (現在 hidden なら) |
| `hidden` | scratchpad pool **に居る**   (隠れている)   | `scratchpad assign` (新規 spawn 後) / `scratchpad toggle` (現在 shown なら) |

新規 op kinds:
- `SpawnCockpitWindow`: open -na Ghostty + tmux grouped clone + 直後に `scratchpad assign` で hidden 化
- `ShowCockpitAll`: 全 cockpit windows を visible に。実装は `scratchpad toggle` 1 回 (toggle は全 scratchpad windows を一括 toggle するため)
- `HideCockpitAll`: 全 cockpit windows を hidden に。`scratchpad toggle` 1 回
- `CloseCockpitWindow`: window kill (display 切断時)

### 5.2 多 display 同期

OmniWM の scratchpad は **per-display ではなく global**。`scratchpad toggle` で全 cockpit windows が一斉に切り替わる。これが「全モニタ同時 show/hide」(要件 K1.4) の自然な実装。

### 5.3 Tier 4 cockpit window 保護

cockpit window をユーザが手動 close した場合:
- 既存の Tier 4 (managed window user-close → respawn) が発動
- ただし 60s 内 2 回 close で suppress 機構 (T4.4) が cockpit にも適用される

cockpit window を手動で別 workspace へ移動した場合:
- Tier 4 cross-workspace revert が発動 — 元の scratchpad assignment 状態に戻す

---

## 6. Executor / sigwm 拡張

### 6.1 spawnCockpit

既存 `spawnGhostty` を再利用するため、`SpawnRequest.Kind = WindowCockpit` を受け付ける:

```go
case WindowCockpit:
    return s.spawnCockpit(ctx, r)

func (s *SigWM) spawnCockpit(ctx context.Context, r SpawnRequest) (bool, error) {
    // 1. tmux base session `projwm-cockpit` を保証 (なければ作成)
    //    内容は `projwm-cockpit` バイナリ実行
    if exists, _ := s.Tmux.HasSession(ctx, "projwm-cockpit"); !exists {
        s.Tmux.EnsureSession(ctx, "projwm-cockpit", "")
        // session 内で projwm-cockpit binary を起動
    }
    // 2. display-specific grouped clone session
    clone := fmt.Sprintf("projwm-cockpit-D%d", r.DisplayIdx)
    s.Tmux.EnsureGroupedSession(ctx, "projwm-cockpit", clone)
    // 3. Ghostty を `open -na` で起動、grouped session に attach
    args := []string{
        fmt.Sprintf("--title=%s", r.Title),  // projwm-cockpit-D<idx>
        "-e", "tmux", "new-session", "-A", "-s", clone, "-t", "projwm-cockpit",
    }
    if err := s.Launcher.Launch(ctx, ghosttyAppPath, ghosttyBundleID, args); err != nil {
        return false, err
    }
    // 4. spawn 後の post-step: scratchpad assign (Visibility=hidden の場合)
    //    これは planner が SpawnOp の後に AssignScratchpad op を生成して実行
    return false, nil
}
```

### 6.2 scratchpad operations

新規 sigwm メソッド:
```go
func (s *SigWM) AssignScratchpad(ctx context.Context, winID w.LiveWindowID) error {
    // 1. window focus by ID
    s.Exec.Run(ctx, "window", "focus", string(winID))
    // 2. 150ms wait for focus settle
    time.Sleep(150 * time.Millisecond)
    // 3. command scratchpad assign
    _, err := s.Exec.Run(ctx, "command", "scratchpad", "assign")
    return err
}

func (s *SigWM) ToggleScratchpad(ctx context.Context) error {
    _, err := s.Exec.Run(ctx, "command", "scratchpad", "toggle")
    return err
}
```

### 6.3 Identity resolver

cockpit window:
- `App.BundleID == "com.mitchellh.ghostty"`
- `TitleContract.Authority = ControllerOwned`
- `TitleContract.Expected = "projwm-cockpit-D<idx>"`

→ 既存の identity resolver (title exact + bundle-id strong) で一意に解決される。

`is-scratchpad` フラグも `ObservedWindow` に追加し、planner が「期待 = scratchpad 内 / 実 = 違う」差分を検出できるようにする。

---

## 7. IPC 拡張

```go
const (
    KindSetCockpitVisibility Kind = "set-cockpit-visibility"
    KindToggleCockpit        Kind = "toggle-cockpit"
    KindFocusCockpit         Kind = "focus-cockpit"
)
```

`DecodeIntent` に case 追加。projwm CLI から submit。

---

## 8. CLI 拡張

```
projwm cockpit show           # SetCockpitVisibility{shown}
projwm cockpit hide           # SetCockpitVisibility{hidden}
projwm cockpit toggle         # ToggleCockpit
projwm cockpit focus          # FocusCockpit
```

加えて Mode 2 auto-hide のため、既存 navigation 系 CLI を更新:
- `projwm jump <T>`: omniwmctl 直叩き → その後 `cockpit hide` を内部発行
- `projwm profile switch <N>`: SwitchProfile intent → その後 `cockpit hide` を内部発行

---

## 9. Karabiner 配線 (simultaneous mode 維持)

すでに simultaneous モードに切替済。`space+f` → `projwm cockpit toggle`。

---

## 10. Manifest

CP1-CP6 を **完全削除** (projwm manifest からも omniwm workspace 定義からも):
- cockpit は workspace を占有しないので park workspace 不要
- app-rules.nix の cockpit 用 titleRegex も削除 (workspace 割当不要のため)

monitor-profiles の CP1-CP6 エントリも削除。workspace-builder.nix の CP1-CP6 エントリも削除。

これで manifest digest が原型に戻り、store の bootstrap digest と一致する (再 bootstrap 不要)。

---

## 11. 既存削除リスト

新設計が動いたら以下を削除:
- `cmd/projwmd/cockpit_manager.go`
- `cmd/projwmd/cockpit_manager_test.go`
- `modules/darwin/omniwm/scripts/cockpit-show.sh`
- `modules/darwin/omniwm/scripts/cockpit-hide.sh`
- `modules/darwin/omniwm/scripts/cockpit-toggle.sh`
- `modules/darwin/omniwm/scripts/cockpit_test.sh`
- `app-rules.nix` の cockpit 系 4 規則
- `monitor-profiles/*.nix` の CP1-CP6 mapping
- `workspace-builder.nix` の CP1-CP6 entries
- `default.nix` (projwm) の CP1-CP6 manifest entries (もう削除済)

---

## 12. Phase 計画

各 Phase の **完了時に必ず一旦止まる** — その時点で要件文書 (projwm-spec.md + cockpit-requirements.md) を再読し、Phase 内容が要件のどの項目を満たし・どこにギャップがあるかを 0 から書き出す (実装に進む前のチェックポイント)。

### Phase A: 基盤型の追加 (1-2h)
- `WindowKind = "cockpit"` 追加
- `DesiredWorld.SystemWindows` 構造体追加
- `SystemWindow` / `SystemWindowID` / `CockpitVisibility` 定義
- `ObservedWindow.IsScratchpad` フィールド追加
- 既存テストが落ちないこと

**Phase A 後の確認事項**:
- 新型は要件 §8 (cockpit 常駐) を表現できるか
- 既存 (projwm 元仕様) の `(kind, id)` ペアの identity 規則を壊していないか
- SystemWindows vs Projects の分離は妥当か (むしろ pseudo-project が良いか)

### Phase B: Reducer 拡張 (2-3h)
- Bootstrap event → SystemWindows 構築
- DisplayChanged event → SystemWindows 追加/削除
- intent `SetCockpitVisibility` / `ToggleCockpit` / `FocusCockpit` の reducer 実装
- unit tests (mock Observed.Displays)

**Phase B 後の確認事項**:
- 要件 §8.1 (起動時 spawn)、§8.2 (平時 hidden)、§8.4 (3 mode) の挙動が reducer 側で表現できているか
- Tier 4 (cockpit window 手動 close → respawn) は既存 Tier 4 ロジックでカバーされるか

### Phase C: Planner 拡張 (3-4h)
- SystemWindows の spawn / move / close を扱う phase 追加
- Scratchpad assign / toggle op kinds 追加
- Visibility 状態 ↔ scratchpad 状態のマッピング

**Phase C 後の確認事項**:
- 要件 §8.5 hide 動作テーブルが planner レベルで成立するか
- 要件 §8.3 (overlay, workspace 占有しない) を scratchpad で実現できているか
- spawn order / race の可能性

### Phase D: Executor / sigwm 拡張 (2-3h)
- `WindowKind = cockpit` の spawn path
- `AssignScratchpad` / `ToggleScratchpad` operation
- 既存の `spawnGhostty` を活用、tmux grouped session pattern を共有

**Phase D 後の確認事項**:
- 実 OmniWM で `scratchpad assign` / `toggle` がどう動くか実観測 (mock じゃなく実機)
- 多 display で `scratchpad toggle` が全 cockpit を同時に切り替えるか確認

### Phase E: CLI 拡張 (1-2h)
- `projwm cockpit {show,hide,toggle,focus}` subcommand
- Mode 2 auto-hide: jump / profile switch の後に `cockpit hide` を内部発行

**Phase E 後の確認事項**:
- 要件 §5 CLI catalog の全項目が動作するか
- 要件 §6 mapping table 通りの intent 発行か

### Phase F: 旧実装削除 + manifest clean (1-2h)
- CockpitManager / shell scripts / CP1-CP6 削除
- karabiner 設定確認 (`space+f` が `projwm cockpit toggle` を呼ぶ)
- Nix rebuild + daemon restart

**Phase F 後の確認事項**:
- 旧経路が完全に消えているか (grep で cockpit-show.sh / CockpitManager の参照ゼロ)
- daemon log に旧 error が出ないか

### Phase G: 実機検証 (1-2h)
- macOS sleep / wake
- monitor 接続 / 切断
- cockpit プロセス強制 kill → 再 spawn (T4 grace 適用)
- 全 cockpit が overlay として全 display に同時表示
- typing 中に space が遅延なく入力される (simultaneous mode 動作確認)

**Phase G 後の確認事項**:
- 要件 §8.8 lifecycle table を 1 行ずつ実機確認
- 要件 §16 受け入れ基準を再走

---

## 13. 各 Phase で守るべき原則

1. **DesiredWorld 経由でしか mutation しない**: cockpit_manager のような並列 lifecycle を作らない
2. **shell script で omniwmctl を直叩きしない**: 必ず CLI → intent → reducer → planner → executor
3. **state は store/DesiredWorld にのみ**: state.json (cockpit-state.json) など追加しない
4. **既存パターンを優先**: spawnGhostty / EnsureGroupedSession / FocusPolicy を再利用
5. **app-rules workaround を使わない**: window 配置は planner / scratchpad assignment が制御

---

## 14. 受け入れ基準 (実装完了時のチェック)

- [ ] `omniwmctl query windows --scratchpad` で N 個 (display 数) の cockpit ghostty が pool に居る
- [ ] `space+f` で全 display 同時 toggle、scratchpad pool ↔ overlay 表示
- [ ] cockpit 表示中、ユーザが workspace 切替 (space+q 等) しても cockpit visibility 維持
- [ ] cockpit が overlay として slot workspace の managed window と重ならない
- [ ] manifest digest が原型 (4da89a3...) のまま (再 bootstrap 不要)
- [ ] daemon log に「startup lifecycle blocked」「cockpit-manager start failed」が出ない
- [ ] go test ./... 全 green
- [ ] simultaneous モード: typing 中の space に遅延なし
- [ ] macOS sleep → wake で cockpit tmux session 維持、表示状態 hidden 復帰

---

## 14b. Phase B 完了時に発見された残課題

- **Wake 時の Visibility リセット (要件 §8.8 "macOS wake → 表示状態リセット (hidden に戻る)")**: Phase B では Wake event が cockpit-sync DirtyScope を発行するが、count 変化がなければ no-op。Visibility 強制 hidden リセットは未実装。Phase B では SyncCockpitSystemWindows intent は length のみ扱う。Phase B 後に追加すべき: Wake event → 追加で `SetCockpitVisibility{hidden}` を internal-submit するか、reducer.ReactToEvent(Wake) で別 DirtyScope を発行して controller がそれを SetCockpitVisibility 経由で適用。

## 15. 残課題 / 別途検討

- カード機構 (§10): `[NEW]` / `[CLOSED]` / `[MOVED]` 等のカード生成は projwm-next 既存実装でほぼ揃っている。本設計は cockpit 表示部分の改修のみで、カード機構自体はそのまま残す。
- Tier 1 / Tier 3 (Vivaldi タブ): 既存実装をそのまま使う。本設計外。
- macOS notification 廃止: 既に done。
- `[i]gnore` action 廃止: 既に done。

---

これが統合設計。提案で終わらせず、Phase A から実装に入る。各 Phase 終了時に必ずこの文書と要件文書を再読する。
