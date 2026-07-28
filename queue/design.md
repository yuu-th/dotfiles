# projwm-next 設計骨組み

Status: 設計骨組み合意済み。`queue/implementation-design.md` と合わせて `projwm-next` の設計 SSOT とする。

この文書は実装手順ではなく、`projwm-next` の設計骨組みを固定するための文書である。
実装・schema・validator・IPC contract・store contract・tests は、この文書と `queue/implementation-design.md` に従って整合させる。

## 1. 根本方針

`projwm-next` は既存 `projwm` の内部構造を前提にしない。

既存実装から取り込むのはコード構造ではなく、実運用で得た制約と知見だけである。

- OmniWM / niri の workspace・layout・focus の癖
- macOS アプリの spawn / focus / window 識別の癖
- tmux / editor / browser の lifetime 管理の知見
- launchd daemon が複数実行主体になる危険性
- integration test が見つけた race / 不変条件

中心に置くのは `reconcile` ではなく **World Controller** である。

```text
user intent
watch event
system event
manual layout change
        ↓
World Controller
        ↓
World Model
        ↓
Planner / Simulator
        ↓
Executor
        ↓
Observer
        ↺
```

## 2. 設計上の大きな転換

### 2.1 既存環境に受け身で合わせない

`projwm-next` は OmniWM/niri の現在設定にただ合わせるだけではない。

堅牢に実装しやすくするため、必要なら以下のような環境パラメータ自体を変更してよい。

- `maxVisibleColumns`
- `maxWindowsPerColumn`
- `defaultColumnWidth`
- `centerFocusedColumn`
- workspace topology
- app admission rules
- title / bundle matching rules
- daemon/watch ownership

これらは外部制約ではなく、**Managed Environment Contract** として設計対象にする。

### 2.2 「状態の真実」は合成物である

単一の `state.json` だけを真実にしない。
単一の live query だけも真実にしない。

controller が扱う真実は以下の合成である。

```text
DesiredWorld   : projwm が実現したい状態
ObservedWorld  : 最後に観測した実世界
PredictedWorld : 操作列を適用したらこうなるはずという予測
ControllerMeta : transaction / epoch / event queue / dirty 状態
```

### 2.3 不確定性は限定的に扱う

`Unknown` を中心概念にしない。
通常の状態は observer / settler により `Known` に落とす。

ただし以下は一時的に不確定になりうる。

- spawn 直後で window がまだ出現していない
- browser 内部 ID と OmniWM window ID がまだ対応付いていない
- wake / display change 後で観測が stale
- controller 自身の操作により layout event が出たが、まだ settle 前

```go
type Certainty int

const (
    Known Certainty = iota
    Predicted
    Dirty
    Unknown
    Unsupported
)
```

## 3. Managed Environment Contract

`projwm-next` は自分が動くための環境契約を持つ。

環境パラメータの authority は Nix である。
`projwmd` は Nix が生成した環境契約を読み、検証し、runtime 観測値との drift を検出する。
原則として `projwmd` は Nix 管理値を永続変更しない。

「環境パラメータも設計対象にする」とは、runtime が勝手に書き換えるという意味ではない。
堅牢化のために値を変える場合は、Nix 側の設定を変更し、rebuild 後に `projwmd` が新しい契約として読む。

```go
type ManagedEnvironment struct {
    Source        EnvironmentSource
    WindowManager WindowManagerEnvironment
    Displays      DisplayEnvironment
    Workspaces    WorkspaceEnvironment
    Apps          AppEnvironment
    Daemons       DaemonEnvironment
}

type EnvironmentSource struct {
    Authority EnvironmentAuthority
    Origin    string
    Version   EnvironmentVersion
    Validation EnvironmentValidationPolicy
}

type EnvironmentAuthority string

const (
    EnvironmentAuthorityNix EnvironmentAuthority = "nix"
)

type EnvironmentVersion string

type EnvironmentValidationPolicy struct {
    OnUnsafeMismatch EnvironmentMismatchAction
    OnSoftMismatch   EnvironmentMismatchAction
}

type EnvironmentMismatchAction string

const (
    EnvironmentMismatchBlock  EnvironmentMismatchAction = "block"
    EnvironmentMismatchWarn   EnvironmentMismatchAction = "warn"
    EnvironmentMismatchReport EnvironmentMismatchAction = "report"
)
```

所有権:

- Nix owns: OmniWM/niri layout tuning, focus tuning, workspace topology, app admission rules, launchd/projwmd topology。
- PersistentStore owns: committed durable records for user/project desired state, accepted layout, browser snapshots, controller checkpoint。
- `projwmd` owns in memory: event queue, predicted world, transaction state。
- Observer owns: live display/window/workspace/process/browser observation。

`projwmd` が環境値の変更を推奨できる場合でも、出すのは warning / report / suggested Nix diff までである。
runtime が Nix 管理ファイルや OmniWM 永続設定へ直接書き戻してはいけない。

authority 境界:

- manifest は environment contract であり、DesiredWorld ではない。
- DesiredWorld の永続 truth は PersistentStore の committed generation にだけ置く。
- PersistentStore は判断主体ではなく、Controller commit 後の durable record authority である。
- Observer は live fact を出すだけで、DesiredWorld / accepted layout / browser snapshot を直接書かない。
- PredictedWorld は restart を跨ぐ truth ではない。crash/restart 後は捨て、manifest + committed store + full observation から再計算する。
- 既存 `config.toml` は `projwm-next` の通常 daemon authority から外す。必要なら migration input / developer override に降格する。
- OmniWM の runtime-deployed `~/.config/omniwm/settings.toml` は applied settings cache であり、Nix source truth ではない。

### 3.1 WindowManagerEnvironment

```go
type WindowManagerEnvironment struct {
    Backend WindowManagerBackend
    Layout  LayoutTuning
    Focus   FocusTuning
    IPC     IPCPolicy
}

type WindowManagerBackend string

const (
    BackendOmniWM WindowManagerBackend = "omniwm"
)
```

### 3.2 LayoutTuning

現行調査で確認した値:

- `defaultColumnWidth = 0.5`
- `columnWidthPresets = [0.4, 0.5, 0.66, 0.8, 0.95]`
- `centerFocusedColumn = "never"`
- `alwaysCenterSingleColumn = true`
- `maxVisibleColumns = 4`
- `maxWindowsPerColumn = 4`
- `singleWindowAspectRatio = "16:10"`

ただし、これらは固定ではない。
`projwm-next` がより簡潔で堅牢になるなら変更してよい。

```go
type LayoutTuning struct {
    DefaultColumnWidth     Fraction
    ColumnWidthPresets     []Fraction
    MaxVisibleColumns      int
    MaxWindowsPerColumn    int
    CenterFocusedColumn    CenterPolicy
    AlwaysCenterSingle     bool
    SingleWindowAspect     AspectRatio
    InfiniteLoop           bool
}
```

設計判断:

- `MaxVisibleColumns` は planner の作業予算である。
- ただし planner を複雑にしすぎるくらいなら、`MaxVisibleColumns` を増やす選択肢も持つ。
- `MaxWindowsPerColumn` は stack 設計の制約である。
- target layout だけでなく、一時的な作業 layout もこの budget の影響を受ける。
- 環境値は hidden assumption にせず、WorldState に入れる。

### 3.3 FocusTuning

現行調査で確認した値:

- `followsMouse = false`
- `followsWindowToMonitor = true`
- `moveMouseToFocusedWindow = true`

```go
type FocusTuning struct {
    FollowsMouse             bool
    FollowsWindowToMonitor   bool
    MoveMouseToFocusedWindow bool
}
```

設計判断:

- focus は副作用ではなく model の一部である。
- command ごとに final focus policy を持つ。
- `FocusWindow` は危険 primitive として扱い、盲目的な focus restore に使わない。

### 3.4 DisplayEnvironment

display 構成も環境契約の一部である。

startup / wake / display add / display remove では、`projwmd` が display topology を観測し直し、
workspace 配置・focus・layout の target を再計算する。

```go
type DisplayEnvironment struct {
    Policy DisplayPolicy
}

type DisplayPolicy struct {
    OnStartup       LifecycleTransactionKind
    OnWake          LifecycleTransactionKind
    OnDisplayChange LifecycleTransactionKind
}

type ObservedDisplayState struct {
    Displays  map[DisplayID]ObservedDisplay
    Primary   *DisplayID
    Freshness Freshness
}

type ObservedDisplay struct {
    ID        DisplayID
    Name      string
    Frame     Frame
    Scale     float64
    Connected bool
}
```

設計判断:

- display index を identity として信用しない。
- display 数変更は dirty state ではなく lifecycle event として扱う。
- display 変更時は workspace/layout/focus を部分補正せず、display-aware target を再計算する。
- どの display に workspace を置くかは `DisplayPolicy` と `WorkspaceEnvironment` から決める。

### 3.5 WorkspaceEnvironment

現行調査で確認した workspace topology:

- `A`: viewer workspace
- `Q W E R T Y U I O P`: project slot workspaces
- `B`: browser / isolated browser
- `M`: media
- `1..9`: general numeric workspaces
- rawName と displayName は異なる

```go
type WorkspaceEnvironment struct {
    Viewer WorkspaceSpec
    Slots  []SlotSpec
    Other  []WorkspaceSpec
}

type WorkspaceSpec struct {
    ID          WorkspaceID
    RawName     string
    DisplayName string
    Role        WorkspaceRole
    Policy      WorkspacePolicy
}

type SlotSpec struct {
    ID        SlotID
    Workspace WorkspaceID
    Order     int
}

type WorkspaceRole string

const (
    WorkspaceViewer   WorkspaceRole = "viewer"
    WorkspaceProject  WorkspaceRole = "project"
    WorkspaceBrowser  WorkspaceRole = "browser"
    WorkspaceMedia    WorkspaceRole = "media"
    WorkspaceGeneral  WorkspaceRole = "general"
)
```

設計判断:

- workspace raw/display/name/number を混ぜない。
- slot order は viewer order と planner の決定性に使う。
- workspace topology も managed environment に含める。

### 3.6 AppEnvironment

現行調査で確認した app rule:

- Ghostty は projwm 規約 title の window のみ tile 管理する。
- Ghostty hidden helper window と main window を titleRegex で区別している。
- 一般 Ghostty は min size のみ。
- appRules の `assignToWorkspace` は使わない。
- 起動時の one-shot sort はあるが、手動移動後に引き戻さない方針。

`projwm-next` は既存 appRules をそのまま authority として引き継がない。

app rule は以下に分解する。

- WM admission / safety rule: float すべき system dialog、最小サイズ、helper window 除外など。
- controller app policy: managed app の lifecycle / placement / focus / snapshot。
- legacy startup sort: project 管理外の一般アプリだけを対象にする任意の互換層。

project 配置や profile 切替は appRules ではなく `projwmd` が所有する。

```go
type AppEnvironment struct {
    ManagedApps []ManagedAppPolicy
    Admission   []WindowAdmissionRule
    LegacyRules []LegacyAppRulePolicy
}

type ManagedAppPolicy struct {
    Capability AppCapability
    BundleID   string
    AppPath    string
    Authority  AppPolicyAuthority
    Rules      []AppRule
}

type WindowAdmissionRule struct {
    BundleID   string
    TitleHints []MatchHint
    Decision   AdmissionDecision
    Layout     AdmissionLayout
    MinWidth   *float64
    MinHeight  *float64
}

type AppPolicyAuthority string

const (
    AppPolicyControllerOwned AppPolicyAuthority = "controller-owned"
    AppPolicySafetyOnly      AppPolicyAuthority = "safety-only"
    AppPolicyExternal        AppPolicyAuthority = "external"
)

type AdmissionDecision string

const (
    AdmissionTileEligible AdmissionDecision = "tile-eligible"
    AdmissionForceFloat   AdmissionDecision = "force-float"
    AdmissionIgnore       AdmissionDecision = "ignore"
    AdmissionMinSizeOnly  AdmissionDecision = "min-size-only"
)

type LegacyAppRulePolicy struct {
    Name   string
    Action LegacyRuleAction
}

type LegacyRuleAction string

const (
    LegacyRuleRemove LegacyRuleAction = "remove"
    LegacyRuleKeep   LegacyRuleAction = "keep"
)
```

設計判断:

- core model は Ghostty/Zed/Vivaldi という名前に依存しない。
- core model は terminal/editor/browser/session という能力に依存する。
- app 固有の癖は adapter と admission rule に閉じ込める。
- OmniWM appRules は project placement の authority ではない。
- projwm managed workspace に影響する legacy rule は削除対象にする。
- safety rule は残してよいが、controller の desired state を上書きしてはいけない。
- desired placement と admission が衝突した場合は、admission が安全上の precondition として勝つ。
- admission が勝った場合、planner は代替 plan を作るか、environment mismatch として block/report する。
- admission は desired を暗黙に書き換えない。

### 3.7 DaemonEnvironment

現行調査で確認した daemon:

- windows-changed watch → reconcile
- display-changed watch → reconcile/deploy
- periodic reconcile
- startup reconcile
- wake reconcile
- layout-changed watch → layout-snapshot

`projwm-next` では、これらの既存 daemon を残さない。

残す常駐 daemon は **`projwmd` だけ**である。

既存の wake / startup / display / window / layout / periodic 用 launchd agent は、
それぞれが `projwm reconcile` や `layout-snapshot` を直接実行するため、single writer を破る。
これらは `projwmd` の内部 event source / timer / startup transaction に置き換える。

OS の都合で別プロセス watcher が必要な場合だけ sidecar を許す。
ただし sidecar は state mutation を一切行わず、`projwmd` へ event を送るだけである。

```go
type DaemonEnvironment struct {
    ControllerDaemon DaemonSpec
    EventSources     []EventSourceSpec
    LegacyAgents     []LegacyAgentPolicy
    Lifecycle         SystemLifecyclePolicy
}

type DaemonSpec struct {
    Label     string
    KeepAlive bool
    RunAtLoad bool
}

type EventSourceSpec struct {
    Kind      EventKind
    Source    EventSource
    Mode      EventSourceMode
    Authority EventAuthority
}

type EventSourceMode string

const (
    EventSourceInProcess EventSourceMode = "in-process"
    EventSourceSidecar   EventSourceMode = "sidecar"
)

type EventAuthority string

const (
    EventAuthorityHint     EventAuthority = "hint"
    EventAuthorityEvidence EventAuthority = "evidence"
)

type LegacyAgentPolicy struct {
    Label  string
    Action LegacyAgentAction
}

type LegacyAgentAction string

const (
    LegacyAgentRemove LegacyAgentAction = "remove"
    LegacyAgentReport LegacyAgentAction = "report"
)

type SystemLifecyclePolicy struct {
    Startup       LifecycleTransactionKind
    Wake          LifecycleTransactionKind
    DisplayChange LifecycleTransactionKind
    SafetyTimer   LifecycleTransactionKind
}

type LifecycleTransactionKind string

const (
    LifecycleBootstrap          LifecycleTransactionKind = "bootstrap"
    LifecycleWakeRecovery       LifecycleTransactionKind = "wake-recovery"
    LifecycleDisplayReconfigure LifecycleTransactionKind = "display-reconfigure"
    LifecycleFullReconcile      LifecycleTransactionKind = "full-reconcile"
)
```

設計判断:

- launchd が管理する通常 daemon は `projwmd` のみにする。
- 既存の `projwm-reconcile-*` / `projwm-layout-watch` は削除対象である。
- startup reconcile は `projwmd` 起動時 transaction にする。
- periodic reconcile は外部 daemon ではなく、`projwmd` 内部の safety timer にする。
- wake / display / window / layout watcher は event source に降格する。
- startup / wake / display change / periodic は lifecycle transaction として扱う。
- watcher は controller に event を送るだけにする。
- event は truth ではなく hint/evidence である。live evidence は observer が `ObserveWorld` で再構成する。
- controller の判断対象は Desired / Observed / Predicted / Meta の合成である。
- layout-changed event は controller operation 由来か user 由来かを epoch で区別する。
- environment validation は legacy agent が残っていないか検出し、Nix 管理下では削除対象として報告する。

single writer / IPC:

- 通常運用における GUI / window / session / browser / desired-state mutation は `projwmd` だけが行う。
- `projwmctl` は intent client であり、store や adapter を直接変更しない。
- daemon 不在時でも direct mutation fallback は持たない。
- mutation IPC は local Unix domain socket とし、protocol/version/store schema を handshake で検査する。
- sidecar が許される場合でも、送れるのは `EventHint` だけである。store read/write、adapter call、reconcile 実行は禁止。
- test/admin/migration は raw file patch ではなく gated controller intent または store API を通す。
- offline status は stale な last committed store snapshot だけを読む。live query は daemon 経由のみ。
- arbitrary direct edit mode は持たない。emergency repair は daemon 停止中の offline store recovery primitive に限定し、GUI mutation をしない。

lifecycle transaction:

- `LifecycleBootstrap`: `projwmd` 起動時に managed environment / desired / observed を読み、全 scope を収束する。
- `LifecycleWakeRecovery`: wake 後に短い settle を置き、display/window/layout/focus を full observe して収束する。
- `LifecycleDisplayReconfigure`: display topology を読み直し、workspace target と layout target を再計算して収束する。
- `LifecycleFullReconcile`: event 取りこぼし対策の低頻度 safety reconcile。

## 4. Identity model

用途ごとに ID を分ける。

```go
type ProfileID string
type ProjectID string
type SlotID string
type WorkspaceID string
type DisplayID string

type DesiredWindowID struct {
    Project ProjectID
    Kind    WindowKind
    Index   int
}

type LiveWindowID string
type ProcessID int
type SessionID string
type BrowserWindowID string
type BrowserTabID string
type BrowserSnapshotID string
type BrowserSnapshotTabID string

type TransactionID string
type Epoch uint64
type OperationID string
type EventID string
```

禁止事項:

- `DesiredWindowID` と `LiveWindowID` を混ぜない。
- `BrowserWindowID` と `LiveWindowID` を混ぜない。
- workspace rawName / displayName / numeric number を混ぜない。
- window title を identity の代用品にしない。

`DesiredWindowID.Index` は表示順ではなく、project 内で安定した ordinal である。
window の並び替えや一時的な非表示で詰め直してはいけない。
表示順は layout / slot order 側で扱う。

## 5. Domain model

### 5.1 WindowKind

```go
type WindowKind string

const (
    WindowAI       WindowKind = "ai"
    WindowShell    WindowKind = "shell"
    WindowEditor   WindowKind = "editor"
    WindowBrowser  WindowKind = "browser"
    WindowViewer   WindowKind = "viewer"
    WindowExternal WindowKind = "external"
)
```

`viewer` は AI の副産物ではなく、core model 上の window kind として扱う。

### 5.2 DesiredWindow

```go
type DesiredWindow struct {
    ID            DesiredWindowID
    App           AppRequirement
    Session       *DesiredSession
    Browser       *DesiredBrowserSession
    TitleContract TitleContract
    MatchHints    []MatchHint
    Layout        *DesiredLayoutRef
    Lifecycle     WindowLifecycle
}
```

`DesiredWindow` は「この window の title は必ずこの文字列である」とは言わない。
title は identity ではなく、契約・照合ヒント・観測証拠に分けて扱う。

### 5.3 ObservedWindow

```go
type ObservedWindow struct {
    ID         LiveWindowID
    App        ObservedAppRef
    Title      ObservedTitle
    PID        ProcessID
    Workspace  WorkspaceID
    Visibility Visibility
    Focused    bool
    Frame      *Frame
    MatchedTo  *DesiredWindowID
    Freshness  Freshness
}
```

### 5.4 Title / Matching

title は一律の derived value ではない。

同じ `title` でも、意味は window の種類によって異なる。

- controller が厳密に所有する title
- controller が prefix / namespace だけ所有する title
- app が自然に変更する title
- user 操作で変わる title
- external window の観測値でしかない title

そのため title を以下に分ける。

```go
type TitleContract struct {
    Authority TitleAuthority
    Expected  *DerivedString
    Prefix    *DerivedString
    Drift     TitleDriftPolicy
}

type ObservedTitle struct {
    Value     string
    Freshness Freshness
}

type MatchHint struct {
    Kind       MatchHintKind
    Pattern    string
    Confidence MatchConfidence
}

type MatchHintKind string

const (
    MatchByTitleRegex      MatchHintKind = "title-regex"
    MatchByTitlePrefix     MatchHintKind = "title-prefix"
    MatchByBundleID        MatchHintKind = "bundle-id"
    MatchBySessionID       MatchHintKind = "session-id"
    MatchByBrowserWindowID MatchHintKind = "browser-window-id"
)

type MatchConfidence string

const (
    MatchStrong MatchConfidence = "strong"
    MatchWeak   MatchConfidence = "weak"
)

type TitleAuthority string

const (
    TitleControllerOwned TitleAuthority = "controller-owned"
    TitlePrefixOwned     TitleAuthority = "prefix-owned"
    TitleAppOwned        TitleAuthority = "app-owned"
    TitleUserOwned       TitleAuthority = "user-owned"
    TitleExternal        TitleAuthority = "external"
)

type TitleDriftPolicy string

const (
    TitleDriftRepair      TitleDriftPolicy = "repair"
    TitleDriftRematch     TitleDriftPolicy = "rematch"
    TitleDriftObserveOnly TitleDriftPolicy = "observe-only"
)
```

設計判断:

- title を window identity にしない。
- `Expected` / `Prefix` は controller-owned な部分だけを表す。
- `ObservedTitle` は実世界から見えた値であり、変化して当然のことがある。
- `MatchHint` は admission / rematch の証拠であり、不変条件ではない。
- cmux / Ghostty のような管理 window は exact title 固定ではなく、必要なら `TitlePrefixOwned` にする。
- editor / browser / external window は原則として `TitleAppOwned` または `TitleExternal` にする。
- mutation target の resolver は `unique-strong` 以外を返したら実行してはいけない。
- title fallback / BundleID-only / SavedURLs-only / frontmost / last focused は mutation identity に使わない。
- ambiguity は best-effort で選ばず、BlockedAmbiguous として report する。

例:

```go
// 厳密に管理する helper window
TitleContract{
    Authority: TitleControllerOwned,
    Expected:  derived("ai-1: dotfiles"),
    Drift:     TitleDriftRepair,
}

// cmux のように名前空間は管理するが、内部操作による suffix 変更を許す window
TitleContract{
    Authority: TitlePrefixOwned,
    Prefix:    derived("cmux:dotfiles:"),
    Drift:     TitleDriftRematch,
}

// 外部 window
TitleContract{
    Authority: TitleExternal,
    Drift:     TitleDriftObserveOnly,
}
```

### 5.5 Layout

layout は物理座標ではなく semantic layout として持つ。

```go
type DesiredLayout struct {
    Columns []DesiredColumn
    Policy  LayoutPolicy
}

type DesiredColumn struct {
    Windows []DesiredWindowID
    Mode    ColumnMode
}

type ObservedLayout struct {
    Workspace WorkspaceID
    Columns   []ObservedColumn
    Source    LayoutObservationSource
    Freshness Freshness
}

type ObservedColumn struct {
    Windows []LiveWindowID
    Mode    ColumnMode
    Frame   *Frame
}

type ColumnMode string

const (
    ColumnSolo    ColumnMode = "solo"
    ColumnStacked ColumnMode = "stacked"
    ColumnTabbed  ColumnMode = "tabbed"
)
```

`Frame` は observation の根拠であり、desired truth ではない。

### 5.6 LayoutPolicy

```go
type LayoutPolicy struct {
    MaxVisibleColumns    int
    MaxWindowsPerColumn  int
    AllowOffscreenColumns bool
    RequireStableFrames  bool
}
```

`LayoutPolicy` は `ManagedEnvironment` から導出される。

### 5.7 Project / Profile

```go
type DesiredProject struct {
    ID       ProjectID
    Root     ProjectRoot
    Archived bool
    Windows  []DesiredWindow
    Layouts  DesiredProjectLayouts
}

type DesiredProjectLayouts struct {
    Workspaces map[WorkspaceID]DesiredLayout
    Source     LayoutAuthority
}

type LayoutAuthority string

const (
    LayoutAuthorityDefault        LayoutAuthority = "default"
    LayoutAuthorityAcceptedManual LayoutAuthority = "accepted-manual"
    LayoutAuthorityImported       LayoutAuthority = "imported"
)

type DesiredProfile struct {
    ID          ProfileID
    Description string
    Assignments map[SlotID]ProjectID
}
```

## 6. App capability model

core は具体アプリ名ではなく能力を見る。

```go
type AppCapability string

const (
    CapabilityTerminal AppCapability = "terminal"
    CapabilityEditor   AppCapability = "editor"
    CapabilityBrowser  AppCapability = "browser"
    CapabilitySession  AppCapability = "session"
    CapabilitySystem   AppCapability = "system"
)

type AppRequirement struct {
    Capability  AppCapability
    BundleID    string
    AppPath     string
    Constraints []AppConstraint
}
```

## 7. Adapter contracts

### 7.1 WindowManagerAdapter

```go
type WindowManagerCapabilities struct {
    MaxVisibleColumns            int
    MaxWindowsPerColumn          int
    SupportsSummonRight          bool
    SupportsTabbedColumn         bool
    SupportsMoveToWorkspaceByName bool
}

type WindowManagerAdapter interface {
    Capabilities(ctx context.Context) (WindowManagerCapabilities, error)

    ObserveWindows(ctx context.Context) ([]ObservedWindow, error)
    ObserveWorkspaces(ctx context.Context) ([]ObservedWorkspace, error)
    ObserveFocus(ctx context.Context) (ObservedFocus, error)

    // 以下の primitive は semantic operation wrapper の内部からだけ呼ぶ。
    // Planner / Controller / projwmctl から raw primitive として露出しない。
    FocusWorkspace(ctx context.Context, id WorkspaceID) error
    FocusWindow(ctx context.Context, id LiveWindowID) error
    MoveWindowToWorkspace(ctx context.Context, id LiveWindowID, ws WorkspaceID) error
    MoveColumn(ctx context.Context, direction Direction) error
    MoveStackMember(ctx context.Context, direction Direction) error
    ToggleTabbed(ctx context.Context) error
    SummonRight(ctx context.Context, id LiveWindowID) error
}
```

real mutation safety:

- real backend mutation は resolver / operation wrapper / verifier / app contract が全て成立する場合だけ許可する。
- resolver は `unique-strong` / `missing` / `ambiguous` / `weak-match` / `stale` を区別して返す。
- `candidate_count != 1` または `confidence < 1.0` は mutation block。
- focus 依存 primitive は semantic operation 全体を `wmMutationLock` 内で直列化し、lock 取得後に再観測・再解決する。
- wrapper は `Precondition -> Resolve -> Execute -> Settle -> Verify -> Commit` の順にする。
- Verify failure では state/cache/layout commit を禁止し、dependent mutation を止める。
- first implementation では full layout restore real mutation を禁止し、dry-run にする。

### 7.2 SessionCapabilityAdapter

```go
type DesiredSession struct {
    ID      SessionID
    CWD     ProjectRoot
    Command []string
    GroupOf *SessionID
}

type ObservedSession struct {
    ID      SessionID
    Exists  bool
    GroupOf *SessionID
}

type SessionCapabilityAdapter interface {
    ObserveSessions(ctx context.Context) (ObservedSessionState, error)
    EnsureSession(ctx context.Context, desired DesiredSession) error
    KillSession(ctx context.Context, id SessionID) error
}
```

### 7.3 BrowserCapabilityAdapter

```go
type DesiredBrowserSession struct {
    ProfileName   string
    InitialURLs   []string
    RestoreFrom   *BrowserSnapshotID
    Placement     DesiredWindowID
    ContentPolicy BrowserContentPolicy
}

type BrowserContentPolicy string

const (
    BrowserStructureSnapshotManaged BrowserContentPolicy = "structure-snapshot-managed"
    BrowserContentPrivatePayload    BrowserContentPolicy = "private-payload"
    BrowserContentObservedOnly      BrowserContentPolicy = "observed-only"
)

type ObservedBrowserWindow struct {
    BrowserID BrowserWindowID
    LiveID    *LiveWindowID
    Profile   string
    Tabs      []ObservedBrowserTab
    Freshness Freshness
}

type ObservedBrowserTab struct {
    ID        BrowserTabID
    Window    BrowserWindowID
    URL       SensitiveURL
    Title     SensitiveTitle
    Active    bool
    Management BrowserTabManagement
    Freshness Freshness
}

type ObservedBrowserState struct {
    Windows   map[BrowserWindowID]ObservedBrowserWindow
    Tabs      map[BrowserTabID]ObservedBrowserTab
    Freshness Freshness
}

type BrowserSnapshot struct {
    ID          BrowserSnapshotID
    Project     ProjectID
    ProfileName string
    Windows     []BrowserWindowSnapshot
}

type BrowserWindowSnapshot struct {
    Tabs []BrowserTabSnapshot
}

type BrowserTabSnapshot struct {
    Ref      BrowserSnapshotTabID
    Position int
    Active   bool
    Pinned   bool
    Privacy  BrowserSnapshotPrivacyMode
    Content  *PrivatePayloadRef
}

type BrowserSnapshotPrivacyMode string

const (
    BrowserSnapshotStructureOnly   BrowserSnapshotPrivacyMode = "structure-only"
    BrowserSnapshotRedactedContent BrowserSnapshotPrivacyMode = "redacted-content"
    BrowserSnapshotPrivateContent  BrowserSnapshotPrivacyMode = "private-content"
)

type BrowserTabManagement string

const (
    BrowserTabSnapshotManaged BrowserTabManagement = "snapshot-managed"
    BrowserTabObservedOnly    BrowserTabManagement = "observed-only"
    BrowserTabIgnored         BrowserTabManagement = "ignored"
    BrowserTabBlockedPrivate  BrowserTabManagement = "blocked-private"
)

type SensitiveURL struct {
    Ref  *PrivatePayloadRef
    Safe BrowserContentDescriptor
}

type SensitiveTitle struct {
    Ref  *PrivatePayloadRef
    Safe BrowserContentDescriptor
}

type PrivatePayloadRef string

type BrowserContentDescriptor struct {
    Class       URLClass
    OriginHMAC *string
    PathDepth  *int
    HasQuery   bool
    HasFragment bool
    TitleBucket *string
}

type URLClass string

const (
    URLClassHTTP      URLClass = "http"
    URLClassHTTPS     URLClass = "https"
    URLClassFile      URLClass = "file"
    URLClassBrowser   URLClass = "browser-internal"
    URLClassExtension URLClass = "extension"
    URLClassLocalhost URLClass = "localhost"
    URLClassPrivateNet URLClass = "private-network"
    URLClassUnknown   URLClass = "unknown"
)

type BrowserCapabilityAdapter interface {
    ObserveBrowser(ctx context.Context) (ObservedBrowserState, error)
    SpawnBrowserWindow(ctx context.Context, desired DesiredBrowserSession) (BrowserSpawnResult, error)
    CloseBrowserWindow(ctx context.Context, id BrowserWindowID) error
    SnapshotBrowserWindow(ctx context.Context, id BrowserWindowID) (BrowserSnapshot, error)
}
```

Browser adapter の責務:

- browser 内部 ID と live window ID の対応付け
- stale ID の検出
- URL/session snapshot
- spawn/close の focus 副作用の明示
- last-focused workspace 問題の観測と報告

core planner は Vivaldi / chrome-cli / marker title を知らない。

ブラウザ内部状態は adapter の付属物ではなく、WorldState 内の一級 sub-world として扱う。
ただし core が全タブを常時 desired invariant にするわけではない。

設計判断:

- `DesiredBrowserSession` は「開き方」と「復元元」を表す。
- `ObservedBrowserState` はブラウザ内部から観測した現在状態を表す。
- `BrowserSnapshot` は archive / unarchive / profile transition のための永続復元単位である。
- `BrowserStructureSnapshotManaged` は window/tab の構造を snapshot 境界で保存・復元するが、通常操作中の手動 tab 変更は drift にしない。
- raw URL/title content は snapshot の通常 field ではなく、明示 policy がある場合だけ PrivatePayloadStore の opaque ref として扱う。
- `BrowserContentObservedOnly` は観測・debug 用であり、復元対象にしない。
- 通常の project browser は `BrowserStructureSnapshotManaged` を既定にする。
- raw URL/title/SavedURLs は PersistentStore / log / trace / test artifact / migration report / diagnostics に直接出さない。
- Observed tab は自動で snapshot-managed にならない。`observed-only` / `ignored` / `blocked-private` を区別する。
- incognito/private/sensitive surface は default で count-only とし、content persistence しない。
- PersistentStore は inspect/share safe by default とし、raw browser content は PrivatePayloadStore に分離する。

## 8. WorldState

```go
type WorldState struct {
    Environment ManagedEnvironment
    Desired     DesiredWorld
    Observed    ObservedWorld
    Predicted   *PredictedWorld
    Meta        ControllerMeta
}

type DesiredWorld struct {
    ActiveProfile ProfileID
    Profiles      map[ProfileID]DesiredProfile
    Projects      map[ProjectID]DesiredProject
    Browsers      DesiredBrowserWorld
    FocusPolicy   FocusPolicySet
}

type DesiredBrowserWorld struct {
    Sessions map[DesiredWindowID]DesiredBrowserSession
}

type ObservedWorld struct {
    Displays   ObservedDisplayState
    Workspaces map[WorkspaceID]ObservedWorkspace
    Windows    map[LiveWindowID]ObservedWindow
    Layouts    map[WorkspaceID]ObservedLayout
    Focus      ObservedFocus
    Sessions   ObservedSessionState
    Browsers   ObservedBrowserState
    Apps       ObservedAppState
    Freshness  Freshness
}

type PredictedWorld struct {
    ObservedWorld
    BasedOnEpoch Epoch
    Effects      []Effect
}

type ControllerMeta struct {
    Epoch         Epoch
    Transaction  *TransactionID
    PendingEvents []EventID
    DirtyScopes   []DirtyScope
}
```

### 8.1 PersistentStore

`state.json` は「単一の真実」としては残さない。

ただし永続 store は必要である。
`projwm-next` では、保存対象を明示的に分けた `PersistentStore` として扱う。

```go
type PersistentStore interface {
    LoadCurrentGeneration(ctx context.Context) (CommittedGeneration, error)
    BeginCommit(ctx context.Context, commit ControllerCommit) (StagedCommit, error)
    Commit(ctx context.Context, staged StagedCommit) (GenerationID, error)
    Abort(ctx context.Context, staged StagedCommit) error
}

type CommittedGeneration struct {
    ID              GenerationID
    Parent          *GenerationID
    Desired         DesiredWorld
    AcceptedLayouts AcceptedLayoutSet
    BrowserSnapshots BrowserSnapshotSet
    Checkpoint      ControllerCheckpoint
    Manifest        StoreManifest
}

type ControllerCommit struct {
    TransactionID TransactionID
    Parent        GenerationID
    Desired       DesiredWorld
    AcceptedLayouts AcceptedLayoutSet
    BrowserSnapshots BrowserSnapshotSet
    Checkpoint    ControllerCheckpoint
    Journal       []JournalEntry
}

type ControllerCheckpoint struct {
    Epoch        Epoch
    LastClean    *TransactionID
    DirtyScopes  []DirtyScope
    StoreVersion StoreVersion
}

type StoreVersion string
```

設計判断:

- `state.json` 相当のファイルは store backend の一実装であり、WorldState そのものではない。
- `DesiredWorld` / browser snapshot / controller checkpoint は保存する。
- `ObservedWorld` は起動時に observer で再構成する。debug cache として保存しても truth にはしない。
- 書き込みは Controller commit による immutable generation 作成だけに限定する。
- generation は `.staging` に書いて検証し、`generations/<id>` への rename と `CURRENT` pointer 更新で visible にする。
- committed generation は immutable であり、修正は次 generation として作る。
- `manifest.json` は parent generation、schema versions、artifact checksums を持つ。
- `journal` は audit/debug 用であり、通常 recovery の replay authority にしない。
- `checkpoint` / artifacts が recovery authority である。
- production / test store は `.store_identity.json` で機械的に分離する。
- offline repair は daemon 停止中に限り、`CURRENT` rollback / quarantine / migration retry など構造修復だけを行う。DesiredWorld などの semantic mutation はしない。
- 既存 `state.json` は migration input として読めるようにしてよいが、新設計の authority にはしない。

AI workspace の管理状態は `DesiredWorld` として保存する。

保存するもの:

- active profile
- profile の slot assignment
- project の archived / active 状態
- project ごとの desired windows
- desired session / browser session
- accepted semantic layout
- browser snapshot への参照

保存しないもの:

- `LiveWindowID`
- 現在の frame 座標
- 現在 focus されている live window
- query で再観測できる app/window/process の一時状態

これらの live state は `ObservedWorld` として毎回再構成する。

legacy migration:

- legacy `state.json` は quarantine に copy してから whitelist migration する。
- `ActiveProfile` / `Profiles` / `Projects` / `Window.kind` / stable ordinal は validation 後に DesiredWorld 候補にできる。
- `LiveWindowID` / frame / current focus / live PID は破棄する。
- `SavedURLs` は private browser snapshot 候補として扱い、log/report/test artifact には URL 本体を出さない。
- `Layout` は accepted semantic layout 候補だが、semantic validation 不能なら quarantine/manual review に回す。

## 9. Intent / Event

### 9.1 Intent

```go
type Intent interface {
    IntentKind() IntentKind
}

type IntentKind string

const (
    IntentSwitchProfile      IntentKind = "switch-profile"
    IntentArchiveProject     IntentKind = "archive-project"
    IntentUnarchiveProject   IntentKind = "unarchive-project"
    IntentAssignProject      IntentKind = "assign-project"
    IntentUnassignSlot       IntentKind = "unassign-slot"
    IntentReconcile          IntentKind = "reconcile"
    IntentAcceptManualLayout IntentKind = "accept-manual-layout"
    IntentValidateEnvironment IntentKind = "validate-environment"
)
```

### 9.2 Event

```go
type Event struct {
    ID     EventID
    Source EventSource
    Kind   EventKind
    Epoch  Epoch
    Data   EventData
}

type EventSource string

const (
    EventSourceUser       EventSource = "user"
    EventSourceWindowMgr  EventSource = "window-manager"
    EventSourceSystem     EventSource = "system"
    EventSourceTimer      EventSource = "timer"
    EventSourceController EventSource = "controller"
)
```

## 10. Operation contract

```go
type Operation struct {
    ID            OperationID
    Kind          OperationKind
    Target        OperationTarget
    Preconditions []Precondition
    Effects       []Effect
    Settle        SettlePolicy
    Retry         RetryPolicy
    Risk          RiskClass
}
```

### 10.1 OperationKind

```go
type OperationKind string

const (
    OpObserveWorld OperationKind = "observe-world"

    OpEnsureSession OperationKind = "ensure-session"
    OpKillSession   OperationKind = "kill-session"

    OpSpawnTerminal OperationKind = "spawn-terminal"
    OpSpawnEditor   OperationKind = "spawn-editor"
    OpSpawnBrowser  OperationKind = "spawn-browser"
    OpCloseWindow   OperationKind = "close-window"

    OpFocusWorkspace OperationKind = "focus-workspace"
    OpFocusWindow    OperationKind = "focus-window"

    OpMoveWindowToWorkspace OperationKind = "move-window-to-workspace"
    OpMoveColumn            OperationKind = "move-column"
    OpMoveStackMember       OperationKind = "move-stack-member"
    OpToggleTabbed          OperationKind = "toggle-tabbed"
    OpSummonRight           OperationKind = "summon-right"

    OpAcceptLayoutObservation OperationKind = "accept-layout-observation"
    OpValidateEnvironment     OperationKind = "validate-environment"
)
```

### 10.2 Precondition

```go
type PreconditionKind string

const (
    PreWindowExists      PreconditionKind = "window-exists"
    PreWorkspaceExists   PreconditionKind = "workspace-exists"
    PreAnchorVisible     PreconditionKind = "anchor-visible"
    PreColumnBudget      PreconditionKind = "column-budget"
    PreStackCapacity     PreconditionKind = "stack-capacity"
    PreAdapterCapability PreconditionKind = "adapter-capability"
    PreEnvironmentReady  PreconditionKind = "environment-ready"
)
```

`PreColumnBudget` は `MaxVisibleColumns` を扱う。
ただし planner が苦しくなるなら、runtime で値を書き換えず、Nix 側の `ManagedEnvironment` 契約変更を提案する。

## 11. Planner / Simulator / Executor

```go
type Planner interface {
    Plan(ctx context.Context, world WorldState, target DesiredWorld) (Plan, error)
}

type Simulator interface {
    Apply(ctx context.Context, predicted PredictedWorld, op Operation) (PredictedWorld, error)
}

type Executor interface {
    Execute(ctx context.Context, op Operation) (ExecutionResult, error)
}

type Settler interface {
    Wait(ctx context.Context, op Operation, expected PredictedWorld) (ObservedWorld, error)
}

type Verifier interface {
    Diff(predicted PredictedWorld, observed ObservedWorld) WorldDiff
    Check(world WorldState, invariants []Invariant) error
}
```

責務:

- Planner: 目標状態への操作列を作る。
- Simulator: 操作効果を予測状態に適用する。
- Executor: 実世界に操作を送る。
- Settler: 操作後の安定化を待つ。
- Verifier: 予測・目標・観測を比較する。

境界:

- Planner は observe / execute / sleep / store write をしない。
- Simulator は semantic world に対して賢く予測してよいが、完全な WM 再実装を目指さず、uncertainty を明示する。
- Executor は 1 operation の副作用だけを実行し、retry / replan を決めない。
- Settler は observation polling のみ行い、mutation しない。
- Verifier は diff を分類するだけで、補正操作を実行しない。
- retry / replan / abort / restart の所有者は Controller である。

初期実装は汎用探索 planner ではなく deterministic rule-based planner から始める。
ただし candidate plan は Simulator で評価し、将来的な bounded search へ発展できる形にする。

## 12. Controller transaction

```go
type Transaction struct {
    ID      TransactionID
    Epoch   Epoch
    Trigger Event
    Before  WorldState
    Target  DesiredWorld
    Plan    Plan
    Results []OperationResult
    After   WorldState
}
```

transaction contract:

1. transaction は同時に 1 つだけ。
2. 開始時に observe する。
3. user intent は reducer に渡して target を作る。
4. external event は desired を直接変えず、dirty scope / lifecycle trigger / observe request に変換する。
5. 必要なら environment drift を扱う。
6. planner が plan を作る。
7. 各 operation を simulator に通す。
8. executor が 1 operation ずつ実行する。
9. settler が安定化を待つ。
10. observer が再観測する。
11. verifier が predicted と observed を比較する。
12. 差分が許容不能なら replan する。
13. invariant が通ったら commit する。

event queue contract:

- stale epoch の event は捨てるか evidence としてだけ残す。
- wake / display change は window/layout/focus event を supersede する。
- controller-origin event は settle 中の evidence として扱う。
- user-origin layout event は manual-layout candidate にするが、`IntentAcceptManualLayout` なしに desired へ保存しない。
- safety timer event は既存 dirty transaction を増やさない。
- event storm は dirty scope 単位で coalesce する。

## 13. PersistentStore / Observer / Reducer

```go
type TraceStore interface {
    AppendTrace(ctx context.Context, trace TraceEvent) error
}

type Observer interface {
    ObserveWorld(ctx context.Context) (ObservedWorld, error)
}

type Reducer interface {
    ReduceIntent(world WorldState, intent Intent) (DesiredWorld, error)
    ReactToEvent(world WorldState, event Event) (EventReaction, error)
}

type EventReaction struct {
    DirtyScopes   []DirtyScope
    ObserveScopes []WorldScope
    Lifecycle     *LifecycleTransactionKind
}
```

Reducer は user intent から desired state を変える。
external event は desired state を直接変えない。
実 GUI 操作はしない。

Observer は実世界を読むだけである。
mutation してはいけない。

## 14. Invariant

```go
type Invariant interface {
    Check(world WorldState) error
}
```

基本 invariant:

- managed environment が許容範囲内である。
- active profile が desired と一致する。
- slot assignment が desired と一致する。
- active project の desired windows が存在する。
- archived project の managed windows が存在しない。
- inactive project の managed windows が policy 通りである。
- viewer windows は active AI windows と一致する。
- viewer order は slot order と一致する。
- project layout は desired semantic layout と一致する。
- final focus は command policy と一致する。
- isolated/external apps は managed project と混ざらない。
- title drift は `TitleContract` が要求する場合だけ violation にする。
- transaction 後に未処理 Dirty scope が残らない。

### 14.1 Command policy 表

specs.md §2.1-10 (final focus は command policy と一致する) を満たすための
`DesiredWorld.FocusPolicy.FinalFocus` map の **manifest authority 向け推奨 mapping** を
ここで固定する。実装は静的 map lookup
(`internal/planner/planner.go::Plan` における
`target.FocusPolicy.FinalFocus[command]`) のみを行い、
runtime 計算は行わない。manifest authority (Nix) が下表の semantics を満たす
workspace ID を直接書き込む責任を負う。

key の命名は projwmd 内部で `intent:<kind>` / `lifecycle:<kind>` を使う
(具体は `internal/controller/controller.go::commandKeyForIntent` および
`commandKeyForLifecycle`)。

| command (Intent / Lifecycle) | key | 推奨 final focus workspace（manifest が満たすべき semantics）|
|---|---|---|
| IntentSwitchProfile | `intent:switch-profile` | 切替先 Profile の active slot #0 (= slot order 最小) の workspace |
| IntentArchiveProject | `intent:archive-project` | archive 操作前に対象 project が居た slot の次 slot の workspace（無ければ viewer） |
| IntentUnarchiveProject | `intent:unarchive-project` | unarchive 先 slot の workspace |
| IntentAssignProject | `intent:assign-project` | assign された slot の workspace |
| IntentUnassignSlot | `intent:unassign-slot` | unassign された slot の次 slot の workspace（無ければ viewer） |
| IntentReconcile | `intent:reconcile` | 変化なし（設定なし） |
| IntentAcceptManualLayout | `intent:accept-manual-layout` | 変化なし |
| IntentValidateEnvironment | `intent:validate-environment` | 変化なし |
| LifecycleBootstrap | `lifecycle:bootstrap` | active profile の slot #0 workspace |
| LifecycleWakeRecovery | `lifecycle:wake-recovery` | 設定なし（前回 focus を維持。observed.Focus.Workspace を尊重） |
| LifecycleDisplayReconfigure | `lifecycle:display-reconfigure` | 変化なし |
| LifecycleFullReconcile | `lifecycle:full-reconcile` | 変化なし |

「変化なし」/「設定なし」となる command では `FocusPolicy.FinalFocus[key]` を空文字
または未設定にし、planner が `OpFocusWorkspace` を emit しない。
test 側は `internal/scenario.Step.SkipFinalFocus = true` で Invariant 10 を skip する。

slot の「次 slot」「前 slot」は `WorkspaceEnvironment.Slots[].Order` 昇順で評価する。

manifest authority が IntentArchiveProject の対象 slot を pre-archive 状態で
解決できないケース (= archive 動作中に slot が切り離されている) は、
manifest authority が不可知なため上表を **静的 map に書き起こせない単一の値** にする
ことはできない。そのため上表は **command key と semantics の対応** を固定し、
manifest 側は「IntentArchiveProject の固定遷移先ワークスペース」を一つ選んで
`FocusPolicy.FinalFocus["intent:archive-project"]` に設定するか、
未設定にして「archive 後に focus が動かない」ことを許容する。
planner / reducer / controller は FinalFocus map のみを参照する。

## 15. Go E2E story contract

projwm-next のテスト設計は test pyramid ではない。
完成判定は、real OmniWM / sigwm を通る Human-operation E2E acceptance story だけで行う。
unit / integration / fake / simulator / recorded test は実装補助、preflight、diagnostics、failure reproduction
として使ってよいが、acceptance の代替ではない。

テスト仕様は Go code として書く。
独自 scenario file format は作らない。

E2E story は、個別 Step を独立 reset で大量に走らせるものではない。
実 workspace `A/Q/W/E` と isolated test PersistentStore を用意し、少数の通し story を最初から最後まで
documented `projwmctl`、keyboard / window manager 操作、実アプリ操作で駆動する。
primary oracle は visible window/workspace/layout/focus/CLI output/restart 後の復元であり、
store / trace inspection は transaction property と failure diagnosis の補助に限る。

```go
type E2EStory struct {
    Name      string
    Fixture   FixtureSpec
    Steps     []E2EStep
    Auxiliary bool // physical lifecycle / privacy など canonical story に混ぜると危険な場合だけ true
}

type E2EStep struct {
    Name       string
    Action     HumanOperation
    Assertions []E2EAssertion
}
```

action は原則として `projwmctl` subprocess、keyboard / window manager 操作、実アプリ操作である。
Controller / Adapter / Reducer を test が直接呼ぶ場合、それは E2E acceptance ではない。

reset / fixture load も raw state file edit ではない。
fixture は test-mode daemon に対する human-visible/admin-test command で読み込み、必要なら lifecycle transaction で収束させる。
production で表現できない hidden repair を test reset に入れてはいけない。

canonical story は 1 本を中心にする。
補助 story は、sleep/wake、display reconfigure、browser privacy、stale event race など、canonical story に混ぜると
安全性・再現性・privacy が悪化するものに限る。

trace は debug artifact であり、テスト定義ではない。
ただし transaction contract は trace / transaction log / observed WorldState / test PersistentStore から監査可能でなければならない。

## 16. Package boundary

ここで承認するのは責務境界であり、物理的な Go package 名を固定することではない。
実装初期に循環依存や過剰 interface が見えた場合、package 分割は調整してよい。

```text
cmd/projwmd
cmd/projwmctl

internal/world
internal/identity
internal/environment
internal/store
internal/observe
internal/reduce
internal/ops
internal/sim
internal/plan
internal/exec
internal/settle
internal/verify
internal/control
internal/story

internal/adapters/windowmgr
internal/adapters/session
internal/adapters/app
internal/adapters/browser
internal/adapters/system
```

既存 `internal/reconcile` や `internal/browserwrap` を前提にしない。
必要に応じて adapter 実装の参考にするだけである。

`projwmctl` は通常、直接 GUI/window mutation を実行しない。
`projwmd` に intent を送る client として扱う。

## 17. 承認対象

実装前に合意すべきもの:

1. World Controller を中心にすること。
2. `projwmd` を唯一の通常実行主体にし、既存の複数 launchd agent は削除対象にすること。
3. watcher は state mutation しない event source に降格すること。
4. environment parameter は設計対象だが、authority は Nix に置くこと。
5. desired / observed / predicted / meta を分けること。
6. ID 型を用途ごとに分けること。
7. title を identity にせず、`TitleContract` / `ObservedTitle` / `MatchHint` に分けること。
8. `state.json` は truth ではなく `PersistentStore` backend に格下げすること。
9. OmniWM appRules は project placement の authority にしないこと。
10. operation に precondition/effect/settle/retry/risk を持たせること。
11. browser は Vivaldi 固有ではなく能力 adapter に閉じ込めること。
12. test は Go E2E story + invariant で書き、完成判定は real Human-operation E2E acceptance のみで行うこと。
13. trace はテスト定義ではなく debug artifact とすること。
14. 既存設計に縛られず、新しい責務境界で設計すること。
