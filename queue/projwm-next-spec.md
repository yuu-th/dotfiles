# projwm-next 統合仕様 (SSOT)

> **Status**: v1.11 — 2026-05-23
> **位置づけ**: projwm-next の唯一の SSOT。既存の `design.md`, `implementation-design.md`, `specs.md`, `projwm-spec.md`, `projwm-cockpit-*.md` は参照資料として残すが、本ドキュメントが優先する。
>
> **変更履歴**:
> - v1.11: §10.9「現在のテスト保証境界」を追加。現行テストが緑になっても SSOT 全文達成を宣言できない未保証領域を明文化。
> - v1.6: §10「テスト戦略」を追加。L0-L4 5レイヤー設計、L3 実操作単体テストの具体設計、テストカバレッジ表、テスト用 manifest。
> - v1.5: §5.1-5.3 + §4.5 を §4.1 の 17 操作と整合。browser tab 操作を §4.1 に追加。
> - v1.4: §4.1 に browser jump / add-window / remove-window / browser tab 操作を追加。
> - v1.3: §4.1 に複数インスタンス循環を追加。§4.4 browser セクションに tab 管理詳細を追加。

---

## 構成

| セクション | 内容 |
|---|---|
| §1 概要 | 何を、なぜ |
| §2 メンタルモデル | 3原則 + summon フロー + slot + edge cases + スコープ |
| §3 システム状態 | 状態網羅、状態遷移図、不変条件、障害復帰 |
| §4 操作の定義 | 状態遷移 + 内部操作体系 |
| §5 UI の定義 | cockpit TUI、キーショートカット、エラー通知、status/doctor 出力 |
| §6 設計原則 | 8つの設計原則 |
| §7 アーキテクチャ | transaction loop、パッケージ境界、命名規約、ドメインモデル、アダプタ契約 |
| §8 状態管理 | PersistentStore、算出フィールド、排他制御 |
| §9 受入仕様 | 受入シナリオ、完了定義 |
| §10 テスト戦略 | テストレイヤー、L3 実操作単体テスト、カバレッジ |
| §11 付録 | 参照文書、既知の問題、意思決定記録 |

---

## 1. 概要

### 1.1 projwm とは

macOS 上で「AI コーディング project」を OmniWM の named workspace に割り当てて管理するシステム。

- 1 project = 1 OmniWM workspace (slot Q〜P の 10 個)
- 各 slot 内に AI（複数可）+ shell + Zed editor + browser が同居
- viewer WS A に全 project の AI が read-only で並ぶ
- Profile で slot 構成セットを保存・切替
- Cockpit TUI で操作・状態確認

### 1.2 なぜ projwm が必要か

AI コーディングでは1つの project に複数のプロセス（AI、シェル、エディタ）が紐づく。これらを macOS のウィンドウとして管理するとき：

- **ウィンドウは project と紐づいている**：「dotfiles の AI」「dotfiles のシェル」という単位で扱いたい
- **複数 project を並行したい**： slot Q に dotfiles、slot W に manaflow を割り当てたい
- **project を切り替えたい**： プロファイル切替で一括表示を替えたい
- **状態が壊れても復帰したい**： macOS 再起動後でも自動で復帰したい

### 1.3 v1 からの教訓

| 教訓 | v1 の問題 |
|---|---|
| 状態管理が分散 | state.json + shell scripts + Go の混在で整合性を保てなかった |
| 個別操作がテスト不能 | WM 操作が shell にベタ書きで、ユニットテストできなかった |
| エッジケースへの対応が場当たり的 | リトライ・タイムアウトが各所に散在 |
| Cockpit の状態管理が独立 | race condition、34 インスタンス発生 |

---

## 2. メンタルモデル

> **このセクションが SSOT の根幹。** すべての設計・実装はこのメンタルモデルに従う。

### 2.1 3つの原則

#### 原則1: 特別 (Special)

projwm が管理するすべてのウィンドウは **「特別」**。

- すべてのウィンドウに **所有者 (project)** と **役割 (kind)** がある
- `(project, kind, id)` の組合せで一意に識別される
- 同じ組合せのウィンドウは世界に **1 つだけ** 存在する
- slot の外にウィンドウが出ることがある（手動移動、OmniWM 再起動等）。transaction loop が正しい slot に戻す

**例**: `shell-1:dotfiles` は「dotfiles project の 1 番目のシェル」。このウィンドウは世界に 1 つだけ。

#### 原則2: summon (召喚)

「開く」 = **「ユーザーの前に出す」**。

- 既存の「特別な」ウィンドウがあれば **再利用** (focus)。新しいのを作らない
- 無ければ **作る**
- ウィンドウが今どこにあるか（どの WS にいるか）は **気にしない**
- 幂等: 何回呼んでも同じ結果

**例**: 「dotfiles の shell を開く」と言うと、`shell-1:dotfiles` が存在すれば focus する。存在しなければ作る。別の slot にいても気にしない（focus して WS を切替える）。

#### 原則3: observe → plan → execute → observe → replan

**システムは完璧な条件を仮定しない。現実を観測し、計画し、実行し、また観測する。**

- すべての操作は transaction loop を通る
- 実行前の「予想」と実行後の「現実」を比較し、差分があれば replan
- 最大 MaxReplans 回まで繰り返し、収束する
- これにより「正しい slot への配置」が保証される

**フロー**:
```
observe（現実を観測）
  → plan（現実に基づいて計画）
    → execute（計画を実行）
      → observe（実行後の現実を観測）
        → 差分がある？ → replan（計画を立て直して再実行）
        → 差分がない → commit（状態を確定）
```

### 2.2 「特別」の識別子

各ウィンドウは `(project, kind, id)` で一意に識別される：

| 要素 | 説明 | 例 |
|---|---|---|
| `project` | project 名 | `dotfiles` |
| `kind` | ウィンドウの役割 | `ai`, `shell`, `editor`, `browser`, `viewer`, `cockpit`, `scratch` |
| `id` | kind 内の連番（1 始まり、永続） | `1`, `2`, `3` |

**例**:
- `ai-1:dotfiles` = dotfiles の 1 番目の AI
- `shell-2:manaflow` = manaflow の 2 番目のシェル
- `editor-1:dotfiles` = dotfiles のエディタ (Zed)

### 2.3 summon のフロー

```
ユーザーが「project X の kind Y を開く」と言う

1. kind に応じた「生存確認」:
   - kind=ai, shell: tmux session (Y-N/X) はあるか？
     ├─ ある → 2 へ（再利用）
     └─ 無い → 作る → 2 へ
   - kind=editor (Zed): tmux session 不要 → 直接 2 へ
   - kind=browser (Vivaldi): tmux session 不要 → 直接 2 へ
   - kind=cockpit: tmux session (cockpit base) はあるか？
     ├─ ある → 2 へ（再利用）
     └─ 無い → 作る → 2 へ

2. この session/app に紐づくウィンドウは存在するか？
   ├─ ある → 3 へ（再利用）
   └─ 無い → 作る → 4 へ

3. そのウィンドウは今ユーザーの前にあるか？
   ├─ ある → focus するだけ（noop）
   └─ 無い → WS を切替えて focus

4. done（transaction loop が後で正しい slot に配置する）

重要な制約: ステップ2で「作る」の前に存在確認を必ず行う。同じ identity のウィンドウを重複して作らない。
```

### 2.4 slot の定義

slot は projwm が「正しい場所」として定義する workspace。**物理的な境界ではない。**

| slot | 用途 |
|---|---|
| Q, W, E, R, T, Y, U, I, O, P | project slot |
| A | viewer slot |
| CP1 | cockpit slot (projwm-managed モニタに1つだけ) |

**slot は物理的な境界ではない理由**:
- ウィンドウは OmniWM の機能で自由にどの WS にも移動できる
- macOS のディスプレイ変更でウィンドウが再配置されることがある
- OmniWM 再起動でウィンドウがどこに飛ぶか保証がない
- ユーザーが意図的に slot の外に移動することがある

**slot の外に出たウィンドウの扱い**:
- identity (project, kind, id) で識別できる（場所に関係ない）
- summon: 今いる場所に切替えて focus
- transaction loop: 正しい slot に戻す

**slot の意味**:
1. **slot の定義**: Q〜P が project 用、A が viewer 用という約束
2. **transaction loop の基準**: 「正しい slot」を決める基準
3. **ユーザーの一貫性**: 「Q を押すと dotfiles」の一貫性

### 2.5 エッジケースとルール

メンタルモデル（特別 + summon + transaction loop）で解決できるエッジケース：

| ケース | ルール |
|---|---|
| 既存ウィンドウが別の slot にある | summon: その slot に切替えて focus。transaction loop が正しい slot に移す |
| 既存ウィンドウが slot の外 (M, B, 1-9 等) にある | summon: その WS に切替えて focus。transaction loop が正しい slot に移す |
| 既存ウィンドウが同じ slot にある | summon: focus するだけ |
| ウィンドウが複数ある（バグ） | 最も recently focused なものを正とする。他は orphan。cockpit に [INVARIANT] カード通知 |
| macOS 再起動後 | transaction loop が全部再作成 |
| OmniWM 再起動後 | tmux session は生きている。窓が消えていたら再作成、あれば正しい slot に戻す |
| 長時間放置後 | tmux session は生きている。窓が消えていたら再作成 |
| ウィンドウが slot の外にある | transaction loop が正しい slot に戻す |
| viewer のウィンドウが消えている | transaction loop が再作成 |
| archived project のウィンドウが残っている | transaction loop が閉じる |

### 2.6 非要件（明示的にやらないこと）

| ID | 内容 | 理由 |
|---|---|---|
| NR-01 | 複数プロファイルを同時に active にする | slot 衝突解消のロジックが複雑 |
| NR-02 | アーカイブ済み project の自動再活性 | 暗黙的な復活はバグの温床 |
| NR-03 | modal / leader-key UX | 押下数増を嫌う |
| NR-04 | GUI editor を tmux に押し込む | tmux は terminal multiplexer |
| NR-05 | state を SQLite 等の DB で持つ | JSON で十分 |

### 2.7 スコープ

> 今実装する全ての機能。後回しはない。

| カテゴリ | 機能 |
|---|---|
| **基本** | project の追加・削除・アーカイブ・復活 |
| **基本** | shell, editor, viewer の summon |
| **基本** | profile の作成・切替・削除 |
| **基本** | cockpit TUI の表示/非表示 |
| **基本** | scratch shell |
| **堅牢性** | macOS/OmniWM/tmux/Ghostty/Zed 再起動後の自動復帰 |
| **堅牢性** | ディスプレイ追加/切断への対応 |
| **堅牢性** | ドリフト検出と修正 |
| **堅牢性** | 不変条件の検証と修正 |
| **堅牢性** | OmniWM 自己修復 (Recovery ladder Lv1-Lv4) |
| **確認** | status, doctor, reconcile CLI |
| **確認** | cockpit のスロット一覧・カード表示 |
| **高度** | 複数 AI の同居 |
| **高度** | browser (Vivaldi) 統合 |
| **高度** | 手動レイアウト変更の検出と対応 |

---

## 3. システム状態

### 3.1 システム状態の網羅

| 状態 | 説明 | ユーザーが見るもの |
|---|---|---|
| **初期** | macOS 起動直後。project が1つもない | 全 slot が空。cockpit に「project なし」表示 |
| **正常稼働** | 全ウィンドウが正しい slot にある | 各 slot に project のウィンドウ群が並ぶ |
| **ドリフト** | ウィンドウが slot の外にある | ウィンドウが別の WS にいる。summon は動く |
| **復旧中** | macOS/OmniWM 再起動後の復帰中 | 一時的にウィンドウが無い。数秒で復帰 |
| **部分障害** | 1つのウィンドウだけ消えている | 他のウィンドウは見えるが、1つだけ欠けている |
| **profile 切替中** | 旧ウィンドウ close 中、新ウィンドウ spawn 中 | 一時的にウィンドウが消えてから新しいのが出る |
| **cockpit 表示中** | cockpit TUI が表示されている | cockpit が前面に表示 |
| **エラー** | transaction loop が収束しない | cockpit に [INVARIANT] カードが表示 |

### 3.2 状態遷移図

```
初期 ──(projwm up)──→ 正常稼働
                         │
         ┌───────────────┼───────────────┐
         │               │               │
    (手動ドラッグ)    (再起動)      (profile 切替)
         │               │               │
         ▼               ▼               ▼
      ドリフト ──→ 復旧中 ──→ 正常稼働
         │                       ▲
         │                       │
    (transaction loop)───────────┘

         ┌───────────────┐
             部分障害 ──→ (transaction loop) → 正常稼働
```

### 3.3 各状態での操作可能性

| 状態 | summon | profile 切替 | project 追加 | archive |
|---|---|---|---|---|
| 初期 | × (project がない) | × (profile がない) | ○ | × |
| 正常稼働 | ○ | ○ | ○ | ○ |
| ドリフト | ○ (summon は動く) | ○ | ○ | ○ |
| 復旧中 | △ (待つ必要がある) | △ | △ | △ |
| 部分障害 | ○ (残りは動く) | ○ | ○ | ○ |
| cockpit 表示中 | ○ (WS 切替で cockpit から離れる) | ○ | ○ | ○ |

### 3.4 不変条件

> システムが常に守るべきルール。違反した場合は transaction loop が修正する。

| ID | 不変条件 | 違反時の修正 |
|---|---|---|
| INV-01 | 同一 (project, kind, id) のウィンドウは世界に1つだけ | orphan として検出、最近 focused なものを正とする |
| INV-02 | active profile の全 project のウィンドウは正しい slot にある | transaction loop が move で修正 |
| INV-03 | tmux session はユーザーが明示的に kill しない限り生きている | session が無ければ再作成 |
| INV-04 | archived project のウィンドウは存在しない | transaction loop が close で修正 |
| INV-05 | viewer は active profile の project の AI のみ表示 | transaction loop が close/spawn で修正 |
| INV-06 | cockpit は常に park workspace CP1 に存在する | transaction loop が move で修正 |
| INV-07 | Zed ウィンドウの title は basename(cwd) に一致する | 再起動で自然に修正 |
| INV-08 | 同一 profile 内で同一 slot に複数 project はない | state で排他 (map 構造) |
| INV-09 | active profile は profiles の既存キーである | state で validate |
| INV-10 | 全ウィンドウの identity は title から復元可能 | naming.Resolve() が保証 |
| INV-11 | 管理対象外 workspace 上の managed candidate window は Tier 1 提案カードを発火させない | planner が workspace role で判定 |
| INV-12 | viewwer order は slot order と一致する | transaction loop が reorder で修正 |

### 3.5 障害復帰フロー

#### macOS 再起動

```
1. OmniWM + projwmd が自動起動 (launchd)
2. transaction loop が実行 (LifecycleBootstrap)
3. state.json の DesiredWorld を読む
4. 全 project の tmux session + ウィンドウを再作成
5. 正しい slot に配置
所要時間: 1分以内
```

#### OmniWM 再起動

```
1. tmux session は生きている (独立プロセス)
2. transaction loop が実行
3. ウィンドウの存在確認
4. 無ければ再作成、あれば正しい slot に配置
所要時間: 30秒以内
```

#### tmux server クラッシュ

```
1. 全 tmux session が消える
2. transaction loop が実行
3. tmux session を再作成
4. ウィンドウは再作成不要 (Ghostty は tmux に再接続)
所要時間: 10秒以内
```

#### Ghostty クラッシュ

```
1. 1つの Ghostty ウィンドウが消える
2. tmux session は生きている（tmux は独立プロセス）
3. transaction loop が実行
4. Ghostty ウィンドウだけ再作成（既存 tmux session に再接続）
所要時間: 5秒以内
```

#### Zed クラッシュ

```
1. Zed ウィンドウが消える
2. tmux session は無い（Zed は tmux を使わない）
3. transaction loop が実行
4. `zed -n <cwd>` で再起動。empty project 自動 close
所要時間: 10秒以内
```

#### Vivaldi クラッシュ

```
1. Vivaldi ウィンドウが消える
2. tmux session は無い（Vivaldi は tmux を使わない）
3. transaction loop が実行
4. Vivaldi automation profile で再起動
5. PrivatePayloadStore から保存されたタブ構造を復元
所要時間: 10秒以内
```

#### Cockpit クラッシュ

```
1. cockpit Ghostty が消える
2. cockpit tmux session (projwm-cockpit) も消失する可能性
3. transaction loop が実行
4. tmux session を再作成 + Ghostty を再起動。park workspace CP1 に再配置
5. 30 秒間隔の health probe が検出するまで最大 30 秒
```

#### ディスプレイ切断 / 追加

```
1. ディスプレイ上のウィンドウが再配置される可能性
2. transaction loop が実行
3. 残存ディスプレイにウィンドウを再配置 / cockpit の park workspace を追加
所要時間: 5秒以内
```

#### 起動時復帰 (前回の異常終了後)

projwmd が起動するたびに、以下を実行する：

```
1. PersistentStore から最後に committed された DesiredWorld を読み込む
2. observe で現在の実際の状態 (ObservedWorld) を取得
3. 差分を検出
4. transaction loop で修正
```

**ケースA**: state にウィンドウあり → 実際はない → 再作成
**ケースB**: 実際にウィンドウあり → state にない (orphan) → title から identity 復元を試みる。復元できれば再登録 (adopt)、できなければ orphan として尊重。
  - adopt は **managed slot workspace 上のウィンドウに限定** する（§6.9）。user 自身の
    workspace 上のウィンドウは title が一致しても adopt しない（slot 領域外は不可侵 =
    user のウィンドウを cold-start で巻き込まない安全弁）。
  - adopt 成立時は当該 live window-ID を provenance に記録し、以後は ID 帰属へ移行する。
  - single-process アプリ (Zed) では title 衝突が起きやすいため、この slot 領域限定が
    特に効く（§4.1 single-process 制約）。
**ケースC**: 一致 → 何もしない
**ケースD**: state 破損 → bak から復旧。無ければ実際のウィンドウから再構築

**前提**: PersistentStore は atomic rename。state.json.bak は常時1世代保持。`naming.Parse()` で title から identity を復元するロジックが存在する。

---

## 4. 操作の定義

> ユーザーが trigger する操作と、transaction loop が内部的に使う操作を分けて定義する。ユーザー操作は state 変更と summon (close/spawn/focus) のみ。物理的な移動 (move) は transaction loop の専権。

### 4.1 ユーザー操作の状態遷移

各瞬間のシステム状態は以下で表現される：

| 要素 | 説明 | 例 |
|---|---|---|
| **current_ws** | ユーザーが今いるワークスペース | `Q`, `A`, `B`, `1` |
| **focused_window** | 今フォーカスしているウィンドウの identity | `shell-1:dotfiles` |
| **visible_windows** | 現在の WS に表示されているウィンドウ群 | `[shell-1:dotfiles, ai-1:dotfiles, editor-1:dotfiles]` |
| **active_profile** | 現在のプロファイル | `work` |

#### 操作1: slot の shell に jump (summon)

```
操作前:
  current_ws: 任意
  focused_window: 任意

トリガー: slot Q キー

操作後:
  current_ws: shell-1:dotfiles が存在する WS
  focused_window: shell-1:dotfiles（最も若い id の shell）
  visible_windows: slot Q のウィンドウ群

状態遷移: 任意 → (shell-1:dotfiles の ws, shell-1:dotfiles)

複数 shell の jump:
  - slot キーを連打 → shell-1 → shell-2 → ... → shell-N を循環
  - 最後にフォーカスしていた shell を記憶し、次回 slot キーでそこに戻る
```

#### 操作2: slot の editor に jump (summon)

```
操作前:
  current_ws: 任意
  focused_window: 任意

トリガー: slot Q の editor キー

操作後:
  current_ws: editor-1:dotfiles が存在する WS
  focused_window: editor-1:dotfiles（最も若い id の editor）

状態遷移: 任意 → (editor-1:dotfiles の ws, editor-1:dotfiles)

複数 editor の jump:
  - editor キーを連打 → editor-1 → editor-2 → ... を循環
```

#### 操作3: slot の browser に jump (summon)

```
操作前:
  current_ws: 任意
  focused_window: 任意

トリガー: slot Q の browser キー

操作後:
  current_ws: browser-1:dotfiles が存在する WS
  focused_window: browser-1:dotfiles（最も若い id の browser）

状態遷移: 任意 → (browser-1:dotfiles の ws, browser-1:dotfiles)

複数 browser の jump:
  - browser キーを連打 → browser-1 → browser-2 → ... を循環
```

#### 操作4: 別の project に切り替え

```
操作前:
  current_ws: Q (dotfiles)
  focused_window: shell-1:dotfiles

トリガー: slot W キー

操作後:
  current_ws: W (manaflow)
  focused_window: 直前にフォーカスしていた manaflow のウィンドウ
  visible_windows: slot W のウィンドウ群

状態遷移: (Q, shell-1:dotfiles) → (W, shell-1:manaflow)
```

daemon に到達する intent kind は `switch-project`、payload は `{"slot":"W"}` のように target slot を持つ。実行結果は workspace 変更だけではなく、target project/slot で直前に focused だった managed window への focus 復帰まで含む。

#### 操作5: 同じ slot 内のウィンドウ切替

```
操作前:
  current_ws: Q
  focused_window: shell-1:dotfiles

トリガー: slot Q 内の editor 切替キー

操作後:
  current_ws: Q (変わらない)
  focused_window: editor-1:dotfiles
  visible_windows: 変わらない

状態遷移: (Q, shell-1:dotfiles) → (Q, editor-1:dotfiles)
```

daemon に到達する intent kind は `cycle-slot-window`、payload は `{"slot":"Q","kind":"editor"}` のように current slot と target window kind を持つ。workspace は変えず、slot 内の managed identity だけを focus 対象として切り替える。

#### 操作6: viewer に jump

```
操作前:
  current_ws: 任意
  focused_window: 任意

トリガー: viewer キー

操作後:
  current_ws: A
  focused_window: 直前にフォーカスしていた AI の viewer
  visible_windows: [ai-view-1:dotfiles, ai-view-1:manaflow, ...]

状態遷移: 任意 → (A, ai-view-N:project)
```

#### 操作7: cockpit を表示/非表示

```
表示:
  操作前: 任意 → トリガー: cockpit キー
  操作後: current_ws: CP1, focused_window: cockpit

非表示:
  操作前: current_ws: CP1, focused_window: cockpit
  トリガー: cockpit キー または Esc
  操作後: current_ws: 表示前の WS に戻る, focused_window: 表示前のウィンドウに戻る

状態遷移: 任意 → (CP1, cockpit) → (元の ws, 元の window)
```

#### 操作8: profile を切り替える

```
操作前:
  current_ws: Q (work profile の dotfiles)
  active_profile: work

トリガー: profile 切替 (cockpit から、または CLI)

操作後:
  current_ws: Q (personal profile の blog)
  active_profile: personal

状態遷移: (work, Q) → (personal, Q)。tmux session は殺さない。
```

#### 操作9: project を追加する

```
操作前:
  slots: Q=dotfiles, W=manaflow, E=空き

トリガー: project 追加 (cockpit または CLI)

操作後:
  current_ws: E (新しく割り当てられた slot)
  focused_window: shell-1:new-project
  slots: Q=dotfiles, W=manaflow, E=new-project
```

#### 操作10: project をアーカイブする

```
操作前:
  current_ws: W (manaflow)
  slots: Q=dotfiles, W=manaflow

トリガー: archive (cockpit または CLI)

操作後:
  current_ws: W (解放された slot)。focused_window: なし
  slots: Q=dotfiles, W=空き
```

#### 操作11: scratch shell を開く/閉じる

```
操作前: 任意

トリガー: scratch キー

表示時: focused_window: scratch shell
非表示時: focused_window: scratch 表示前のウィンドウに戻る
```

scratch shell は project/profile に属さない system-level managed Ghostty window である。グローバルに 1 つだけ存在し、表示は冪等で、既存 scratch shell があれば新規作成せず focus する。実機テストで観測できるよう、tmux session と Ghostty title はどちらも `projwm-scratch-shell` とする。非表示時は scratch 表示直前の focused window に戻る。CLI/cockpit 操作ではなく shortcut 専用の user operation だが、daemon に到達する intent kind は `show-scratch-shell` / `hide-scratch-shell` として扱う。

#### 操作12: window を追加する (add-shell / add-ai / add-editor / add-browser)

```
操作前:
  project: dotfiles（slot Q に割り当て済み）
  windows: [ai-1, shell-1, editor-1]

トリガー: cockpit または CLI (例: projwm add-shell dotfiles)

操作後:
  windows: [ai-1, shell-1, shell-2, editor-1]
  shell-2 が生成され、slot Q 内に配置される

id 採番: 既存の最大 id + 1。削除で穴が空いても再利用しない
```

#### 操作13: window を削除する (remove-window)

```
操作前:
  project: dotfiles
  windows: [ai-1, shell-1, shell-2, editor-1]

トリガー: cockpit または CLI (例: projwm remove --window shell-2 dotfiles)

操作後:
  windows: [ai-1, shell-1, editor-1]
  shell-2 の tmux session が kill、ウィンドウが close

最後の window を削除する場合:
  - デフォルト: 空 windows[] を許容し、project を残す
  - --purge-if-empty: project ごと削除（確認プロンプト）
```

#### 操作14: browser のタブを追加する

```
操作前:
  project: dotfiles（browser-1 が存在）
  browser-1 のタブ: [tab-1 (URL-A), tab-2 (URL-B)]

トリガー: cockpit または CLI (例: projwm browser add-tab --project dotfiles --url <URL>)

操作後:
  browser-1 のタブ: [tab-1 (URL-A), tab-2 (URL-B), tab-3 (URL-C)]
  新しいタブが browser-1 内に追加される

ブラウザ内の位置: 最後尾に追加
```

daemon intent kind は `browser-add-tab`、payload は `{"project":"dotfiles","window":"browser-1","url":"https://..."}`。URL 本文は PrivatePayloadStore に保存し、DesiredWorld には opaque ref と URLCount だけを残す。

#### 操作15: browser のタブを削除する

```
操作前:
  browser-1 のタブ: [tab-1, tab-2, tab-3]

トリガー: cockpit または CLI

操作後:
  browser-1 のタブ: [tab-1, tab-2]
  tab-3 が削除される

最後のタブを削除する場合: browser window ごと close
```

daemon intent kind は `browser-remove-tab`、payload は `{"project":"dotfiles","window":"browser-1","tab":3}`。削除後のタブ構造は PrivatePayloadStore に保存し、DesiredWorld の URLCount / opaque ref を更新する。

#### 操作16: browser のタブ URL を変更する

```
操作前:
  browser-1 の tab-2 の URL: URL-B

トリガー: cockpit または CLI、またはユーザーが Vivaldi 内で直接 URL 入力

操作後:
  browser-1 の tab-2 の URL: URL-C
  システムが自動観測し、PrivatePayloadStore に保存
```

daemon intent kind は `browser-change-tab-url`、payload は `{"project":"dotfiles","window":"browser-1","tab":2,"url":"https://..."}`。ユーザーが Vivaldi 内で直接 URL 入力した場合も、最終的には同じ DesiredBrowserSession metadata 更新に収束する。

#### 操作17: browser のタブを並び替える

```
操作前:
  browser-1 のタブ: [tab-1, tab-2, tab-3]

トリガー: ユーザーが Vivaldi 内で手動でタブをドラッグ

操作後:
  browser-1 のタブ: [tab-1, tab-3, tab-2]
  システムが自動観測し、新しい順序を DesiredWorld に反映（§6.3 Level 3 の自動上書き）
```

daemon intent kind は `browser-reorder-tabs`、payload は `{"project":"dotfiles","window":"browser-1","from":3,"to":1}`。URL 本文や順序詳細は private payload 側に置き、DesiredWorld には opaque ref と URLCount だけを保存する。

### 4.2 システム操作（transaction loop が内部的に使う）

ユーザーが直接 trigger しない。planner がドリフトを検出し、transaction loop が実行する。

| 操作 | トリガー | 文脈 |
|---|---|---|
| `move-window-to-workspace` | ウィンドウが間違った WS にいる | ユーザーの手動ドラッグ、display 変更、OmniWM 再起動等 |
| `reorder-columns` | カラム配置が desired と異なる | spawn の着地ミス、ユーザーの手動並び替え、viewer 順序のドリフト |
| `close-window` | 不要なウィンドウが残っている | archived project のウィンドウ残留、viewer の orphan |
| `spawn-*` | 必要なウィンドウが存在しない | project 追加、再起動後の復帰、viewer の再作成 |
| `kill-session` | ライフサイクル削除 | profile 切替時の旧ウィンドウ close、archive 時の close |

### 4.3 ユーザーの手動操作とシステムの関係

ユーザーが OmniWM の機能でウィンドウを手動で移動・並び替えた場合：
1. システムはイベントとして検出する
2. **cross-workspace 移動**: transaction loop が `move-window-to-workspace` で元の slot へ強制 revert (Tier 4)
3. **同一 workspace 内の並び替え**: transaction loop が自動的に desired layout に上書き (Tier 2)

**drift の検出と修正**（§6.3 状態の階層を具体化）:

transaction loop は全階層で desired と observed の差分を検出し、自動修正する。修正の方法は差分の種類によって決まる：

| 差分の種類 | Level | identity | 修正操作 | 通知 |
|---|---|---|---|---|
| **drift**（間違った場所/順序） | L2 / L3 | 既知 | move-to-workspace / reorder-columns | カード通知（事後） |
| **missing**（ウィンドウ消失） | L1 | 既知（state に記録あり） | spawn | カード通知（事後） |
| **orphan**（正体不明のウィンドウ） | — | 不明（命名規約に一致しない） | 自動判断不可 | cockpit カード提案（要ユーザー判断） |
| **stale**（不要なウィンドウ残留） | L1 | 既知（archived 等） | close / kill-session | カード通知（事後） |

**ユーザーの手動操作と drift の関係**:

ユーザーが OmniWM の機能でウィンドウを手動で移動・並び替えた場合、システムは「意図的か偶発的か」を直接は読み取れない（OmniWM に intent チャネルが無い）。代わりに **操作の種類** と **復旧状態** で機械的に判定する：

1. **cross-workspace 移動（間違った slot）**: 常に drift として扱い、`move-window-to-workspace` で元の slot へ revert（Tier 4）。slot 割り当ては projwm の専権であり、ユーザーが別 slot に移しても「間違った場所」とみなす。事後通知（cockpit カード）。
2. **同一 workspace 内の並び替え（列順序）／ 定常状態**: intentional とみなし、観測順序を `AcceptedLayouts` へ上書き（Tier 2 auto-sync = accept）。revert しない。drift ではないので通知も不要。
3. **同一 workspace 内の順序食い違い ／ 復旧中**: 復旧中（startup・OmniWM 再起動直後など、当該 workspace のレイアウトを現 OmniWM インスタンス上で未だ converge できていない状態）は観測順序を accept せず、保存済み `AcceptedLayouts` を `reorder-columns` で **復元** する（§3.5・§6.3 L3）。
4. ユーザーが cross-workspace revert に抗って同じ移動を 60 秒以内に 2 回繰り返す場合、grace period 発動 → 修正停止 + warning カード。

**「判断できない」問題の解決 — recovery-gate**: 「ユーザーが意図的に並び替えたか」は観測では判定不能だが、**定常 / 復旧** は判定できる。定常状態で同一-ws の列順序を変える主体はユーザーのみ（spawn 着地・viewer 順序・OmniWM リセットはいずれも projwm 起因または復旧として別経路で扱う）であるため、「**定常状態の同一-ws 順序 divergence = ユーザー並び替え → accept**」「**復旧中の divergence = リセット → 保存値へ復元**」という機械的区別が安全に成立する。これにより §4.3 冒頭（cross-ws=revert / same-ws=accept）と §6.3 L3・§3.5（復旧時の順序復元）が両立する。

**orphan の扱い**（§2.2 原則1 から導出）:

- managed slot に出現したが title が命名規約に一致しない → identity 不明
- システムは自動判断できないため、cockpit カードで提案：
  - `[Enter]`: 新規 project として登録 + slot 割り当て
  - `[c]`: close
  - `[t]`: TUI で詳細操作
- identity が判明しているウィンドウは orphan 扱いしない（drift として自動修正）

### 4.4 各 kind の spawn 詳細

> §2.3 summon フローを kind ごとに具体化する。

#### ai (Ghostty + tmux)

```
spawn 条件:
  - identity: ai-N:project。N = project 内の最大 ai id + 1（最初は 1）
  - tmux session を作成: tmux new-session -d -s ai-N/project
  - viewer 用 grouped session: tmux new-session -d -t ai-N/project -s ai-N/project_v
  - Ghostty を起動: ghostty --title="ai-N:project" --working-directory=<cwd> -e tmux new-session -A -s ai-N/project
  - viewer window を起動: ghostty --title="ai-view-N:project" --working-directory=<cwd> -e tmux attach -r -t ai-N/project_v

既存確認:
  - ghostty title "ai-N:project" で既存ウィンドウを検索
  - 既存があれば focus。無ければ spawn

AI 起動コマンド:
  - ai-N/project の tmux session 内で AI CLI (Claude / Copilot) を起動
  - 初回 spawn 時に tmux send-keys で起動コマンドを送信
  - 次回以降は既存 session に attach するだけ（AI は既に起動中）

複数 AI:
  - add-ai --ai <name> CLI で追加: id は自動採番（最大値+1）
  - 各 AI は独立した tmux session + viewer grouped session を持つ
  - viewer は AI ウィンドウ単位で複製を表示
  - すべての AI は完全に対等（primary/default の概念はない）

remove:
  - remove-window ai-N <project> CLI で削除
  - AI tmux session + viewer grouped session を kill
  - AI window + viewer window を close
```

#### shell (Ghostty + tmux)

```
spawn 条件:
  - identity: shell-N:project。N = project 内の最大 shell id + 1（最初は 1）
  - tmux session を作成: tmux new-session -d -s shell-N/project
  - Ghostty を起動: ghostty --title="shell-N:project" --working-directory=<cwd> -e tmux new-session -A -s shell-N/project

既存確認:
  - ghostty title "shell-N:project" で既存ウィンドウを検索
  - 既存があれば focus。無ければ spawn

複数 shell:
  - add-shell CLI で追加: id は自動採番（最大値+1）
  - 各 shell は独立した tmux session を持つ
  - 同じ project 内で shell-1, shell-2, ... が共存可能

remove:
  - remove-window shell-N <project> CLI で削除
  - tmux session を kill、ウィンドウを close
```

#### editor (Zed)

```
spawn 条件:
  - identity: editor-N:project。N は通常 1（1 project に 1 Zed が標準）
  - tmux session 不要（Zed は GUI app）
  - Zed を起動: zed -n --user-data-dir ~/.cache/projwm-next/zed-data <cwd>
    （`zed` = Zed CLI shim: /Applications/Zed.app/Contents/MacOS/cli）
    -n: 新しい「ウィンドウ」で開く。フラグなしの `zed <cwd>` は既存 workspace を再利用する
    --user-data-dir: 設定分離は「Zed プロセスを projwm が最初に起動したとき」のみ有効
    設定: restore_on_startup = "none", auto_install_extensions = {}

  重要な実機制約 (single-process、2026-05-29 実機確認):
  Zed は GPUI single-instance。`-n` は新しい「ウィンドウ」を開くだけで新しい
  「プロセス」は作らない。`open -na` や別 --user-data-dir でも新プロセスは立たず、
  既存プロセスへ routing される。よって user の Zed が既に起動していれば managed
  ウィンドウもその同一プロセスに同居する。**Vivaldi の --user-data-dir のような
  プロセス単位の隔離・帰属は Zed では原理的に不可能**。帰結:
  - managed Zed プロセスを **kill してはならない** (user の編集中ウィンドウごと
    巻き込む)。ウィンドウ単位の AXClose のみ。
  - 識別・帰属もプロセスでなく **ウィンドウ単位** で行う (§6.9 参照)。

識別・帰属 (§6.9 に従う):
  - 通常運用 = provenance: spawn 時に得た live window-ID を (identity → liveID) で
    内部保持し所有の根拠とする。毎 observe で「ID 存在 + bundleId + title 整合」を
    検証し、崩れたら破棄して reconcile (盲信しない検証キャッシュ)。
    → user が後から開いた同名ウィンドウは provenance に無い = External、掴まない。
  - cold-start / 復旧 (provenance 無し or 失効) = title→identity で adopt を試みる。
    ただし adopt は **managed slot workspace 上のウィンドウに限定**。user 自身の
    workspace 上のウィンドウは title 一致でも触らない。該当が無ければ spawn。
  - 「既存があれば focus、無ければ spawn」の「既存」判定は上記の帰属で行う。

spawn 後の empty project 処理 (provenance-scoped):
  - Zed 起動直後、一時的に "empty project" / 空 title のウィンドウが出現する場合がある
  - この spawn の前後 diff で「新たに出現した」dev.zed.Zed ウィンドウのうち、project
    ウィンドウ (title=basename(cwd)) でないものは「我々の spawn 由来の余計ウィンドウ」と
    確定できるので AXClose で閉じる
  - spawn 前から存在した "empty project" は user のものとして触らない

複数 editor:
  - add-editor CLI で追加: editor-2, editor-3, ...
  - 各 Zed は `basename(cwd)` が同じなので、bundleId + title + workspace の組合せで識別
  - 基本的には editor-1 のみ推奨。複数 editor は高度なケース

remove:
  - remove-window editor-N <project> CLI で削除
  - Zed ウィンドウを AXClose で閉じる。Zed アプリ自体は kill しない
```

#### browser (Vivaldi)

```
spawn 条件:
  - identity: browser-N:project。N = project 内の最大 browser id + 1
  - tmux session 不要（Vivaldi は GUI app）
  - Vivaldi automation profile (projwm-next) を使用
  - 起動方法: 実装時に決定（chrome-cli / AppleScript / open -na）
  - window title: "browser-N:project"

既存確認:
  - Vivaldi automation profile の window で、title "browser-N:project" を検索
  - 既存があれば focus。無ければ spawn

profile isolation:
  - Vivaldi の user profile（デフォルト）は projwm の管理対象外
  - automation profile (projwm-next) のみ管理
  - user profile の Vivaldi window は origin (a) External 扱い

複数 browser:
  - add-browser CLI で追加
  - 各 browser は独立した Vivaldi window。別タブではなく別ウィンドウ

remove:
  - remove-window browser-N <project> CLI で削除
  - lifecycle removal: browser-window-close（VivaldiCloser 経由）
  - pre-observe → pre-validate → close → post-observe → post-validate

tab 管理:
  - 各 browser window 内のタブは §6.3 の Level 3 (ordering) 同様、システムが管理する
  - ユーザーが手動でタブを追加/削除/並び替え/URL 移動した場合、システムが自動観測し PrivatePayloadStore に保存
  - DesiredWorld にはタブの構造（どのタブがどの window に属するか）だけを保存し、URL 本体は保存しない

タブ操作のユーザー操作:

  タブ追加:
    - トリガー: cockpit または CLI
    - 指定された URL を Vivaldi automation profile で開く
    - 対象 browser window 内にタブとして追加される

  タブ削除:
    - トリガー: cockpit または CLI
    - 指定されたタブを close
    - 最後のタブを削除する場合は browser window ごと close

  タブ URL 移動:
    - トリガー: cockpit または CLI
    - 対象タブの URL を変更する
    - またはユーザーが Vivaldi 内で直接 URL を入力した場合、システムが自動観測

  タブ並び替え:
    - ユーザーが Vivaldi 内で手動でタブを並び替えた場合
    - 自動観測し、新しい順序を DesiredWorld に反映（Level 3 の自動上書きと同様）

profile 切替時の復元:
  - profile 切替で browser window が close された後、再開時に archive 時のタブ状態（構造のみ）を復元する
  - URL は PrivatePayloadStore から復元。復元に失敗した場合は空タブで開く

privacy:
  - URL 本体・cookie・session token は PersistentStore に保存しない
  - PrivatePayloadStore に分離保存する
  - log / trace / status 出力では URL を redact 表示する
```

### 4.5 複合操作の体系

| 複合操作 | 対応する §4.1 操作 | 構成 | 備考 |
|---|---|---|---|
| `profile_switch` | 操作8 | close 旧 → observe-barrier → spawn 新 → viewer 更新 | tmux session は殺さない |
| `archive_project` | 操作10 | close 全 → observe-barrier | tmux session も kill |
| `unarchive_project` | 操作10 | state 更新のみ → 自動再展開しない (park 状態に) | — |
| `assign_project` | 操作9（一部） | state 変更 → viewer 更新 | move は transaction loop が後で実行 |
| `reconcile` | —（システム） | transaction loop の入口（observe → plan → execute → verify） | 手動・自動いずれも可 |

**原則**: ユーザー操作は state 変更と summon (close/spawn/focus) のみ。ウィンドウの物理的な移動 (move) は transaction loop の専権。

### 4.6 リトライ・タイムアウト体系

| パラメータ | 値 | 用途 |
|---|---|---|
| `CtlExecutor.Timeout` | 5s | omniwmctl サブプロセス |
| `observeBarrierSleep` | 500ms | AX/SkyLight 伝播待機 |
| `SettleTimeout` | 30s | spawn/move 後のポーリング |
| `DisappearWait` | 15s | close 後の消失待機 |
| `waitFocusedWindow` | 1.5s | focus 確認 |
| `preMoveGrace` | 150ms | niri 内部状態の安定化 |

**process-alive fallback**: spawn settle タイムアウト + OS プロセス生存 → 成功扱い (OmniWM catalog 遅延対策)

**max replans 超過時**: トランザクション fail → ロールバック → cockpit に [INVARIANT] カード通知 → dirty scope 記録 → 次の intent/event で再挑戦

---

## 5. UI の定義

> ユーザーが触る具体的な UI を定義する。

### 5.1 インタラクション方法の分類

> §4.1 の全 17 操作を trigger 方法で分類する。

| 操作の種類 | trigger 方法 | 該当操作 |
|---|---|---|
| **日常操作** (jump, 切替) | ショートカットキー | 操作1-6 (shell/editor/browser jump, project 切替, 同 slot 切替, viewer jump) |
| **日常操作** (表示切替) | ショートカットキー | 操作7 (cockpit), 操作11 (scratch) |
| **確認操作** (status, 一覧) | CLI または cockpit | — |
| **設定操作** (追加・管理) | cockpit または CLI | 操作9 (project 追加), 操作12 (add-window) |
| **破壊操作** (削除・停止) | cockpit または CLI | 操作10 (archive), 操作13 (remove-window) |
| **profile 操作** | cockpit または CLI | 操作8 (profile 切替) |
| **browser tab 操作** | cockpit または CLI | 操作14-17 (tab 追加/削除/URL 変更/並び替え) |

### 5.2 インタラクション方法の対応表

> §4.1 の全 17 操作の trigger 方法。

| 操作 | §4.1 | ショートカットキー | cockpit | CLI |
|---|---|---|---|---|
| shell に jump | 操作1 | ✓ | — | — |
| editor に jump | 操作2 | ✓ | — | — |
| browser に jump | 操作3 | ✓ | — | — |
| project 切替 | 操作4 | ✓ | — | — |
| 同 slot 内 window 切替 | 操作5 | ✓ | — | — |
| viewer に jump | 操作6 | ✓ | — | — |
| cockpit 表示/非表示 | 操作7 | ✓ | — | — |
| scratch shell | 操作11 | ✓ | — | — |
| status 確認 | — | — | ✓ | ✓ |
| project 追加 | 操作9 | — | ✓ | ✓ |
| profile 切替 | 操作8 | — | ✓ | ✓ |
| profile 管理 | — | — | ✓ | ✓ |
| archive / unarchive | 操作10 | — | ✓ | ✓ |
| add-window | 操作12 | — | ✓ | ✓ |
| remove-window | 操作13 | — | ✓ | ✓ |
| tab 追加 | 操作14 | — | ✓ | ✓ |
| tab 削除 | 操作15 | — | ✓ | ✓ |
| tab URL 変更 | 操作16 | — | ✓ | ✓ |
| tab 並び替え | 操作17 | — | ✓ | ✓ |

### 5.3 キーショートカット

> キー配置は後から決定する。ここでは「どの操作にショートカットキーを使うか」だけを定義する。

| 操作 | §4.1 | ショートカットキーを使う | 備考 |
|---|---|---|---|
| slot の shell に jump | 操作1 | space + slotキー | Q-P。連打で shell-1 → shell-2 → ... 循環 |
| slot の editor に jump | 操作2 | space + modifier + slotキー | shift 等。連打で循環 |
| slot の browser に jump | 操作3 | space + modifier + slotキー | 別 modifier |
| project 切替 | 操作4 | space + slotキー | 操作1と共用 |
| 同 slot 内 window 切替 | 操作5 | space + ctrl + slotキー | slot 内の別ウィンドウへ |
| viewer に jump | 操作6 | space + A | — |
| cockpit 表示/非表示 | 操作7 | space + F | park workspace に切替 |
| scratch shell | 操作11 | 別のキー | alt+space 等。daemon intent は `show-scratch-shell` / `hide-scratch-shell` |

### 5.4 Cockpit TUI

#### 概要

- projwm-managed モニタ (workspace A / Q-P が住むディスプレイ) に 1 つだけ
- tmux session `projwm-cockpit` を backing にもつ
- park workspace CP1 に永続配置
- 表示/非表示は workspace 切替で実現（cockpit ウィンドウは移動しない）
- 表示時: active workspace が CP1 に切り替わる
- 非表示時: 元の workspace に戻る（§5.3 の cockpit キー / Esc で trigger）

#### 全体構造

```
┌─ projwm-cockpit ───────────────────────────────────┐
│ topbar: gen / epoch / profile / convergence / cards │
├─[ Slots ]─ Cards (N) ─ Archived ─ Profiles ─ Trace ─┤
│                                                      │
│  main content (depends on active tab)                 │
│                                                      │
├──────────────────────────────────────────────────────┤
│ Navigate: [↑↓] [1-5] tabs  [Tab] profile  [/] filter│
│ Actions:  context-dependent available actions         │
│ Help: [?] help  [Ctrl-P] palette  [Esc] hide         │
└──────────────────────────────────────────────────────┘
```

#### タブ構成

| Tab | キー | 内容 |
|---|---|---|
| **Slots** | `1` | active profile の slot Q-P assignment、viewer (A) AI stream 一覧、park 一覧 |
| **Cards (N)** | `2` | カードモーダル。N=未対応カード件数。左: detail、右: workspace zoom-out |
| **Archived** | `3` | archived project 一覧 (unarchive 操作) |
| **Profiles** | `4` | 全 profile + assignments (active 強調)、profile create/delete/rename |
| **Trace** | `5` | 最近の transaction trace |

#### カードシステム

| カード | 説明 |
|---|---|
| `[NEW]` | 新規ウィンドウが managed workspace に出現（Tier 1） |
| `[CLOSED]` | ウィンドウがユーザーに閉じられた（Tier 4 自動 respawn の通知） |
| `[MOVED]` | ウィンドウが別 workspace に移動された（Tier 4 revert の通知） |
| `[INVARIANT]` | 不変条件違反が検出された |
| `[MANIFEST]` | manifest 変更が検出された |
| `[OMNIWM-RECOVERY]` | OmniWM 自己修復が実行された |

#### Wizard (新規 project / profile 作成)

`n` キーで起動。全項目同時表示 (B2 form)。defaults prefill。Tab で field 移動、Enter で submit。

#### Command palette (Ctrl-P)

fuzzy 検索。全 action を 1 つの list として網羅。Enter で実行。

#### 表示モード

| Mode | 入口 | 出口 |
|---|---|---|
| **Proposal**（強制表示） | システムが提案カードを push | 応答後、元の visibility 状態へ復帰 |
| **Navigation** | space+F で開く | 操作後 自動 hide |
| **Management** | space+F で開く | 操作後も stay、space+F で hide |

### 5.5 エラー通知

| 通知方法 | 用途 |
|---|---|
| cockpit カード | 不変条件違反、OmniWM 自己修復、orphan 提案 |
| cockpit topbar | convergence status (CONVERGED / CONVERGING / REPLAN_FAILED) |
| `projwm doctor` | PASS / WARN / FAIL 形式で健全性検査結果を表示 |

macOS notification は一切使わない。すべて cockpit または CLI に集約。

### 5.6 status / doctor 出力

#### `projwm status`

- Generation ID, Epoch
- Active profile name + description
- 全 profile の slot → project 割り当て一覧
- 各 active project の windows 状態（kind, index, tmux 生死, live window 生死）
- viewer workspace A 上の AI stream 一覧
- park 状態の project 一覧
- archived project 一覧
- convergence status (CONVERGED / CONVERGING / REPLAN_FAILED)
- manifest digest 検証状態

#### `projwm doctor`

- projwmd プロセスの存在確認
- PersistentStore の読み取り可否
- manifest の存在と digest 検証
- IPC socket の到達性
- 必要アプリ（Ghostty, Vivaldi, Zed, tmux, omniwmctl）の存在
- 不変条件チェック
- 各検査項目は PASS / WARN / FAIL で報告

### 5.7 CLI コマンド一覧

| コマンド | 用途 |
|---|---|
| `projwm up --ai <name> --slot <SLOT>` | project の新規作成と割り当て |
| `projwm add-ai --ai <name>` | AI ウィンドウの追加 |
| `projwm add-shell` | shell ウィンドウの追加 |
| `projwm add-editor` | editor ウィンドウの追加 |
| `projwm remove --window <KIND-N>` | ウィンドウの削除 |
| `projwm profile create <NAME>` | プロファイル作成 |
| `projwm profile switch <NAME>` | プロファイル切替 |
| `projwm profile assign <SLOT> <PROJECT>` | project の slot 割り当て |
| `projwm profile unassign <SLOT>` | slot の割り当て解除 |
| `projwm profile delete <NAME>` | プロファイル削除 |
| `projwm archive <PROJECT>` | project のアーカイブ |
| `projwm unarchive <PROJECT>` | project の復活 |
| `projwm jump <SLOT\|PROJECT>` | slot/project への jump |
| `projwm reconcile [--dry-run]` | 状態の整合性確認と修正 |
| `projwm status [--json]` | 状態の表示 |
| `projwm doctor` | 健全性診断 |
| `projwm trace [--last\|<txid>]` | transaction trace の表示 |
| `projwm tui` | cockpit の手動起動（通常は常駐） |

---

## 6. 設計原則

### 6.1 session > window

tmux session が真の生存。ウィンドウは session の「画面」でしかない。

- ウィンドウが消えても session があれば復活可能
- session が消えたら project は「死んでいる」（archive と同等）
- macOS 再起動後は session も無いため、両方再作成

### 6.2 identity > location

ウィンドウの「正体」は `(project, kind, id)`。場所 (slot) は後付け。

- ウィンドウがどこにいても、`(project, kind, id)` で識別される
- slot は transaction loop が管理する配置情報
- ユーザー操作は identity に対して行う（「dotfiles の shell を開く」）

### 6.3 状態の階層 (State Hierarchy)

projwm が管理する状態には階層がある。transaction loop は **全階層** で desired と observed の一致を保証する。

| Level | 対象 | desired | observed | drift 例 | 修正操作 |
|---|---|---|---|---|---|
| **L1: identity** | どのウィンドウが存在すべきか | DesiredWorld.Projects[].Windows[] | ObservedWorld.Windows[] (title から identity 解決) | ウィンドウが消えている / 余分なウィンドウがある | spawn / close |
| **L2: placement** | どの slot にいるべきか | DesiredProfile.Assignments | ObservedWindow.Workspace | ウィンドウが別の slot にいる | move-to-workspace |
| **L3: ordering** | workspace 内でどう並ぶべきか | DesiredLayout.Columns | ObservedLayout.Columns | カラム順序が異なる / viewer 順序が slot 順と不一致 | reorder-columns |

**設計の意味**:
- すべての階層で desired と observed の差分を planner が検出する
- 差分があれば transaction loop が自動修正する
- 階層が深いほど修正の優先度が低い（L1 > L2 > L3）
- ユーザーは L1（何があるか）を操作する。L2（どの slot か）はシステムの専権（cross-ws 移動は Tier 4 で revert）。L3（workspace 内の並び順）は spawn 時の slot 順をシステムが決めるが、**定常状態でのユーザーの同一-ws 並び替えは Tier 2 で desired（AcceptedLayouts）に取り込まれる**（§4.3）。システムはユーザー並び替えを取り込んだ desired を、復旧・spawn 着地時に enforce する
- この階層構造により、システムの reorder や move は「特別な操作」ではなく、「L2/L3 の drift 修正」として統一的に扱える（ただし L3 の**定常状態でのユーザー並び替え**は drift ではなく Tier 2 accept 対象 — §4.3）

### 6.4 状態の所有権 (State Ownership)

DesiredWorld は projwm が **唯一の authority** として管理する。ユーザーは intent を送り、システムが desired を更新する。constraint（不変条件）は desired がとりうる範囲を制限する。

```
ユーザー ──(intent)──→ システム ──(constraint を適用)──→ DesiredWorld
                              │
                     transaction loop
                              │
                              ▼
                         ObservedWorld（実状態）
```

- **ユーザー**: intent を発行する（「dotfiles を slot Q に割り当てたい」）。直接 DesiredWorld を書き換えない
- **システム**: intent を受け取り、constraint を適用して DesiredWorld を更新する。不変条件に違反する intent は拒否する
- **transaction loop**: DesiredWorld と ObservedWorld の差分を全階層で検出し、操作を実行して収束させる
- **constraint**: 不変条件 (§3.4) が DesiredWorld のとりうる範囲を制限する。例: 「同一 (project, kind, id) は世界に1つだけ」「archived project は active profile に存在しない」

### 6.5 single writer

WM への変更は 1 プロセス (projwmd) のみ。

- IPC 経由の intent はすべて daemon に集約
- read-only コマンド (status, doctor) は直接 WM を読む
- `wmMutationLock` で直列化

### 6.6 idempotency

すべての操作は冪等。同じ操作を繰り返しても壊れない。

idempotency は identity (§6.2) と紐付いて初めて成立する：
- 「開く」は identity `(project, kind, id)` で既存ウィンドウを検出し、あれば focus するだけ（新規作成しない）
- 「閉じる」は identity で対象を特定し、既に無ければ noop

**誤解の防止**: 「同じ操作を2回呼んでも大丈夫」だけでは「2回目も新しいウィンドウが作られる」と誤解される。正しくは「identity で既存を検出し、再利用する」。これが summon (§2.1 原則2) と直接つながる。

### 6.7 testability

各操作は独立してテスト可能。

- アダプタインターフェースで WM 操作を抽象化
- fake adapter でユニットテスト
- リトライ・タイムアウトは体系化されたルールに従う

### 6.8 graceful degradation

部分的な失敗で全体が壊れない。

- 1 つのウィンドウの spawn が失敗しても、他のウィンドウの操作は続行
- transaction loop が次の iteration で replan して修復
- ユーザーには cockpit で状態を表示

### 6.9 identity の永続性

`(project, kind, id)` は macOS 再起動後にも必ず回復可能でなければならない。

- ウィンドウの identity は **title** に符号化される（`naming.Resolve()` が生成）
- title はウィンドウの属性であり、macOS 再起動後もアプリが再起動すれば同じ title が付けられる
- transaction loop が実行されたとき、全ウィンドウを query して title を読み、state.json の (project, kind, id) と突き合わせる

**前提条件**:
- title は `naming.Resolve()` が一意に生成する（手動で変更しない）
- Ghostty の `--title` で起動時に固定し、tmux 内からの上書きを防ぐ
- Zed は `basename(cwd)` が title になる（Zed の仕様）

**provenance（由来）による帰属 — single-process アプリ対策**:

title は再起動後の identity 回復に必須だが、**識別子としては曖昧**（user が同名
project を自分で開けば衝突する）。特に Zed は single-process（§4.1）でプロセス単位の
帰属ができないため、title だけでは user のウィンドウを誤って掴む危険がある。これを
防ぐため、identity は2層で帰属する：

1. **provenance（通常運用、第一優先）**: projwm が spawn / adopt した瞬間の live
   window-ID を `(project, kind, id) → liveWindowID` として state に永続保持する
   （store に載せる）。「我々が現に所有しているウィンドウ」の根拠。
   - **検証キャッシュであり盲信しない**: 毎 observe サイクルで「その liveWindowID が
     今も存在し、bundleId と title が期待どおりか」を検証する。window-ID は再利用や
     silent close で stale 化し得るので、1つでも崩れたら entry を破棄し reconcile
     （再 adopt / 再 spawn / release）に委ねる。これにより「内部で持つ ID が実は
     別物を指す」ズレも次サイクルで自動是正される。
   - user が後から開いた同名ウィンドウは provenance に無い → External として尊重し、
     決して掴まない。

2. **title → identity（cold-start / 復旧、provenance が無い・失効した場合のみ）**:
   `naming.Resolve()` で title から identity を復元し adopt（再登録）する。ただし
   adopt は **managed slot workspace 上のウィンドウに限定**する（§3.5 ケースB）。
   user 自身の workspace 上のウィンドウは title が一致しても adopt しない（slot 領域外
   は不可侵）。該当が無ければ spawn して provenance を確立する。

3. **非有効化 project は対象外**: identity が DesiredWorld に存在する（active profile
   かつ slot 割当済み）ときのみ adopt / spawn の対象。slot に有効化されていない
   project のウィンドウは provenance も title 照合もせず、orphan として尊重する。
   adopt / spawn のトリガは「slot への有効化」。

> 不変条件: provenance により「同一 identity に複数の所有窓」(INV-01) を厳密化する。
> 我々が spawn / adopt していない同名ウィンドウは INV-01 の duplicate ではなく
> External として扱い、close 対象にしない（user のウィンドウ保護）。

#### 6.9.1 帰属の振る舞い表（attribution edge-case contract）

各行が1つのエッジケースの**期待挙動**を規定する正規表。各行は test ID へ 1:1 で
対応する（test owner は `TestZedAttr_<ID>` 系。L0/L2 は deterministic、L3 は実機
single-op、SSOT §10 レイヤ戦略）。⚠️ = single-process + basename-title の原理的
限界として**防御せず受容**するケース。

**A. provenance 所有・検証（通常運用）**

| ID | エッジ | 期待挙動 | 層 |
|---|---|---|---|
| ATTR-A1 | editor spawn | (identity→liveID) を provenance 記録 | L2 |
| ATTR-A2 | provenance窓存在 + bundle+title 整合 | managed identity をそれに解決（title-only より優先） | L0 |
| ATTR-A3 | provenance liveID が観測に無い | entry 破棄→unmatched→respawn | L0/L2 |
| ATTR-A4 | liveID 存在だが bundle≠Zed or title 不一致 | entry 棄却（stale/reuse）→title/unmatched | L0 |
| ATTR-A5 | ⚠️ liveID 再利用 + 偶然 bundle+title 一致 | 誤所有し得る（天文学的に稀、防御せず明記） | — |
| ATTR-A6 | spawn-dedup（既存発見で新規 spawn せず） | 既存窓 ID を provenance に反映、二重記録しない | L2 |

**B. title 衝突（cold-start / 復旧 / 同時 user 窓）**

| ID | エッジ | 期待挙動 | 層 |
|---|---|---|---|
| ATTR-B1 | 所有確立後に user が同名窓を開く | user窓=External、adopt/move/close しない | L0/L2 |
| ATTR-B2 | cold-start・同名窓が **slot 上**・provenance無 | adopt（provenance記録+管理） | L0/L2 |
| ATTR-B3 | cold-start・同名窓が **非slot(user)上** | adopt しない（不可侵）→slot に fresh spawn | L0/L2 |
| ATTR-B4 | 同 title 3窓（1 provenance+2非） | provenance窓に解決、他は External、INV-01 で close しない | L0 |
| ATTR-B5 | ⚠️ 別ディレクトリが同 basename | title で区別不能。provenance(ID)が通常運用を救い、cold-start は slot限定で被害局限 | L0（限界文書化） |

**C. 複数 editor**

| ID | エッジ | 期待挙動 | 層 |
|---|---|---|---|
| ATTR-C1 | editor-1 & editor-2 同 project（同 title） | provenance（別 liveID）で区別 | L0/L2 |
| ATTR-C2 | ⚠️ 復旧時（provenance失効）に editor-1/2 | title で区別不能=best-effort（editor-1 推奨を明記） | L0（限界文書化） |

**D. empty-project 窓**

| ID | エッジ | 期待挙動 | 層 |
|---|---|---|---|
| ATTR-D1 | spawn diff 窓内で出現 | provenance-scoped で AXClose | L2/L3 |
| ATTR-D2 | spawn 前から存在（user の） | 不可侵 | L2/L3 |
| ATTR-D3 | grace 後に遅延出現 | cleanup 取りこぼし → reorder の stray 透過（managed-相対）が backstop | L3 |
| ATTR-D4 | project 窓がロード中 title="" の瞬間 | empty-project と誤判定して close しない（provenance 捕捉予定窓を保護） | L2/L3 |
| ATTR-D5 | 同時多 Zed spawn（profile切替） | 各 project窓=title一致で保持、各余計窓=close、帰属は spawn batch 単位 | L2 |

**E. ライフサイクル遷移（provenance は identity に追従）**

| ID | エッジ | 期待挙動 | 層 |
|---|---|---|---|
| ATTR-E1 | remove-window editor-N | provenance窓を AXClose + entry クリア、**プロセス kill せず** | L2/L3 |
| ATTR-E2 | archive project | INV-04 で全窓 AXClose + provenance クリア | L2 |
| ATTR-E3 | profile 切替（project が active 外へ） | 計画どおり close/移動 + provenance クリア | L2 |
| ATTR-E4 | slot 有効化 | adopt（slot限定 title）or spawn → provenance 確立 | L2 |
| ATTR-E5 | slot 無効化（active→inactive） | inactive policy で処理、close 時 provenance クリア | L2 |

**F. プロセス安全（single-process 不変条件）**

| ID | エッジ | 期待挙動 | 層 |
|---|---|---|---|
| ATTR-F1 | managed Zed 窓を1つ除去 | プロセス + 他 Zed 窓は生存（AXClose, kill 禁止） | L3実機 |
| ATTR-F2 | テストの crash 模擬 | user Zed 稼働時（zed-count≠1）は skip（誤 kill 防止） | L3/L4 precond |

**G. 復旧（daemon / macOS 再起動）**

| ID | エッジ | 期待挙動 | 層 |
|---|---|---|---|
| ATTR-G1 | daemon のみ再起動・Zed 生存・provenance 永続 | ID 有効 → 再 match、**respawn しない**（良路） | L2（永続state） |
| ATTR-G2 | macOS 再起動・両死亡・provenance stale | 全 stale 破棄 → slot限定 title adopt → 非該当は fresh spawn | L2 |
| ATTR-G3 | 再起動で user Zed が自分の workspace に自動復帰 | 非slot=不可侵、projwm は slot に再構築 | L2 |

**H. 適用範囲**

| ID | エッジ | 期待挙動 | 層 |
|---|---|---|---|
| ATTR-H1 | provenance の scope | Zed（single-process/title曖昧）で必須。Ghostty=title固定で信頼、Vivaldi=プロセス帰属(B-05)。primary matcher として全 kind 適用可だが load-bearing は Zed | — |

> ⚠️ 限界（ATTR-A5 / B5 / C2）: 完全な帰属は single-process（プロセス区別不可）+
> basename-title（同名衝突）では原理的に不可能。provenance（ID 帰属）+ slot 領域限定
> adopt + 「非 provenance 窓は不可侵」で**被害を局限**する設計とし、残余リスクは明記して
> 受容する。

**既存列挙との接続（重複を作らない）**: ATTR は独立体系ではなく既存要求の拡張。
- ATTR-B4（唯一性）は **INV-01**（§3.4 / Check14）を provenance-aware 化したもの。
- ATTR-A2 の title 部分は **INV-10**（identity-from-title）、衝突解決のみ provenance 新規。
- ATTR-B1/B3（非 provenance 窓不可侵）は **INV-11**（managed candidate outside ws）の拡張。
- ATTR-D1（empty-project close）は既存 **ZED-CONFIG / §10.4 spawn S4**
  `TestSpawnEditorEmptyProjectCleanup` が owner（再実装しない）。ATTR-A6 は §10.4 S5。
- ledger 上の新規 provenance 要求は **`ZED-ATTR`** ファミリ（`internal/ssottest/ledger_test.go`、
  Section §6.9/§6.9.1、Status は実装まで statusRed）。authoritative 保証は L4 実機行。

### 6.10 操作の順序

複数ウィンドウの操作は、planner が順序を決定する。executor は順序を無視しない。

- **close → observe-barrier → spawn**: 旧ウィンドウを閉じてから新ウィンドウを開く。逆順だと slot が埋まっている
- **spawn → settle → verify**: ウィンドウを開いた後、安定化を待ってから検証する
- **profile switch**: 旧ウィンドウを全て閉じ → observe-barrier → 新ウィンドウを全て開く
- **archive**: 全ウィンドウを閉じ → tmux session を kill → state を更新

---

## 7. アーキテクチャ

### 7.1 Transaction Loop

```
intent / event
      │
      ▼
  ┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
  │ Observe │────▶│ Reduce  │────▶│  Plan   │────▶│ Execute │
  └─────────┘     └─────────┘     └─────────┘     └─────────┘
       │                                                │
       │              ┌─────────┐     ┌─────────┐      │
       └─────────────│ Commit  │◀────│ Settle  │◀─────┘
                     └─────────┘     └─────────┘
                           ▲              │
                           │         ┌────┴────┐
                           └─────────│ Verifier│
                                     └─────────┘
```

| フェーズ | 責務 |
|---|---|
| **Observe** | WM から最新の ObservedWorld を取得 |
| **Reduce** | (WorldState, Intent) → DesiredWorld。純粋関数 |
| **Plan** | (WorldState, DesiredWorld) → []Operation。決定論的 rule-based planner |
| **Execute** | Operation を adapter 経由で実行。Phase 分離（A: removal, B: spawn, C: layout） |
| **Settle** | 実行後の状態安定化を wait |
| **Verify** | PredictedWorld と ObservedWorld を比較 |
| **Commit** | 状態の世代を進め、PersistentStore に保存 |

**planner の phase 分離**:
- Phase A: removals（close, kill-session）
- Phase B: spawns（spawn-terminal, spawn-editor, spawn-browser, spawn-viewer, spawn-cockpit）
- Phase C: layout（move-to-workspace, reorder-columns, focus）
- 各 phase 間に `observe-barrier` を挿入し、前 phase の変更が WM に反映されるのを待つ

**max replans 超過時の動作**:
- トランザクションは fail する（commit されない）
- WorldState はトランザクション開始前の状態にロールバックする
- cockpit に [INVARIANT] カードを通知する
- 次の intent/event が到来したときに再挑戦する（自動リトライはしない）
- `dirty scope` を記録し、次回の transaction loop で確実に再処理されるようにする

### 7.2 パッケージ境界

```
cmd/
  projwmd/              # daemon（single-writer）
  projwmctl/            # 制御 CLI（intent client）
  projwmevent/          # イベントサイドカー（state mutation 不可）
  projwm/               # ユーザー CLI
  projwm-cockpit/       # TUI (bubbletea)
  projwmstore-bootstrap/# store 初期化

internal/
  adapter/
    wm/                 #   OmniWM (sigwm.go)
    browser/            #   Vivaldi
    zed/                #   Zed
    session/            #   tmux
  controller/           # transaction loop 制御
  reducer/              # 純粋関数: (WorldState, Intent) → DesiredWorld
  planner/              # 決定論的 rule-based plan 生成
  executor/             # Operation 実行
  settler/              # 安定化 wait
  simulator/            # 予測状態計算
  verifier/             # 予測 vs 実測の比較
  store/                # PersistentStore (MemoryStore / FileStore)
  world/                # コア型定義
  intent/               # Intent 型
  event/                # Event 型
  op/                   # Operation カタログ
  invariant/            # 不変条件チェック
  manifest/             # Manifest パーサ/バリデータ
  ipc/                  # IPC プロトコル
  identity/             # ウィンドウ identity 解決
  naming/               # 命名規約 (Resolve, Parse)
  semop/                # セマンティック操作 wrapper
```

### 7.3 命名規約

`(project, kind, id)` から `naming.Resolve()` が一意に生成する：

| kind | 算出物 | 規則 | 例 |
|---|---|---|---|
| ai | tmux session | `ai-<id>/<project>` | `ai-1/dotfiles` |
| ai | ghostty title | `ai-<id>:<project>` | `ai-1:dotfiles` |
| ai | viewer tmux | `ai-<id>/<project>_v` | `ai-1/dotfiles_v` |
| ai | viewer title | `ai-view-<id>:<project>` | `ai-view-1:dotfiles` |
| shell | tmux session | `shell-<id>/<project>` | `shell-1/dotfiles` |
| shell | ghostty title | `shell-<id>:<project>` | `shell-1:dotfiles` |
| editor | Zed title | `basename(<cwd>)` | `dotfiles` |
| browser | Vivaldi title | `browser-<id>:<project>` | `browser-1:dotfiles` |
| cockpit | tmux session | `projwm-cockpit` | `projwm-cockpit` |
| cockpit | ghostty title | `projwm-cockpit-<display>` | `projwm-cockpit-0` |
| scratch | tmux session | `projwm-scratch-shell` | `projwm-scratch-shell` |
| scratch | ghostty title | `projwm-scratch-shell` | `projwm-scratch-shell` |

**注意**:
- editor (Zed) は `basename(cwd)` が title。同じ basename の project が複数ある場合は `bundleId + title + workspace` の組合せで識別する
- external は projwm が管理しないため、命名規約なし
- title の `:` と `/` の使い分け: Ghostty title は `:`、tmux session 名は `/`（tmux が `:` を許容しないため）

### 7.4 ドメインモデル

#### WorldState

```
WorldState = {
  Environment:  ManagedEnvironment    // Nix が書いた manifest（静的）
  Desired:      DesiredWorld          // あるべき姿（intent で変化）
  Observed:     ObservedWorld         // WM から観測した実際の状態
  Predicted:    PredictedWorld        // operation 実行後の予測状態
  Meta:         ControllerMeta        // epoch, generation, cockpit state 等
}
```

#### ManagedEnvironment

Nix が author し、`projwmd` が読み込む manifest JSON：

| フィールド | 内容 |
|---|---|
| `windowManager` | backend 名、layout/focus tuning |
| `workspaces` | 全 workspace の定義 (ID, rawName, role, display affinity) |
| `slots` | Q〜P の 10 slot + viewer A |
| `apps` | 管理対象アプリ (Ghostty, Zed, Vivaldi) の bundleId, lifecycle removal method |
| `daemons` | 管理対象デーモン (tmux server 等) |

#### DesiredWorld

```go
type DesiredWorld struct {
    ActiveProfile   ProfileID
    Profiles        map[ProfileID]Profile
    Projects        map[ProjectID]DesiredProject
    FocusPolicy     FocusPolicySet
    CockpitVisibility CockpitVisibility  // Shown / Hidden
    SystemWindows   []SystemWindow       // cockpit 等
}
```

#### ObservedWorld

```go
type ObservedWorld struct {
    Windows     map[LiveWindowID]ObservedWindow
    Workspaces  map[WorkspaceID]ObservedWorkspace
    Displays    ObservedDisplayState
    Focused     *LiveWindowID
    Tmux        TmuxSnapshot
    Timestamp   time.Time
}
```

#### 識別子

| 識別子 | 用途 | 例 |
|---|---|---|
| `ProfileID` | プロファイル名 | `"work"` |
| `ProjectID` | project 名 | `"dotfiles"` |
| `SlotID` | slot 名 | `"Q"` |
| `WorkspaceID` | OmniWM workspace | `"Q"`, `"A"`, `"CP1"` |
| `DesiredWindowID` | 意図上のウィンドウ | `{Project: "dotfiles", Kind: "ai", Index: 1}` |
| `LiveWindowID` | 実際のウィンドウ | `"window:42"` |
| `DisplayID` | 物理ディスプレイ | `"display:1"` |

#### WindowKind

| Kind | 説明 | アプリ | tmux |
|---|---|---|---|
| `ai` | AI CLI | Ghostty | あり |
| `shell` | 自由シェル | Ghostty | あり |
| `editor` | GUI エディタ | Zed | なし |
| `browser` | ブラウザ | Vivaldi | なし |
| `viewer` | AI の read-only 複製 | Ghostty | あり (grouped) |
| `external` | 管理対象外 | 任意 | — |
| `cockpit` | TUI 操縦席 | Ghostty | あり |
| `scratch` | 一時作業用 shell | Ghostty | あり |

### 7.5 アダプタ契約

> projwm と外部システムの境界を定義するインターフェース。各アダプタは §10 のテスト戦略に従って独立にテストされる。

#### WindowManagerAdapter (OmniWM)

```go
type Adapter interface {
    Observe(ctx context.Context) (ObservedWorld, error)
    Spawn(ctx context.Context, req SpawnRequest) (LiveWindowID, error)
    Close(ctx context.Context, id LiveWindowID) error
    FocusWorkspace(ctx context.Context, ws WorkspaceID) error
    FocusWindow(ctx context.Context, id LiveWindowID) error
    MoveToWorkspace(ctx context.Context, id LiveWindowID, ws WorkspaceID) error
    ReorderColumns(ctx context.Context, ws WorkspaceID, cols [][]LiveWindowID) error
    SpawnCockpit(ctx context.Context, displayIdx int, title string) error
    ShowCockpitOnDisplay(ctx context.Context, display DisplayID, parkWS string) error
    HideCockpitOnDisplay(ctx context.Context, display DisplayID, priorWS string) error
    MoveCockpitToParkWorkspace(ctx context.Context, id LiveWindowID, parkWS string) error
    ShowScratchShell(ctx context.Context) (LiveWindowID, error)
    HideScratchShell(ctx context.Context, priorWindow LiveWindowID) error
}
```

| メソッド | テスト | 備考 |
|---|---|---|
| `Observe` | ○ `TestObserveWorld` | 全 window/workspace/display/focus を query |
| `Spawn` | S1-S11 (L3), S12-S13 (L2) | kind に応じて Ghostty/Zed/Vivaldi/cockpit を起動。settle timeout / process-alive fallback 分岐は L2 deterministic harness |
| `Close` / lifecycle removal | C1/C4/C5 (L3), C2/C3 (L2) | raw close-window は production では原則 blocked。managed lifecycle removal は manifest の `lifecycleRemoval.method` に従い、primary close surface 優先、不可なら method 別 fallback。cockpit は close-block 対象外 |
| `FocusWorkspace` | F1-F2 | workspace 切替 |
| `FocusWindow` | F3-F4 (L3), F5 (L2) | 実 focus 結果は L3、navigate → focus の command-order contract は L2 |
| `MoveToWorkspace` | M1-M2 (L3), M3-M5 (L2) | 実移動結果は L3、3 回 retry / re-verify focus / vanished 分岐は L2 deterministic harness |
| `ReorderColumns` | R1-R4 | 最大 3 pass、各 window を move-column left で配置 |
| `SpawnCockpit` | S10-S11 | park workspace に cockpit Ghostty を配置 |
| `ShowCockpitOnDisplay` | — | display の active workspace を park workspace に切替 |
| `HideCockpitOnDisplay` | — | display の active workspace を元に戻す |
| `MoveCockpitToParkWorkspace` | — | cockpit window を強制移動（invariant 違反時） |
| `ShowScratchShell` / `HideScratchShell` | U1 | scratch shell を表示して focus / 非表示にして prior window へ focus 復帰 |

#### SessionCapabilityAdapter (tmux)

```go
type SessionAdapter interface {
    HasSession(ctx context.Context, name string) (bool, error)
    EnsureSession(ctx context.Context, name, cwd string) (created bool, err error)
    EnsureGroupedSession(ctx context.Context, base, clone string) error
    KillSession(ctx context.Context, name string) error
    SendKeys(ctx context.Context, session string, keys ...string) error
}
```

| メソッド | L3 テスト | 備考 |
|---|---|---|
| `HasSession` | T1-T2 | tmux session の存在確認 |
| `EnsureSession` | T1-T2 | 無ければ `tmux new-session -d -s name` で作成。幂等 |
| `EnsureGroupedSession` | T3 | `tmux new-session -d -t base -s clone` |
| `KillSession` | T4 | `tmux kill-session -t name` |

#### EditorCapabilityAdapter (Zed)

```go
type EditorAdapter interface {
    LaunchProject(ctx context.Context, projectPath string, extraArgs []string) error
    CollectCloseObservation(ctx context.Context, params CloseParams) (CloseObservation, error)
    CloseLiveWindow(ctx context.Context, id LiveWindowID) error
}
```

| メソッド | L3 テスト | 備考 |
|---|---|---|
| `LaunchProject` | S3-S5 | `zed -n --user-data-dir <path>`。`-n` 必須 |
| `CollectCloseObservation` | — | close 前後のプロジェクト情報、unsaved changes 収集 |
| `CloseLiveWindow` | — | lifecycle removal: project-scoped-app |

#### BrowserCapabilityAdapter (Vivaldi)

```go
type BrowserAdapter interface {
    OpenURL(ctx context.Context, url string, profile string) error
    CollectCloseObservation(ctx context.Context, params CloseParams) (CloseObservation, error)
    CloseLiveWindow(ctx context.Context, id LiveWindowID) error
}
```

| メソッド | L3 テスト | 備考 |
|---|---|---|
| `OpenURL` | S6-S7 | Vivaldi automation profile で URL を開く |
| `CollectCloseObservation` | — | close 前後の browser profile、payload 照合を収集 |
| `CloseLiveWindow` | — | lifecycle removal: browser-window-close |

**アダプタのテスト方針**: 各アダプタメソッドは §10.3 の L2 (mock executor / deterministic harness) と §10.4 の L3 (実操作単体) に分けてテストする。L2 では retry 回数、タイムアウト分岐、エラーハンドリング、fallback 呼び出し順、command order を検証する。L3 では実際のアプリ / OmniWM / tmux と疎通し、外部から観測できる最終状態だけを検証する。mock/unit body を L3 実操作の証拠として扱ってはいけない。

---

## 8. 状態管理

### 8.1 PersistentStore

generation-based の不変ストア。MemoryStore (テスト) と FileStore (本番)。

- 各コミットで generation ディレクトリが増える
- atomic rename で crash safety を保証
- DesiredWorld、AcceptedLayouts、BrowserSnapshots、ControllerCheckpoint を保存
- ObservedWorld は保存しない（起動時に observer で再構成する）

### 8.2 算出フィールド

title / tmux session 名 / viewer 窓は state に保存せず、`naming.Resolve()` で算出。これによりリネーム時の不整合を構造的に防止。

### 8.3 排他制御

- 全書き込みは `flock(2)` で排他
- 書き込みは tmpfile + atomic rename
- 読み込みは lock 不要

---

## 9. 受入仕様

### 9.1 受入シナリオ

| シナリオ | 内容 |
|---|---|
| S1 | SwitchProfile: 旧ウィンドウ close、新ウィンドウ summon |
| S2 | ArchiveProject: ウィンドウ close + tmux kill |
| S3 | UnarchiveProject: park 状態に復帰 |
| S4 | Assign/Unassign: slot 割当と解除 |
| S5 | Reconcile: 差分修正 |
| S6 | macOS 再起動後: 全自動復帰 |
| S7 | OmniWM 再起動後: 窓の再作成 |
| S8 | summon の冪等性: 何回呼んでも同じ結果 |
| S9 | ドリフト修正: ウィンドウが slot の外に出た場合の自動復帰 |
| S10 | 障害復帰: tmux/Ghostty/Zed クラッシュ後の自動復帰 |

### 9.2 完了定義

1. 全受入シナリオが real E2E (Human-operation) でパス
2. 全不変条件 (§3.4) が invariant checker で検証
3. 1 分以内の自動復帰が保証
4. プロファイル切替が 5 秒以内
5. 個別操作が独立テスト可能

---

## 10. テスト戦略

> どのレイヤーで何を検証し、どうやって「正しさ」を保証するか。

### 10.1 テストレイヤー

| レイヤー | 実行速度 | 依存 | 検証対象 | 開発中に常時実行 |
|---|---|---|---|---|
| **L0: 純粋関数** | ミリ秒 | なし | reducer, planner, verifier, naming, identity の入出力 | ○ `go test ./...` |
| **L1: fake 操作** | ミリ秒 | `wm.Fake` | 状態遷移、不変条件、transaction loop 全体 | ○ `go test ./...` |
| **L2: mock executor** | ミリ秒 | `MockCtlExecutor` | adapter の retry ロジック、タイムアウト、エラーハンドリング | ○ `go test ./...` |
| **L3: 実操作単体** | 数秒 | OmniWM 実機 | 各操作の実機での動作（1操作ずつ） | ○ `go test -tags real_ops ./...` |
| **L4: 実 E2E** | 数分 | OmniWM + tmux + Ghostty + Zed | 複合シナリオ、障害復帰 | △ マイルストーン時のみ |

### 10.2 L3 実操作単体テストの設計

**基本原則**: L2/L3 で単一操作 contract が独立に検証できれば、L4 は「操作の組合せ」として保証される。

L3 は「実アプリ / 実 OmniWM / 実 tmux に対して操作を実行し、その結果を外部 observe で確認できるもの」だけを扱う。mock executor、fake `osascript`、stubbed process checker、事前に作った query sequence でしか発生させられない failure branch は L3 ではなく L2 deterministic harness で検証する。

L3 の owner として認められる条件:
- 実際の外部プロセスまたは実 OmniWM 状態を setup する
- production operation を 1 つだけ execute する
- execute 後に OmniWM / tmux / OS process 等を observe する
- observe した最終状態を assert する
- 作成した window/session/process を cleanup する

L3 の owner として認めないもの:
- mock executor の call sequence だけを見るテスト
- unit test への単純委譲
- coverage map / ledger / function-existence gate
- fake command shim で branch 到達だけを見るテスト
- `PROJWM_REAL_OP_TESTS=1` で gated されているだけで、実体が mock/unit のテスト

**テスト構造**:

```
テストごとの流れ:
  1. setup: 前提条件を整える（workspace を空にする、tmux session を用意する等）
  2. execute: 1つの操作だけを実行
  3. observe: OmniWM に問い合わせて結果を取得
  4. assert: 結果が期待通りか検証
  5. cleanup: 後片付け（作成したウィンドウを閉じる、session を kill）
```

**実機要件**:
- テストは `//go:build real_ops` タグでビルド
- 環境変数 `PROJWM_REAL_OP_TESTS=1` で明示的に有効化
- 必要なアプリ（Ghostty, Zed, tmux, omniwmctl）がインストールされていること
- テストは直列実行（`-parallel 1`）。相互干渉を防止

**テスト用 workspace**:
- テストには管理外 workspace（8, 9 等）を使う
- 通常の managed slot（Q-P）は使わない
- これにより開発中の実際の作業環境を汚さない

**cleanup の保証**:
- 各テストの defer で必ず cleanup を実行
- テストが panic しても cleanup が走るように `t.Cleanup()` を使用
- cleanup: 作成した全 Ghostty window を close、全 tmux session を kill

**Nix 不要の開発フロー**:

```bash
# L0-L2: 常時実行（数秒）
cd modules/darwin/projwm/projwm-next
go test ./... -count=1

# L3: 実機テスト（1-2分、OmniWM 稼働中のみ）
PROJWM_REAL_OP_TESTS=1 go test -tags real_ops ./... -count=1 -parallel 1

# 単一操作のテスト
PROJWM_REAL_OP_TESTS=1 go test -tags real_ops ./internal/adapter/wm/ -run TestMoveToWorkspace -count=1

# 実機で projwmd を手動起動して確認（Nix 不要）
go build ./cmd/projwmd && ./projwmd --managed-environment testdata/manifest.json
```

### 10.3 L2: mock executor テストの設計

**目的**: sigwm の retry/タイムアウトロジック、failure branch、fallback surface、command order を OmniWM 実機なしでテストする。

L2 は deterministic harness である。実機で失敗条件を安定して作ることが難しい場合、または実 app の挙動に依存して branch が不安定になる場合、その contract は L2 で検証する。L2 が保証するのは「特定の観測/エラー列に対して adapter が正しい判断と呼び出し順を選ぶこと」であり、実 app が最終的に閉じる/移動する/起動することではない。最終状態は L3 の別テストで保証する。

L2 は OmniWM 実機を要求しないため、原則として `PROJWM_REAL_OP_TESTS=1` や `real_ops` build tag に依存させない。production command builder を fake command shim で実行する場合でも、検証対象が branch / command surface だけなら L2 として扱い、L3 real operation coverage には登録しない。

L2 に置くもの:
- retry 回数と retry 停止条件
- timeout 後の fallback 分岐
- process-alive fallback / process-dead error
- focus drift を検出した後の re-verify 手順
- window vanished を検出した後の retry 中止
- lifecycle removal fallback surface の選択
- navigate → focus などの command order

**MockCtlExecutor**:
```go
type MockCtlExecutor struct {
    Responses map[string][]MockResponse  // コマンド文字列 → 応答のキュー
    Calls     []string                    // 呼び出し履歴
}

type MockResponse struct {
    Output []byte
    Err    error
}
```

**テスト例: move の retry**:
```
mock の設定:
  "window navigate id-1":   [{ok}, {ok}, {ok}]
  "window focus id-1":      [{ok}, {ok}, {ok}]
  focused-window query:     ["id-1", "id-1", "id-1"]
  "command move-to-workspace 8": [{err: exit 1}, {err: exit 1}, {ok}]

期待: 3回 retry し、3回目で成功。Calls 履歴で検証。
```

**テスト例: spawn の process-alive fallback**:
```
mock の設定:
  query-windows: 延々と空を返す（settle がタイムアウトする）
  processAlive: true

期待: process-alive fallback が発動し、("", nil) が返る
```

### 10.4 単一操作テスト一覧

> §4.1 の全 17 ユーザー操作と §4.2 の全システム操作を、L0-L4 の適切なレイヤーでカバーする。L3 は実機で観測できる最終状態の証拠であり、failure injection や retry/timeout branch の証拠ではない。

この節の表で `L3` と書かれた行だけが real operation coverage の対象である。`L2` と書かれた行は deterministic harness coverage の対象であり、`real_ops` coverage map に入れてはいけない。

#### spawn 系

| # | Layer | テスト | 検証内容 |
|---|---|---|---|
| S1 | L3 | `TestSpawnShell` | ghostty --title="shell-1:test" -e tmux new-session -A -s shell-1/test。title 一致、tmux session 存在 |
| S2 | L3 | `TestSpawnShellAlreadyExists` | 既存 shell あり。重複して新 window を作らない。focus のみ |
| S3 | L3 | `TestSpawnEditor` | zed -n <path>。title = basename(cwd) |
| S4 | L3 | `TestSpawnEditorEmptyProjectCleanup` | Zed 起動後に "empty project" window が自動 AXClose される |
| S5 | L3 | `TestSpawnEditorAlreadyExists` | 既存 editor に focus のみ |
| S6 | L3 | `TestSpawnBrowser` | Vivaldi automation profile で起動。window 存在確認 |
| S7 | L3 | `TestSpawnBrowserAlreadyExists` | 既存 browser に focus のみ |
| S8 | L3 | `TestSpawnViewer` | tmux grouped session + ghostty --title="ai-view-1:test"。grouped session 存在確認 |
| S9 | L3 | `TestSpawnViewerAlreadyExists` | 既存 viewer に focus のみ |
| S10 | L3 | `TestSpawnCockpit` | cockpit ghostty --title="projwm-cockpit-0"。tmux session "projwm-cockpit" 存在 |
| S11 | L3 | `TestSpawnCockpitAlreadyExists` | 既存 cockpit に focus のみ |
| S12 | L2 | `TestSpawnSettleTimeoutProcessAlive` | settle タイムアウト + process checker が生存 → process-alive fallback 発動、成功扱い |
| S13 | L2 | `TestSpawnSettleTimeoutProcessDead` | settle タイムアウト + process checker が不在 → エラー返却 |

#### move 系

| # | Layer | テスト | 検証内容 |
|---|---|---|---|
| M1 | L3 | `TestMoveToWorkspace` | window を WS 8 → WS 9 に移動。omniwmctl query で workspace=9 確認 |
| M2 | L3 | `TestMoveToWorkspaceAlreadyOnTarget` | 既に正しい WS にいる → 最終状態が変わらない |
| M3 | L2 | `TestMoveToWorkspaceFocusDrift` | focus が別 window に奪われる観測列 → re-verify で検出し retry |
| M4 | L2 | `TestMoveToWorkspaceRetry` | 1-2 回目 move-to-workspace が exit 1 → 3 回目で成功 |
| M5 | L2 | `TestMoveToWorkspaceWindowVanished` | move 中に window 消失の観測列 → retry 継続せずエラー返却 |

#### reorder 系 (4 件)

| # | Layer | テスト | 検証内容 |
|---|---|---|---|
| R1 | L3 | `TestReorderColumns` | 2 カラムの順序を入れ替え → omniwmctl query で新順序確認 |
| R2 | L3 | `TestReorderColumnsAlreadyCorrect` | 既に正しい順序 → 最終状態が変わらない |
| R3 | L3 | `TestReorderColumnsPartialMatch` | 3 カラム中 2 カラム一致・1 カラム不一致 → 必要な列だけ修復される |
| R4 | L3 | `TestReorderColumnsEmptyWorkspace` | 空 workspace → noop |

#### close / lifecycle removal 系 (5 件)

| # | Layer | テスト | 検証内容 |
|---|---|---|---|
| C1 | L3 | `TestLifecycleRemovalPrimaryCloseSurfaces` | `lifecycleRemoval.method` ごとの実 app close が target window を消失させる → post-observe / wait-gone 確認 |
| C2 | L2 | `TestLifecycleRemovalFallbackCloseSurface` | primary close surface 不可/失敗 → `lifecycleRemoval.method` ごとの fallback close surface が呼ばれることを deterministic command shim で確認 |
| C3 | L2 | `TestCloseWindowRetry` | 1 回目 close 後も window 残存 → 2 回目 close + wait-gone。実 app で deterministic に「close したが残る」状態を作る fixture は不安定なため、retry / wait-gone contract は deterministic harness で検証する |
| C4 | L3 | `TestCloseWindowAlreadyGone` | `lifecycleRemoval.method` ごとに、既に target window 無し → noop |
| C5 | L3 | `TestCloseCockpit` | 実 cockpit window を spawn → raw Close 例外として close → post-observe で消失確認 |

C1 は実 app / OmniWM / post-observe を使う L3 final-disappearance test である。`ax-close-guarded` / `project-scoped-app` / `browser-window-close` の各 method は C1 内の subtest として必ず単体検証する。

C2 は Cmd+W そのものを保証するテストではない。保証するのは、primary close surface が使えない場合でも、対象 app の `lifecycleRemoval.method` に対応した fallback close surface に進むことである。実 app で primary failure を deterministic に作る fixture は macOS/アプリ実装に依存して不安定なので、C2 は production closer を fake `osascript` command shim で実行し、primary failure / unavailable response から fallback invocation へ進むことを検証する。fallback 後に exact target が消える最終結果は C1 の実 app final-disappearance test が保証する。

| `lifecycleRemoval.method` | primary close surface | fallback close surface |
|---|---|---|
| `ax-close-guarded` | AXCloseButton / AXPress | Cmd+W |
| `project-scoped-app` | app-specific AX close button | Cmd+Shift+W 等、window 全体を閉じる app-specific shortcut |
| `browser-window-close` | browser/app dictionary close window | Cmd+Shift+W 等、tab ではなく window 全体を閉じる app-specific shortcut |

C2 の合格条件:
1. pre-observe で target を一意に特定する
2. primary close surface を試す
3. primary が不可/失敗した場合だけ fallback を使う
4. fallback は app ごとに window 全体を閉じる手段を選ぶ
5. `project-scoped-app` / `browser-window-close` では Cmd+W ではなく Cmd+Shift+W 等の whole-window close を使う
6. app quit、別 window close、tab/pane だけ close につながる surface は不合格

#### scratch shell 系 (1 件)

| # | Layer | テスト | 検証内容 |
|---|---|---|---|
| U1 | L3 | `TestScratchShellShowHideRestoresPriorFocus` | 実 Ghostty/tmux/OmniWM 上で scratch shell を表示 → focused window が `projwm-scratch-shell` になる。再表示しても重複しない。非表示 → 表示前の focused window に戻る |

#### focus 系 (5 件)

| # | Layer | テスト | 検証内容 |
|---|---|---|---|
| F1 | L3 | `TestFocusWorkspace` | workspace 8 → 9 に切替。omniwmctl query で active workspace=9 確認 |
| F2 | L3 | `TestFocusWorkspaceNonExistent` | 存在しない workspace → エラー |
| F3 | L3 | `TestFocusWindow` | window に focus。omniwmctl query で focused window 一致確認 |
| F4 | L3 | `TestFocusWindowVanished` | focus 対象の window が消失 → エラー |
| F5 | L2 | `TestFocusWindowNavigationBeforeFocus` | navigate → focus の command order を mock executor call sequence で確認。実 focus 結果は F3 が保証する |

#### identity 復元 (3 件)

| # | Layer | テスト | 検証内容 |
|---|---|---|---|
| I1 | L3 | `TestIdentityFromTitle` | 実 window title "shell-1:dotfiles" を observe → naming.Parse() → (project="dotfiles", kind="shell", id=1) |
| I2 | L3 | `TestIdentityFromTitleViewer` | 実 window title "ai-view-1:dotfiles" を observe → (project="dotfiles", kind="viewer", id=1) |
| I3 | L3 | `TestIdentityFromTitleUnknown` | 実 window title "random-window" を observe → 復元不可。orphan 扱い |

#### tmux session 管理 (4 件)

| # | Layer | テスト | 検証内容 |
|---|---|---|---|
| T1 | L3 | `TestTmuxEnsureSession` | session が無ければ `tmux new-session -d -s name` で作成 |
| T2 | L3 | `TestTmuxEnsureSessionAlreadyExists` | session 既存 → 作成せず noop |
| T3 | L3 | `TestTmuxEnsureGroupedSession` | `tmux new-session -d -t base -s clone` で grouped session 作成 |
| T4 | L3 | `TestTmuxKillSession` | `tmux kill-session -t name` で session 削除 |

#### 起動時復帰 (4 件)

| # | Layer | テスト | 検証内容 |
|---|---|---|---|
| B1 | L3 | `TestStartupNormal` | PersistentStore と実状態が一致 → 何もしない |
| B2 | L3 | `TestStartupMissingWindow` | state にあり実際に無し → spawn で再作成 |
| B3 | L3 | `TestStartupOrphanWindow` | 実際にあり state に無し → title から identity 復元試行。復元成功 → 再登録。失敗 → orphan 通知 |
| B4 | L3 | `TestStartupStateCorrupted` | state.json 破損 → state.json.bak から復旧。bak も無し → 実際の window から state 再構築 |

#### 合計: 43 件

- L3 real operation: 35 件
- L2 deterministic harness: 8 件

### 10.5 L4: 実 E2E の役割

L2/L3 で単一操作 contract が独立検証された後、L4 で検証するのは：

| 検証項目 | 例 |
|---|---|
| 操作間の遷移 | observe-barrier 後の状態が正しいか |
| 複合操作 | profile switch, archive が正しく完了するか |
| 障害復帰 | macOS 再起動後、OmniWM 再起動後の自動復帰 |
| 不変条件の維持 | 全シナリオ完了後に §3.4 の不変条件が成立するか |

L4 の実行条件:
- `PROJWM_NEXT_REAL_ACCEPTANCE=1` 環境変数
- 全アプリ（Ghostty, Zed, Vivaldi, tmux, omniwmctl）が利用可能
- マイルストーン完了時にのみ実行。日常開発では L0-L3 で十分

### 10.6 テストカバレッジと §4.1 操作の対応

| §4.1 操作 | L0 | L1 | L2 | L3 | L4 |
|---|---|---|---|---|---|
| 操作1-3: shell/editor/browser jump | — | ○ | ○ | ○ | ○ |
| 操作4: project 切替 | — | ○ | — | — | ○ |
| 操作5: 同 slot 内 window 切替 | — | ○ | ○ | — | ○ |
| 操作6-7: viewer/cockpit | — | ○ | — | ○ | ○ |
| 操作8: profile 切替 | ○ | ○ | — | — | ○ |
| 操作9-10: project 追加/アーカイブ | ○ | ○ | — | — | ○ |
| 操作11: scratch shell | — | ○ | — | ○ | ○ |
| 操作12-13: add/remove window | ○ | ○ | ○ | ○ | ○ |
| 操作14-17: browser tab | — | ○ | — | — | ○ |
| §4.2: システム操作全般 | — | ○ | ○ | ○ | ○ |

この表の L3 は「実機で観測できる最終状態」を要求するという意味である。§10.4 で L2 と分類された retry/timeout/fallback/command-order branch は、§4.1/§4.2 の contract の一部であっても L3 real operation coverage には数えない。

### 10.7 テスト用 manifest

L3/L4 テスト用の最小 manifest (`testdata/manifest.json`):

```json
{
  "schemaVersion": 1,
  "authority": "nix",
  "windowManager": { "backend": "omniwm" },
  "workspaces": [
    { "id": "8", "rawName": "8", "role": "general" },
    { "id": "9", "rawName": "9", "role": "general" }
  ],
  "slots": [],
  "apps": [
    { "bundleId": "com.mitchellh.ghostty", "capability": "terminal",
      "lifecycleRemoval": { "method": "ax-close-guarded", "allowed": true, "allowedKinds": ["ai", "shell", "viewer"] } },
    { "bundleId": "dev.zed.Zed", "capability": "editor",
      "lifecycleRemoval": { "method": "project-scoped-app", "allowed": true, "allowedKinds": ["editor"] } },
    { "bundleId": "com.vivaldi.Vivaldi", "capability": "browser",
      "lifecycleRemoval": { "method": "browser-window-close", "allowed": true, "allowedKinds": ["browser"] } }
  ],
  "daemons": {}
}
```

### 10.8 実装フロー

> §10.4 の単一操作テストが「仕様」。実装はテストを pass させる手段。L3 は実機で観測できる最終状態、L2 は deterministic に注入した branch contract を担当する。

#### 原則

```
1. テストを書く（期待する挙動を宣言。実環境の知識は不要）
2. 実装する（この段階で初めて実環境と向き合う）
3. 検証する（元々書いたテストで正しさを確認）
```

テストは実環境に関係なく期待する contract を先に書ける。例えば L3 `TestMoveToWorkspace` は「window が target workspace に移動すること」を期待する。L2 `TestMoveToWorkspaceRetry` は「特定の失敗列に対して 3 回 retry すること」を期待する。OmniWM の癖を知らなくても、どの contract を L2/L3 のどちらで検証するかは先に固定する。
実装のときに初めて実環境と向き合い、観察し、retry/タイムアウト/fallback の具体値を調整する。ただし、実機で deterministic に作れない failure branch を L3 の証拠として扱ってはいけない。

#### 実装中の観察

実装時、以下の方法で実環境の挙動を理解する：

- `omniwmctl query windows/workspaces` で状態を直接確認
- `time` で各サブステップの所要時間を計測
- 意図的にエラー（focus を奪う、window を消す等）を起こして反応を見る
- 発見した知見はコードのコメントに残す（「いつ・なぜ・何を発見したか」）

#### 実行環境の分離

テスト中に本番環境を壊さないために、以下を分離する：

| リソース | 本番 | テスト |
|---|---|---|
| PersistentStore dir | `~/.local/state/projwm-next/store/` | `~/.local/state/projwm-next-test/store/` |
| socket path | `projwmd.sock` | `projwm-next-test/projwmd.sock` |
| log dir | `~/.local/state/projwm-next/logs/` | `~/.local/state/projwm-next-test/logs/` または stderr |
| manifest | Nix 生成（全 slot 含む） | テスト用最小 manifest（§10.7） |
| tmux session | `ai-N/project` 等 | `test-ai-N/project` 等、prefix で識別 |
| Ghostty title | `ai-N:project` 等 | `test-ai-N:project` 等 |
| workspace | Q-P, A（完成後に使用開始） | Q-P は未使用のためテスト可能 |

#### 本番のデフォルト停止

**開発中は本番環境を一切動かさない。** 本番 launchd agent を起動するのは L4 E2E テストの実行時だけ。

理由:
- 開発中の projwmd は未完成。本番環境で誤動作すると実作業を壊す
- テスト用 projwmd と本番 projwmd が競合すると OmniWM の状態が破綻する
- 開発中の調査で workspace や window が予期せず操作されるのを防ぐ

```bash
# 開発開始時: 本番を確実に停止
launchctl bootout gui/$(id -u)/org.nixos.projwmd-next 2>/dev/null

# L4 E2E のときだけ本番を起動
# （L0-L3 の開発中は起動しない）
```

#### launchd の停止

テスト中は本番の launchd agent を停止する（複数 projwmd が OmniWM を操作すると相互に drift と誤判定し無限修正ループになるため）：

```bash
# テスト環境に入る前: 本番を停止
launchctl bootout gui/$(id -u)/org.nixos.projwmd-next

# テスト終了後: 本番を再開（必要なら）
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/org.nixos.projwmd-next.plist
```

#### テスト用 projwmd の起動

```bash
projwmd \
  --managed-environment testdata/manifest.json \
  --socket-path ~/.local/state/projwm-next-test/projwmd.sock \
  --store-dir ~/.local/state/projwm-next-test/store
```

#### テスト間の分離

- 各テストの `t.Cleanup()` で後片付け（作成した window を close、tmux session を kill）
- テスト開始時に title が `test-*` の既存 window を全 close（前回の残骸掃除）
- テスト用 tmux session は prefix `test-` で一括識別・kill 可能

#### 探索用コマンド（実装中に随時使う）

```bash
# omniwmctl の生の挙動を確認
omniwmctl query windows
omniwmctl query workspaces
omniwmctl window focus <id>
omniwmctl command move-to-workspace 9

# Ghostty / Zed / Vivaldi の title 挙動を確認
ghostty --title="test-title" -e tmux new-session -A -s test-session &
zed -n /tmp/test-project &
# Vivaldi は automation profile で起動

# L3 テストを 1 件だけ実行
PROJWM_REAL_OP_TESTS=1 go test -tags real_ops -run TestSpawnShell -count=1 ./internal/adapter/wm/

# projwmd を手動起動して確認
go build ./cmd/projwmd && ./projwmd --managed-environment testdata/manifest.json
```

### 10.9 現在のテスト保証境界

> **重要**: この節は 2026-05-23 時点の作業メモではなく、次の実装者がテスト結果を誤読しないための SSOT 上の境界定義である。ここに列挙された項目は、現行テスト群がすべて green になっても「仕様達成」と宣言してはいけない。

#### green の意味

現行テストは §4.1 の 17 操作、§7.3 命名規約、§9.1 L4 acceptance matrix、§10.4 単一操作テスト一覧、§10.6 coverage/ledger を中心に executable spec 化している。したがって以下がすべて green になった場合に宣言できるのは、**ledger/matrix に登録された範囲を満たした**ということだけである。

| 実行方法 | green の意味 | green でも宣言できないこと |
|---|---|---|
| `go test ./...` | L0-L2 と通常単体テストの範囲で、純粋関数・fake・mock contract が成立 | L3/L4 実機操作、実アプリ挙動、実 E2E 完了 |
| `PROJWM_REAL_OP_TESTS=1 go test -tags real_ops ./...` | L3 real operation として登録された単一操作が実機で成立 | §5 UI 全体、§8 crash safety、§9.2 時間要件、L4 複合運用 |
| `PROJWM_NEXT_REAL_ACCEPTANCE=1 go test -tags integration ./scenarios` | §9.1 の登録済み L4 scenario が実環境で成立 | SSOT 全文の全要求文、UI 細部、運用/分離/性能の全条件 |
| `go test ./internal/ssottest` | layer matrix / ledger / coverage map / test function reference が整合 | 参照先テストが実際にアプリ挙動を満たすこと |

#### 現在 executable spec として扱える範囲

次の領域は現行テストが仕様検証として意図的に設計されている。ここに属するテストが green になれば、その範囲については仕様を満たしたと扱ってよい。

| SSOT 領域 | 現在の扱い |
|---|---|
| §4.1 操作1-17 の主要状態遷移 | `internal/ssottest/ledger_test.go` と `layer_matrix_test.go` が owner と layer を管理 |
| §7.3 命名規約 | `naming` / `identity` / `semop` 系 SSOT テストで `kind-id:project`、1 始まり ID、tmux session 名を検証 |
| §9.1 L4 受入シナリオ S1-S10 | `scenarios/ssot_l4_acceptance_*` が wrapper と real owner を管理 |
| §10.4 単一操作テスト一覧 | L2/L3 の coverage map と real_ops owner で管理 |
| C2 lifecycle fallback | L2 deterministic harness として primary failure → app 別 fallback surface を検証 |
| OP-07 cockpit show/hide | prior workspace/window 復帰を reducer/planner/L3 real op で検証対象化 |
| OP-11 scratch shell | `show-scratch-shell` / `hide-scratch-shell` intent と L3/L4 test owner を定義 |
| §10.7 テスト用 manifest | top-level `workspaces` / `slots` / `apps` shape を test fixture と L0 manifest test で検証対象化 |

#### green でも未保証の領域

以下は現時点では **未完成 / 未保証** として扱う。これらはテストが全 green になっても、追加の SSOT test ID と owner が割り当てられるまで仕様達成を宣言してはいけない。

| ID | 未保証領域 | SSOT 要件 | 足りないテスト |
|---|---|---|---|
| GAP-01 | duplicate window の正本選択 | §2.5: 複数ある場合、最も recently focused なものを正とし、他は orphan、cockpit に `[INVARIANT]` card | recently focused selection、余剰 window の orphan/card 化 |
| ~~GAP-02~~ 解消 | 状態一覧のユーザー可視表示 | §3.1: 初期/正常/ドリフト/復旧中/部分障害/profile 切替中/cockpit/エラー時にユーザーが見る表示 | **解消**: ledger `STATE-DISPLAY` が owner。status が profile/slot/parked/archived (正常/初期) 表示、convergence 語彙が 正常(CONVERGED)/復旧中(CONVERGING) を区別、cockpit tab は TUI-SNAPSHOT、error 状態は [INVARIANT]/[CLOSED]/[MOVED] card 群で可視化。 |
| ~~GAP-03~~ 解消 | 状態ごとの操作可否 | §3.3: 初期では summon/profile/archive 不可、復旧中は待機など | **解消**: ledger `STATE-OP-MATRIX` が owner (`TestSSOTState33_*` が unknown project/profile・active profile 不在での reject を検証)。「復旧中は待機」は wmMutationLock 直列化 = `SINGLE-WRITER`。 |
| ~~GAP-04~~ 解消 | 手動 drift の通知と grace period | §4.3: drift は自動修正、事後カード通知、60秒以内2回で grace period | **解消**: ledger `DRIFT-NOTIFY` が owner。[CLOSED]/[MOVED] card 発行、grace (2 closes/60s) で rateLimited warning card + user-close-suppress scope (reducer)、planner T4.4 が respawn 停止。 |
| ~~GAP-05~~ 解消 | orphan 提案 UI | §4.3: orphan card の `[Enter]` 登録、`[c]` close、`[t]` 詳細操作 | **解消**: ledger `ORPHAN-ACTION` が owner。card は Enter/c/t action 保持、AdoptOrphanWindow は desired window 追加 (不明 project は reject)、DismissOrphanWindow は desired 不変 (close は executor)。 |
| ~~GAP-06~~ 解消 | AI spawn の詳細 | §4.4 ai | **解消**: 実機 `TestRealOpsSpawnAISendsAICommand` (send-keys)、`TestSpawnShellAlreadyExists` (attach-only 冪等)、OP-12 add-ai 自動採番 (multi-AI parity)。|
| ~~GAP-07~~ 解消 | Zed spawn の詳細 | §4.4 editor | **解消**: ledger `ZED-CONFIG` (`-n`+`--user-data-dir`+restore_on_startup=none/extension 分離) + 実機 `TestSpawnEditorEmptyProjectCleanup` (empty-project 保護)。|
| ~~GAP-08~~ 解消 | Vivaldi profile isolation | §4.4 browser | **解消**: `TestClassifyLiveWindow_VivaldiAutomationProfile`/`_VivaldiUserProfile_External` が automation=管理 / user profile=External 分類を検証。|
| ~~GAP-09~~ 解消 | browser tab 自動観測 | §4.4 browser | **解消**: ledger `BR-TAB-OBS` が owner (BrowserTabsSync が per-window tab を poll/diff し Browser*Tab intent 発行)。|
| ~~GAP-10~~ 解消 | browser 復元と privacy 完全性 | §4.4 browser | **解消**: ledger `OP-14/15/16` + `PRIV-ORPHAN-GC`。DesiredWorld は raw URL field を持たず opaque `URLPayloadRefs`+`URLCount`+`RedactionPolicyID` のみ (構造的に log/status へ URL 漏れ無し)、controller が URL→token を reducer 前に rewrite、**deleted(purge) ref は GC**(archive は復帰可能なので保持。GC trigger は `DeleteProject{Purge}` であって archive ではない — §1.2 復帰 / line214 削除≠アーカイブ / §4.4 line913 復元)。|
| GAP-11 | Cockpit TUI 全体構造 | §5.4: topbar、Slots/Cards/Archived/Profiles/Trace、footer actions | 全タブ・全領域の SSOT snapshot / interaction tests |
| ~~GAP-12~~ 解消 | Cockpit wizard / palette / modes | §5.4: wizard、Ctrl-P、Proposal/Navigation/Management mode | **解消**: ledger `COCKPIT-MODES` が owner (wizard prompt/field/submit, Ctrl-P palette open/filter/run, ProposalMode 入口+復帰, context bottom menu)。|
| GAP-13 | status / doctor 完全出力 | §5.6: status 全項目、doctor PASS/WARN/FAIL 全検査 | 全項目 presence と failure classification |
| ~~GAP-14~~ 解消 | CLI 全コマンド | §5.7: profile create/delete、doctor、trace、tui、browser tab CLI 等 | **解消**: ledger `CLI-EFFECT` が owner。全 writable command が args→正しい intent を fake daemon 経由 run() で検証 (archive/profile/reconcile/add-*/remove/browser tab)。status/doctor は cmd_status_test、intent 効果は reducer 層。|
| ~~GAP-15~~ 解消 | 状態階層の優先順位 | §6.3: L1 > L2 > L3 の優先度で全階層差分を自動修正 | **解消**: ledger `HIER-PRIORITY` (`TestPlanHierarchyL1BeforeL2BeforeL3` / `TestPlanHierarchyDefersOrderingUntilIdentityResolved`) が owner。複合 drift で spawn<move<reorder、識別未解決時は reorder defer。 |
| ~~GAP-16~~ 解消 | single writer / mutation lock | §6.5: WM 変更は projwmd のみ、IPC intent は daemon 集約、`wmMutationLock` で直列化 | **解消**: ledger `SINGLE-WRITER` が owner。`TestTransactionContractS8A_SingleWriter` が並行 6 intent で max-in-flight mutation≤1 を検証、`S8E` が external event は DesiredWorld を変えないことを検証。 |
| ~~GAP-17~~ 解消 | graceful degradation | §6.8: 1 window spawn 失敗でも他を継続、次 iteration で replan、cockpit 表示 | **解消**: ledger `GRACEFUL-DEGRADE` が owner。per-window spawn 失敗は transaction を abort せず degraded [INVARIANT] card を出して残り op を続行、未達 window は replan 経路へ (永続失敗は §7.1 max-replans に落ち健全分は残存)。removal/layout 失敗は §6.10 順序依存ゆえ hard-abort 維持。 |
| ~~GAP-18~~ 解消 | operation order 全体 | §6.10: close → observe-barrier → spawn、spawn → settle → verify、archive の順序 | **解消**: planner phase order は ledger `ORDER-PHASE` (`TestPlanPhaseOrderRemovalBarrierSpawnBarrierLayout`) が owner。archive 順序は scenarios、settle→verify は transaction contract。 |
| GAP-19 | max replans 超過時の全挙動 | §7.1: fail、rollback、`[INVARIANT]` card、次 intent/event で再挑戦、dirty scope 記録 | rollback/card/dirty scope/next retry の統合テスト |
| GAP-20 | `MoveCockpitToParkWorkspace` | §7.5: cockpit window を強制移動 | adapter method の L2/L3 owner |
| ~~GAP-21~~ 解消 | Zed/Vivaldi close observation | §7.5 | **解消**: `internal/adapter/zed/zed_test.go` + `internal/adapter/browser/vivaldi_test.go` が `CollectCloseObservation` の証拠収集を検証。|
| GAP-22 | PersistentStore 完全性 | §8.1: generation 増加、atomic rename、DesiredWorld/AcceptedLayouts/BrowserSnapshots/ControllerCheckpoint 保存、ObservedWorld 非保存 | artifact presence / no ObservedWorld / crash-safe generation |
| GAP-23 | 排他制御 | §8.3: 全書き込み flock、tmpfile + atomic rename、読み込み lock 不要 | concurrent writer / interrupted write / reader during write |
| GAP-24 | 完了定義の時間保証 | §9.2: 1分以内復帰、profile 切替5秒以内 | 実 E2E timing assertions |
| ~~GAP-25~~ 解消 | L3/L4 実行条件の強制 | §10.2/§10.5: real_ops / integration は実アプリと env flag が必要 | **解消**: ledger `GATE-ENFORCE` が owner。L3 owner は real_ops tag・L4 は integration tag 必須、L2 は不可、を meta-audit が build tag 解析で強制。|
| GAP-26 | テスト環境分離 | §10.8: store/socket/log/manifest/tmux/title/workspace を本番と分離 | 全 real/integration test の prefix/path/workspace meta-audit |

#### 次の作業者への扱い

1. 既存の `ledger_test.go` / `layer_matrix_test.go` に載っている項目は、実装を green にする対象である。
2. 上記 GAP-* は、単に実装を green にするのではなく、まず SSOT test ID と owner test を追加してから実装する。
3. GAP-* を解消したら、この節から該当行を削除するのではなく、対応する §10.4 / §10.6 / §9.1 などの正式表へ移動し、traceability を維持する。
4. `go test ./...` が green でも L3/L4 は保証されない。L3/L4 の保証を主張するには、環境変数付きで skip なし実行した結果が必要である。
5. 「実装が未達で red」は許容するが、「テストが何も検証せず green」は不可。placeholder / stub / meta-only test を behavior owner として扱ってはいけない。

---

## 11. 付録

### 付録 A: 参照文書一覧

| 文書 | 状態 |
|---|---|
| `queue/design.md` | 参照資料（構造的詳細のソース） |
| `queue/implementation-design.md` | 参照資料（実装判断のソース） |
| `queue/specs.md` | 参照資料（受入仕様のソース） |
| `queue/projwm-spec.md` | 参照資料 (v12 旧仕様) |
| `queue/projwm-decisions.md` | 参照資料 |
| `queue/projwm-cockpit-requirements.md` | 参照資料 |
| `queue/projwm-cockpit-unified-design.md` | 参照資料 |
| `queue/projwm-cockpit-implementation-design.md` | 参照資料 |
| `queue/projwm-roadmap.md` | 参照資料 |
| `queue/projwm-ux.md` | 参照資料 |
| `queue/projwm-history.md` | 参照資料 |
| `queue/projwm-reconcile-gap-analysis.md` | 参照資料 |
| `queue/archived/projwm-design.md` | 参照資料 (旧版) |
| `queue/archived/projwm-report.md` | 参照資料 (旧版) |

### 付録 B: 既知の問題

| ID | 問題 | 状態 |
|---|---|---|
| B-01 | launchd が無効化中 | 開発中の一時措置 |
| B-02 | legacy projwm ディレクトリが残存 | 削除予定 |
| B-03 | OmniWM self-heal Lv3-Lv4 が未完成 | Lv3 実装済み、Lv4 未着手 |
| B-04 | sudoers.nix が一時的 | Phase 7 で削除予定 |
| B-05 | ~~**browser 識別不能**~~ **解決済 (2026-05-27)** | 実測でプロセスレベルまで究明。3 層の限界が重なる: (1) omniwm window query は Vivaldi の profile/argv field を非公開、(2) Vivaldi window は page title (`<page> - Vivaldi`) を持ち projwm の `browser-N:project` title を載せられない、(3) **Vivaldi は single-process-multi-window (Chromium)**。projwm は `--new-window --profile-directory=projwm-next` で automation window を開く (vivaldi.go:301) が、同一 user-data-dir のため**同一プロセス**で開き、main process の argv に `--profile-directory` が残らない (`ps` 確認: pid の command は `.../MacOS/Vivaldi` のみ)。→ `vivaldiManaged(pid)` の ps-args 検査が automation window を分類できず WindowExternal 扱い → `identity.Resolve(browser-N)` が ClassMissing → planner が spawn-browser を毎 reconcile 再発行し収束しない。**BR-EXIST (L3-S S7) と L4 acceptance 全シナリオ (ideal reconcile が browser を含む) の共通 root**。<br>**実装済 (commit c43ffe6/9f0572c/69b35b9/4197a32)**: (1) identity 層で browser を title でなく kind+bundleID で match (across-workspace で drift も移動可)。(2) **根治**: 管理 Vivaldi を `--user-data-dir=~/.cache/projwm-next/vivaldi-data` で起動 (vivaldi.go OpenInProfile) — **実測で確認**: Chromium は別プロセスを fork し main process argv に `--user-data-dir` を保持 (`ps`: `.../MacOS/Vivaldi --user-data-dir=...`)、user の Vivaldi から完全隔離。(3) vivaldiManaged / killVivaldiAutomationProcesses を user-data-dir leaf 検出に更新。L0-L2 全 green。<br>**残課題 (real-machine timing debug)**: L4 S1 実走では未だ `spawn-browser:Q` 4-replan 非収束。Vivaldi の **fresh --user-data-dir 初回起動 (first-run profile 生成) が遅く** settle/replan 予算内に omniwm 登録 or vivaldiManaged 分類が間に合わない疑い。要 daemon-level 診断 (vivaldiManaged/spawn にログ追加 → build+run+inspect 反復) + 初回起動 pre-warm or browser spawn debounce。placement (Vivaldi が focus display=M に開く→Q へ move) も要確認。multi-browser-per-workspace の per-window 識別は別途 Vivaldi extension/native-messaging。 |
| B-06 | L4 acceptance の test-environment isolation | **B-05 解決後の full L4 実走 (2026-05-27): 10 シナリオ中 4 green (S1 SwitchProfile / S3 Unarchive / S4 Assign / S5 Reconcile、全て browser 含む)**。残 6 の root: (S8) summon-shell が runtime exit 1 (intent/CLI は実装済、daemon-level debug 要)、(S2) unarchive CLI exit 2、(S9) cross-workspace move が user 占有 ws へ適用されず、(S6) startup-provenance の launchd event-source proof が test daemon(非 launchd) で not-observed、(S7) omniwm restart recovery、(S10) tmux/Zed crash fixtures 未実装。うち S2/S9 の external-workspace 誤発火は **user の実 window (Brave/Helium/Zed/Discord/Spotify/cmux) が ws 1/2/3/B/M を占有**しているため = L4 は user window のない専用クリーン環境を要する (ISO-01 と同根)。各々 scenario 固有で、共通 blocker (B-05) は解消済。 |

### 付録 C: 意思決定記録

#### v1 からの引継ぎ (D-1〜D-55)

> 詳細は `queue/projwm-decisions.md` 参照。

#### v2 の新決定

| 決定 | 内容 | 日付 |
|---|---|---|
| N-01 | transaction loop (observe→plan→execute→observe→replan) を採用 | 2026-05 |
| N-02 | single-writer ルール (projwmd のみが mutation) | 2026-05 |
| N-03 | ManagedEnvironment は Nix が author、projwmd が読み込み | 2026-05 |
| N-04 | adapter インターフェースで WM 操作を抽象化 (fake adapter でテスト) | 2026-05 |
| N-05 | process-alive fallback を導入 (2026-05-19 の unarchive バグ対応) | 2026-05-19 |
| N-06 | ToggleCockpit を summon 固定に変更（トグル廃止） | 2026-05-20 |
| N-07 | TitlePrefixOwned → TitleControllerOwned に変更 | 2026-05-19 |
| N-08 | メンタルモデル: 「特別 + summon + transaction loop」を採用 | 2026-05-20 |
| N-09 | cockpit を park workspace モデルに変更 (CP1 のみ) | 2026-05 |
| N-10 | 「managed workspace」を「slot」に統一。slot は物理的境界ではない | 2026-05-20 |
| N-11 | ユーザー操作は state 変更と summon のみ。move は transaction loop の専権 | 2026-05-20 |
| N-12 | ManualLayoutCandidate / AcceptManualLayout は廃止。Tier 2 自動上書きに変更 | 2026-05-20 |
| N-13 | 状態変更を 4-tier モデルで分類 (構造/配置/内部/強制復元) | 2026-05-20 |
| N-14 | cockpit visibility と active workspace の双方向同期 | 2026-05-20 |
| N-15 | §4.3 矛盾解消: 同一-ws reorder を「定常=Tier 2 accept / 復旧=AcceptedLayouts 復元」と **recovery-gate** で区別。旧記述「判断できない以上すべて drift→revert」と §6.3 L3「L3 はシステムに任せる」を整合修正 | 2026-06-08 |
