# projwm-next 受け入れ仕様 (specs.md)

## 0. 位置付け

- 真実は `queue/design.md` と `queue/implementation-design.md` の 2 文書のみ。本書はそれらに基づく acceptance criteria（projwm-next が完成したと認めるシナリオ集）を、`design.md §15 Scenario contract` の形式で固定する。
- 本書は SSOT を要約しない。SSOT に明示されない acceptance level（観測可能な振る舞い）だけを fix する。
- 本書は SSOT に登場しない概念・語彙（および legacy 実装由来の用語）を一切使わない。

## 1. 用語参照（再定義しない）

| 領域 | 出典 |
|---|---|
| WorldState / DesiredWorld / ObservedWorld / PredictedWorld | `design.md §3, §11, §13` |
| Profile / Project / Slot / WorkspaceRole / WorkspaceID / SlotID / ProfileID / ProjectID | `design.md §3.5–§3.6, §4` |
| Window / WindowKind / WindowID（identity） / TitleContract | `design.md §5–§7` |
| Identity classification（unique-strong / ambiguous / weak / stale / missing） | `design.md §5–§7` |
| Intent / IntentKind | `design.md §9.1` |
| Event / EventSource / EventAuthority（hint / evidence） | `design.md §9.2, §3` |
| Operation / OperationKind / Precondition / Effect / SettlePolicy / RiskClass | `design.md §10` |
| Planner / Simulator / Executor / Settler / Verifier / WorldDiff | `design.md §11` |
| Controller transaction / Epoch / DirtyScope / WorldScope | `design.md §12, §13` |
| Reducer / EventReaction | `design.md §13` |
| Invariant | `design.md §14` |
| LifecycleTransactionKind（Bootstrap / WakeRecovery / DisplayReconfigure / FullReconcile） | `design.md §3` |
| Scenario / Step / ScenarioBackend | `design.md §15` |
| ManagedEnvironment manifest | `implementation-design.md §3` |
| PersistentStore / generation directory | `implementation-design.md §4, design.md §8` |
| `projwmd` / `projwmctl` / single writer / `wmMutationLock` / IPC handshake | `design.md §3.7, implementation-design.md §5` |
| BrowserCapabilityAdapter / PrivatePayloadStore | `implementation-design.md §6` |

## 2. Acceptance Invariants

acceptance gate は次の二層で構成する。

- **§2.1 State snapshot invariants（1〜13）**: 各 Step 完了直後の `WorldState` に対し評価する。Scenario harness が Step ごとに **同時に成立すること** を検査する。
- **§2.2 Transaction-path properties（A〜F）**: `Controller transaction` の経路が満たすべき semantics。snapshot では検出できないため、§3 の Scenario 群に **専用 Step / 専用 Scenario** を置いて個別に検証する（後段 §3.8 を参照）。

### 2.1 State snapshot invariants（design.md §14 をそのまま固定）

1. ManagedEnvironment manifest が許容範囲内である（schema/version/min daemon version 整合）。
2. ActiveProfile が DesiredWorld と一致する。
3. Slot assignment が DesiredWorld と一致する。
4. active project の desired window 集合が WorldState 上に存在する（identity unique-strong）。
5. archived project の managed window 集合が WorldState 上に存在しない。
6. inactive project の managed window 集合が policy（DesiredWorld 上の inactive policy）と一致する。
7. `WorkspaceViewer` 役割を持つ workspace に存在する viewer window 集合が、active な AI window 集合と一致する。
8. viewer order が slot order と一致する。
9. 各 project の semantic layout が DesiredWorld と一致する（column / stack の意味的一致。pixel exact は要求しない）。
10. final focus が **command policy** と一致する（`design.md §14`）。command policy 自体は projwmd 内部の Intent ↔ final focus 規則に従う（具体は projwmd 設計の責任。本書は整合判定のみを要求）。
11. `WorkspaceProject` 以外の役割を持つ workspace（`Browser / Media / General` 等）と managed project の window が混在しない（`design.md §14` 末項）。
12. TitleContract が要求する title drift 制約が満たされている。
13. transaction commit 後に未処理 DirtyScope が残らない。

### 2.2 Transaction-path properties（design.md §12 semantics）

A〜F は WorldState の snapshot では観測できない。`§3.8 Transaction Contract Scenarios` の各 Step が **専用に** 検証する（snapshot invariants 1〜13 とは独立）。

A. transaction は同時に 1 つだけ進行する（single writer + `wmMutationLock`）。並行 Intent 投入時、observed mutation の同時実行数 ≤ 1。
B. mutation を伴う Operation は対象 window の identity classification が **unique-strong** であることを Precondition として要求する。ambiguous / weak / stale / missing に対する mutation は executor が拒否する。
C. Verifier が predicted vs observed を Diff し、許容範囲外（`WorldDiff.Empty() == false`）なら Controller は replan ループを継続する。`MaxReplans` 超過時はエラー返却し commit しない。許容範囲外のまま commit してはならない。
D. user-origin layout event は `ManualLayoutCandidate` として Reducer が記録するが、`IntentAcceptManualLayout` を経由しない限り `DesiredWorld.AcceptedLayouts` / `Projects[].Layouts` を変更しない。
E. external event (`window-manager / system / timer`) は DesiredWorld を直接変更しない（`design.md §12 step 4`）。
F. stale epoch の event（`ev.Epoch < state.Meta.Epoch`）は Reducer が `Discard` を返し、DesiredWorld にも DirtyScope にも影響を与えない。

## 3. Acceptance Scenarios

`design.md §15 E2E story` 形式。各 Step は `{ human operation, ExpectedInvariants = §2 全項目 }` として、少数の通し story 上で連続実行する。
完成判定は **real OmniWM / sigwm を通る Human-operation E2E acceptance** で行う（§7 参照）。
fake / simulator / recorded は safety / fixture validation / diagnostics / failure reproduction の補助であり、acceptance の代替ではない。

### 3.1 IntentSwitchProfile

シナリオ意図: ManagedEnvironment manifest 上で複数 Profile が定義されているとき、Profile 間の切替は WorldState を新 Profile の DesiredWorld に収束させる。

- **Step S1.1**: 任意 ProfileA → ProfileB に IntentSwitchProfile。ExpectedInvariants 全件成立。
- **Step S1.2**: 同じ Intent (B → B) を再投入。ExpectedInvariants 全件成立、observed WorldState は S1.1 完了時と同等。
- **Step S1.3**: B → A を IntentSwitchProfile。S1.1 直前の WorldState と同等まで復元。
- **Step S1.4**: ProfileB に slot へ assign された project が存在しない場合（空 Profile）、`WorkspaceProject` 役割の workspace は managed window を 0 個まで減じる。

### 3.2 IntentArchiveProject

シナリオ意図: ActiveProfile に assign された Project を archived 状態に遷移させ、その managed window を WorldState から消す。

- **Step S2.1**: ActiveProfile の任意 slot に assign された Project P に対し IntentArchiveProject。ExpectedInvariants 全件成立。Invariant 5（archived project の managed windows が存在しない）が満たされる。
- **Step S2.2**: その後 IntentReconcile を投入。ExpectedInvariants 全件成立かつ observed WorldState は S2.1 完了時と同等（Reducer/Planner 決定性、§5 参照）。

### 3.3 IntentUnarchiveProject

シナリオ意図: archived state にある Project を slot に再展開する。

- **Step S3.1**: archived な Project P を target slot S（ActiveProfile において空 slot または再 assign 可能な slot）に IntentUnarchiveProject。ExpectedInvariants 全件成立。Invariant 4（active project の desired windows が存在）と Invariant 9（layout 一致）が満たされる。
- **Step S3.2**: 同 Intent を再投入。ExpectedInvariants 全件成立、WorldState は S3.1 完了時と同等。

### 3.4 IntentAssignProject / IntentUnassignSlot

シナリオ意図: ActiveProfile の slot manifest を変更し、slot ↔ Project の対応を更新する。

- **Step S4.1**: 既に Project が assign された slot S に対し IntentUnassignSlot。Invariant 3（slot assignment 一致）が更新後 DesiredWorld と整合し、対応 managed window 集合が消失する（Invariant 5 / 6 に従う）。
- **Step S4.2**: 空 slot S に対し IntentAssignProject(P)。Invariant 3 / 4 / 9 が成立する。
- **Step S4.3**: 任意の Step 後に IntentReconcile を投入。観測 WorldState は変化しない。

### 3.5 IntentReconcile

シナリオ意図: 現状 WorldState を ActiveProfile の DesiredWorld に整合させる。

- **Step S5.1**: WorldState が既に DesiredWorld と一致している状態で IntentReconcile。ExpectedInvariants 全件成立。Executor が発行した実 mutation Operation 数は 0（Verifier の Diff が空）。
- **Step S5.2**: Step S5.1 を連続 N 回投入。同上。

### 3.6 IntentAcceptManualLayout

シナリオ意図: user-origin layout event（同 workspace 内の column reorder 等、`WorkspaceProject` 役割上で生じた user 操作由来の semantic layout 変化）を、`IntentAcceptManualLayout` 受領を以って DesiredWorld に取り込む。

- **Step S6.1**: ある Project P の WorldState に user-origin layout event が累積している状態で IntentAcceptManualLayout(P)。受領後 DesiredWorld 上の P semantic layout が更新され、Invariant 9 が新 layout に対し成立する。
- **Step S6.2**: 受領後に IntentSwitchProfile で ProfileA → ProfileB → ProfileA を round-trip。Invariant 9 は S6.1 で更新された layout に対して成立する（古い layout に戻らない）。
- **Step S6.3（受領前の挙動）**: user-origin layout event が累積していて IntentAcceptManualLayout を発行していない状態では、Reducer は DesiredWorld を変更しない（§2-D）。`IntentReconcile` を投入すれば観測 WorldState は古い DesiredWorld に向かって収束する（design §12 contract）。

### 3.7 IntentValidateEnvironment と Lifecycle transaction

シナリオ意図: external event（system / window-manager / timer）に応じて Controller が Lifecycle transaction を起こし、WorldState を DesiredWorld へ再収束させる。

- **Step S7.1（LifecycleBootstrap）**: `projwmd` 起動直後の transaction commit 後、ExpectedInvariants 全件成立。
- **Step S7.2（LifecycleWakeRecovery）**: macOS sleep / wake 後に発火する transaction commit 後、ExpectedInvariants 全件成立。
- **Step S7.3（LifecycleDisplayReconfigure）**: display topology 変更（外部モニタの抜き差し / 解像度変更）後の transaction commit 後、ExpectedInvariants 全件成立。
- **Step S7.4（LifecycleFullReconcile）**: safety timer による低頻度発火後の transaction commit 後、ExpectedInvariants 全件成立。
- **Step S7.5（IntentValidateEnvironment）**: legacy agent が環境に残っていれば `LegacyAgentReport` 相当の検出を上げ、`LegacyAgentRemove` policy 下では報告通り削除されている（design §3）。

### 3.8 Transaction Contract Scenarios（§2.2 A〜F の専用検証）

各 Step の終了条件は `WorldState` snapshot ではなく **下記に明記された property** が成立することである。snapshot invariants 1〜13 はこの Scenario 群では評価しない（property に集中するため）。

- **Step S8.A (single writer)**: N 並行で異なる `projwmctl` Intent を daemon IPC へ投入する。trace / transaction log / observed mutation span 上で mutation transaction が重ならず、全 transaction が直列に完了する。
- **Step S8.B (precondition unique-strong)**: 実 workspace `A/Q/W/E` 上で同一 DesiredWindow に該当しうる real candidate が 2 件以上ある状態（identity classification = ambiguous）を安全に作り、`projwmctl reconcile` 等の human-visible Intent を投入する。daemon は unsafe mutation を拒否し、対象 window への mutation を発行せず、commit しないか明示的 error generation を残す。
- **Step S8.C (verifier replan)**: real backend 上で predicted と observed が安全に不一致になる状況を作り、Controller が bounded replan を行うことを trace で検証する。`MaxReplans` 超過時は error を返し、許容不能 diff のまま commit しない。
- **Step S8.D (user-origin layout no-write)**: 実機 OmniWM 操作として同一 workspace 内 column reorder を発生させる。`IntentAcceptManualLayout` を呼ばずに `projwmctl reconcile` を投入した時点では `DesiredWorld.AcceptedLayouts` および `Projects[P].Layouts` がイベント前と等しく、ManualLayoutCandidate 相当の evidence だけが記録される。
- **Step S8.E (external event no DesiredWorld write)**: `window-manager` (`WindowsChanged`) / `system` (`Wake`, `DisplayChanged`) / `timer` (`SafetyTimer`) に相当する external event を human-visible operation / daemon sidecar に発生させる。DesiredWorld の任意フィールド（ActiveProfile / Profiles / Projects / AcceptedLayouts / FocusPolicy）が event 投入前と等しい（DirtyScope / Lifecycle のみ立ち上がる）。
- **Step S8.F (stale epoch discard)**: 先行 transaction の epoch に属する event が後続 transaction 後に遅延到着する状況を human-visible operation / daemon sidecar に作る。stale event は DesiredWorld / DirtyScope / ManualLayoutCandidate に影響せず、最新 epoch の WorldState が維持される。

## 4. External Event Reaction（user Intent ではないが Acceptance に必須）

`design.md §12 / §13` の EventReaction semantics に従う。Reducer は次の事象を DirtyScope / Lifecycle / ObserveScope に変換し、Controller transaction として処理する。

### 4.1 managed window の OS-level 強制終了
EventSource = `window-manager`（windows-changed 由来）。Reducer は対応 DirtyScope を発行。Controller は対象 Project の DesiredWorld を target に Plan を構築し、unique-strong identity が再達成されるまで spawn 系 Operation を発行する。transaction commit 後 ExpectedInvariants 全件成立。

### 4.2 managed window の cross-workspace 移動（user 操作）
EventSource = `user`（layout-changed user-origin）。Reducer は manual-layout candidate **にしない**（cross-workspace 移動は同一 workspace 内 layout 変化ではない）。Controller は target workspace への `OpMoveWindowToWorkspace` を発行し、transaction commit 後 Invariant 9 / 11 が成立する。対象 window の identity は維持される（move であって respawn ではない）。

### 4.3 managed window の close（user 操作）
EventSource = `user` または `window-manager`（windows-changed）。Reducer は missing classification として DirtyScope を発行。Controller は §4.1 と同等の Plan を構築する。

### 4.4 同一 workspace 内 column reorder（user 操作）
EventSource = `user`（layout-changed user-origin）。Reducer は manual-layout candidate に登録するのみで DesiredWorld を変更しない（§2-D）。Controller は当該 layout に対し violation 判定を行わない（design §12 contract）。`IntentAcceptManualLayout` 受領で初めて DesiredWorld が更新される（§3.6）。

### 4.5 isolated / external apps
`design.md §14` 末項にある「isolated / external apps が managed project と混ざらない」 invariant を保つため、projwmd は `WorkspaceProject` 以外の役割を持つ workspace 上の window に対して mutation Operation を発行しない。これら workspace 上の event は ObserveScope 補足や evidence として扱われ、DesiredWorld の対応 managed window 集合に影響を与えない。

## 5. Determinism Requirements

`design.md §11, §13` から導く acceptance：

- Reducer は同一 (WorldState, Intent) に対し同一 DesiredWorld を返す（pure function）。
- Reducer は同一 (WorldState, Event) に対し同一 EventReaction を返す。
- Planner は同一 (WorldState, DesiredWorld, DirtyScopes) に対し同一 Plan を返す（deterministic rule-based）。
- Verifier の Diff classification は同一 (PredictedWorld, ObservedWorld) に対し同一 WorldDiff を返す。
- Invariant 10（final focus = command policy）と決定論性の系として、Intent 投入時点の current focus / current workspace / 当時 frontmost window は最終 commit 後 WorldState に影響しない。

## 6. Privacy Requirements

`implementation-design.md §6` から導く acceptance：

- BrowserCapabilityAdapter が取得する URL / cookie / session token / authentication state は **PersistentStore に保存されない**。
- これらは PrivatePayloadStore（PersistentStore とは別の persistence boundary）に保存される。PersistentStore 上には redacted reference のみ存在する。
- log / report / test artifact / `projwmctl` 任意 query 出力から URL 本体・cookie 本体・token 本体が読み取れない。
- legacy `SavedURLs` 由来 entry は migration で PrivatePayloadStore へ移送し、PersistentStore からは削除される。
- archived browser project の unarchive 後、当該 Project の browser window の tab URL / login state は archive 直前の状態に restore される。
- canonical Human E2E の visible ideal state は Vivaldi window の workspace / layout / isolation だけを扱う。
  Vivaldi の tab URL / cookie / token / session content は canonical layout oracle には含めず、本 §6 の privacy acceptance で検証する。

## 7. E2E Acceptance Authority

`design.md §15` の E2E story 契約より：

- projwm-next の完成判定は **Human-operation real backend acceptance run** だけで行う。
- ここでいう acceptance run は軽量な確認ではなく、§3 の Scenario / Step と §2 の Invariant が
  documented `projwmctl`、keyboard / window manager 操作、実アプリ操作を入力とし、
  real `projwmd -> Controller -> OmniWM / sigwm -> visible observation -> restart-visible persistence`
  の経路で成立することを指す。store / trace inspection は transaction property と failure diagnosis の補助であり、
  visible oracle の代替ではない。
- fake / simulator / recorded backend は real E2E を安全に実行するための preflight、fixture validation、
  diagnostics、failure reproduction に使ってよい。ただし、それらの成功は acceptance の代替ではなく、
  projwm-next 完成の証拠にもならない。
- real E2E が unsafe / unobservable / unconstructible な Step は skip せず、acceptance blocked として赤く出す。

## 8. Out of Scope（spec ではないもの）

- 具体的な WorkspaceID 文字列、ProjectID 文字列、bundle identifier、window title 文字列、ManagedEnvironment manifest 上の application path 等は **Nix manifest fixture** の責任である。本書は fixture が変わっても変わらない。
- CLI 出力フォーマット、socket protocol の wire format（バージョン互換は §1 IPC handshake に内包）、launchd plist 構成は本書の範囲外（implementation-design §3, §5 で扱われる）。
- command policy の **具体規則**（どの Intent が完了後にどの WorkspaceID へ focus を遷移させるか）は projwmd 内部設計の責任であり、本書は Invariant 10 で「整合判定が成立する」ことだけを要求する。
- test infra（input-lock 機構、ideal-state setup、fixture reset 手順）は本書の acceptance 基準ではなく、E2E story harness の運用詳細である。

## 9. 完成定義

projwm-next が「完成」と認められる条件：

1. §3 の全 Step と §4 / §6 / §8 の acceptance が、real backend (OmniWM / sigwm 経由) 上の
   Human-operation E2E story として成立する。
2. Controller transaction の §12 contract（single writer / wmMutationLock / Verifier replan /
   user-origin event の DesiredWorld 直接書込み禁止）が、E2E trace / transaction log / test PersistentStore
   から監査可能な形で satisfy されている。
3. §6 Privacy Requirements が Human-operation E2E leak test で検証される。
4. fake / simulator / recorded diagnostics が、real E2E の preflight / failure reproduction として
   必要な範囲で成立している。ただしこれは 1〜3 の代替ではない。
5. legacy agent / legacy daemon の launchd / store 痕跡が WorldState / process tree から物理的に存在しない（Phase C cutover で削除）。

## 10. 非目標（spec が言わないこと）

- 旧実装の挙動互換は acceptance 基準ではない。本書は新 design の Intent / Invariant で完全に規定される。
- Operation の具体列（Plan の中身）は spec の責任ではない。Plan の正しさは Verifier が観測 Invariant で判定する。
- 特定 backend（OmniWM / niri）の API 詳細は spec に持ち込まない。adapter contract は `design.md §7` が所有する。
