# cmux ワークフロー設計書

> **設計思想**:
> cmux はワークスペース管理・AI通知・組み込みブラウザを担い、Zellij はプロセス永続化・内部ペイン分割を担う。
> 役割を明確に分離し、直感的なキーボード操作で全ツールにアクセスできるワークフローを実現する。

---

## スタック全体像

```
【cmux】        ... ワークスペース管理（サイドバー）・AI通知・WebViewブラウザ
  【Zellij】   ... プロセス永続化・ペイン分割（AI用・shell用）
    【AI】     ... Claude Code 等の AIエージェント（名前付きセッションで永続）
  【nvim】     ... エディタ（cmux Surface として直接）
  【browser】  ... フロントエンド確認 等（cmux WebView Surface）
```

### 各レイヤーの責任分担

| レイヤー | 担当 | 担わないこと |
|---|---|---|
| **cmux** | ワークスペース切替・通知・WebView・worktree単位の管理 | タブ/ペインの詳細制御 |
| **Zellij（AI用）** | AIセッションの永続化 | UI 表示・タブ管理 |
| **Zellij（shell用）** | shellの柔軟な分割・セッション | UI 表示 |
| **nvim** | エディタ作業 | — |
| **cmux browser** | フロントエンド確認・Web参照 | — |

---

## ワークスペース構造

### プロジェクトワークスペース

各 cmux ワークスペース = git worktree の 1 ディレクトリに対応する。

```
cmux workspace [proj-X]   cwd = ~/dev/proj-X (worktree)

┌──────────────────────────────┬────────────────────────┐
│  左ペイン (60%)               │  右ペイン (40%)         │
│                              │                        │
│  Zellij session              │  [⌃1] Zellij session   │
│  "proj-x-ai"                 │        shell           │
│   └── AI (claude 等)         │        └─ 内部で分割・  │
│       [プロセス永続]          │           タブ使い放題  │
│                              ├────────────────────────┤
│                              │  [⌃2] nvim             │
│                              │       (direct terminal)│
│                              ├────────────────────────┤
│                              │  [⌃3] browser          │
│                              │       (cmux WebView)   │
└──────────────────────────────┴────────────────────────┘

⌘⇧Enter: 現在のペインをズーム全画面 / もう一度で戻る
```

**ポイント:**
- 左ペインの Zellij セッションは名前付き (`proj-x-ai`)。cmux 再起動後も AI セッションが生きている
- 右ペインの 3 サーフェスは cmux.json で **ワークスペース起動時に自動作成** される（「開く」操作不要）
- shell サーフェス（⌃1）内は Zellij が動いており、`⌘⇧Enter` でズームしてから自由に内部分割できる

### viewerワークスペース

全 AI セッションをまとめて監視する専用ワークスペース。入力は行わない（read-only 的運用）。

```
cmux workspace [viewer]   cwd = ~

┌──────────────────────────┬───────────────────────────┐
│  zellij attach proj-a-ai │  zellij attach proj-b-ai  │
│  (監視のみ)              │  (監視のみ)               │
├──────────────────────────┼───────────────────────────┤
│  zellij attach proj-c-ai │  (追加可)                 │
│  (監視のみ)              │                           │
└──────────────────────────┴───────────────────────────┘
```

---

## キーボード操作早見表

### cmux レベル（常に有効）

| キー | 動作 |
|---|---|
| `⌘1` … `⌘9` | ワークスペース 1〜9 を選択 |
| `⌘P` | ワークスペーススイッチャー（名前検索） |
| `⌘N` | 新規ワークスペース |
| `⌘⇧P` | コマンドパレット（cmux.json のコマンドを呼ぶ） |
| `⌘⇧W` | ワークスペースを閉じる |
| `⌃⌘]` / `⌃⌘[` | 次/前のワークスペース |

### ペイン操作（cmux レベル）

| キー | 動作 |
|---|---|
| `⌥⌘←` | 左ペインにフォーカス（AI ペインへ） |
| `⌥⌘→` | 右ペインにフォーカス（ツールペインへ） |
| `⌥⌘↑` / `⌥⌘↓` | 上/下ペインにフォーカス（分割後） |
| `⌘⇧Enter` | 現在のペインをズーム全画面 / トグル |

### サーフェス（タブ）操作（右ペイン内）

| キー | 動作 |
|---|---|
| `⌃1` | shell（Zellij）サーフェスに切替 |
| `⌃2` | nvim サーフェスに切替 |
| `⌃3` | browser（WebView）サーフェスに切替 |
| `⌘T` | 新サーフェス追加（現在のペインに） |
| `⌘W` | 現在のサーフェスを閉じる |

### ブラウザ操作（browser サーフェス内）

| キー | 動作 |
|---|---|
| `⌘L` | アドレスバーにフォーカス（URL 入力） |
| `⌘R` | ページ再読み込み |
| `⌘[` / `⌘]` | 戻る / 進む |
| `⌥⌘I` | DevTools トグル |
| `⌥⌘D` | ブラウザペインを右に追加分割（追加で使いたい時） |

### Zellij 内（shell サーフェス ⌃1 内で有効）

| キー | 動作 |
|---|---|
| `Ctrl+D` | Zellij セッションからデタッチ |
| `Alt+S` | スクロールモード |
| （Zellij keybind を別途設定） | ペイン分割・タブ切替 等 |

---

## 典型的なワークフロー

### ① 新しいプロジェクトを始める

```
1. git worktree を作成
   git worktree add ~/dev/my-feature origin/my-feature

2. cmux で ⌘⇧P（コマンドパレット）
   → "Start AI Dev" を選択
   → 自動的に左: AI + 右: shell/nvim/browser のレイアウトが展開される

3. 左ペイン（AI）に自動フォーカス
   → AI に指示を出す
```

### ② AIを走らせながらフロントエンドを確認する

```
AI が動いている間:

⌥⌘→     → 右ペインに移動
⌃3       → browser に切替
⌘L       → URL 入力（例: localhost:3000）
Enter    → 確認

⌘⇧Enter  → browser を全画面にして確認
⌘⇧Enter  → 元の分割に戻る
⌥⌘←     → AI ペインに戻って状況確認
```

### ③ コードを確認・編集する

```
⌥⌘→     → 右ペインに移動
⌃2       → nvim に切替
（nvim で編集）

または

⌃1       → shell に切替して git diff, grep 等
```

### ④ shell で複数のコマンドを同時に見たい

```
⌥⌘→      → 右ペインに移動
⌃1        → shell（Zellij）に切替
⌘⇧Enter   → shell ペインを全画面に

（Zellij 内で自由に分割: Zellij keybind で）
  → サーバー起動ログ + テスト実行 等を横に並べる

⌘⇧Enter   → 元の cmux 分割に戻る
```

### ⑤ 全 AI の進捗を俯瞰する

```
⌘P または ⌘1   → viewer ワークスペースに切替
→ 全 proj-x-ai セッションが見える
→ 気になるセッションのあるプロジェクトに ⌘P で飛ぶ
```

---

## 新しいワークスペース（プロジェクト）の開き方

### フロー

```
① git worktree add ~/dev/proj-name <branch>
② cmux で ⌘⇧P → "Start AI Dev" を実行
   または ⌘O でワークスペースのフォルダを開く
③ レイアウトが自動展開される
```

### cmux.json テンプレート（グローバル: ~/.config/cmux/cmux.json）

```json
{
  "commands": [
    {
      "name": "Start AI Dev",
      "keywords": ["ai", "dev", "start", "setup"],
      "restart": "confirm",
      "workspace": {
        "cwd": ".",
        "layout": {
          "direction": "horizontal",
          "split": 0.6,
          "children": [
            {
              "pane": {
                "surfaces": [
                  {
                    "type": "terminal",
                    "name": "AI",
                    "command": "zellij attach \"$(basename $(pwd))-ai\" --create",
                    "focus": true
                  }
                ]
              }
            },
            {
              "pane": {
                "surfaces": [
                  {
                    "type": "terminal",
                    "name": "shell",
                    "command": "zellij attach \"$(basename $(pwd))-tools\" --create"
                  },
                  {
                    "type": "terminal",
                    "name": "nvim",
                    "command": "nvim ."
                  },
                  {
                    "type": "browser",
                    "name": "browser",
                    "url": "http://localhost:3000"
                  }
                ]
              }
            }
          ]
        }
      }
    },
    {
      "name": "Open AI Viewer",
      "keywords": ["viewer", "watch", "monitor", "all"],
      "restart": "confirm",
      "workspace": {
        "name": "viewer",
        "cwd": "~"
      }
    }
  ]
}
```

> **注意:** viewer ワークスペースのレイアウト（どのプロジェクトを監視するか）は
> 手動で構成する（プロジェクト数が動的なため、テンプレートでは自動化困難）。
> viewer 内で `⌘D` 等で分割し、各ペインで `zellij attach <proj>-ai` を手動実行。

---

## Zellij セッション命名規則

| セッション | 命名規則 | 例 |
|---|---|---|
| AI セッション | `<プロジェクト名>-ai` | `my-feature-ai` |
| shell ツールセッション | `<プロジェクト名>-tools` | `my-feature-tools` |

cmux ワークスペース名 = プロジェクト名（worktree の basename）で統一する。

---

## 設計上の判断メモ（Why）

| 判断 | 理由 |
|---|---|
| AI セッションを Zellij 内で動かす | cmux 再起動後もプロセスが生きる。AI は長時間動くことが多いため |
| shell サーフェスも Zellij にする | 内部分割・タブを Zellij のkeybindで自由に制御できる。cmux ペインを増やさなくていい |
| nvim は直接 cmux Surface | Zellij 不要。シンプルで速い |
| browser は cmux WebView Surface | Zellij では WebView を提供できない。cmux 独自機能を活用 |
| Ghostty keybind はカスタムスクリプトに使わない | Ghostty は built-in アクションのみで任意コマンド実行不可 |
| 領域移動は ⌥⌘ 方向キー（相対） | 3ペイン構成なら最大 2 回で到達できる。絶対指定は Karabiner が必要（将来検討） |

---

## 未解決・将来検討事項

- [ ] **Zellij keybind 設定**: shell サーフェス（⌃1）内の Zellij 分割・タブ操作のキーをどう定義するか
- [ ] **viewer の構造**: セッション数が増えたときのレイアウト管理方法
- [ ] **絶対位置ジャンプ**: 「右上ペインに直接ジャンプ」には Karabiner + cmux socket API が必要（今は ⌥⌘ 相対移動で対応）
- [ ] **Nix での管理**: cmux 設定（settings.json, cmux.json）を Nix flake でどう管理するか
- [ ] **worktree 作成コマンド**: `git worktree add` → cmux ワークスペース起動 を 1 コマンドにまとめるか
