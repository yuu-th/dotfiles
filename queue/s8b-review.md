# S8.B Unique-Strong Identity Review (Myソクラテス)

## 設計の3つの検証点

### 1. Honest Human-operation authority ✅ **承認**

- **Real E2E test**: `TestHumanE2EPreconditionUniqueStrongAmbiguousSteps` が実機 OmniWM/Ghostty に対して実行される。
- **Human operation**: 実際の Ghostty ウィンドウを `spawnDuplicateGhosttyWindow` で生成し、実際の workspace Q に移動させている。
- **Real cleanup**: duplicate window のみを PID 指定で削除し、controller 管理の既存 window は触らない。
- **No simulator overclaim**: fake/simulator は preflight validation や failure reproduction 用であり、この S8.B の acceptance 判定には使われていない。

**判断**: Human-operation authority を尊重し、実機 OS 状態に対する real operation/observation を acceptance の根拠としている。

---

### 2. Fake/simulator overclaiming の回避 ✅ **承認**

#### Evidence policy の厳密化

`internal/identity/identity.go` の `ResolveWithOptions`:

1. **ExpectedWorkspace evidence 要求**: 
   - `opts.ExpectedWorkspace` が設定されている場合、`ow.Workspace != opts.ExpectedWorkspace` の window は候補から除外される。
   - workspace mismatch があり、かつ controller-owned identity link (title/bundle) を持つ window は `ForbiddenEvidenceUsed` として記録される。

2. **MatchedTo を strong evidence として受け入れない条件**:
   - workspace mismatch → `stale` に分類される (line 15-18)
   - bundle-id mismatch → `stale` に分類される (line 30-33)
   - title drift (controller-owned exact title 不一致) → `stale` に分類され、`matched-to-stale-title` evidence が記録される (line 39-43)

3. **Strong classification の条件**:
   - `len(strong) == 1` かつ以下のいずれか:
     - controller-owned exact title + bundle-id + workspace が全て一致
     - `matchedToDesired` AND 上記 evidence が揃っている場合のみ

4. **Planner error**:
   - `planner.Plan` は active desired window が `ClassUniqueStrong` 以外の場合、`fmt.Errorf("identity ... classified %s, refusing mutation without unique-strong evidence")` を返す。
   - この error により transaction は commit されず、no-commit trace が記録される。

**判断**: MatchedTo だけでは strong 判定せず、controller-owned exact evidence の全てが揃わない限り mutation を拒否する。Stale/drift を明示的に検出し、silently best-effort selection しない。

---

### 3. Real E2E test の安全性と観測可能性 ✅ **承認**

#### Test assertions (real_acceptance_test.go)

1. **Fixture validation**:
   - Preflight: `countWindowsByTitleBundleWorkspace` で title/bundle/workspace 一致が exactly 1 である確認。
   - Ambiguity setup: duplicate spawn 後に count >= 2 を確認。実機での ambiguity 成立を検証。

2. **Failure assertions**:
   - `reconcile` が exit error を返すこと。
   - error message または stdout に `"unique-strong"` が含まれること (observability)。

3. **No-mutation assertions**:
   - `currentGenerationName` が変わらないこと (CURRENT generation advance なし)。
   - `currentDesiredWorldKey` が変わらないこと (DesiredWorld 書き込みなし)。
   - `snapshotHumanWorkspaces` が before/after で完全一致すること (live workspace mutation なし)。

4. **Trace observability**:
   - `readLatestRecordedTransactionTrace` が `NoCommitReason == "planner-error"` かつ `AttemptedOperations == 0` かつ `ExecutedMutations == 0` であること。

5. **Cleanup safety**:
   - `t.Cleanup` で duplicate window の PID を指定して削除。
   - `waitForLiveWindowMissing` で削除完了を待機し、次テストへの汚染を防ぐ。

**判断**: Real E2E として必要な safety/observability/cleanup を備えている。Fake/simulator に頼らず、実機状態の before/after snapshot 比較で mutation 不発を証明している。

---

## Blockers (深刻な問題)

**なし。**

---

## Minor recommendations (非ブロッカー)

1. **acceptance.go の RealMode 設定**:
   - 現在 `RealMode: RealModeUnsafe` となっているが、この test は cleanup 付きで idempotent なため、`RealModeReady` への昇格を検討してもよい (opt-in 実行後に判断)。

2. **Evidence trace の human-readable report**:
   - `ForbiddenEvidenceUsed` と `MissingEvidence` を projwmctl reconcile の error output に含めると、user が ambiguity 原因を即座に理解できる (将来改善)。

---

## 結論

**✅ Opt-in real E2E 実行を承認します。**

理由:
- Human-operation authority を尊重している。
- Fake/simulator を acceptance の代替としていない。
- Evidence policy が厳密で、stale/drift を silent best-effort で誤魔化さない。
- Real E2E test が safety/observability/cleanup を備えている。
- Planner が unique-strong 要求を enforc し、violation 時に error を返す。
- Invariant Check4ActiveDesiredPresent が ExpectedWorkspace evidence を要求している。

この設計は implementation-design.md の以下の原則に忠実です:

> "Controller は `BlockedAmbiguous` として candidates/evidence/reason を report し、best-effort selection しない。"

> "fake / simulator / recorded は acceptance の代替ではなく、preflight / diagnostics / failure reproduction の補助である。"

Opt-in 実行前の最終確認:
1. `go test ./...` と `go test -tags integration ./...` が pass していること (✅ confirmed)
2. real harness fixture (ideal state reconcile) が clean であること (test preflight で検証済み)
3. opt-in 実行環境の OmniWM/Ghostty が最新であること

以上、Myソクラテスの視点から問題ありません。
