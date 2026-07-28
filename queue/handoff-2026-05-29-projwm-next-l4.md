# projwm-next L4 Acceptance ハンドオフ — 2026-05-29

> Handoff from previous Claude session (compacted context, autonomous /
> "全部やって良い" 認可下で深夜稼働の末に判断ミスが顕在化し、自ら別エージェント
> への引継ぎを user に提案した結果作られたドキュメント)。
>
> 受け手は **clean context の新しい Claude エージェント** を想定。
> 前任(私)の仮説を信用せず、観測と SSOT 本文から自力で再構築すること。

---

## 0. あなたへの直接メッセージ

あなたは projwm-next の **L4 acceptance を完成させる**ために召喚されました。
前任は context が汚染され、深夜の判断精度が落ちて誤診断(後述 §6)を犯したため、
clean context で再開する判断を user と合意してあなたを呼びました。

### 守るべき方針 (user の明示)

1. **「やることを受動的に理解するのではなく動的に全体像を理解」**
   引継ぎ直後の最初の着手は急がない。**まず読む → 質問する → 動く**。
   過去にこの方針で red test 2 件が spot-check で発見され S27 切片の設計が
   修正された実績がある (`queue/orchestration-state.md` §0ter A1)。
2. **agent hallucination 警戒**: 過去 Agent 1 が §4.1 OP01-06/11/14-17 の実装
   ステータスを systematic に捏造した。**字面信用禁止、必ず spot-check**
   (memory: `feedback_orchestrator_responsibility`)。前任(私)の本文書内の
   主張も例外ではない、git log と実コードで verify すること。
3. **実機 verify granularity**: 実機検証は orchestrator 自走で済ませる、
   user 依頼は「全体機能完成時のみ + 具体手順付き」
   (memory: `feedback_verification_granularity`)。
4. **production 環境使用は許可済**: `production 環境を使って良い`
   `私の環境をいじっても良い` `darwin-rebuild をしても良い` (user 明示)。
   ただし §7 の安全 precondition は厳守。

---

## 1. 必読リスト (この順序で)

| # | path | 役割 | 行数目安 |
|---|------|------|----------|
| 1 | **本ドキュメント** | あなたが今読んでいるもの | ~ |
| 2 | `~/.claude/projects/-Users-yuta-dev-dotfiles/memory/MEMORY.md` | index | <20 |
| 3 | `~/.claude/projects/.../memory/project_projwm_overhaul.md` | プロジェクト履歴 (本セッション末尾に追記済) | ~230 |
| 4 | `~/.claude/projects/.../memory/feedback_*.md` (3 件) | user feedback 内面化 | 各 ~30 |
| 5 | `queue/projwm-next-spec.md` v1.11 | **SSOT 本体**、これが真理 | 2081 |
| 6 | `queue/ssot-coverage-matrix.md` | atomic 要求 375 件マトリクス | 594 |
| 7 | `queue/ssot-slice-plan.md` | 29 切片計画 + DoD 13 条項 | 182 |
| 8 | `queue/orchestration-state.md` | 過去セッション snapshot 群 | 776 |
| 9 | `queue/handoff.md` | 初期 handoff (Phase A-C 歴史的経緯) | 長 |
| 10 | `modules/darwin/projwm/projwm-next/internal/ssottest/ledger_test.go` | **現状ステータスの真理**、SSOT 全項目の status と test owner が並ぶ | 長 |
| 11 | `modules/darwin/projwm/projwm-next/queue/staging/s10_zed_crash_fixture.patch.todo` | 前任が staging に退避した未 commit パッチ (S10 fixture、reorder バグ解決待ち) | 75 |

**§5 と §6 を読む前に、まず §1 の #5 SSOT 本体 §9.1 (L4 受入シナリオ S1-S10) と
#10 ledger の現状を読むこと**。前任の解釈に汚染される前に一次情報で像を作る。

---

## 2. 現状サマリ — 観測事実のみ (前任の仮説は §6 に隔離)

### L3: SSOT 全項目 green (2026-05-29 確定)

- ISO-01 (test prefix discipline): `productionAuditAllowlist` 空 + statusCovered
  昇格。commit `dd1395f`。
- L3 cleanup の test isolation flake: pkill-by-title backstop で堅牢化
  (`00ff777` / `3b0a463`)。
- S6 startup-provenance generation 検査の monotonic 修正: `813129e`。

### L4: 6/10 green、残 4/10

| ACC | status | green 化セッション |
|-----|--------|-----------------|
| S1 SwitchProfile | ✅ statusRealOnly | B-05 fix で 2026-05-27 |
| S2 ArchiveProject | ❌ statusRed | **未達** |
| S3 UnarchiveProject | ✅ statusRealOnly | 2026-05-27 |
| S4 Assign/Unassign | ✅ statusRealOnly | 2026-05-27 |
| S5 Reconcile | ✅ statusRealOnly | 2026-05-27 |
| S6 macOS restart | ❌ statusRed | **未達** |
| S7 OmniWM restart | ❌ statusRed | **未達** |
| S8 Summon idempotency | ✅ statusRealOnly | cycle anchor fix `fc2033c` |
| S9 Drift repair | ✅ statusRealOnly | focus-race fix `0d0fb22` (2026-05-29) |
| S10 Crash recovery | ❌ statusRed | **未達** (tmux は PASS、Zed は別経路 fail、§6 参照) |

### 直近 commit (本セッション 2026-05-29)

```
b9d7d7a  fix: vivaldiInspectFunc は Chromium helper PID を親まで遡って判定
0d0fb22  fix: S9 を実機 green — focus-race 除去 + browser bundle 整合
dd1395f  test: ISO-01 を strict mode で promote — allowlist 削除 + statusCovered
813129e  test: startup-provenance の generation 検査を monotonic に修正 (S6)
3b0a463  test: controller startup L3 cleanup に pkill-by-title backstop
00ff777  test: L3 real_ops cleanup を pkill-by-title backstop で堅牢化
cfbaa59  fix: shell/AI/viewer spawn を冪等化 (SSOT §6.6 IDEMP、L3 実機回帰)
72b74ca  fix: executor は ambiguous identity を focus-tiebreak で解決 (INV-01)
```

git push されているかは未確認 (`git log origin/main..HEAD` で確認)。

---

## 3. 観測済の omniwm 実機挙動 (SSOT に明文化されていない実機知見)

これらは前任が実機実験 + 失敗 + 観察から抽出した「コード前提の事実」。
**まず疑い、もう一度自分で実機で verify してから採用すること**。`omniwmctl` で
直接叩けば全て再現可能。

### 3.1 cross-display move は動く (重要な訂正)

前任の前々セッションは「omniwm は cross-display move できない」と誤断したが、
user の指摘で再実験した結果 `omniwmctl command move-to-workspace <num>` は
focused window を別 display の workspace に移動できることが確定。**ただし**
タイミング依存があり、`window focus <id>` 直後に `command move-to-workspace`
を発行すると、focus が settle する前に move 対象が想定外の window になる
race が存在 (S9 で実害)。修正は `waitForFocusedLiveWindowID` で focus settle
を待ってから move する。

### 3.2 `open -na <App>` の起動位置

`open -na Vivaldi --args ...` は **Vivaldi が最後に active だった display の
現 active workspace** に新窓を出す。projwmd は spawn 直後に
`moveLiveToWorkspaceLocked` で intended workspace へ移動する設計 (sigwm.go:1077
付近)。だが移動先が cross-display だと §3.1 の race が絡む。

### 3.3 Chromium helper PID は `--user-data-dir` を argv に持たない

managed Vivaldi の main process だけが `--user-data-dir=...projwm-next/vivaldi-data`
を argv に持ち、renderer/GPU helper は持たない。omniwm が window owner として
helper PID を一瞬報告すると単 PID 検査では External 誤分類。前任の commit
`b9d7d7a` で parent (ppid 上限 4 hop) も検査する fix を入れた。**ただし S2 の
本問題はこれではない (§6 参照)**。

### 3.4 Zed は single-process (GPUI が `--user-data-dir` を無視)

test の managed Zed window と user の Zed window が **同一 pid を共有する**。
test 内で `terminateLiveWindowProcess(zedWindow)` を実行すると user の編集中
Zed も殺される。前任は実際にこの事故を起こし user の Zed pid 99057 を kill
した。**S1/S2/S10-Zed は precondition (Zed が 1 main process だけ存在) を
満たさないと走らせてはいけない**。

加えて Zed は常に `--crash-handler` subprocess を 1 つ持つので
`pgrep -x zed` は最低 2 を返す。Zed 数を数えるときは:

```bash
pgrep -fl "Zed.app/Contents/MacOS/zed" | grep -v "\-\-crash-handler"
```

### 3.5 omniwm catalog の flicker

managed Vivaldi が off-display (例: workspace 13、main display ではない側) に
居ると、omniwm の catalog が **間欠的に当該 window を見失う** ("class=missing
cands=0 ↔ unique-strong cands=1" を交互観測)。これが S2 の旧仮説の根 (前々
セッション)。後述 §6 の S2 真因とは別問題で、後述経路と複合する可能性大。

### 3.6 launchd 管理 daemon との衝突

production daemon は launchd `gui/501/org.nixos.projwmd-next` で常駐している。
L4 test は `quiesceProductionDaemon` で bootout、test 終了時に
`restoreProductionDaemon` で bootstrap し直す。test 走行中に手動で
`launchctl bootstrap` すると socket が衝突する。test cleanup が確実に
restore するかは run 毎に確認すること (前任は時々 leftover に遭遇)。

---

## 4. 残課題 4 件 — 観測された fail mode と切り分け

各項目は **(a) 観測された失敗、(b) 確定済の事実、(c) 仮説の状態** の 3 段。
(c) は前任の仮説で必ずしも正しくない。

### 4.1 ACC-S2 ArchiveProject (close→re-spawn 経路)

**(a) 失敗**: `TestHumanE2EArchiveUnarchiveSteps` で
`FAIL_INVARIANT[cli/assign Q projwm-next-test-main]: controller: failed to
converge after 4 replans (last ops=[spawn-browser:Q observe-barrier focus-workspace:A])`
+ cleanup で `killVivaldiAutomationProcesses: SIGKILL 6 Vivaldi automation pids`。

**(b) 確定**:
- `TestHumanE2EProductionRemovalWithoutCloseWindowSteps` (同 helper 関数、ただし
  close-window 経由しない) は **PASS**。差分は close→post-close 再 spawn 経路の
  有無のみ。
- 6 個の Vivaldi が累積する = planner が `spawn-browser:Q` を replan 毎に再 emit
  している = 何かが「Vivaldi まだ無い」と判定し続けている。
- §3.3 の parent-walk fix を入れても症状不変 → vivaldiInspect の helper PID
  問題ではない。

**(c) 仮説候補** (どれも未 verify):
- 仮説α: close 後、planner が古い `LiveWindow` ref をまだ Desired に持ち、
  「target slot に既に居る」と判定して新 spawn 後の move を発火しない。
- 仮説β: `open -na Vivaldi` が user の生 Vivaldi instance に hop して
  managed と認識されない (B-06 経路の再発)。
- 仮説γ: identity resolver が close 直後の dead ref と新 PID を区別できず
  ambiguous→refuse mutation。

**verify するには**: planner replan trace を stderr に出す env を立てるか、
`omniwmctl query windows` を 500ms 周期で background record するのが効く
(§9 で詳述)。

### 4.2 ACC-S6 macOS restart recovery

**(a) 失敗**: scenarios 内に未実装、ledger 上 statusRed (coverage-gate のみ
fail を表明)。

**(b) 確定**:
- production daemon が launchd 管理外で test daemon を bootstrap する
  仕組みになっており、launchd event-source proof not-observed
  (memory `project_projwm_overhaul.md` §残 L4 boundary)。
- startup-provenance generation 検査は前任セッションで monotonic 修正済
  (`813129e`)。

**(c) 仮説**: clean session で launchd 管理 daemon を再起動 (`launchctl kickstart -k`)
した後の reconstruction-from-live が走れば走れる、設計は既に B3/B4 で実装済
(task #111 completed)。**fixture 不足**が真の理由。

### 4.3 ACC-S7 OmniWM restart recovery

**(a) 失敗**: scenarios 内に未実装、ledger 上 statusRed。

**(b) 確定**:
- 前任セッションで executor の resolveDesired は ambiguous identity を refuse
  していたバグを `72b74ca` で `ResolveWithFocusTiebreak` 化して修正済。
- omniwm restart は user の non-managed window も動かす副作用があり、
  external-workspaces invariant 違反になりやすい。

**(c) 仮説**: clean session + B-06 残対応で実装可能、ただし user の生
windows を動かすので user 不在/承諾下が安全。

### 4.4 ACC-S10 Crash recovery

**(a) 失敗 (前任 2026-05-29)**: 前任が S10 Zed-crash fixture を実装
(precondition 付き、Zed が 1 個だけのとき走る) → `op-reorder-2:
sigwm.ReorderColumns[Q]: order did not settle` + ext-workspace invariant
で M に dev.zed.Zed が漏れる。fixture patch は
`projwm-next/queue/staging/s10_zed_crash_fixture.patch.todo` に保存
(未 commit)。

**(b) 確定**:
- `want=[76730, 75921, 76030, 76230, 73554]` vs
  `got =[76730, 76230, 76030, 75921, 73554]` — **中間 3 列が完全逆順**。
  PID 昇順 vs 降順の問題に見える。
- tmux crash 部分は前任の前々セッションで実機 PASS 確認済 (`18411a0`)。
  Ghostty 部分も既存テストでカバー。残るは Zed 部分。
- M (main display) の active workspace に新 Zed が出てしまっている = controller
  が Q に move する前 (または move 失敗) で観測されている。reorder 失敗が
  原因で move 全体が abort された可能性。

**(c) 仮説**:
- reorder の column 順を omniwm に渡す段で sign 反転 or 配列が逆順に渡されて
  いる。executor (sigwm.go) の `ReorderColumns` 周辺 + omniwm `command
  move-column` の引数仕様を突合する必要あり。
- ただしこの reorder 順序逆転は S2/S6/S7/S9 にも波及する低層バグの可能性
  あり、優先度高。

---

## 5. SSOT § 番号と関連コード位置 (orientation)

| SSOT § | 内容 | 主要コード |
|--------|------|----------|
| §3.4 | INV-01〜INV-13 invariants | `internal/invariant/`, `internal/naming/` |
| §4.1 | 17 user ops の intent | `internal/intent/`, `internal/reducer/`, `internal/planner/` |
| §4.4 | browser tab CRUD + payload store | `internal/controller/controller_browser_*.go` |
| §4.5 | archive/unarchive (park 復帰) | `scenarios/archive_project_test.go` |
| §5 | UI (cockpit / status / errors) | `cmd/projwm-cockpit/`, `internal/cockpit/` |
| §6.5 | single writer / mutation lock | `internal/controller/controller.go` (`wmMutationLock`) |
| §6.6 | IDEMP (spawn 冪等) | `internal/adapter/wm/sigwm.go` (Spawn 冒頭) |
| §6.10 | operation order phase 順 | `internal/planner/planner.go` |
| §7.1 | transaction loop | `internal/controller/controller.go` |
| §7.3 | identity grammar (`<kind>-<id>:<project>`) | `internal/naming/identity.go` |
| §8.9 | crash recovery (tmux/Ghostty/Zed) | `internal/controller/recovery_*.go` |
| §9.1 | L4 acceptance S1-S10 | `scenarios/ssot_l4_acceptance_spec_test.go` |
| §10.4 | spawn settle table | `internal/adapter/wm/ssot_l3_wm_spec_test.go` |
| §10.8 | test isolation (ISO-01) | `internal/ssottest/test_isolation_audit_test.go` |
| §10.9 | GAP-01〜26 | (横串、各所) |

**コード読みの導線**: L4 fail から逆引きするなら
`scenarios/ssot_l4_acceptance_spec_test.go` (test owner table) →
`scenarios/real_acceptance_test.go` (helper) →
`scenarios/ssot_real_acceptance_test.go` (S10 含む新シナリオ) →
`internal/controller/controller.go` (transaction loop) → adapters。

---

## 6. 前任の外れ仮説 (踏まないため)

### 6.1 「S2 の根因は vivaldiInspectFunc の transient false」

前任(私)は freshly-spawned PID で `vivaldiInspectFunc` が transient false を
返す仮説で `b9d7d7a` parent-walk fix を入れたが、実機で S2 は症状不変。
**この commit 自体は害がなく Chromium helper PID 経路の堅牢化として有用**
だが、**S2 の修正ではない**。S2 は §4.1 (c) の仮説α/β/γから観測しなおすこと。

### 6.2 「Vivaldi が cross-display で開くから cross-display move が必要」

前任の前々セッションの誤断。`omniwm command move-to-workspace` は実機で
cross-display 動作する (§3.1)。user が直接訂正した。

### 6.3 「S10 zed-crash は safety boundary で実装不可」

前任の前々セッションはこう判断したが、precondition (Zed が 1 main process
だけ) を付ければ実装可能と本セッションで再評価し fixture を試作。ただし
reorder 順序逆転バグに当たり未完。fixture patch は staging にある。

---

## 7. 安全 precondition (毎 L4 run の前に確認)

### 必須

1. **Zed が 1 個だけ**: `pgrep -fl "Zed.app/Contents/MacOS/zed" | grep -v
   "\-\-crash-handler" | wc -l` が `1` であること。`2` 以上ならその Zed が
   user の編集中なので kill 厳禁。S1/S2/S10-Zed は走らせない。
2. **production daemon bootout**: `sudo launchctl bootout
   gui/$(id -u)/org.nixos.projwmd-next`。test cleanup が `bootstrap` で戻す
   ので、test 失敗時は手動で確認。
3. **Vivaldi automation leftover**: 前回 run の test Vivaldi が残っているなら
   non-Helper だけ kill:
   ```bash
   for pid in $(pgrep -f "MacOS/Vivaldi"); do
     ps -p $pid -o args= | grep -q Helper || kill -9 $pid 2>/dev/null
   done
   ```
4. **ghostty/tmux leftover**: `pkill -9 -f "ghostty.*--title=shell-\|ghostty.*--title=ai-"`
   + `tmux list-sessions | grep -E "projwm|reconstruct|scratch|cockpit" | xargs kill`.

### user の生窓を動かさない

`PROJWM_NEXT_REAL_ACCEPTANCE=1 go test -tags integration ./scenarios/`
は L4 production 環境で走らせるが、test fixture は `projwm-next-test-{main,alt,emmo}`
project に限定されているので user 用の `dotfiles` `manaflow` 等の生窓は
触らない設計 (ISO-01 完了済)。**ただし omniwm restart 系 (S7) は副作用が
広い**、要注意。

---

## 8. 進め方の選択肢 (あなたが選ぶ)

これは固定指示ではない。あなたが必読リスト読了後に **自分で決めて** 提案する
ことを期待している (user は「ゼロから考えてやらせる」と明示)。
以下は前任が叩き台として置いておく候補のみ。

### Option A — Solo (単独)

あなた一人で §1 必読 → omniwm/code 精読 → S10 reorder バグ修正 → S2 仮説検証
→ S6/S7 fixture 実装 を順に進める。

**利点**: 文脈統一、orchestration オーバーヘッド無し。
**欠点**: context 肥大化で前任と同じ末路 (深夜判断ミス) を辿るリスク。

### Option B — Stage 1 並列観察 + Stage 2 統合 (推奨)

**Stage 1**: 以下 2 つを並列で sub-agent に投げる (read-only、parallel)。

- **Agent-omniwm-truth**: `omniwmctl` の各 command (`window focus`,
  `command move-to-workspace`, `command move-column`, `query windows`,
  `query focused-window`, …) を実機で叩いて入力/出力/副作用/タイミング
  /idempotency を表化し、`queue/omniwm-truth.md` に書く。projwmd は touch
  しない。
- **Agent-code-flow**: 現コードが omniwm の各 command を「どの状況で・どの順で
  ・どの引数で」呼ぶかを reorder / spawn / move / observe 経路ごとに
  flow chart 化し、`queue/code-flow.md` に書く。コードは触らない。

**Stage 2 (orchestrator = あなた)**: 2 つの成果物を突合して「実装の前提 vs
実機 omniwm 挙動」の gap を抽出。これが S10 reorder 順序逆転 + S2 post-close
経路の **コードレベル根因仮説** になる。

**Stage 3**: 仮説に基づく修正 → 実機 verify。失敗したら Stage 1 に新しい
観察項目を追加して再回。

**利点**: 各 agent は narrow scope なので hallucination 起こしにくい、成果物
が永続化ドキュメントとして残り次セッションも使える、context が分散。
**欠点**: orchestration コスト、agent 出力の検証は orchestrator がやる必要
(spot-check)。

### Option C — Plan agent で計画のみ作成 → あなたが実装

Plan agent に omniwm 理解 + 修正計画を立てさせ、あなたは計画受領 → 実装。
**注意**: Plan agent は出力を整然と書きがちで、hallucination が見えにくい。
spot-check 必須。

### user への提案

§1 必読読了後、あなたから user に **「Option X で進めようと思いますが
良いですか」** + **1-2 個の confirm 質問** を投げて承認を取ってから動く
ことを推奨。直前の user 介入で「全体像を理解してから動く」が方針。

---

## 9. 並列 agent 設計 — 「片方の発見が外につながる」を実装する

user の指示で「片方の発見が外につながることもあり得るわけだろうし」と
されていた箇所。並列 agent の発見を統合する仕組み:

### 9.1 共有メモリ = ファイルベース

各並列 agent は自分の領域に append-only で書く:

- `queue/omniwm-truth.md` — omniwm 実機挙動の確定済事実
- `queue/code-flow.md` — 現コードのフロー
- `queue/handoff-2026-05-29-projwm-next-l4.md` (このファイル) — 状況スナップ

orchestrator (あなた) は両方を読み、突合結果を本ファイル §10 (gap matrix) に
追記する。

### 9.2 「外につながる発見」のシグナル

並列 agent の出力に **conflict marker** を仕込ませる:

- agent が「想定と違った観察」を見つけたら、ドキュメント冒頭の
  `## ⚠️ ANOMALY` セクションに 1 行で書く。
- orchestrator は ANOMALY セクションを最優先で読み、他 agent に
  「この観察を踏まえて再観察してくれ」と SendMessage で fan-out できる。

### 9.3 観測 instrumentation 案 (S2/S10 verify 用)

- `PROJWM_NEXT_PLANNER_TRACE=1` を planner に追加 (1 行追加で済む):
  各 replan の input observed windows + emit ops を stderr に出す。
- 別 process で `omniwmctl query windows` を 500ms 周期で record する
  shell script を background で回し、`/tmp/omniwm-trace-<run-id>.jsonl`
  に保存。fail 後に時系列で omniwm 観測を verify できる。

これらは test 走行コスト ~5% 程度、デフォルト off なので入れて損なし。

---

## 10. Gap matrix (次のあなたが埋める領域)

| ACC | 実装の前提 (code-flow.md) | omniwm 実機挙動 (omniwm-truth.md) | gap (= 仮説) |
|-----|--------------------------|-----------------------------------|-------------|
| S2 close→spawn | (要 fill) | (要 fill) | (要 fill) |
| S6 macOS restart | (要 fill) | (要 fill) | (要 fill) |
| S7 omniwm restart | (要 fill) | (要 fill) | (要 fill) |
| S10 reorder | (要 fill) | (要 fill) | (要 fill) |

---

## 11. 完了の定義

あなたのゴール:

1. ACC-S2 / S6 / S7 / S10 を ledger 上 `statusRealOnly` に昇格
2. 各 commit は **(a) 修正内容、(b) 根因の説明、(c) verify 手順** を message に含む
3. 完了時に本 handoff を `queue/handoff-2026-05-29-projwm-next-l4-DONE.md` へ
   rename + `memory/project_projwm_overhaul.md` に「2026-XX-XX 完了」追記
4. user に「L4 全部 green になりました、次の確認は X です」と報告 (実機 verify
   手順を 5 行以内で書く)

**全項目 green 化が無理だと判明したら**: honest に「ここまで green、ここは
こういう環境制約で blocked」を本 handoff に追記、user に決定を仰ぐ。
前任のような「自重判断で止める」は **やらない**、止めるときも必ず報告と
決定要請をセットで。

---

## 12. 前任からの最後のメモ

- 私 (前任) は 6 件 commit を残し、S9 を実機 green 化、ISO-01 を strict 化、
  L3 を完全に閉じた。しかし S2 で誤診断、S10 で新バグ発覚、深夜判断で
  自重停止して user の指示で本 handoff を書いた。
- 「自由に進めて良い」認可下でも、観測が薄いまま仮説修正を重ねるのは反生産的。
  あなたは **観察を多く、仮説の commit は少なく**。
- user は深夜以降「報告で起きたい」可能性あり。あなたが朝までに完了させる
  必要は無い、ただし朝起きた user が状況を 30 秒で把握できるよう、本
  handoff か memory に逐次状態を残すこと。

幸運を。

---

## 13. 後任セッション追記 — reorder 調査 + focus-before-observe 監査 (2026-05-29 後続)

clean-context の後続エージェントが実機検証主導で進めた結果。**前任 §4.4(c)/§6.3 の
「reorder 順序逆転 = 低層バグ、S2/S6/S7/S9 に波及」仮説は実機で否定された。**

### 13.1 実機で確定した事実(全て omniwmctl 直叩き + L3 real_ops で検証)

1. **reorder アルゴリズムは健全**。`PROJWM_REAL_OP_TESTS=1 go test -tags real_ops -run
   '^TestReorderColumns'` の R1-R4 + 新規 5窓+stack+非focus テストが全 PASS。
   2窓/3窓/4窓反転/5窓stack、focus/非focus いずれも正しく並ぶ。「中間3列逆転」は
   **Ghostty では再現せず** = reorder ロジック自体のバグではない。
2. **omniwm `query windows` の workspace 内順序の正体**(ws10 の Zed+Vivaldi 2窓で
   move-column を実機反復して確定): **focus された workspace では query 返却順 =
   視覚的列順(左→右)で信頼できる**。`move-column left/right` 後 settle すれば順序は
   追従する。**ただし非focus の workspace では frame.x も query順も stale/不整合に
   なり得る**(state1: Zed を x=1439=右と報告するが実列位置は左端)。
   → 「**観測の真理は focus された workspace に対してのみ**」が原則。
3. **frame.x で sort し直すのは誤り**(私が一度試して R1-R4 を全 regression させた)。
   multi-display では focused-ws の frame.x が query順に単調対応しないため、frame.x
   sort は**列を反転させる**。旧 projwm の colMap(frame.x)は -next の omniwm では
   不適。**query順(focus 前提)を信頼するのが正**。`liveOrderInWorkspace` は query順
   のまま据え置き。

### 13.2 入れた修正(commit 前、working tree)

`internal/adapter/wm/sigwm.go`:
- **`ReorderColumns` が観測/移動の前に対象 ws を focus する**(`focusWorkspaceLocked`
  + preMoveGrace 150ms = SSOT §4.6)。focus 復元は**しない**(列移動が自然に ws を
  focus 状態に残す = 従来挙動、最終 focus は controller Phase C が設定。復元すると
  post-reorder verify が非focus ws を観測して壊れる)。
- `FocusWorkspace` を lock-free `focusWorkspaceLocked` に分離(ReorderColumns は
  ロック保持中なので公開版を呼ぶと self-deadlock)。
- 新規 real_ops テスト `reorder_focus_repro_test.go` の
  `TestReorderColumnsWhileWorkspaceUnfocused`(5窓+stack を非focus 状態で reorder)。
- 注意: この修正は **「証明された S10 fix」ではなく防御的 hardening**。非focus repro は
  **元コードでも PASS** していた(per-move の `window focus` が結果的に ws を active 化
  して自己修復していた)。根拠は state1 の「非focus 観測は unreliable」+ 防御。

### 13.3 focus-before-observe の全体監査(結論: ReorderColumns が唯一の chokepoint)

- 列を動かす omniwm コマンド(`move-column`/`move`/`move-to-root`)は **ReorderColumns
  経路内のみ**。`move-to-workspace` は cross-workspace 窓移動で列順非依存(窓を先に
  focus)。
- `Observe()` は inactive workspace を **height ヒューリスティック
  (`inactiveObservedColumnsFromCtl`)** で扱い frame.x stale を回避済み。focus 不要。
- planner が ReorderColumns に渡す `columns` は **desired(spec 由来)** であり observed
  でない → executor は focus+再観測で actual を desired に自己修復、観測 stale に
  非依存。
- → **focus-before-observe を要する箇所は ReorderColumns だけ。修正は完結。**

### 13.4 S10 の真因仮説(未検証、次フェーズ = Zed 問題)

S10 ログの「M(main display)に dev.zed.Zed 漏れ」+「reorder did not settle」は
**Zed の spurious「empty project」window** が真因の可能性が高い:
- Zed は single-process(GPUI が --user-data-dir 無視)で managed/user 窓が pid 共有。
- managed --user-data-dir 起動時、Zed は project 窓と並んで**遅延的に「empty project」
  窓を default display(M)に開く**。これが (a) 列を1つ増やして reorder settle を壊し、
  (b) M に漏れる。**Ghostty は余計な窓を出さないので S10 が Ghostty repro で再現
  しなかった理由がこれで説明できる**。
- `closeNewZedEmptyProjects`(sigwm.go:1260、6s polling + PID+title scoped close)が
  対処コードだが、**2026-05-29 時点で ws10 に「empty project」窓(pid 76730)が残留
  = 掃除が不完全な実例**。
- 次フェーズ: Zed の窓/プロセス管理が SSOT の原則どおりか(全窓が正しい仕組みで
  管理されるか)を spec 全文と突合して監査する(user 指示)。**Zed kill は user の
  編集中 Zed(現在 pid 76730 単独)を巻き込むので、安全 session が必要**。

### 13.5 現在の ledger ステータス(再確認、変更なし)

L4: 6/10 green(S1/S3/S4/S5/S8/S9)、red 4(S2/S6/S7/S10)。本セッションの reorder
修正は hardening でありこの数字は未変動。未 push commit が origin/main..HEAD で 101 件
あった(本セッション開始時点)。

---

## 14. Zed 帰属方針の決定 + SSOT 反映(2026-05-29 後続、user と設計合意済)

S10 の真因追求から「Zed の窓帰属」を user と詰め、方針を決定して SSOT に反映した。
**実装はまだ(SSOT が先行、TDD で実装予定)**。

### 14.1 実機で確定した事実
- **Zed は single-instance**(実機確認: `open -na` + 別 --user-data-dir でも新プロセス
  立たず既存へ routing)。`-n`(= PATH の `zed` = Zed CLI shim `.app/Contents/MacOS/cli`)は
  新「ウィンドウ」を開くだけで新「プロセス」ではない。→ **Vivaldi 流のプロセス帰属は
  Zed では原理的に不可能**。
- `zed -n` は **CLI shim 経由なら正当**(直 binary `MacOS/zed` は `-n` 拒否)。実装
  sigwm.go `LaunchZedProject` は PATH の `zed`(=cli)を呼ぶので**バグではない**。
- SSOT §4.1 旧記述「--user-data-dir で user の Zed と分離」は **single-process では
  成立しない誤記** → 訂正済。

### 14.2 決定した帰属方針(SSOT §6.9 / §6.9.1 / §4.1 / §3.5 に反映済)
- **2層帰属**: (1) 通常運用 = **provenance**(spawn/adopt 時の live window-ID を
  `(project,kind,id)→liveID` で永続保持、**毎 observe で ID存在+bundle+title を検証する
  キャッシュ。盲信禁止**)。(2) cold-start/復旧 = **title→identity adopt だが slot
  workspace 上に限定**(user の workspace 上は title 一致でも不可侵)。
- **プロセス kill 禁止**、窓は AXClose のみ(§4.1)。
- **非有効化 project は対象外**(adopt/spawn のトリガは slot 有効化)。
- **INV-01 を provenance-aware に**(非 provenance の同名窓は duplicate でなく External)。
- empty-project は **provenance-scoped close**(spawn 由来の余計窓のみ)。

### 14.3 振る舞い表 = テスト契約(SSOT §6.9.1、user 合意済)
8次元30ケースを `ATTR-A1〜H1` として SSOT §6.9.1 に正規表化。各行 → `TestZedAttr_<ID>`
へ 1:1 対応(L0/L2 deterministic で厳格に、L3 実機 single-op で確認)。
**⚠️ 受容する限界**: ATTR-A5(window-ID 再利用+偶然一致)/ B5(別dir同basename)/
C2(複数 editor 復旧時の title 区別不能)— single-process + basename-title では完全帰属は
原理的不可能、provenance + slot限定で被害局限し残余は明記受容、と user 合意。

### 14.4 実装スコープ(未着手、次タスク)
provenance feature は reorder hardening より大きい:
- world.State に `WindowProvenance map[DesiredWindowID]LiveWindowID` 追加 + store 永続化。
- Spawn の before/after diff で捕捉 → controller が spawn/adopt 成功時に記録。
- identity resolver(PopulateMatchedTo)に provenance 優先層 + 毎サイクル検証。
- lifecycle(close/archive/profile/有効化/stale)で entry クリア。
- INV-01(Check14)を provenance-aware 化。
- empty-project を provenance-scoped に(現 closeNewZedEmptyProjects の発展)。
- ATTR-* L0-L2 テストを feature と同時 TDD。L3/L4 は実機(Zed safe session)で後追い。

### 14.5 注記: queue/ は gitignored
SSOT(projwm-next-spec.md)も本 handoff もディスク永続のみ(版管理外)。コード変更
(sigwm.go の reorder fix 2件: 6cc869a / 13c3afe)は git commit 済。

### 14.6 ATTR テストスキャフォルド状態(2026-05-30、未 commit・working tree)

user 方針:「走らせなくてよい。全 green なら実装が堅牢と言える検証スキャフォルドを
全層に先に敷く」。ただし**まだ完成していない**(全層網羅の途上)。

**surface 追加(stub、実装はまだ)**:
- `identity.ResolveOptions.Provenance map[DesiredWindowID]LiveWindowID`
- `world.ControllerMeta.WindowProvenance map[DesiredWindowID]LiveWindowID`

**書いたテスト(compile 済)**:
- L0 `internal/identity/ssot_attr_test.go`: A2/B4/C1=**red**、A3/A4=guard(green)。
- L2 `internal/controller/ssot_attr_test.go`: B1/B3/A2/A3=guard(green)、**A1/B2/C1=red**。
- L3 `internal/adapter/wm/ssot_attr_real_ops_test.go`: F1/A1=**gated**(safe session で走行)。
- ledger `ZED-ATTR`(statusRed)に全テスト登録、`ZED-CONFIG` の --user-data-dir 誤り訂正、
  D1/A6 は既存(ZED-CONFIG/§10.4 S4/S5)へ重複排除。SSOT §6.9.1 ↔ INV-01/10/11 相互参照。

**残ギャップ(✗要、まだ書いていない = 「十分」でない)**:
- **L4 humanE2E(authoritative、0件)**: B1/B3/A2/C1/G3/E1/F1 を実機契約として。最大の穴。
- **L2 残**: E2(archive clears)/E3(profile)/E4(slot有効化)/E5(無効化)/G1(daemon再起動・
  persisted store 再構築)/G2(reboot stale→adopt)/G3(=B3 の reboot framing)/D4(title="")/
  D5(同時多spawn、要2slot env)。
- **L1**: Check14 を provenance-aware にする test。
- **L3 残**: D2(既存empty不可侵)/D4。

**重要な honest 評価(user に既報)**: 全 ATTR が green でも
(a)帰属/(b)empty-project/(d)kill安全 は担保されるが、**(c) S10 crash-recovery / cross-display
は ATTR の対象外**で別トラック。「Zed 問題が全部解決」には L4 完成 + 全ケース owner +
実機実行 + S10 別途攻略 が必要。

**次タスク**: ✗要 を埋める(L2 残→L1→L4)→ provenance 実装(`WindowProvenance` 捕捉 +
resolver 優先層 + 毎サイクル検証 + lifecycle + 永続化)で red→green、ZED-ATTR を
statusRed→covered → safe session で L3/L4 実行。

### 14.7 ZED-ATTR 完了(2026-05-30、commit 69b0034)

ultracode workflow ×2 で完成。**provenance attribution を実装 + 全層スキャフォルド + ledger 統合**。
- **実装(7 source files、+443/-15)**: identity(provenance pre-pass + provenanceValid +
  ownedByOtherIdentity + PopulateMatchedToWithProvenance)/ controller(captureProvenance +
  captureSlotAdoptions + pruneProvenance(Inactive) + cloneMeta deep-clone + restore)/
  planner(spawn/layout resolve に Provenance thread)/ executor+semop(Provenance field)/
  invariant(Check14 provenance-aware + Check4/Check9 thread)/ store(ControllerCheckpoint.
  WindowProvenance 永続化=G1)/ world.state(WindowProvenance field)。
- **検証(私 + adversarial verifier 独立)**: go build clean、**決定論 24 TestZedAttr_*
  (L0 5/L1 2/L2 17)全 PASS**、go test ./... は cmd/projwmd の /tmp 環境のみ fail(既存無関係)。
  stash-revert で「impl 無しでは ATTR test が compile 不可=vacuous green 不能」実証。
- **ledger**: ZED-ATTR statusRed→**statusCovered** + evidenceBehavior、全テスト名列挙。

**実機 verify(2026-05-30、commit 57bd6aa/e733d28)**:
1a. **L3 real_ops 実機 PASS**(safe session = `launchctl bootout` で daemon 停止 + Zed quit/kill
   で 0-Zed): **F1 PASS = managed Zed 窓除去でプロセス生存(single-process kill 防止を実機確定)**、
   A1 PASS(spawn live ID 捕捉)、D3 PASS(遅延 stray が reorder 壊さず)、D2 honest-skip
   (bare Zed empty-project が settle 内未登録)。F1 fix = realOpsEnv に Zed lifecycle policy +
   terminate BundleID。**provenance + single-process 安全を実機で裏付け**。
1b. **L4 humanE2E は BLOCKED(follow-up)**: `requireSoleTestZed` gate が 0-Zed を要求するが
   humanE2E harness 自身が managed Zed を spawn する → 常に skip。gate を「user の Zed と
   harness の test Zed を区別」する設計に直す必要(+ gate が Zed helper subprocess を main と
   overcount する点も)。**daemon 操作の作法**: `launchctl bootout gui/$(id -u)/
   org.nixos.projwmd-next` で停止、`launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/
   org.nixos.projwmd-next.plist` で復元(本セッションで実証、env 復旧済)。
2. **ATTR-D4(transient title="" load guard)**: L2 では Fake に empty-title logic が無く
   faithful に書けない(untestable と判定)→ L3 real_ops で書く必要(未)。
3. ~~**RemoveWindow executor バグ**~~ **修正済 (commit b61800a)**: same-title sibling
   (同 basename editor 2窓のうち1つ remove)で窓が close されない 2層バグ(planner 過保護 +
   executor stale-MatchedTo 再解決)を provenance 活用で修正(orphan close を e.Provenance!=nil
   gate + active-provenance 窓不可侵)。決定論 green、回帰なし。OP-13 ledger に登録(44e237d)。
   ※ shell remove は元々 OK(distinct title)、editor-same-title の穴だった。
4. **S10(c)= cross-display 配置**は依然 ATTR 対象外・別トラック(未着手)。

### 14.8 セッション総括 + 残作業の性質(2026-05-30)

**本セッション commit(全て決定論 green・回帰 /tmp 環境のみ)**:
6cc869a/13c3afe(reorder hardening)/ 69b0034(ZED-ATTR provenance)/ b61800a(RemoveWindow
fix)/ 44e237d(OP-13 ledger)。

**残作業は性質が2分**:
- **autonomous 決定論で攻めきれる分はほぼ枯渇**(ZED-ATTR + RemoveWindow 完了)。S2/S6/S7/S10 の
  残は **実機 E2E**で、L2 Fake では再現不能(cross-display 配置・browser respawn loop・launchd・
  Zed single-process は実機固有)。
- **次の高価値は実機検証 batch、ただし safe session 必要**: ZED-ATTR L3/L4 (authoritative)、
  S2/S10 real は **user が Zed を閉じ席を外す**clean session 推奨(ISO名+zed-count gate で
  user Zed 巻き込み無しだが、現在 user Zed 稼働中なので Zed 系は SKIP される)。
- S2 の browser respawn loop は **Vivaldi 系(Zed 非依存)なので user Zed 稼働中でも実機調査可能**
  だが、production daemon quiesce + 反復観測が要る深い real-machine debug(前任が苦戦した領域)。
- S6/S7 は launchd + user 不在の clean-env。

### 14.9 実機検証セッション(2026-05-31)— regression 捕捉 + S2 ground truth

**🚨 production-breaking regression を実機が捕捉 → 修正(commit 1bff340)**:
ZED-ATTR 永続化(69b0034)で `ControllerCheckpoint.WindowProvenance` を
`map[DesiredWindowID]LiveWindowID` で json 化したが、**Go は struct キーの map を
marshal 不可** → 実 FileStore daemon が commit 時に `marshal checkpoint.json: json:
unsupported type` で全 transaction fail = daemon 機能不全(production を壊す)。
**決定論テスト(MemoryStore)+ adversarial verifier(FileStore round-trip 未実施)が
両方見逃し、実機 S2 走行が即検出**。修正: 永続化形を slice-of-pairs に + 決定論
回帰テスト `TestFileStoreCommitRoundTripsWindowProvenance` 追加。
**教訓: 永続化の主張は MemoryStore でなく必ず FileStore(JSON)で検証。**

**S2 ArchiveProject の実際の失敗(marshal fix 後に surface、safe session で observe)**:
- `TestHumanE2EProductionRemovalWithoutCloseWindowSteps`(close なし)= **PASS**。
- `TestHumanE2EArchiveUnarchiveSteps` = **FAIL**: `cli/assign Q projwm-test-main` で
  `failed to converge after 4 replans, last ops=[spawn-browser:Q ...]` + Vivaldi 6個累積。
  = **handoff §4.1 文書化済の「Vivaldi browser respawn loop」が現コードでも root**。
  assign(project 割当=browser spawn)が収束せず、archive 以前の setup で fail。
  **Zed/reorder/provenance とは無関係の Vivaldi identity/placement 問題(root A)**。

**S2 の次の attack = browser respawn loop の instrumented 調査**(deep real-machine、
前任が ~22 run で苦戦 + vivaldiInspect 誤診した領域 = 慎重に、observe-first):
- planner が `spawn-browser:Q` を毎 replan 再 emit = browser identity が ClassMissing 判定。
  なぜ managed Vivaldi が認識されないか(cross-display 着地? vivaldiManaged の pid 判定?
  omniwm catalog flicker? identity ambiguous→refuse?)を **planner trace + omniwm window
  時系列記録**(handoff §9.3 案)で観測してから仮説を立てる。コード: planner.go:228-298/845-871、
  identity.go:194-201/361、sigwm.go:818-826 classifyLiveWindow + vivaldiManaged:859。

**本セッション commit 累計**: 6cc869a 13c3afe 69b0034 b61800a 44e237d 57bd6aa e733d28 1bff340
(全て local、未 push)。env は daemon restore 済。

### 14.11 S2 真因 **完全確定 + 修正済 GREEN**（2026-05-31 夜、commit 1f47390）

> **✅ FIXED（2026-05-31 夜、commit 1f47390）— `TestHumanE2EArchiveUnarchiveSteps` 実機 PASS (150.46s, exit=0)。**
> 2層の根本原因を spec から導出して修正、実機 trace ×3 で確証(症状でなく根治):
> - **層1**: `controller.collectBrowserRefs` が archived project を除外 → archive で browser payload を GC → 再 deploy で PrivateStore.Get が no-such-file → hard-fail。修正: archived を含める。GC は project が DesiredWorld から消える `DeleteProject{Purge}`(spec の purge point: line214「削除≠アーカイブ」+ intent.DeleteProject doc)でのみ発火。
> - **層2**: `browser.OpenInProfile` の `inFlight` skip(managed プロセス生存→launch skip)が「再 open は 2nd window を作らない」という**偽前提**(実験で反証: alive instance に open --new-window は新 window を作る)。archive は window close するが shared instance は生存 → reassign で inFlight=true → skip → 来ない window を待ち timeout → respawn loop。修正: 常に launch(75s settle が cold first-run を吸収するので churn 重複なし)。
> - 検証: 決定論(archive keeps / delete forgets / 内部全 green) + 実機(VIV_TRACE: reassign が managedAlive=true でも LAUNCHING→settle FOUND→move Q OK)。SSOT line2146 + ledger PRIV-ORPHAN-GC を「GC は delete(purge)、archive は保持」に訂正。
> - 結果は**タブ復元**(空タブではない): GC を delete に寄せたので payload は archive を生存 → reassign で OpenInProfile が URL を復元(VIV_TRACE が `LAUNCHING open ... https://canary.example.test/...` と URL 付き launch を確認)。§1.2 復帰 + §4.4 line913「タブ復元」を完全充足。line914「復元失敗→空タブ」は genuine 失敗時の fallback として温存(現状は hard-fail のままなので将来 graceful 化の余地あり、S2 には不要)。
> - 残: ACC-S2 ledger statusRed→statusRealOnly 昇格は **L4 owner(TestSSOTL4S2ArchiveProject = Archive + ProductionRemoval 両 sub-test)実機確認後**。
>
> **🎯 TRUE ROOT（run 2 の VIV_TRACE/spawnVivaldi trace で確定）— これが最終結論。下の「shared-instance inFlight 説」は run 1 の不完全な中間仮説で run 2 が否定した。投機を重ねない教訓として両方残す。**
>
> **S2 fail = `archive` が browser の private payload を GC 削除し、`assign` 再 deploy がそれを必要として hard-fail（respawn loop）。** 実機 trace の決定的証拠:
> - `reconcileIdeal`: `spawnVivaldi OK`（payload 存在、browser deploy 成功）。
> - `assign` 再 deploy: `spawnVivaldi ERROR: browser/private-store: read payload: open /tmp/.../private-payloads/browser-payload-v1-<hash>.json: no such file or directory`（~1-2s で fast error = launch 前 = settle 前 = 全 dead-code 説と整合）。
> - 機構: `controller.forgetOrphanedBrowserPayloads`(commit 後毎回) → `collectBrowserRefs`(controller.go:1977-1980) が **archived project を意図的に除外** → archive で token が orphan 判定 → `PayloadStore.Forget` で **payload ファイル削除**。だが archive reducer は `DesiredProject.Windows`(token 含む)を**保持**し、unarchive→assign が同 token で browser を再 deploy → payload 無し → `OpenInProfile` が `PrivateStore.Get` で fail。
> - layout 検査は `colBundle("com.vivaldi.Vivaldi")` = **bundle 存在のみ**(URL/tab 非検査) → 空 browser でも S2 layout は pass する。
>
> **これは SSOT 自体の設計矛盾**:
> - line 2146 (GAP-10) + ledger `PRIV-ORPHAN-GC` + test `TestControllerArchiveProject_ForgetsOrphanedBrowserPayloads`(archive で 2 Forget を assert) = **「archived ref は GC」(privacy/BR-PRIV-NOSTORE)**。
> - line 913 = **「再開時に archive 時のタブ状態（構造のみ）を復元」**。
> - S2 test = archive→unarchive→assign で browser 含む full layout 再 deploy 期待。
> → 単一 full-payload model では三者同時成立不能。**privacy(GC) vs 復元性 の resolution は user(=SSOT owner)の判断事項**。
>
> **fix 候補（user 決定待ち）**:
> - (A) payload 欠如時 spawn-browser を **graceful に空 browser** で開く（tab 喪失、privacy-GC 維持、S2 layout pass、最小、hard-fail 解消）。
> - (B) archive で **GC しない**（tab 復元、line2146/PRIV-ORPHAN-GC test+SSOT 改訂要、archived project は disk に browsing data 保持）。
> - (C) **structure-only redacted payload を残す**（line 913「構造のみ」に最忠実、最大工数）。
> - いずれも「hard-fail(respawn loop)→graceful」は最低限必要。
>
> **この 2 run の instrumentation（commit 候補・全 gated read-only）**: planner/sigwm/vivaldi trace に wall-clock timestamp、sigwm `Spawn dispatch`/`settle[browser]`/`spawnVivaldi ERROR|OK`、vivaldi `OpenInProfile` の inFlight/launch/settle trace、harness `PROJWM_NEXT_DAEMON_STDERR_FILE` tee。recorder script `/tmp/s2_recorder.py` + runner `/tmp/s2_run.sh`。

---

### 14.12 次セッション resume checklist（2026-05-31 夜、compact 前に作成）

**完了済(commit、working tree clean、env=daemon UP)**:
- S2 根治 `1f47390` + 診断 instrumentation `2acd881` + ACC-S2 昇格 `3d0b621`。**L4 受入 7/10 green(S1/S2/S3/S4/S5/S8/S9)**。全 local 未 push。決定論内部全 package green。

**user 確認事項(今セッションで確定、次も適用)**:
- privacy 厳格化の意図なし(個人用)→ browser tab は復帰優先。
- **S10 で user の Zed を kill して可**(「今 Zed 使ってないので落ちて問題ない」)。ただし Zed safe session(daemon quiesce + 0-Zed)は依然要る。
- **S6 macOS 再起動**: 私は reboot 不可 → 手順を user に提示 → user が実行 → 私が復帰検証(役割分担合意済)。
- 進め方: 観測駆動(instrument→実機 run→生データ→根治)、投機 fix 禁止、原文 spot-check 必須(agent hallucination 警戒)。

**残作業 map(優先順)**:
1. **S7**(omniwm 再起動復帰)— 自走可。omniwm restart は user の生窓も一瞬再配置する副作用に注意。
2. **S10**(Zed crash recovery)— 自走可(Zed-kill 許可済)。staged fixture `queue/staging/s10_zed_crash_fixture.patch.todo` を restore + **直近修正(ZED-ATTR/closeNewZedEmptyProjects/reorder hardening/S2)後に既知失敗が残るか再観測**。既知失敗: `reorder did not settle` / Zed spurious "empty project" 窓が display M に漏れる。S2 級の深掘りになり得る。
3. **SYS-ALL**(§4.2/§10.6)— move/reorder/close/spawn-*/kill-session 全 op の実機網羅。自走可。
4. **C: ATTR L4 gate rework** — `requireSoleTestZed` を「user Zed と test Zed を provenance で区別」に(コード)+ ATTR-D4(title="" load guard、real-only 未記述)。
5. **D: timing assert** — §9.2 ③1分復帰 / ④profile切替5秒 を test で実測 assert(spec line306/336)。profile=S1 で計測可。
6. **S6**(上記、user reboot)。

**trace harness(再利用)**: `queue/s2_run.sh`(safe session 構築 + 実機 test + daemon 復元)+ `queue/s2_recorder.py`(外部 omniwm 時系列)。`PROJWM_NEXT_PLANNER_TRACE=1` + `PROJWM_NEXT_DAEMON_STDERR_FILE=<file>` で daemon の timestamped trace 全取得(PLANNER_TRACE/WM_TRACE/VIV_TRACE/settle/spawnVivaldi)。S10/SYS-ALL でも有効。runner の `-run` を該当 test に変える。

**3 ディスプレイ構成(実機確認済)**: 内蔵 Retina=main=slot M(omniwm ws10)、外部 HP×2。slot Q→ws13(QWERTY 上段=ws13-22)。`open -na Vivaldi` の着地 display は「その時次第」(user 談)→ cross-display move が placement で吸収。S10 復帰配置でも cross-display が絡みうる。

**§9.2 完了定義(ゴール)**: ①全受入 real E2E PASS ②全 INV checker 検証 ③1分復帰 ④profile切替5秒 ⑤個別操作独立テスト。①は S6/S7/S10 green + SYS-ALL で充足、③④は timing assert 要。

---

#### （中間仮説・run 1、run 2 が否定）shared-instance inFlight 説

**結論（code logic + 実機 data が一致して確定。§14.10 の (i)親timeout / (ii)daemon-observe-differs は両方否定、bcf10c9 は dead code を直していた）:**

S2 fail の真因は **cross-display でも observe でもなく、shared-instance な managed Vivaldi の post-archive ライフサイクル**。

経路: `KindSpawnBrowser → semop.SpawnProjectBrowser → sigwm.Spawn(browser) → spawnVivaldi → browser.OpenInProfile`。
- **`OpenInProfile` は自前の settle `settleNewVivaldiWindow`(vivaldi.go:376、timeout 15s)を持つ** → `sigwm.settleNewBrowserWindowByDiff`（bcf10c9 の 75s fix）は **この経路で一度も呼ばれない dead code**。実機 trace で `Spawn dispatch kind=browser` が **0 回**、`settle[browser]` も **0 回**（ai/editor/shell/viewer は 16 回出る）= sigwm.Spawn(browser) は spawnVivaldi error で **dispatch trace 前に return**。
- managed Vivaldi は **全 project で 1 つの `--user-data-dir` instance を共有**（`a.UserDataDir` 単一）。`reconcileIdeal` で instance 起動（pid 55533、window on ws M）→ **成功**（daemon が `managed=true kind=browser` で 40 回観測）。
- `archive` は **window だけ close、共有 process は生存**（他 project が instance を使うので kill 不可）。
- `assign` 再 deploy 時: `inFlight := WindowQuerier!=nil && UserDataDir!="" && managedProcessAlive()` = **true**（55533 生存）→ **`open --new-window` を SKIP**（vivaldi.go:355-356）→ `settleNewVivaldiWindow` が「新規 window」を 15s 待つが **launch していないので永遠に来ない** → timeout → `OpenInProfile` error → `sigwm.Spawn` が settle 前に return → executor degradable → `spawn-browser` 再 emit → 4 replans → fail。
- 外部 recorder が決定的: 失敗 assign 中（16:47:30-47）**`vivaldi=1 managed=0`** = 再 deploy は managed window を **1 つも作っていない**。`reconcileIdeal` 中は `managed=1`(pid 55533 on M)で daemon も外部も**一致して観測**（→ (ii) 完全否定）。
- ProductionRemovalWithoutClose が PASS する理由: close→再deploy 経路を通らないので inFlight-skip-without-window の deadlock に当たらない。

**`inFlight` skip(vivaldi.go:344-356)のバグ本質**: 「managed process 生存 = window が in-flight で生成中、待てばよい」と仮定するが、**post-archive は process 生存 ∧ window 無し ∧ launch skip = 来ない window を待つ deadlock**。コメント自身は「re-issuing open は 2nd window を作らない(Chromium single-instance)」と主張 → **alive-but-windowless instance に `open --new-window` が window を作るか否か**が fix 設計の核心の経験的未確認点。

**未解決の設計判断（user と相談 or 経験的 test 要）**:
- (a) `open -na Vivaldi --new-window --user-data-dir=<dir>` は alive-but-windowless instance に新 window を作るか？ → 作るなら inFlight skip を「0 window なら launch」に直すだけ。作らないなら AppleScript `make new window` 等の別機構が要る。
- (b) `settleNewVivaldiWindow` の 15s timeout は cold first-run(~40s)に届かない → first-run でも初回 replan で timeout → replan churn。settle を first-run 以上(~75-90s)にすれば初回成功で replan 無し（bcf10c9 が**意図したが層を間違えた**もの。正しくは vivaldi.go の settle に適用）。
- → fix は (a)の経験的確認 + inFlight semantics 修正 + settle timeout 是正の組合せ。**production browser 経路なので慎重に、1 実機 run で検証してから commit**。

**この run の instrumentation（commit 候補、全 gated・read-only）**: planner/sigwm trace に wall-clock timestamp、sigwm に `Spawn dispatch`/`settle[browser]` timeline、test harness に `PROJWM_NEXT_DAEMON_STDERR_FILE` tee。`OpenInProfile` 経路には**まだ trace 未挿入** → 次 run で inFlight/skip/settle-timeout を直接 log して機構を最終確認すべき。

---

### 14.10 S2 browser respawn loop — 診断確定 + fix precursor + 残る深い coupling(2026-05-31)

> ⚠️ **2026-05-31 夕方 §14.11 で否定済**: 下記 (i)/(ii) 仮説および bcf10c9 は真因ではなかった。真因は §14.11（OpenInProfile inFlight skip）。下記は履歴として残す。

**observe-first 調査(sub-agent fresh context + 私の code 検証)で root 確定**:
S2 `TestHumanE2EArchiveUnarchiveSteps` の `cli/assign Q` が `failed to converge after 4 replans,
spawn-browser:Q` で fail。raw 観測(PROJWM_NEXT_PLANNER_TRACE + 外部 omniwm 120 snapshots):
- 4 replan 全てで daemon ObservedWorld に **WindowBrowser=0**、見えたのは user の Vivaldi
  (pid 80468, external)のみ。**managed Vivaldi(pid 4657、正しい --new-window
  --user-data-dir=~/.cache/projwm-next/vivaldi-data argv)は 4 replan 中ずっと daemon の
  観測に不在**(WM_TRACE が pid=4657 を一度も log せず)。
- 外部 omniwm snapshot では同 window が **14:23:09 に catalog**(spawn の ~40s 後、flicker なし、
  ws M→Q へ到達も遅すぎ)。
- root 機構(code 確定): `settleNewWindowByDiff` が 30s で window 出ず → **process-alive
  fallback が空 live を返し spawn 成功扱い** → converge loop が window-less で再 emit。
  **vivaldiInspect/分類のバグではない**(前任の誤診を回避)。

**fix 試行(commit bcf10c9、決定論検証済・回帰なし、但し S2 単独未収束)**:
browser 専用 settle(75s `BrowserSettleTimeout` + managedVivaldiProcessAlive keyed、空 live
fallback 廃止)。だが **実機 S2 re-run で同一失敗(4 replans, sub-test 158s=~40s/replan)**。
75s settle なら 4×75=300s のはずが 158s。

**残る深い coupling(2つの仮説、要 disambiguate)**:
- **(i) 親 op/transaction timeout ~40s が 75s browser settle を cancel**。daemon main.go に
  明白な <75s は無いが、converge loop / IPC / projwmctl CLI のいずれかに per-op/transaction
  deadline がある可能性。→ 確認: controller converge の op 実行 ctx、daemon IPC handler の
  per-request deadline、projwmctl の request timeout。fix: browser spawn に ≥75s の budget を通す。
- **(ii) settle は window を見つけるが daemon の後続 Observe が managed window を surface しない**
  (daemon-observe-differs)。pre-fix で daemon が pid=4657 をついに観測しなかった事実と整合。
  → 確認: post-fix で adapter settle が window を返すか + その後の Observe(omniwmctl query)が
  当該 window を含むか。fix: daemon の omniwm query が当該 display/workspace の window を
  surface するようにする(cross-display 絡みの可能性)。

**次の一手(observe-first、fresh context 推奨)**:
post-fix で `PROJWM_NEXT_PLANNER_TRACE=1` 実機 S2 run + 外部 omniwm 時系列 + **trace に
wall-clock timestamp を足す** → 「browser settle は 75s 走るか ~40s で切れるか」「settle が
window を返した後 Observe が surface するか」を見て (i)/(ii) を確定 → 対応する fix。
これは S10(cross-display)とも root を共有しうる(managed window が daemon observe に
surface しない問題)。

**safe-session 手順**(本セッションで確立): `launchctl bootout gui/$(id -u)/
org.nixos.projwmd-next` → `pkill -f "Zed.app/Contents/MacOS/zed"` → 0-Zed 確認 → 実機 test →
harness が daemon を自動 restore(or `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/
org.nixos.projwmd-next.plist`)。実機 test は単一環境で**並列不可**(sub-agent も1体)。
user の Vivaldi/Zed は触らない(harness の killVivaldiAutomationProcesses は automation profile のみ)。

**本セッション commit 累計(11)**: 6cc869a 13c3afe 69b0034 b61800a 44e237d 57bd6aa e733d28
1bff340 dd1a70f bcf10c9(+ OP-13 等)。全 local 未 push、env(daemon)復旧済、working tree clean。
