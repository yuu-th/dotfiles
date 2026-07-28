# projwm-cockpit TUI bubbletea 全面移行 plan

> 要件 §9 / §10 (requirements v2.6) を bubbletea で実装し直す全面再設計。
> 設計書 T10 (charmbracelet/bubbletea v0.25+) に従う。ユーザ承認済。
>
> Last updated: 2026-05-18 (session in progress)

---

## 0. 移行スコープ

**置換対象** (~2200 行):
- `cmd/projwm-cockpit/main.go` (event loop / signal / lifecycle)
- `cmd/projwm-cockpit/model.go` (state + rebuildItems)
- `cmd/projwm-cockpit/view.go` (renderModel)
- `cmd/projwm-cockpit/actions.go` (key dispatch から呼ばれる action 群)
- `cmd/projwm-cockpit/prompt.go` (modal prompt)
- `cmd/projwm-cockpit/term.go` (raw mode tty) ← **bubbletea が代替するので削除候補**
- `cmd/projwm-cockpit/mode.go` (uiMode enum) ← 残す、bubbletea model 内で利用

**残置 / 微修正**:
- `client.go` (IPC client、subscribe / SubmitIntent)
- `snapshot.go` (snapshot 型定義 + loader)

---

## 1. パッケージ構成 (新)

```
cmd/projwm-cockpit/
├── main.go                # tea.Program entry, OS signal, IPC ctx, lifecycle
├── tui/
│   ├── model.go           # type Model struct (tea.Model)
│   ├── update.go          # Update(msg) → (Model, tea.Cmd)
│   ├── view.go            # View() string (lipgloss compose)
│   ├── messages.go        # tea.Msg types: snapshotMsg, cardAddedMsg, ...
│   ├── keymap.go          # key.Binding 定義 + Help formatter
│   ├── commands.go        # tea.Cmd factories: listenSnapshot, submitIntent...
│   └── components/
│       ├── topbar.go      # §9.1
│       ├── cards.go       # §9.3 / §10
│       ├── items.go       # §9.2
│       ├── filter.go      # §9.4 (bubbles/textinput)
│       ├── prompt.go      # modal prompt
│       ├── help.go        # ?
│       └── status.go      # status line
├── client.go              # 既存維持 (IPC)
├── snapshot.go            # 既存維持 (snapshot 型)
└── *_test.go
```

依存パッケージ:
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles` (textinput, help, key, viewport, list)
- `github.com/charmbracelet/lipgloss` (style / layout)

---

## 2. tea.Model

```go
type Model struct {
    // World snapshot (from daemon)
    snap      snapshot.Snapshot
    cards     []cards.Item       // 最新が前 (§10.4)
    items     []items.Item       // slot/project/profile/viewer
    cursor    int

    // Filter (§9.4)
    filter        textinput.Model
    filterFocused bool            // letter は filter or action key を切り替える

    // Modal prompt
    prompt *prompt.State          // nil = no prompt

    // UI mode (§8.4)
    uiMode uiMode                 // proposal | navigation | management

    // Display
    width, height int
    help          help.Model
    showHelp      bool

    // Status line
    status string

    // IPC channels (tea.Cmd で listen)
    snapshotCh chan snapshot.Snapshot
    cardCh     chan cards.Event

    // Daemon client
    client *cockpitClient

    // Config (manifest, socket path 等)
    cfg config
}
```

---

## 3. tea.Msg types

```go
type snapshotMsg   struct{ Snap snapshot.Snapshot }
type cardAddedMsg  struct{ Card cards.Item }
type cardRemovedMsg struct{ CardID string }
type genCommittedMsg struct{ Gen string }
type ipcErrorMsg   struct{ Err error }
type intentReplyMsg struct{ Resp intent.Response; Err error }
type statusMsg     struct{ Text string }
type promptSubmitMsg struct{ Value string }
type tickMsg       time.Time     // 1s tick for relative timestamps
```

---

## 4. tea.Cmd factories

```go
func listenSnapshot(ch <-chan snapshot.Snapshot) tea.Cmd
func listenCards(ch <-chan cards.Event) tea.Cmd
func submitIntent(c *client, in intent.Intent) tea.Cmd
func setCockpitVisibility(c *client, v world.CockpitVisibility) tea.Cmd
func tickEvery(d time.Duration) tea.Cmd
```

各 listener cmd は 1 メッセージを受けて返り、Update で再 arm する re-entrant パターン。

---

## 5. View 合成 (lipgloss)

```
┌────────────────────────────────────────────────────────┐
│ gen=G004906  epoch=42  prof=alpha  CONVERGED  digest=ok │
│                                                cards=3 │
├────────────────────────────────────────────────────────┤
│ Cards (newest first):                                   │
│ > [NEW] 12:34:56 ai-1 ghostty needs respawn  [Enter:re] │
│   [CLOSED] 12:33:01 shell-2 restored           [Enter:k] │
├────────────────────────────────────────────────────────┤
│ Slots:                                                  │
│ ▸ Q  proj=foo  ai-1 ✓  shell-1 ✓  editor ✓  br ✓        │
│   W  proj=bar  ai-1 ✗  shell-1 ✓                        │
│ Park projects:                                          │
│   baz                                                   │
│ Viewer AI streams (workspace A):                        │
│   ai-1 (foo)  ai-2 (foo)                                │
├────────────────────────────────────────────────────────┤
│ /filter > foo█                                          │
│ [help: ?]                                               │
└────────────────────────────────────────────────────────┘
```

lipgloss.Border + JoinVertical で構成。

---

## 6. Keymap (§9.5 完全反映)

```go
type keyMap struct {
    Up, Down, CtrlJ, CtrlK   key.Binding
    Enter, Esc                key.Binding
    Tab                       key.Binding
    NewProj                   key.Binding  // n
    Unassign                  key.Binding  // d
    Archive                   key.Binding  // a
    Unarchive                 key.Binding  // u
    Remove                    key.Binding  // r
    Help                      key.Binding  // ?
    DismissAll                key.Binding  // Ctrl-L
    Quit                      key.Binding  // Ctrl-C → hide cockpit
    CarryOver                 key.Binding  // t (カード上で)
}
```

挙動ルール:
- filterFocused == true → 任意 rune は filter へ。Esc/Up/Down/Enter のみ extract
- filterFocused == false → letter alternate action / 任意 rune で filter 起動

---

## 7. G1-G9 ギャップ吸収マッピング

| ID | 要件 | bubbletea での解決 |
|---|---|---|
| G1 | カード CreatedAt 表示 | cards.Item の Render で `relativeTime(Created)` を含める |
| G2 | Esc = カード個別 dismiss | keymap.Esc + uiMode 階層分岐 |
| G3 | letter alt action vs filter | filterFocused state |
| G4 | t carry-over detail | promptState=carryOver modal で詳細表示 |
| G5 | omniwm-cockpit-{show,hide} deadcode | **完了済** (前 step で IPC 化) |
| G6 | stty -icrnl | bubbletea が tty mode を自前管理 → 自動解決 |
| G7 | 最新カード上 | cards slice の前置 insert |
| G8 | Esc 階層化 | filter → prompt → cockpit hide の 3 段階で Esc 反応 |
| G9 | `projwm status` 出力 (convergence/digest/cards/park) | cmd/projwm/cmd_status を別途修正 (TUI 外) |

---

## 8. 実装フェーズ

### Phase 2.1 — 依存追加 + Nix 反映
1. `go get github.com/charmbracelet/{bubbletea,bubbles,lipgloss}`
2. `go mod tidy`
3. `darwin-rebuild` → vendorHash error → 正しい hash で `default.nix` 更新
4. 再 build 成功

### Phase 2.2 — skeleton
1. `tui/` パッケージ作成
2. 最小 tea.Program で hello world (snapshot 表示のみ)
3. main.go を tea.Program 起動に書き換え
4. ビルド・テスト pass

### Phase 2.3 — components 1: snapshot + topbar + items
- listenSnapshot cmd → snapshotMsg
- topbar / itemlist component
- §9.1, §9.2 充足

### Phase 2.4 — components 2: cards + filter
- listenCards cmd → cardAddedMsg
- cards top render
- filter (textinput) integration
- §9.3, §9.4, G1, G3, G7 充足

### Phase 2.5 — keymap + actions
- key.Binding 完全反映
- existing actions.go (jumpToSlot, cycleActiveProfile, ...) を tea.Cmd 経由で呼ぶ
- §9.5 充足、G2, G8 充足

### Phase 2.6 — modal prompt + help
- prompt component (new proj cwd 入力, unarchive 等)
- help component (`?` 表示)
- §9.5 残り、G4 充足

### Phase 2.7 — proposal mode K1.5
- card-added 時に uiMode=proposal、SetCockpitVisibility{Shown} 自動発火
- §8.4-8.6 充足

### Phase 2.8 — テスト + 実機検証
- 各 component の unit test
- snapshot golden test
- 実機 darwin-rebuild → 実際に operate

### Phase 2.9 — 旧コード削除
- 既存 model.go / view.go / actions.go / prompt.go / term.go を削除
- main.go を完全 tea.Program 形に
- prompt_test.go / main_test.go を tui/ 内 test に置き換え

---

## 9. リスク・落とし穴

- **vendorHash**: `vendorHash = null` だと依存追加で Nix ビルドが落ちる。fakeHash で取得 → 正しい hash 設定の二段必要
- **tty mode**: bubbletea は init で raw mode に入り、exit で restore。signal handling は tea.Program 内に組み込み済。既存 term.go の処理を全部委譲
- **IPC subscribe**: 既存 client.Subscribe は goroutine + channel で push を流す。tea.Cmd で listen して msg 化する。channel をブロックしない設計
- **既存 _test.go**: 大半は main package のため、tui/ 移行で test も再配置必要。golden output ベース推奨

---

## 10. 完成判定

- 要件 §9.1 〜 §9.6 を 1 つずつ実機 verify
- 要件 §10.1 〜 §10.5 を実機 verify
- G1-G9 全て解消 (G5 は既に完了)
- 全テスト pass、`darwin-rebuild` deploy、cockpit 1 個常駐 + IPC で操作可能

---

## 11. 次に着手すること (このセッションの即着手項目)

1. `go get` で bubbletea/bubbles/lipgloss を追加
2. `go mod tidy`
3. `darwin-rebuild` で vendorHash error 取得
4. `default.nix` を fakeHash → 正しい hash に
5. ビルド成功確認 → Phase 2.2 skeleton 着手
