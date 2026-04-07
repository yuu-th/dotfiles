# Terminal 設計書

> **設計思想**: タブなし・ペイン分割なし。ウィンドウを単位として AeroSpace で管理し、
> Zellij はセッション永続性のみを担う。シンプルなレイヤー分割で予測可能な操作体系を実現する。

---

## スタック全体像

```
【Layer 1】 AeroSpace   ... ウィンドウ配置・ワークスペース切替・Ghosttyウィンドウ番号フォーカス
【Layer 2】 Ghostty     ... ターミナルエミュレータ。ウィンドウ単位で使う（タブ・分割なし）
              └─ Quick Terminal   ... opt+space でトグルするドロップダウン端末
【Layer 3】 Zellij      ... セッション永続化のみ（タブなし・ステータスバーなし）
【Layer 4】 Fish        ... シェル。Starship/Atuin/zoxide/fzf 統合
```

### 各レイヤーの責任分担

| 操作 | 担当 |
|------|------|
| 新規ウィンドウを開く | **Ghostty** `Cmd+N` |
| ウィンドウを閉じる | **Ghostty** `Cmd+W` |
| 特定のウィンドウにフォーカス | **AeroSpace** `Alt+Ctrl+1/2/3` |
| ワークスペース切替 | **AeroSpace** `Alt+[文字/数字]` |
| セッションを保持して離席 | **Zellij** `Ctrl+D`（デタッチ） |
| セッションに戻る | **fish** `zj <name>` or 新規ウィンドウ起動時のfzf |

---

## Ghostty

### ウィンドウ操作

Ghosttyはタブ・ペイン分割を**使わない**。1ウィンドウ = 1シェル。

| キー | 動作 |
|------|------|
| `Cmd+N` | 新規ウィンドウを開く |
| `Cmd+W` | 現在のウィンドウを閉じる |

> **HHKB ユーザー向けメモ**: Karabiner で物理 Cmd ↔ 物理 Opt が入れ替わっている。
> このドキュメントでは macOS が認識するキー名（Cmd = ⌘、Opt = ⌥）で統一して表記する。
> HHKB では物理 Opt キーが macOS の Cmd として機能する。

### Quick Terminal

| キー | 動作 |
|------|------|
| `Opt+Space` | Quick Terminal トグル（画面下からドロップダウン） |

- **グローバルショートカット**: Ghosttyが非アクティブでも動作する
- **仕組み**: Karabiner が `opt+space` を F13 に変換 → Ghostty が `F13=toggle_quick_terminal` を処理
- **用途**: ちょっとしたコマンド実行、クイックチェック

### 外観設定

| 設定 | 値 | 変更方法 |
|------|------|---------|
| フォント | UDEV Gothic NF, 13pt | `profiles/fav_fonts.nix` を編集 |
| テーマ | tokyonight | `modules/darwin/ghostty.nix` の `theme =` を変更 |
| 背景透明度 | 0.92 | `background-opacity =` を変更 |
| ウィンドウ装飾 | なし（タイトルバー非表示） | `window-decoration = false` |

---

## Zellij（セッション永続化）

### 基本思想

Zellij は **セッション永続化のみ** に使う。タブ・ステータスバー・ペイン分割はすべて無効。  
「プロジェクト = Zellijセッション」として、離席しても状態を保持できる。

### Zellij 内のキーバインド

Zellij のキーバインドは最小限。タブ・分割操作は Ghostty（ウィンドウ）に任せる。

| キー | 動作 |
|------|------|
| `Ctrl+D` | セッションからデタッチ（Ghosttyウィンドウは閉じずセッションは残る） |
| `Alt+S` | スクロールモードに入る |

**スクロールモード内:**

| キー | 動作 |
|------|------|
| `j` / `↓` | 1行下にスクロール |
| `k` / `↑` | 1行上にスクロール |
| `f` / `PageDown` | 1ページ下 |
| `b` / `PageUp` | 1ページ上 |
| `d` | 半ページ下 |
| `u` | 半ページ上 |
| `s` | インクリメンタル検索 |
| `Esc` または `Alt+S` | スクロールモードを抜ける |

**スクロール検索モード内:**

| キー | 動作 |
|------|------|
| `n` | 次のマッチへ |
| `p` | 前のマッチへ |
| `Esc` | 終了 |

### `zj` コマンド（セッションマネージャー）

```fish
zj <name>    # ~/dev/<name> に移動してセッションを起動 or 復帰
zj           # 既存セッション一覧を fzf で表示してアタッチ
```

**動作詳細:**

| 状況 | 動作 |
|------|------|
| `<name>` が `~/dev/<name>` として存在する | そのディレクトリに移動してからセッション管理 |
| `<name>` が相対/絶対パスとして存在する | `realpath` 解決してからセッション名 = `basename` |
| 同名セッションが既存 | `zellij attach <name>` |
| 同名セッションなし | `zellij -s <name>` で新規作成 |
| 引数なし、セッションあり | fzf でセッション選択 → アタッチ |
| 引数なし、セッションなし | エラーメッセージ |
| Zellij 内から `zj` を実行 | エラーを出して終了（二重起動防止） |

---

## 新規ウィンドウ起動フロー

Ghostty で新しいインタラクティブシェルが起動すると、**自動的にセッション選択画面が表示される**。

```
【条件】 TERM_PROGRAM=ghostty かつ ZELLIJ 変数未設定
         ↓
【表示】 fzf でセッション選択画面
         ├─ "Zellijなし (plain fish)" ... そのまま Fish で使う
         ├─ "新規セッション..."       ... セッション名を入力 → Zellij 起動
         └─ [既存セッション名...]     ... zj で直接アタッチ
```

- **発火タイミング**: 新規ウィンドウ（`Cmd+N`）、Quick Terminal 初回起動
- `Ctrl+C` または空白選択でキャンセル → plain fish として続行
- 一度 Zellij に入った後に `Ctrl+D` でデタッチしても、そのシェルはセッション選択に戻らない  
  （`ZELLIJ` 変数が unset されないため）

---

## AeroSpace 連携

### Ghosttyウィンドウへのフォーカス

現在のワークスペースにある Ghostty ウィンドウを X 座標（左から）順に番号付けしてフォーカスする。

| キー | 動作 |
|------|------|
| `Alt+Ctrl+1` | 現WS 内で左から1番目の Ghostty ウィンドウにフォーカス |
| `Alt+Ctrl+2` | 現WS 内で左から2番目の Ghostty ウィンドウにフォーカス |
| `Alt+Ctrl+3` | 現WS 内で左から3番目の Ghostty ウィンドウにフォーカス |

- **仕組み**: `focus-tool win N` スクリプト（AppleScript + AeroSpace CLI）
- **番号の決まり方**: ウィンドウ位置（X座標）の昇順。動かせば番号も変わる
- タイトルフィルタなし。**Ghosttyのウィンドウであれば何でも対象**

### Ghostty 関連のワークスペース設定

`on-window-detected` に Ghostty の自動移動ルールは**ない**。  
新規ウィンドウは開いた時点のワークスペースにそのまま配置される。

---

## キーバインド早見表

### Ghostty

| キー | 動作 |
|------|------|
| `Cmd+N` | 新規ウィンドウ |
| `Cmd+W` | ウィンドウを閉じる |
| `Opt+Space` | Quick Terminal トグル |

*(タブ `Cmd+T`、ペイン `Cmd+D` は無効化済み)*

### Zellij 内

| キー | 動作 |
|------|------|
| `Ctrl+D` | デタッチ（セッション保持） |
| `Alt+S` | スクロールモード |

### AeroSpace（Ghostty関連のみ）

| キー | 動作 |
|------|------|
| `Alt+Ctrl+1` | 現WSの左から1番目の Ghostty ウィンドウへ |
| `Alt+Ctrl+2` | 現WSの左から2番目の Ghostty ウィンドウへ |
| `Alt+Ctrl+3` | 現WSの左から3番目の Ghostty ウィンドウへ |

---

## Nix ファイル構成

| ファイル | 内容 |
|---------|------|
| `modules/darwin/ghostty.nix` | Ghosttyの外観・キーバインド・Quick Terminal設定 |
| `modules/common/zellij.nix` | Zellijの設定（config.kdl）、`zj` コマンド |
| `modules/common/fish.nix` | 新規ウィンドウ起動時のセッション選択fzf、Fish全般設定 |
| `modules/darwin/karabiner/karabiner.json` | `opt+space→F13`変換、HHKBキースワップ |
| `modules/darwin/aerospace/common.nix` | `Alt+Ctrl+1/2/3` バインド |
| `modules/darwin/aerospace/scripts/focus-tool.sh` | Ghosttyウィンドウフォーカス実装 |
| `profiles/fav_fonts.nix` | フォント管理（ここだけ編集すれば全体に反映） |

### よくある変更

**フォントを変える**: `profiles/fav_fonts.nix` の `fonts.packages` と `myConfig.darwin.ghostty.fontFamily` を編集

**Ghosttyのテーマを変える**: `modules/darwin/ghostty.nix` の `theme = ` を変更（候補: `ghostty +list-themes`）

**セッション選択を無効にする**: `modules/common/fish.nix` の `interactiveShellInit` 内の Zellij fzf ブロックをコメントアウト

**新しいキーバインドを追加**: `modules/darwin/ghostty.nix` に `keybind = <combo>=<action>` を追記（アクション一覧: https://ghostty.org/docs/config/keybind/reference）
