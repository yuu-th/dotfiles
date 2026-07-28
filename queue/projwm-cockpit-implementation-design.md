# projwm-next Cockpit & CLI 実装方針・設計 v3

> 要件: `queue/projwm-cockpit-requirements.md` (v2.3)  
> v1 で先延ばしにしていた 14 項目を実機検証 + 設計確定。  
> v3 で 5 件の網羅性検証 (V1-V5) を反映。

---

## 0. 改版履歴

- **v1**: 初版
- **v3**: 5 件の網羅性検証 (V1-V5) 反映
  - V1: 既存 karabiner binding 4 件 (alt+ctrl+m / alt+ctrl+hjkl / alt+s / alt+c) の space+ 移行を追加、cmd+h block 維持を明記
  - V2: 旧 projwm TUI 全機能カバレッジ確認 (Story 7 Quake は意図的廃止)
  - V3: cockpit visibility を Vision A (park ws 切替) で統一、§3.10 と T7 の整合、cockpit-park を 4 → 6 個に拡張
  - V4: 要件 v2.3 §16 全項目チェック完了
  - V5: Tier 2 を `intent.AutoSyncLayout` 経由に変更 (single-writer invariant 維持)、影響範囲 16 ファイル明示
- **v2**: 14 項目の検証完了
  - T1: OmniWM cockpit visibility → floating + per-display park workspace + switch-workspace anywhere
  - T2: grouped tmux → 完全同期確認、`window-size smallest` で sizing
  - T3: Tier 4 revert → **planner に既に実装済み** ([MOVED] card のみ追加要)
  - T4: Workspace E → 既に管理内 slot として定義済み、修正不要
  - T5: Vivaldi profile 識別 → ps + PID キャッシュ、3ms/call
  - T6: Vivaldi tab observation → 188ms/call、event trigger + 5s debounce + diff
  - T7: cockpit-toggle script → show/hide/toggle 3 スクリプト分離設計
  - T8: PrivatePayloadStore 自動更新 → SyncBrowserTabs intent + token rotation
  - T9: projwm jump → SLOT/PROFILE/PROJECT/WORKSPACE 4 段階解決
  - T10: bubbletea + IPC → tea.Program.Send パターン
  - T11: karabiner timeout → 200ms 初期、Phase 5 後 tuning
  - T12: doctor checks → 14 check インタフェース
  - T13: app-rules title → 既存 titleRegex 機能で実装可
  - T14: monitor 変化 → omniwmctl subscribe display-changed + spawner diff

---

## 1. 全体アーキテクチャ変更概観

新規追加物:

| 種別 | 追加内容 |
|---|---|
| バイナリ | `cmd/projwm/` (Layer 2 CLI)、`cmd/projwm-cockpit/` (Layer 3 TUI) |
| IPC メッセージ | `MsgQueryRequest` / `MsgQueryResponse` / `MsgSubscribe` / `MsgSubscriptionPush` / `MsgSubscriptionCancel` |
| Intent | 12 種類追加 (10 機能 + SyncBrowserTabs + AutoSyncLayout) |
| Controller meta | `ActiveCards []Card`、`PendingOrphans` |
| Adapter 拡張 | `vivaldiClassifier` (PID キャッシュ)、`BrowserTabsSync` (新コンポーネント) |
| Nix 拡張 | `projwm` / `projwm-cockpit` の buildGoModule subpackage、cockpit launchd integration、karabiner 全面書き直し、Quake 無効化、cockpit-park workspace 4 個追加、cockpit-toggle script |

既存への破壊変更:

- `intent.AcceptManualLayout` 削除
- `world.ManualLayoutCandidate` 削除
- `ControllerMeta.ManualLayoutCandidates` 削除
- `classifyLiveWindow`: Vivaldi に PID-based profile 判定追加
- Reducer: `ReactToEvent` で Tier 1/2 を直接ハンドル、ManualLayoutCandidate 経由を廃止
- Planner: ManualLayoutCandidate 参照削除 (Tier 4 revert は既存実装で自動動作)

---

## 2. ファイル変更マトリクス

```
modules/darwin/projwm/projwm-next/
├── cmd/
│   ├── projwmd/main.go              [MODIFY] cockpit spawn、handleConn 拡張、ConnHub、CockpitDisplayWatcher
│   ├── projwmctl/main.go            [KEEP]   debug 用に残す
│   ├── projwmevent/main.go          [KEEP]
│   ├── projwmstore-bootstrap/main.go [KEEP]
│   ├── projwm/                      [NEW]    Layer 2 CLI
│   │   ├── main.go
│   │   ├── cmd_status.go / cmd_doctor.go / cmd_trace.go
│   │   ├── cmd_project.go / cmd_profile.go / cmd_archive.go
│   │   ├── cmd_jump.go / cmd_reconcile.go / cmd_tui.go
│   │   ├── client.go / store_reader.go / render.go
│   │   └── doctor_checks.go         (T12 で具体化した 14 check)
│   └── projwm-cockpit/              [NEW]    Layer 3 TUI
│       ├── main.go
│       ├── model.go / update.go / view.go
│       ├── ipc.go                   (T10 で具体化した tea.Program.Send パターン)
│       ├── keymap.go / search.go
│       └── cards.go
├── internal/
│   ├── controller/controller.go     [MODIFY] ActiveCards 管理、ConnHub.Broadcast 呼出、internal intent 対応
│   ├── intent/intent.go             [MODIFY] 新 intent 11 種、AcceptManualLayout 削除
│   ├── reducer/reducer.go           [MODIFY] 新 intent 分岐、Tier 1 orphan、Tier 2 layout 自動上書き、Tier 4 card emit
│   ├── planner/planner.go           [MODIFY] ManualLayoutCandidate 参照削除のみ (Tier 4 既存実装維持)
│   ├── ipc/ipc.go                   [MODIFY] 新 envelope type、DecodeIntent 拡張
│   ├── world/observed.go            [MODIFY] ManualLayoutCandidate 削除、ActiveCards 追加、PendingOrphans 追加
│   ├── world/desired.go             [KEEP]   構造変更なし (URLPayloadRefs は既存)
│   ├── adapter/wm/sigwm.go          [MODIFY] classifyLiveWindow に vivaldiClassifier 統合
│   ├── adapter/browser/vivaldi.go   [MODIFY] InspectTabsByPID 追加、profile-directory cache
│   ├── adapter/observer/observer.go [MODIFY] CockpitDisplayWatcher hook、BrowserTabsSync 連携
│   ├── adapter/observer/browser_tabs.go [NEW] BrowserTabsSync (Tier 3)
│   └── store/*.go                   [KEEP]
└── scenarios/real_acceptance_test.go [UPDATE] 新 intent/挙動 検証、accept-manual-layout テスト削除
```

```
modules/darwin/projwm/default.nix    [MODIFY] subPackages 追加、projwm wrapper、cockpit plist 統合 (projwmd へ args)、cockpit-park workspace 4 個 (CP1-CP4)
modules/darwin/omniwm/
├── common.nix                       [MODIFY] quakeTerminal.enabled = false
├── app-rules.nix                    [MODIFY] cockpit Ghostty 用 titleRegex rule 追加
├── karabiner-rules.nix              [REWRITE] space_held variable 方式
├── default.nix                      [MODIFY] cockpit toggle script 追加
└── scripts/
    ├── cockpit-show.sh              [NEW]    T7 設計通り
    ├── cockpit-hide.sh              [NEW]    T7 設計通り
    └── cockpit-toggle.sh            [NEW]    T7 設計通り
modules/darwin/karabiner/karabiner.json [MODIFY] to_if_alone_timeout=200ms 設定
```

---

## 3. コンポーネント設計

### 3.1 `cmd/projwm/` — Layer 2 ユーザ向け CLI

サブコマンド → 内部処理マッピング (T9 と T12 で具体化済み):

| Subcommand | 実装方式 |
|---|---|
| `projwm status [--json]` | IPC Query (world) → fallback ストア直読み |
| `projwm doctor` | T12 の 14 check 順次実行 |
| `projwm trace [--last\|<txid>]` | IPC Query (trace) → fallback ストア trace dir 読み |
| `projwm up` | CreateProject + AssignProject 合成 |
| `projwm add-*` / `remove` | AddWindow / RemoveWindow intent |
| `projwm profile *` | 各 intent (Create/Delete/Rename/Switch/Assign/Unassign) |
| `projwm archive` / `unarchive` | ArchiveProject / UnarchiveProject (既存) |
| `projwm archive list` | ストア直読み |
| `projwm archive purge` | DeleteProject{Purge:true} |
| `projwm jump` | T9 の 4 段階解決 |
| `projwm reconcile [--dry-run]` | Reconcile (既存) or planner-only |
| `projwm tui` | cockpit 起動 or show トリガー |

### 3.2 `cmd/projwm-cockpit/` — Layer 3 TUI

T10 で確定:
- bubbletea v0.25+
- `tea.Program.Send` パターン (goroutine から直接 Msg 送信)
- IPC subscribe で daemon push を受信
- Model: WorldSnapshot, ActiveCards, Mode (Idle/Search/Modal/Card), visibility

```go
func main() {
    m := initialModel()
    p := tea.NewProgram(m, tea.WithAltScreen())
    
    ipcClient := newIPCClient(socketPath)
    go func() {
        for push := range ipcClient.pushChan {
            p.Send(push)
        }
    }()
    
    p.Run()
}
```

### 3.3 IPC 拡張

新メッセージタイプ (cmd/projwmd/main.go handleConn 拡張):

```go
const (
    MsgQueryRequest       MessageType = "query-request"
    MsgQueryResponse      MessageType = "query-response"
    MsgSubscribe          MessageType = "subscribe"
    MsgSubscriptionPush   MessageType = "subscription-push"
    MsgSubscriptionCancel MessageType = "subscription-cancel"
)
```

handleConn:
- Intent / Event: 単発 (既存セマンティクス維持)
- Query: 接続 1 回で複数回可
- Subscribe: 接続 1 回で push を継続受信

ConnHub:
- subscribe を持つ Subscriber を map で管理
- Controller.commit 後に hub.Broadcast 呼び出し
- Outbox channel buffer 64 で backpressure 制御

Read query は immutable な generation snapshot を store から読む (RWMutex 化不要)。

### 3.4 新 intent と reducer 拡張

12 種類の intent 追加 (`intent/intent.go`):

```go
type CreateProject struct { ID ProjectID; Path string; Windows []world.DesiredWindow }
type DeleteProject struct { ID ProjectID; Purge bool }
type AddWindow struct { Project ProjectID; Kind world.WindowKind; Index int }
type RemoveWindow struct { Project ProjectID; WindowID world.DesiredWindowID }
type CreateProfile struct { ID ProfileID; Description string; InactivePolicy world.InactivePolicy }
type DeleteProfile struct { ID ProfileID }
type RenameProfile struct { Old, New ProfileID }
type AdoptOrphanWindow struct { LiveID world.LiveWindowID; AsProject ProjectID; AsKind world.WindowKind }
type DismissOrphanWindow struct { LiveID world.LiveWindowID; Action string } // "close"
type RespawnOrphanGhostty struct { LiveID world.LiveWindowID; AsProject ProjectID; AsKind world.WindowKind }
type SyncBrowserTabs struct { Project ProjectID; NewPayloadRef PrivatePayloadRef; URLCount, InvalidCount int }
type AutoSyncLayout struct { Project ProjectID; Workspace WorkspaceID; Columns []DesiredColumn }
```

**重要**: `SyncBrowserTabs` と `AutoSyncLayout` は **internal intent** で、daemon が自分自身に submit する。  
External event (layout-changed, browser tab change) は reducer.ReactToEvent で DirtyScope を返すのみ。  
Controller がポスト処理で DirtyScope を検出 → 該当 internal intent を ApplyIntent → DesiredWorld 更新。  
これにより「外部イベントは DesiredWorld を直接書き換えない」 single-writer invariant を維持。

reducer に各 case 追加。AcceptManualLayout は削除。

### 3.5 Tier 1/2/4 イベント駆動検出

#### Tier 1: 手動 window 追加検知 (5 秒 grace period)

reducer.ReactToEvent の windows-changed 処理時:
```go
for id, ow := range cur.Windows {
    if _, existed := prev.Windows[id]; existed { continue }
    if !isManagedWorkspace(ow.Workspace, env) { continue }  // 管理外スキップ
    if isManagedKind(ow.Kind) && ow.MatchedTo == nil {
        // PendingOrphans に積む (DetectedAt = now)
        meta.PendingOrphans = append(meta.PendingOrphans, OrphanCandidate{
            LiveID: id, Kind: ow.Kind, Workspace: ow.Workspace, DetectedAt: now,
        })
    }
}
```

Controller.reconcile 後に ProcessPendingOrphans:
- DetectedAt から 5 秒経過 + 未 matched → ActiveCards に [NEW] card 追加 → ConnHub.Broadcast
- matched / closed → PendingOrphans から除去

#### Tier 2: column 並び替え自動上書き (internal intent 経由)

reducer.ReactToEvent の layout-changed 処理時、**DirtyScope のみ返す** (DesiredWorld 直接書き換えない):
```go
case event.KindLayoutChanged:
    for ws := range observed.Layouts {
        if !isManagedWorkspace(ws, env) { continue }
        proj := identifyProjectForWorkspace(state.Desired, ws)
        if proj == "" { continue }
        reaction.DirtyScopes = append(reaction.DirtyScopes, DirtyScope{
            Kind: "layout-sync", Project: proj, Workspace: ws,
        })
    }
```

Controller のポスト処理で DirtyScope を検出、internal intent を auto-submit:
```go
// in Controller after ApplyEvent
for _, scope := range reaction.DirtyScopes {
    if scope.Kind == "layout-sync" {
        cols := buildColsFromObserved(observed.Layouts[scope.Workspace], observed.Windows)
        c.ApplyIntent(ctx, intent.AutoSyncLayout{
            Project: scope.Project, Workspace: scope.Workspace, Columns: cols,
        })
    }
}
```

reducer.ReduceIntent (AutoSyncLayout):
```go
case intent.AutoSyncLayout:
    state.Desired.AcceptedLayouts[in.Project][in.Workspace] = DesiredLayout{Columns: in.Columns}
    return state, nil
```

これで「外部イベントは DesiredWorld を直接書き換えない」 invariant が維持される。  
intent.AcceptManualLayout は不要 → 削除。

#### Tier 4: cross-workspace move card emit (revert は planner 既存実装)

reducer.ReactToEvent の windows-changed 処理時:
```go
for id, ow := range cur.Windows {
    prevOw, ok := prev.Windows[id]
    if !ok { continue }
    if prevOw.Workspace == ow.Workspace { continue }
    if ow.MatchedTo == nil { continue }
    desiredWS := lookupDesiredWorkspace(state.Desired, *ow.MatchedTo)
    if ow.Workspace == desiredWS { continue }  // 偶然 desired に着地
    // [MOVED] card 追加
    cards = append(cards, NewMovedCard(id, prevOw.Workspace, ow.Workspace, desiredWS))
}
```

T3 で確認: planner は `ow.Workspace != workspace` 検知で MoveWindowToWorkspace op を既に発行する。だから revert は次の reconcile cycle で自動実行される。

### 3.6 Spawn race 対策 (grace period 方式)

T1 で言及した spawn token は不採用。5 秒 grace period のみで対応:
- 新規 observed window は即座に Tier 1 候補にしない
- 5 秒以内に identity resolver が MatchedTo を set すれば silent adopt
- 5 秒経過後も unmatched → Tier 1 card 発火

これで projwm 自身の spawn-settle race condition は自然に解消。

### 3.7 Vivaldi 改修

T5 で確定:
- `internal/adapter/wm/sigwm.go` に vivaldiClassifier 追加 (PID キャッシュ付)
- ps -p <PID> -o args= で `--profile-directory=projwm-next` を検出
- 検出時 → WindowBrowser (managed)
- 非検出 → WindowExternal (origin a)
- Vivaldi PID 数 通常 1-2、初回 3ms/call、キャッシュ後 0ms

T6 で確定:
- `internal/adapter/browser/vivaldi.go` に `InspectTabsByPID` 追加
- `internal/adapter/observer/browser_tabs.go` 新規: BrowserTabsSync
- イベント駆動 (windows-changed + focus-changed) + 5s debounce + URL diff
- T8 の SyncBrowserTabs intent 経由で DesiredWorld 更新

### 3.8 Workspace E

T4 で確定: 既に E は managed slot として定義済み。コード/manifest/karabiner 全て一貫。修正不要。

### 3.9 Card 管理サブシステム

ControllerMeta:
```go
type ControllerMeta struct {
    Epoch          Epoch
    DirtyScopes    []DirtyScope
    PendingEvents  []EventID
    ActiveCards    []Card                       // NEW
    PendingOrphans []OrphanCandidate            // NEW
    // ManualLayoutCandidates 削除
}

type Card struct {
    ID         CardID
    Type       CardType  // "NEW", "CLOSED", "MOVED", "REPLAN", "INVARIANT", "MANIFEST", "ORPHAN"
    Subject    string
    Context    map[string]string
    Actions    []CardAction
    CreatedAt  time.Time
}
```

ActiveCards は in-memory only (再起動で消える)。永続化不要 (orphan は再観測で再生成、INVARIANT は次回 check で再発火等)。

Card 操作:
- 作成: reducer.ReactToEvent (Tier 1/2/4 検知時) または controller (invariant violation 等)
- dismiss: AdoptOrphanWindow / DismissOrphanWindow / RespawnOrphanGhostty intent 経由 (intent 成功で関連 card 自動削除)
- 個別 dismiss: 新 intent `DismissCard{CardID}` (Esc キー対応)
- 一括 dismiss: 新 intent `DismissAllCards` (Ctrl+L 対応)

ConnHub.Broadcast:
- card 追加時: `card-added` event push
- card 削除時: `card-removed` event push

### 3.10 Cockpit lifecycle 管理

T1/T7/T14 で確定 + V3 で Vision A 統一:

**設計方針 (Vision A)**: 表示時に各 display を cockpit-park workspace に切り替える方式。  
- park ws (CP1-CP6) は manifest で N=6 個準備 (Mac 最大 display 数 6 に対応)
- show: 各 display で `omniwmctl command switch-workspace anywhere <CP_n>` 実行
- ユーザの workspace は一時的に置き換わる (cockpit のみ visible)
- hide: state file から prev workspace を読み戻して `switch-workspace anywhere <prev>`

代替案 (Vision B = floating overlay) は採用しない理由: floating window の position/重なりが複雑、cockpit 操作中の visual disturbance が大きい。


```go
// cmd/projwmd/main.go の起動シーケンスに追加
type CockpitManager struct {
    tmuxBase         string  // "projwm-cockpit"
    ghosttyApp       string  // /Applications/Ghostty.app
    cockpitBin       string  // /nix/store/.../bin/projwm-cockpit
    parkWorkspaces   []string  // ["CP1", "CP2", "CP3", "CP4", "CP5", "CP6"]  (V3: 4→6 拡張)
    activePerDisplay map[string]activeMonitor  // display ID → {parkWS, cockpitWinID}
    mu sync.Mutex
}

func (m *CockpitManager) Start(ctx context.Context) error {
    // 1. ensure tmux base session
    exec.Command("tmux", "new-session", "-d", "-s", m.tmuxBase, m.cockpitBin).Run()
    exec.Command("tmux", "set-option", "-t", m.tmuxBase, "window-size", "smallest").Run()
    
    // 2. spawn per current display
    displays := queryDisplays()
    for i, d := range displays {
        m.spawnForDisplay(ctx, d.ID, m.parkWorkspaces[i])
    }
    
    // 3. start display-changed watcher
    go m.watchDisplayChanges(ctx)
    return nil
}

func (m *CockpitManager) spawnForDisplay(ctx context.Context, displayID, parkWS string) {
    // switch park workspace to active on this display
    exec.Command("omniwmctl", "command", "switch-workspace", "anywhere", parkWS).Run()
    // spawn Ghostty grouped to base tmux
    cloneSession := fmt.Sprintf("projwm-cockpit-%s", sanitizeID(displayID))
    title := fmt.Sprintf("projwm-cockpit-D%s", sanitizeID(displayID))
    exec.Command("open", "-na", m.ghosttyApp, "--args",
        "--title="+title,
        "-e", "tmux", "new-session", "-A",
        "-s", cloneSession,
        "-t", m.tmuxBase).Run()
}
```

cockpit toggle は shell scripts (T7):
- `omniwm-cockpit-show.sh`
- `omniwm-cockpit-hide.sh`
- `omniwm-cockpit-toggle.sh`

Karabiner: `space+f` → `omniwm-cockpit-toggle.sh`

Force show (system 提案カード発生):
- projwmd → ConnHub.Broadcast("force-attention")
- cockpit TUI が `omniwm-cockpit-show.sh` を発火 (state ignore)
- cockpit window が focus

### 3.11 Karabiner 設定リライト

```nix
# modules/darwin/omniwm/karabiner-rules.nix
{ omniwmctl, projwmCli, cockpitToggleScript, ... }:
let
  spaceHolderRule = {
    description = "Spacebar as virtual hyper modifier";
    manipulators = [{
      type = "basic";
      from = { key_code = "spacebar"; modifiers.optional = [ "any" ]; };
      parameters = { "basic.to_if_alone_timeout_milliseconds" = 200; };
      to = [{ set_variable = { name = "space_held"; value = 1; }; }];
      to_after_key_up = [{ set_variable = { name = "space_held"; value = 0; }; }];
      to_if_alone = [{ key_code = "spacebar"; }];
    }];
  };
  
  spaceBinding = key: shellCmd: shiftCmd: {
    description = "space+${key}";
    manipulators =
      (lib.optional (shiftCmd != null) {
        type = "basic";
        conditions = [{ name = "space_held"; type = "variable_if"; value = 1; }];
        from = { key_code = key; modifiers.mandatory = [ "shift" ]; };
        to = [{ shell_command = shiftCmd; }];
      }) ++ [{
        type = "basic";
        conditions = [{ name = "space_held"; type = "variable_if"; value = 1; }];
        from = { key_code = key; modifiers.optional = [ "any" ]; };
        to = [{ shell_command = shellCmd; }];
      }];
  };
  
  passThroughRule = key: {
    description = "${key} passthrough when space_held";
    manipulators = [{
      type = "basic";
      conditions = [{ name = "space_held"; type = "variable_if"; value = 1; }];
      from = { key_code = key; modifiers = { mandatory = []; optional = []; }; };
      to = [{ key_code = key; }];
    }];
  };
  
  ctl = args: "${omniwmctl} ${args}";
  pwm = args: "${projwmCli}/bin/projwm ${args}";
in [
  spaceHolderRule
  passThroughRule "comma"
  passThroughRule "period"
  
  # ── projwm workspace jump + window move (slot/viewer/general WS) ──
  (spaceBinding "q" (ctl "workspace focus-name Q") "${moveWinToWS}/bin/move-to-ws Q")
  (spaceBinding "w" (ctl "workspace focus-name W") "${moveWinToWS}/bin/move-to-ws W")
  (spaceBinding "e" (ctl "workspace focus-name E") "${moveWinToWS}/bin/move-to-ws E")
  (spaceBinding "r" (ctl "workspace focus-name R") "${moveWinToWS}/bin/move-to-ws R")
  (spaceBinding "t" (ctl "workspace focus-name T") "${moveWinToWS}/bin/move-to-ws T")
  (spaceBinding "y" (ctl "workspace focus-name Y") "${moveWinToWS}/bin/move-to-ws Y")
  (spaceBinding "u" (ctl "workspace focus-name U") "${moveWinToWS}/bin/move-to-ws U")
  (spaceBinding "i" (ctl "workspace focus-name I") "${moveWinToWS}/bin/move-to-ws I")
  (spaceBinding "o" (ctl "workspace focus-name O") "${moveWinToWS}/bin/move-to-ws O")
  (spaceBinding "p" (ctl "workspace focus-name P") "${moveWinToWS}/bin/move-to-ws P")
  (spaceBinding "a" (ctl "workspace focus-name A") "${moveWinToWS}/bin/move-to-ws A")
  (spaceBinding "m" (ctl "workspace focus-name M") "${moveWinToWS}/bin/move-to-ws M")
  (spaceBinding "b" (ctl "workspace focus-name B") "${moveWinToWS}/bin/move-to-ws B")
  
  # ── numeric workspace 1-9 ──
  (spaceBinding "1" (ctl "command switch-workspace 1") (ctl "command move-to-workspace 1"))
  (spaceBinding "2" (ctl "command switch-workspace 2") (ctl "command move-to-workspace 2"))
  (spaceBinding "3" (ctl "command switch-workspace 3") (ctl "command move-to-workspace 3"))
  (spaceBinding "4" (ctl "command switch-workspace 4") (ctl "command move-to-workspace 4"))
  (spaceBinding "5" (ctl "command switch-workspace 5") (ctl "command move-to-workspace 5"))
  (spaceBinding "6" (ctl "command switch-workspace 6") (ctl "command move-to-workspace 6"))
  (spaceBinding "7" (ctl "command switch-workspace 7") (ctl "command move-to-workspace 7"))
  (spaceBinding "8" (ctl "command switch-workspace 8") (ctl "command move-to-workspace 8"))
  (spaceBinding "9" (ctl "command switch-workspace 9") (ctl "command move-to-workspace 9"))
  
  # ── cockpit ──
  (spaceBinding "f" "${cockpitToggleScript}/bin/omniwm-cockpit-toggle" null)
  
  # ── 既存 OmniWM 機能の opt+ → space+ 移行 (V1 で発見) ──
  # alt+ctrl+m → space+ctrl+m (setup media workspace)
  {
    description = "space+ctrl+m: setup media workspace";
    manipulators = [{
      type = "basic";
      conditions = [{ name = "space_held"; type = "variable_if"; value = 1; }];
      from = { key_code = "m"; modifiers.mandatory = [ "control" ]; };
      to = [{ shell_command = "${setupMedia}/bin/omniwm-setup-media-workspace"; }];
    }];
  }
  # alt+ctrl+h/j/k/l → space+ctrl+h/j/k/l (focus-monitor direction)
  {
    description = "space+ctrl+h: focus-monitor left";
    manipulators = [{
      type = "basic";
      conditions = [{ name = "space_held"; type = "variable_if"; value = 1; }];
      from = { key_code = "h"; modifiers.mandatory = [ "control" ]; };
      to = [{ shell_command = "${focusMonitorDir}/bin/omniwm-focus-monitor-dir left"; }];
    }];
  }
  # ... j (down), k (up), l (right) も同様
  
  # alt+s → space+s (WS M + Spotify)
  (spaceBinding "s" "${wsLaunch}/bin/omniwm-ws-launch M Spotify" null)
  # alt+c → space+c (WS M + Discord)
  (spaceBinding "c" "${wsLaunch}/bin/omniwm-ws-launch M Discord" null)
  
  # ── projwm 以外の維持必要な binding (opt+ ではない、空 migration 対象外) ──
  # cmd+h block (Hide ブロック)
  {
    description = "block cmd-h (macOS Hide)";
    manipulators = [{
      type = "basic";
      from = { key_code = "h"; modifiers.mandatory = [ "left_command" ]; };
      to = [{ key_code = "vk_none"; }];
    }];
  }
  # cmd+alt+h block (Hide Others)
  {
    description = "block cmd-alt-h (macOS Hide Others)";
    manipulators = [{
      type = "basic";
      from = { key_code = "h"; modifiers.mandatory = [ "left_command" "option" ]; };
      to = [{ key_code = "vk_none"; }];
    }];
  }
]
```

旧 opt+ 系完全削除。Quake terminal 無効化:
```nix
# modules/darwin/omniwm/common.nix
quakeTerminal.enabled = false;
```

---

## 4. Phase 計画

### Phase 1: Layer 2 CLI 骨格 (〜2 日)

成果物:
- `cmd/projwm/` 新規バイナリ
- `projwm status / doctor / profile list / archive list / trace` 実装 (T12 の 14 check 含む)
- `projwm profile switch / archive / unarchive / reconcile / validate-environment` (既存 intent ラッパー)
- IPC client + ストア直読み fallback
- Nix module で `projwm` バイナリ追加

検証:
- 既存 daemon と通信して status 表示
- 既存 daemon に intent 送信成功
- daemon 停止状態でも status 表示

### Phase 2: 新 intent + 状態変更モデル (〜3 日)

成果物:
- intent.go に 11 種類追加 (10 + SyncBrowserTabs)
- reducer.go に分岐追加、AcceptManualLayout 削除
- planner.go から ManualLayoutCandidate 参照削除
- ControllerMeta に ActiveCards, PendingOrphans 追加
- classifyLiveWindow 修正 (vivaldiClassifier 統合、T5)
- WindowExternal 単一 origin 化
- Tier 1 (5s grace period)、Tier 2 (自動上書き)、Tier 4 ([MOVED] card 発火) 実装
- `projwm` CLI で新 intent を呼べる
- 既存テスト更新 (accept-manual-layout 削除対応)

検証:
- `projwm up --ai claude --slot Q dotfiles` で project + assign 成功
- 手動で別 workspace に window を移動 → 自動で戻る + [MOVED] card
- 手動で column 並び替え → desired に自動反映
- 手動で managed window 閉じ → 自動 respawn + [CLOSED] card
- Vivaldi user profile が External 扱い
- Cmd+N で Ghostty 開く → 5 秒後に [NEW] card

### Phase 3: IPC 拡張 + Card subsystem (〜2 日)

成果物:
- ipc.go に新 envelope type
- cmd/projwmd/main.go の handleConn を multi-message ループ化
- cmd/projwmd/conn_hub.go 新規
- Controller.commit 内で ConnHub.Broadcast
- `projwm` CLI が Query 経由で world snapshot 取得
- Card add/remove ロジック実装

検証:
- `projwm` CLI が daemon 経由で current world 取得
- Tier 1 候補 5 秒後に cockpit 向け push 発行
- 複数の subscribe 接続が同時動作

### Phase 4: Cockpit TUI + lifecycle (〜4 日)

成果物:
- `cmd/projwm-cockpit/` 新規バイナリ (bubbletea + tea.Program.Send パターン)
- §9.5 keybind 全実装
- fzf-style 検索実装
- CockpitManager in projwmd (T14 設計通り)
- cockpit-show/hide/toggle shell scripts (T7 設計通り)
- OmniWM app-rule 追加 (T13)
- BrowserTabsSync 統合 (T6/T8)

検証:
- macOS 起動時から cockpit 全 monitor に常駐
- monitor plug/unplug で動的追従
- `space+f` で show/hide (Phase 5 後)
- 提案カード発生時に強制表示 + focus 移動
- Vivaldi タブ変更が自動 PrivatePayloadStore に persist

### Phase 5: Karabiner + OmniWM 設定 (〜1 日)

成果物:
- karabiner-rules.nix 全面リライト (space_held variable 方式)
- common.nix の Quake 無効化
- karabiner.json パラメータ調整 (to_if_alone_timeout=200ms)
- 旧 opt+ 系完全削除

検証:
- space 単独 tap で通常 space 入力
- space + q..p / a で workspace jump
- space + shift + q..p で window move
- space + f で cockpit toggle
- typing 中に space + comma/period が誤発火しない
- cmd + s 等の通常ショートカットが space 押下中も生存

### Phase 6: 後続検証タスク (〜2 日)

成果物 (要件 §18):
- 管理外アプリの居座り検証 + planner ハンドリング修正
- Workspace E 完全反映検証
- grouped tmux 同期挙動の実機確認

---

## 5. テスト戦略

### 5.1 Unit test 追加

- `internal/reducer/reducer_orphan_test.go`: Tier 1 orphan 検知 (管理 ws vs 管理外、kind 別、grace period)
- `internal/reducer/reducer_tier4_test.go`: cross-workspace move card emit
- `internal/reducer/reducer_layout_test.go`: Tier 2 自動上書き
- `internal/adapter/wm/sigwm_vivaldi_test.go`: vivaldiClassifier、cache 動作
- `internal/adapter/browser/vivaldi_observe_test.go`: InspectTabsByPID
- `internal/adapter/observer/browser_tabs_test.go`: BrowserTabsSync debounce + diff
- `cmd/projwmd/conn_hub_test.go`: ConnHub 並行 broadcast、backpressure
- `cmd/projwm-cockpit/model_test.go`: bubbletea Model 更新ロジック
- `cmd/projwm/cmd_*_test.go`: 各 subcommand
- `cmd/projwm/doctor_checks_test.go`: 14 check の正常/異常パス

### 5.2 Integration test 修正

- `cmd/projwmd/integration_mutation_test.go`: 新 intent 11 種の transaction cycle
- `cmd/projwmd/integration_real_test.go`: Query/Subscribe handshake、ConnHub broadcast

### 5.3 E2E test 修正

- `scenarios/real_acceptance_test.go`:
  - `TestHumanE2EAcceptManualLayoutSteps` 削除
  - `TestHumanE2EAutoLayoutOverwrite` 新規 (Tier 2)
  - `TestHumanE2EManagedWindowCrossWorkspaceMoveSteps` 改修 (Tier 4 revert + card)
  - `TestHumanE2EVivaldiUserProfileIgnored` 新規
  - `TestHumanE2EOrphanGhosttyTier1Card` 新規
  - `TestHumanE2ECockpitVisibilityToggle` 新規 (実機)
  - `TestHumanE2ESpaceHyperKeybind` 新規 (Karabiner 検証)

---

## 6. 解決済み技術判断 (T1-T14)

| ID | 内容 | 結論 |
|---|---|---|
| T1 | OmniWM cockpit visibility | floating + per-display park workspace + switch-workspace anywhere |
| T2 | grouped tmux 入力同期 | 完璧に同期、window-size smallest 設定 |
| T3 | Tier 4 revert planner 統合 | **既存実装で完了**、card emit のみ追加 |
| T4 | Workspace E 管理内ステータス | 既に正しい、修正不要 |
| T5 | Vivaldi profile 識別 | ps + PID キャッシュ、3ms/call |
| T6 | Vivaldi tab observation cost | 188ms/call、event-trigger + 5s debounce + diff |
| T7 | cockpit-toggle script | show/hide/toggle 3 スクリプト、state file 経由 |
| T8 | PrivatePayloadStore 自動更新 | SyncBrowserTabs intent + token rotation |
| T9 | projwm jump 解決 | SLOT → PROFILE → PROJECT → WORKSPACE 4 段階 |
| T10 | bubbletea + IPC subscribe | tea.Program.Send パターン |
| T11 | karabiner to_if_alone_timeout | 200ms 初期、Phase 5 後 tuning |
| T12 | projwm doctor 検査項目 | 14 check インタフェース |
| T13 | OmniWM app-rules title 一致 | 既存 titleRegex 機能で OK |
| T14 | monitor 動的追従 | omniwmctl subscribe display-changed + spawner diff |

---

## 7. 未解決リスク

1. **karabiner `to_if_alone_timeout` 最適値**: 200ms 初期、実運用で iteration 必要。Phase 5 後 1 週間使用後に確定。
2. **OmniWM scratchpad の cockpit-park park 工程**: 4 park workspace を起動時に各 display に割り当てる順序が決定的に動くか実機確認必要。
3. **bubbletea grouped tmux client での描画**: 各クライアントの screen size が異なる時 `window-size smallest` で描画スムーズか実機確認必要。
4. **PrivatePayloadStore token rotation の race condition**: SyncBrowserTabs intent commit と Forget(oldToken) の順序、failed transaction での cleanup。

これらは Phase 4-5 実装時に再評価。

---

## 8. 完了判定

Phase 1-5 完了 + Phase 6 検証クリアで全要件達成。

各 Phase で:
- 該当成果物動作
- ユニット/integration/E2E テスト全 green
- `nix build .#darwinConfigurations.yuta.config.system.build.toplevel` 通る
- 手動操作で要件 §16 のチェック項目クリア
