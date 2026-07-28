# projwm 意思決定 log

> 今後の重要な意思決定を時系列で追記する。
> 過去の Decision は `projwm-spec.md` §10「確定決定一覧」に集約済（D-1〜D-45）。
> 新たな decision はここに記録 → 確定後に spec.md に移管。

---

## 0. 使い方

### 新しい意思決定を追記する形式

```
## D-NN: <一行サマリ>

**日付**: YYYY-MM-DD
**状態**: [proposal / discussion / accepted / rejected / superseded-by D-XX]

### 背景
何が問題だったか / なぜ判断が必要になったか

### 検討した選択肢
- (A) ...
- (B) ...
- (C) ...

### 決定
採用した案。

### 根拠
なぜそれを選んだか。

### 影響範囲
コード / 仕様 / ユーザ操作のどこに影響するか。

### 関連
関連 issue / 過去 decision / 文書リンク
```

### 採用後の処理

`accepted` になった decision は `projwm-spec.md` §10 の Decision 一覧に移管（番号は spec の最後に追加）。本書には「accepted、spec D-NN に移管済」と残す。

---

## 現在の状態

D-1〜D-53 は `projwm-spec.md` §10 に確定移管済（v12 時点）。

- D-1〜D-45: v11.6 で確定
- D-46〜D-52: v12 browser 統合で確定
- D-53: layout snapshot/restore で確定

新規 decision は以下のテンプレに従ってこのファイルに追記してください。

---

## D-54: 手動操作ポリシー — 3 種の操作を明確に区別する

**日付**: 2026-05-04
**状態**: accepted（spec.md D-54 に移管済）

### 背景

ユーザが手動で WM 操作（window 移動、コラム並べ替え、直接 kill）を行った場合に projwm がどう反応すべきかが未定義だった。「全て revert」「全て尊重」「一部尊重」の 3 選択肢があり、UX の直感性の観点から明確な決定が必要。

### 検討した選択肢

- (A) 全操作を reconcile で revert → ユーザの意図的な並べ替えまで無効化されて混乱
- (B) 全操作を state に反映 → 誤移動や誤 kill も "意図" として扱われて状態が壊れる
- (C) 3 種に分類して異なるポリシーを適用

### 決定

操作を 3 種に分類:

| 操作 | ポリシー | 理由 |
|---|---|---|
| 手動コラム順・スタック順変更（同 WS 内）| state.json の Layout を更新（FR-32）| ユーザの意図的なカスタマイズ |
| 規約 title 窓を誤った WS へ移動 | reconcile が正しい slot へ revert（FR-33）| ユーザの誤操作、projwm の invariant 破壊 |
| 規約 title 窓を直接 kill/close | reconcile が再 spawn + 元 column 位置（FR-34）| 意図しない閉じ → 自動復旧。意図的削除は `projwm remove` のみ |

### 根拠

「手動 WS 内並べ替えは保存、WS 間移動は revert」は直感的。tmux session 状態の複雑さから、ウィンドウ削除は CLI 経由のみを有効削除とし、直接 kill は "事故" として自動復旧する。

### 影響範囲

- FR-32, FR-33, FR-34 (spec.md §2.1)
- T11a-c, T12, T18-T20 (統合テスト)
- `layout-changed` イベント subscription の実装が FR-32 に必要（OI-11）

---

## D-55: stack 検出は `query windows` のフレーム座標のみ使う

**日付**: 2026-05-04
**状態**: accepted（spec.md D-55 に移管済）

### 背景

layout snapshot の実装で、最初に `workspace-bar` の windowCount を使う案を検討したが、スタックされた窓を誤検出することが判明した。

### 問題

`omniwmctl query workspace-bar` はスタックされた各窓を **別々のエントリ** として返し、それぞれの `windowCount=1` となる。スタックの検出に使えない。

### 決定

stack 検出は **`query windows --workspace <name>` のフレーム座標** のみを使う:

1. WS に focus → 150ms 待機（OmniWM 内部状態の安定待ち）
2. `query windows --workspace <name>` でフレーム座標取得
3. `frame.x` ±5px 範囲で同一列グループ化 → 同グループ = 同一コラム
4. `frame.y` 降順（y-up 座標系：高 y が視覚的に上）でスタック順確定

### 影響範囲

- `internal/reconcile/layout.go` の `snapshotWorkspaceColumns`, `groupWindowsByColumn`
- D-53 の注記として追記

---

_本書は projwm の **生きた decision log**。確定したら spec.md に移す。_
