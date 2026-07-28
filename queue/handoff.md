# projwm-next Handoff Document

> Generated from session 34c4a269 — covers Phase A through current live-validation work.
> Source of truth for SSOT: `queue/design.md`, `queue/implementation-design.md`, `queue/specs.md`.

---

## 1. 全体像

`projwm-next` は macOS のワークスペース管理デーモン。Go 実装。
OmniWM / Ghostty / Zed / Vivaldi と連携し、プロジェクト別の可視配置を維持する。

### 稼働構成（現在）

```
OmniWM=true  projwm-next launchd=true  Ghostty=true  AeroSpace=false
```

SocketPath: `~/.local/state/projwm-next/projwmd.sock`  
Store: `~/.local/state/projwm-next/store`（G000001 が現在の最新世代）  
Startup provenance: `~/.local/state/projwm-next/startup-provenance.json`

---

## 2. 今セッションまでにやったこと

### フェーズ A: 基盤整備（初期〜Checkpoint 010）
- Store (FileStore / PersistentStore), Controller, Planner, Executor, Verifier の完全実装
- `projwmd` / `projwmctl` / `projwmevent` / `projwmstore-bootstrap` バイナリ
- Nix モジュール (`modules/darwin/projwm/default.nix`) での wiring
- シミュレーター + フェイク E2E テストフレームワーク

### フェーズ B: Authority・Provenance 強化（Checkpoint 010〜025）
- `projwmd` 起動時 Nix マニフェストダイジェスト検証
- `startup-provenance.json` の完全 provenance 記録（manifest / store / backend / launchd label / socket 等）
- `projwmstore-bootstrap` による管理者所有ルート世代 + `BootstrapManifestDigest` バインド
- `LoadGenerationAncestry` でルートまで遡ってダイジェスト一致を確認
- sidecar (windows-changed / display-changed / layout-changed / wake / safety-timer) の launchd 宣言 + ランタイム証明
- socket authority: manifest の `daemons.socketPath` と実 socket が一致しないと daemon/client 両側で拒否
- `projwmctl` / `projwmevent` クライアント側でも manifest 検証

### フェーズ C: 実際の Human E2E テスト（Checkpoint 020〜033）
- S1〜S8 の実機 Human E2E ストーリー本体を `scenarios/real_acceptance_test.go` に実装
- `newHumanE2E` ハーネス: production-shaped 起動、bootstrap CLI 経由でのストア初期化
- 現在グリーンな実機テスト:
  - S1 (canonical placement), S2 (switch profile), S3 (archive/unarchive), S4 (assign/unassign)
  - S5.1 (zero-mutation reconcile), S5.2 (idempotent reconcile)
  - S6.1〜S6.3 (manual layout accept / profile round-trip / restart persistence)
  - S7.1 (startup lifecycle convergence), S7.4 (validate-environment legacy agent policy)
  - S8.A (single-writer concurrent transactions)
  - S8.B (unique-strong identity: ambiguous duplicate Ghostty → reconcile failure 証明)
  - S8.D (production removal without close-window: Ghostty-only lifecycle removal trace)
  - EVT.4.1 (managed window forced termination recovery)
  - EVT.4.2 (managed window cross-workspace move recovery)
  - EVT.4.4 (same-workspace reorder: ManualLayoutCandidate committed, no DesiredWorld write)
  - EVT.4.5 (external app isolation)
  - AUTH.7.2 (restart-visible persistence)

### フェーズ D: 機能強化（Checkpoint 025〜033）
- **ブラウザプライバシー**: `FilePrivatePayloadStore`、URL を PersistentStore から排除、`about:blank` フォールバック削除
- **Vivaldi automation profile**: `projwm-next` プロファイル強制（`"default"` は拒否）
- **Ghostty lifecycle removal**: controller-owned AI/shell/viewer のみ `KindKillSession` 許可
- **Zed/Vivaldi 削除ブロック**: planner 非生成 + executor 拒否 + `internal/lifecyclecontract` 純粋バリデーター
- **`RunOnRealBackend`**: 明示オプトイン real runner (AllowLive=true + nix authority 必須)
- **determinism 非ライブ証明**: reducer/planner/verifier の繰り返し determinism テスト
- **S8.E 非ライブ証明**: 全外部イベントソースの no-DesiredWorld-write contract
- **non-live lifecycle contract**: `ZedProjectScopedRemovalEvidence` / `VivaldiBrowserWindowCloseEvidence` バリデーター (wired なし)
- **store 再 bootstrap**: 既存の G000001 に `BootstrapManifestDigest` が欠如 → store 移動 + `projwmstore-bootstrap-next` で再作成 → daemon 起動成功

---

## 3. 現在の実装状況

### 起動状態

```
daemon:  running (pid=94631)
socket:  /Users/yuta/.local/state/projwm-next/projwmd.sock (active)
store:   G000001 (BootstrapManifestDigest verified)
provenance: startupLifecycleStatus=blocked
  reason: planner refused: dotfiles/editor/1 ambiguous (2 Zed windows open)
  → 正常動作（S8.B unique-strong policy）; daemon は IPC 受付中
```

`startupLifecycleStatus=blocked` は **正常**: 複数の Zed ウィンドウが open のため planner が ambiguous 拒否。
これは S8.B unique-strong policy の正しい動作。daemon 自体は稼働中で IPC を受け付けている。

### AcceptanceCoverageMatrix 現況

`go test ./... -count=1` → **全 green**

| 状態 | 件数 | 説明 |
|---|---|---|
| `covered` | 16 | 実機グリーン実装済み |
| `partial` | 11 | 実機ストーリー有り / authority 未完 |
| `blocked` | 8 | 実機ストーリー未実装 / policy blocked |

---

## 4. 残作業（優先順）

### 4.1 Zed project-scoped-app 削除契約 (blocked)

現状: planner は Zed に対して close/kill を生成しない。`lifecyclecontract.ZedProjectScopedRemovalEvidence` バリデーターは純粋実装済みだが executor に wired なし。

必要なもの:
1. **Zed 実アプリ証明アダプター**: タイトルで "project-scoped removal" を確認 (exact disappearance, unsaved-change clean)
2. **OmniWM window ID ↔ DesiredWindowID 相関証明**: タイトル/PID 以外の証明
3. **unsaved-change 拒否証明**: ダーティ状態の Zed を閉じようとしたとき拒否する証明

対象ファイル:
- `internal/lifecyclecontract/contract.go` (バリデーター既存)
- `internal/adapter/wm/appcontract.go` (wiring 追加予定)
- `internal/planner/planner.go` (close 生成フラグ)

### 4.2 Vivaldi browser-window-close 契約 (blocked)

現状: Vivaldi 閉じはすべて blocked。`lifecyclecontract.VivaldiBrowserWindowCloseEvidence` は純粋実装済みだが wired なし。

必要なもの:
1. **automation profile ウィンドウマーカー**: `projwm-next` プロファイルのウィンドウを OmniWM ID と相関付ける
2. **payload/tab ↔ OmniWM ウィンドウ相関**: PrivatePayloadRef と実ウィンドウの対応証明
3. **ユーザープロファイル分離証明**: `projwm-next` プロファイルのみ操作していることの証明

対象ファイル:
- `internal/adapter/browser/vivaldi.go` (OpenResult.BrowserWindowID が現在空)
- `internal/adapter/wm/appcontract.go`

### 4.3 S8.E 実機ハーネス (blocked)

現状: 非ライブ no-DesiredWorld-write contract は green。実機ハーネスは未実装。

必要なもの:
- **物理スリープ/ウェイク**: `PROJWM_NEXT_REAL_ACCEPTANCE=1` 下でのスリープ/ウェイクイベント
- **ディスプレイ再設定**: display-changed EventHint の実機ハーネス
- **ユーザーウィンドウ close**: managed window を OS レベルで閉じた後のリカバリー証明 (EVT.4.3)

### 4.4 Live Vivaldi/OmniWM 相関 (partial)

現状: `VivaldiAdapter.Open()` は `BrowserWindowID` が空で返る。
OmniWM での new-window diff によるウィンドウ ID 特定が未実装。

対象: `internal/adapter/browser/vivaldi.go` → `Open()` の戻り値に `BrowserWindowID` を埋める

### 4.5 全実機ストーリーへの invariant audit アタッチ (partial)

現状: `TestHumanE2EFullInvariantAuditSteps` は canonical story のみ。他の実機テストには `invariant.CheckAll` が未アタッチ。

### 4.6 Privacy 実機 Human E2E (blocked)

現状: 非ライブ artifact/CLI/provenance 漏洩テストは green。
実機ブラウザリストア + セッション authority が pending。

### 4.7 Legacy cutover (blocked)

前提: 上記 4.1〜4.4 がすべて green になること。

必要なもの:
1. legacy `cmd/projwm` / `internal/projwm` の削除
2. 旧 launchd エージェントの物理的不在証明
3. single-writer 証明

---

## 5. 重要な実装判断・ポリシー

### 削除ポリシー

| アプリ | 状態 | 条件 |
|---|---|---|
| Ghostty (controller-owned) | **許可** | `KindKillSession` + title/bundle/desired evidence 一致 |
| Zed | **blocked** | app-specific contract 未実装 |
| Vivaldi | **blocked** | app-specific contract 未実装 |

### CloseWindow は production で禁止

`queue/implementation-design.md` first-implementation allow/block matrix により:
- planner は `real`/`omniwm` バックエンドで close-window を生成しない
- executor も拒否
- `SigWM.Close()` はブロック

### Vivaldi は必ず `projwm-next` プロファイル

`internal/adapter/browser` の `VivaldiAutomationProfile = "projwm-next"` を使う。
`"default"` や空文字列は `sigwm` / `appcontract` 両側で拒否される。

### RunOnRealBackend のみが live 実行可能

`RunOnAllBackends` は fake + simulator のみ。
`RunOnRealBackend` は `AllowLive=true` + nix authority + omniwm backend 必須。
`NewBackend(BackendReal, ...)` は直接呼ぶと panic。

### フォールバック禁止

以下は `lifecyclecontract` でも planner/executor でも明示的に禁止:
- title-only / bundle-only / frontmost / single-candidate fallback
- saved-URL-only / tab-count-only / token-only
- default-profile / user-profile での Vivaldi 操作
- exact disappearance なしの removal

---

## 6. アーキテクチャ・重要ファイル

```
cmd/
  projwmd/main.go             — daemon; productionAdminBootstrap / startup provenance
  projwmctl/main.go           — IPC CLI
  projwmevent/main.go         — sidecar EventHint クライアント
  projwmstore-bootstrap/      — 管理者 bootstrap CLI

internal/
  store/file.go               — FileStore; LoadGenerationAncestry (BootstrapManifestDigest 検証)
  controller/controller.go    — 収束ループ; rollback; dirty scope
  planner/planner.go          — intent/event → operations; close-window blocked
  executor/executor.go        — operations 実行; PreUniqueStrong; KindKillSession guard
  identity/identity.go        — ClassUniqueStrong / ClassAmbiguous resolution
  verifier/verifier.go        — diff; duplicate window detection
  scenario/scenario.go        — RunOnRealBackend (explicit opt-in); RunOnAllBackends (fake only)
  lifecyclecontract/contract.go — 純粋バリデーター (Zed/Vivaldi; wired なし)
  adapter/wm/sigwm.go         — OmniWM adapter; Close blocked
  adapter/wm/appcontract.go   — spawnVivaldi; VivaldiAutomationProfile 強制
  adapter/browser/vivaldi.go  — Vivaldi adapter; default/blank profile 拒否
  migration/                  — legacy state → DesiredWorld migration

scenarios/
  real_acceptance_test.go     — 実機 Human E2E ハーネス; newHumanE2E
  acceptance_integrity_test.go — static anti-cheat テスト

modules/darwin/projwm/
  default.nix                 — Nix wiring; launchd; manifest digest; bootstrap wrapper
```

---

## 7. ビルド・テストコマンド

```bash
# 通常テスト（常に green を維持）
cd modules/darwin/projwm/projwm-next
go test ./... -count=1

# integration テスト（非ライブ）
go test -tags integration ./... -count=1

# 実機 Human E2E（OmniWM + Ghostty + projwmd 稼働中が前提）
PROJWM_NEXT_REAL_ACCEPTANCE=1 go test -tags integration ./scenarios -run TestHumanE2E<Name>Steps -v

# Nix ビルド確認
nix build .#darwinConfigurations.yuta.config.system.build.toplevel --no-link

# darwin rebuild
sudo darwin-rebuild switch --flake .#yuta

# store 再 bootstrap（store がない / BootstrapManifestDigest が欠如した場合）
cp ~/.local/state/projwm-next/store ~/.local/state/projwm-next/store-backup-$(date +%Y%m%dT%H%M%S)
mv ~/.local/state/projwm-next/store ~/.local/state/projwm-next/store-old-$(date +%Y%m%dT%H%M%S)
projwmstore-bootstrap-next --desired-world <path/to/desired_world.json>
launchctl kickstart -k gui/$(id -u)/org.nixos.projwmd-next

# daemon 状態確認
launchctl print gui/$(id -u)/org.nixos.projwmd-next | grep -E 'state|pid|last exit'
tail -f ~/.local/state/projwm-next/logs/projwmd.err.log
```

---

## 8. 既知の問題・注意点

### startupLifecycleStatus=blocked（Zed 複数ウィンドウ時）

同一 bundle-id の Zed ウィンドウが複数開いている場合、planner が unique-strong 証拠なしとして
reconcile を拒否する。これは **正常動作**（S8.B ポリシー）。
daemon 自体は稼働しており IPC を受け付けている。
workaround: Zed ウィンドウを 1 つに絞るか、MatchHints に title-prefix を追加する。

### BootstrapManifestDigest 欠如（旧 store）

古い `projwmstore-bootstrap` で作成した store は `BootstrapManifestDigest` が null。
新しい daemon は起動時に拒否する。修正方法は §7 の「store 再 bootstrap」を参照。

### pkill / killall の使用禁止

テストクリーンアップでは `kill <PID>` のみ使用。bundle-id / 名前ベースの kill は禁止。

### Ghostty ウィンドウのタイトルフィルター

テスト所有 Ghostty の絞り込みは `bundle=com.mitchellh.ghostty` AND `title ~ ^(ai|shell|ai-view)-[0-9]+:` の両方。

### PrivatePayloadStore と PersistentStore の分離

生の URL / ブラウザペイロードは `FilePrivatePayloadStore`（`~/.local/state/projwm-next/private-payloads/`）のみ。
PersistentStore の journal / desired_world には opaque `PrivatePayloadRef` のみ保持。

---

## 9. 完了定義（specs.md §9 参照）

以下がすべて満たされたとき「完成」:

- [ ] specs.md §3/§4/§6/§8 の全実機ストーリーが green
- [ ] transaction audit evidence が production-shaped
- [ ] privacy Human E2E が green（実機 Vivaldi セッション authority 含む）
- [ ] diagnostics が補完物でなく証明になっている
- [ ] legacy projwm traces が物理的に不在
- [ ] `AuthorityStatus=covered` が全行（現在: partial=11, blocked=8）

現在の authority coverage: `covered=16 partial=11 blocked=8`（`AcceptanceCoverageMatrix` 参照）

---

*Last updated: 2026-05-08 by session 34c4a269*
