# projwm ロードマップ

> 未着手の計画 + ユーザ確認待ちの作業 + 残された open question を記述する。
> 確定済みの仕様は `projwm-spec.md`、過去の試行錯誤は `projwm-history.md`。

---

## 0. 凡例

| 状態 | 意味 |
|---|---|
| 🟢 ready | 仕様確定、着手可能 |
| 🟡 design | 要件は固まっているが詳細設計が必要 |
| 🔵 research | 技術的可能性の調査が先 |
| 🟣 hold | ユーザの GO サイン待ち |
| 🔴 blocked | 上流（OmniWM 等）の修正待ち |

---

## 統合テスト基盤 (T01〜T21)

**状態**: 🟡 実装中（テスト記述済、大部分 FAILING）

### 概要

`cmd/layout_integration_test.go`（build tag `integration`）に T01〜T21 の E2E テストを記述済み。実機 macOS + 実 OmniWM 環境が必要。詳細は `projwm-spec.md §9.3` を参照。

### 現状

| テスト | 状態 |
|---|---|
| T09 (status ゼロ差分), T10 (reconcile 冪等), T21 (isolated WS 不変) | ✅ PASSING |
| T01 (profile switch 理想配置) | ❌ FAILING — OI-9 バグ群 |
| T11a-c (kill → re-spawn + column restore) | ❌ FR-34 未実装 |
| T12 (remove → 永続削除) | ❌ FR-34 実装必要 |
| T13-T17 (システム操作耐性) | ❌ 手動確認必要 |
| T18 (手動レイアウト → state 反映) | ❌ FR-32 未実装 |
| T19 (誤 WS 移動 → revert) | ❌ FR-33 未実装 |
| T20 (手動コラム順 → state 保存) | ❌ FR-32 未実装 |
| T03-T08 (archive/unarchive/add) | ❌ 未実装 |

### フォーカス独立性テスト（NFR-15）

各シナリオ（T01, T09-T11, T19-T21 等）は以下の 8 つの開始フォーカス位置から繰り返す:

```
A (viewer), Q (primary slot), W (other slot), E (other slot),
M (isolated), 1 (isolated), B (isolated), 3 (cmuxterm)
```

実装: `runFromAllFoci(t, fn)` ヘルパが各 `setFocus(t, ws)` → `fn(t, ws)` を呼ぶ。
現在 `TestTF01`, `TestTF09`, `TestTF10`, `TestTF11`, `TestTF19`, `TestTF21`,
`TestTF_SnapshotFromAllFoci`, `TestTF_ViewerOrderFromAllFoci`,
`TestTF_StatusZeroDiffFromAllFoci` が追加済み。

### 必要な前提実装

1. **`setupIdealState` テストヘルパ**: state.json の Layout フィールドと実 WM の window 配置を理想状態に同期させる。手動理想状態設定 + state snapshot + 全テストの setUp に使う
2. **T01 バグ修正**（OI-9）: profile switch の重複 spawn、viewer WS 誤配置
3. **FR-34 実装**: reconcile が kill された窓を元 column 位置で再 spawn
4. **FR-32/33 実装**: 手動操作の state 反映 / 誤 WS revert

---

## `projwm layout restore` コマンド

**状態**: 🟡 design

### 動機

現在 layout restore は archive/profile-switch の内部処理に埋め込まれている。ユーザが手動でレイアウトを壊した後や、OI-9 バグ後に一発で理想状態に戻すための明示的なコマンドが欲しい。統合テストの `setupIdealState` にも使える。

### 設計案

```sh
projwm layout restore [--project <p>] [--profile <name>]
# 省略時: active profile の全 project を restore
```

1. state.json の `windows[].layout` を参照
2. 各 project の slot WS に focus → stable polling
3. `RestoreProjectLayout` を呼ぶ（既存 internal 実装を流用）
4. viewer WS の `FixViewerOrder` を呼ぶ

### 工数

1 日未満（既存 internal 実装を cmd 層に expose するのみ）。

---

## T01 バグ修正（OI-9）— profile switch 重複 spawn

**状態**: 🟢 ready（原因調査 + 修正が必要）

### 症状（T01 で確認）

profile を `a → b → a` と切り替えた後:
- shell×2、ai×2 が WS Q に出現（計 8 window）
- `ai-view-1:dotfiles` が WS A と WS Q の両方に存在
- `ai-view-1:MyEmmoWorld` が WS W（正しくは WS A）
- Vivaldi が WS W（正しくは WS Q）
- WS E に `ai-1:MyEmmoWorld` が 2 つ

### 仮説

- `runReconcileOnce` と profile switch の window close/spawn の順序競合
- viewer 窓の WS 割当ロジックのバグ（slot WS に viewer が spawn される）
- browser window を move-to-workspace する際の slot 解決エラー

### 着手方針

`cmd/profile.go` の `switch` RunE を `--no-reconcile` でステップ実行してどの段階でバグが現れるかを bisect する。

---

## v12: Browser 統合 — chrome-cli + Chromium profile (paradigm C)

**状態**: 🟡 design 確定 / 実装中

### 経緯（要約）

- **paradigm A**（profile-level の browser_workspace 1 個）: project per-window paradigm と整合せず撤回
- **paradigm B**（Vivaldi Workspaces を AX で操作、per-project window）: AX hack のたび focus 強奪、user の作業を妨げると判明、撤回
- **paradigm C**（chrome-cli + Chromium user profile + close 主義 + frontmost 復帰）: ✅ 採用

### 確定要件

1. **per-project window**: project の windows[] に `kind="browser"` window を追加。ai/shell/editor と並ぶ第 4 kind として完全同 paradigm（profile 切替で close、active 復帰で spawn）
2. **Chromium user profile で login 分離**: `state.Window.BrowserProfile` で Vivaldi の `--profile-directory` を指定。会社用 / 個人用 cookies を完全分離
3. **URL snapshot で内容復元**: profile 切替で close する直前に `chrome-cli list tabs` で全 tab URL を取得 → `state.Window.SavedURLs` に保存。active 復帰で URL list を `--new-window <urls>` で再 open（scroll 位置は失われる、login と URL は保持）
4. **frontmost 復帰**: spawn / close の前後で frontmost app を osascript で保存 → destructive 後に元 app を activate。user の focus が一瞬 flicker するが作業中アプリに戻る
5. **read 系は完全 non-intrusive**: `chrome-cli list windows / tabs` は AppleScript event 経由で focus を奪わない。cockpit が自由に状態 query 可能
6. **通常運用は触らない**: launchd auto-reconcile / display-change / periodic は idempotent な no-op。user が何もしてない時に Vivaldi が動くことはない

### 実装の主要部品

| 部品 | 役割 |
|---|---|
| `internal/browserwrap/chromium/` (新設) | chrome-cli wrapper, frontmost preserve helper, spawn/close/snapshot |
| `state.Window` 拡張 | `BrowserProfile string`, `SavedURLs []string`（既に追加済） |
| `cmd/add-browser` | `--profile=NAME --url=https://... --url=...` で windows[] に追加 |
| `reconcile/reconcile.go` | `ensureBrowserWindow` (idempotent な spawn)、`closeProjectBrowserWindows` (snapshot + close) |
| `cmd/archive` 拡張 | archive 時に snapshot を必ず実行 |
| `cockpit` 表示 | `browser-1  profile=work  (3 tabs saved)` など |

### CLI（予定）

| コマンド | 用途 |
|---|---|
| `projwm add-browser --project=<p> --profile=<vp> --url=<u>... ` | window 追加 + 初回 spawn |
| `projwm browser list` | 全 browser window と profile / saved URL 数を表示 |
| `projwm browser focus <project> [browser-id]` | 該当 window を AX で前面化（user の手動 navigation 補助） |
| `projwm remove --window=browser-N` | windows[] から削除 + window close（既存 cmd 流用）|

### state schema（実装済の field を使う）

```json
"windows": [
  { "id": 1, "kind": "browser",
    "browser_profile": "work",
    "saved_urls": ["https://github.com/...", "https://example.com/..."] }
]
```

### profile 切替時の挙動（paradigm の核）

```
1. frontmost = osascript -e 'frontmost app'
2. for each project that becomes inactive:
     for each browser window in project:
       chrome-cli list tabs -w <wid>  # focus 奪わない
       SavedURLs に保存
       chrome-cli close -w <wid>      # focus 一瞬奪う
3. for each project that becomes active:
     for each browser window in project:
       open -na Vivaldi --args --profile-directory=<bp> --new-window <SavedURLs>
                                                             # focus 一瞬奪う
4. activate frontmost  # 元の app に focus 復帰
```

### 既知の trade-off

- close で **scroll 位置 / form 入力 / 動画再生位置は失われる**（cookies/login は profile に残るため再 spawn 後 login 済み）
- profile 切替の瞬間、Vivaldi が一瞬 flicker（spawn / close は OS 仕様で前面化）。ai/shell/editor の close/spawn と同等の感覚
- chrome-cli は Chromium 系のみ対応（Vivaldi / Brave / Edge / Chrome / Arc）。Firefox 系（Zen）は不可

---

## v13: Zen workspace 統合（deferred）

**状態**: 🟣 hold

ユーザは Zen を **通常運用** browser として独立使用しているため、projwm 制御の優先度は低い。実装する場合の判断点:

- Zen は Firefox fork で AppleScript dictionary なし → System Events keystroke のみ
- workspace 切替の default shortcut は無し → ユーザが Cmd+Opt+1..9 等を Settings で手動設定する必要
- macOS での shortcut 永続化に既知バグ（zen-browser/desktop #4014, #1813, #7341, #9170）あり、信頼性が Vivaldi より低い
- Workspaces は Container Tabs ベースなので tab snapshot 取得は限定的

着手判断はユーザの「Zen も projwm 連動したい」要望が出た時。

---

## v14: Browser tab snapshot（best-effort）

**状態**: 🔵 research

v12 で workspace 単位の分離は実現するが、「workspace の tab 一覧を **projwm 側でも保存**」したい場合の追加機能。

- `projwm browser snapshot <project>`: AppleScript で全 tab URL を取得して state に保存
- `projwm browser apply <project>`: 保存した URL list を browser で開く
- 用途: state.json を別 Mac に持っていったとき、browser 設定がなくても URL は復元

実装可否: Vivaldi の Chromium AppleScript で tab URL list は取得可能（vitorgalvao gist 参照）。Zen は限定的。

---

## Phase 7: cmux / zellij 撤去

**状態**: 🟣 hold（ユーザの GO サイン待ち）

### 内容

projwm が完全動作するようになった現状、旧来の cmux と zellij を物理削除する:

1. `profiles/darwin.nix` から:
   - `myConfig.darwin.cmux.enable = true;` 削除
   - `myConfig.zellij.enable = true;` 削除
   - `imports = [ ... ../modules/darwin/cmux.nix ... ../modules/common/zellij.nix ];` 削除
2. `modules/darwin/cmux.nix` 物理削除
3. `modules/common/zellij.nix` 物理削除
4. `CMUX.md` / `CMUX-WORKFLOW.md` 物理削除
5. `darwin-rebuild switch` で確定
6. （任意）`brew uninstall --cask cmux` で物理 app 除去

### リスク

- 撤去後、cmux 内で開いていた tmux session は zellij 経由なので失われる可能性
- ユーザが現状 cmux を主作業環境にしているなら、移行確認後に実施

### 進め方

ユーザが「OK 撤去して」と明示したら projwm がやる。それまで stay。

---

## `projwm restore` 実装（FR-11）

**状態**: 🟡 design

### 動機

state.json を誤削除した場合や、別マシンで初めて projwm を起動した場合、tmux server に残る `<kind>-<id>/<project>` 系 session を逆解析して state.json を再構築する。

### 実装方針

```
1. tmux ls で全 session 名を取得
2. 命名規則 `<kind>-<id>/<project>` に matche するものを抽出
3. project 名でグループ化、windows[] を再構築
4. cwd 推測:
   - tmux capture-pane で pwd / cd のヒストリから推測
   - or ~/dev/<project> を候補として user prompt
5. AI 種別:
   - tmux capture-pane で claude / copilot のプロンプトを検出
   - 不明なら user prompt（default: claude）
6. profile 推測: 全 project を park（未割当）として登録、ユーザが後で assign
7. state.json を atomic write
```

### 工数

1〜2 日。tmux 解析と user prompt UX 設計が中心。

---

## 多 editor 並走の挙動研究

**状態**: 🔵 research

### 課題

`add-editor` で同 project に id=2 の Zed window を追加可能だが、Zed が「同 folder の 2 window」を許可するか不明。実機では `zed -n <cwd>` で重ねて開くと workspace tab が増える挙動が観察されている。

### 調査項目

- [ ] Zed の `--new-window` flag と `--new-workspace` flag の挙動差
- [ ] 同じ folder を 2 度開く時の Zed 内部の workspace tab 管理
- [ ] 多 editor 並走時の OmniWM での識別（title 完全一致なので 2 つ目以降の区別が困難）
- [ ] symlink を切って別 cwd 風にして basename を変える workaround の妥当性

MVP では id=1 のみ運用、要望次第で実装。

---

## モニタプロファイル別の slot 配置 override

**状態**: 🔵 research

### 動機

3 モニタ環境では「Q-T はメイン display、U-P はサブ display」のように slot を物理 monitor に bind したい。

### 調査項目

- [ ] OmniWM の `monitorAssignment` を profile（projwm の profile）ごとに override する仕組み
- [ ] state.json に `monitor_overrides` を追加するか、config.toml で持つか
- [ ] 物理 monitor 抜き差し時のフォールバック（specificDisplay 解決失敗 → main にフォールバック）

MVP では全 slot main 固定、運用後判断。

---

## 既知の OmniWM upstream 関連 (informational)

projwm 自体の改修ではないが、参考情報として:

| Issue | 内容 | projwm への影響 |
|---|---|---|
| BarutSRB/OmniWM #263 | Ghostty windows の focus ring が rectangular | 別件、現状影響なし |
| BarutSRB/OmniWM #243 | Notion app が manage されない | 同様の symptom 系列、解決策（custom rule）は projwm でも採用済 |
| BarutSRB/OmniWM #128 | EndNote / Adobe Illustrator 認識問題 | 同様 |

---

## 改善・要望リスト（小粒）

優先度低、随時着手:

- [ ] cockpit の lipgloss スタイルを theme 切替可能に（dark/light）
- [ ] `projwm logs --tail N` 実装（reconcile.log を tail）
- [ ] `projwm doctor` を強化（kitty 残骸、古い Ghostty.app 等の検出）
- [ ] state.json migration framework（v2 schema 投入時に備える）
- [ ] cockpit に「Phase 7 撤去推奨」インジケータ
- [ ] `projwm tui` の altscreen 上でのリサイズ追従（terminal の変化に応じて再描画）
- [ ] tmux session の中で claude プロセスが死んだ時の自動再起動（or 通知）

---

---

## TODO: 3 ディスプレイ構成で Q/W/E を HP モニターに固定

**状態**: 🟣 hold（調査・実装は後回し）

### メモ

3 ディスプレイ構成の場合、slot WS（Q/W/E など）を HP モニター（解像度 3456 相当のディスプレイ）に割り当てる。OmniWM の `monitorAssignment` を profile または display-profile で override する形になる見込み。

現在 spec.md §4.2 は全 slot を `main` 固定。この TODO が確定したら spec.md の OI-7 + モニタプロファイル別 override セクションと合わせて設計する。

---

## TODO: opt+shift+r キーバインドの調査・再設定

**状態**: 🟣 hold（調査は後回し）

### メモ

`opt+shift+r` にかつて何らかのコマンドが割り当たっていた。現在の動作・意図・残骸を調査して、必要なら復元または削除する。

調査先: `modules/darwin/omniwm/hotkeys.nix`, `modules/darwin/omniwm/karabiner-rules.nix`。

---

_本書は projwm の **未着手の計画書**。実施完了したら `projwm-history.md` または `projwm-spec.md` の Decision に移管する。_
