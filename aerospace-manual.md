# AeroSpace 4枚モニター操作マニュアル

> macOS (Apple Silicon) × nix-darwin × 4ディスプレイ構成

---

## 📺 モニター配置

```
┌────────────────────────┐  ┌────────────────────────┐
│      DIOS-MF241X       │  │        L2235HW         │
│      Monitor 1         │  │        Monitor 3       │
│       【左上】          │  │        【右上】         │
│                        │  │                        │
│  Editor (VS Code)      │  │  デフォルト柔軟         │
│  + 柔軟スペース        │  │  (未割当アプリはここ)  │
│                        │  │                        │
│  WS: E, 1, 2           │  │  WS: 3, 4, 5, 6, 7, 8, 9│
└────────────────────────┘  └────────────────────────┘
┌────────────────────────┐  ┌────────────────────────┐
│       (名前なし)        │  │ Built-in Retina Display│
│       Monitor 2        │  │      Monitor 4         │
│       【左下】          │  │     【右下/MacBook】    │
│                        │  │                        │
│  ブラウザ専用           │  │  Spotify / Discord     │
│                        │  │  (常駐アプリ)          │
│                        │  │                        │
│  WS: B                 │  │  WS: S, C              │
└────────────────────────┘  └────────────────────────┘
```

## 🖱️ マウス自動追従

キーボード操作でフォーカスを移動すると、マウスカーソルが自動的に追従する。
手動でマウスを使っているときは邪魔しない（lazy 動作）。

| トリガー | マウスの動き |
|---|---|
| `alt + h/j/k/l` でウィンドウフォーカス移動 | フォーカス先ウィンドウの中央にマウスが移動 |
| `alt + 1〜9` / `alt + s/c/b/e` でWS切替 | 切替先WSのフォーカスウィンドウの中央にマウスが移動 |
| `alt + ctrl + h/j/k/l` でモニタ間移動 | 移動先モニタの中央にマウスが移動 |

> **lazy とは**: マウスが既にそのウィンドウ／モニタ上にいる場合は移動しない。
> キーボード操作とマウス操作を混在させても違和感なく使える。

---

## ⌨️ キーバインド一覧

`alt` = Option キー

### 基本操作

| キー | 動作 | 説明 |
|---|---|---|
| `alt + h/j/k/l` | フォーカス移動 | 同じワークスペース内のウィンドウ間で左/下/上/右にフォーカスを移動 |
| `alt + shift + h/j/k/l` | ウィンドウ移動 | フォーカス中のウィンドウをタイル内で左/下/上/右に入れ替え |
| `alt + enter` | フルスクリーン | AeroSpace管理のフルスクリーン（macOSネイティブではない） |
| `alt + shift + space` | floating⇔tiling | ウィンドウをフローティングとタイリングで切替 |
| `alt + /` | レイアウト切替 | tiles: horizontal ⇔ vertical |
| `alt + ,` | accordion切替 | accordion: horizontal ⇔ vertical |
| `alt + -` | 縮小 | スマートリサイズ -50px |
| `alt + =` | 拡大 | スマートリサイズ +50px |

### モニター間フォーカス移動

2×2配置に対して方向ベースで直感的に操作できる。

| キー | 動作 |
|---|---|
| `alt + ctrl + h` | **左** のモニターにフォーカス |
| `alt + ctrl + j` | **下** のモニターにフォーカス |
| `alt + ctrl + k` | **上** のモニターにフォーカス |
| `alt + ctrl + l` | **右** のモニターにフォーカス |

```
例: 右上 (L2235HW) にいるとき
  alt+ctrl+h → 左上 (DIOS) に移動
  alt+ctrl+j → 右下 (MacBook) に移動
```

### ワークスペース切替

#### 用途別ワークスペース（固定）

| キー | WS | モニター | 用途 |
|---|---|---|---|
| `alt + s` | S | MacBook (右下) | Spotify |
| `alt + c` | C | MacBook (右下) | Discord / Chat |
| `alt + b` | B | 名前なし (左下) | ブラウザ |
| `alt + e` | E | DIOS (左上) | VS Code Insiders |

#### 数字ワークスペース（柔軟）

| キー | WS | モニター |
|---|---|---|
| `alt + 1` | 1 | DIOS (左上) |
| `alt + 2` | 2 | DIOS (左上) |
| `alt + 3` | 3 | L2235HW (右上) |
| `alt + 4` | 4 | L2235HW (右上) |
| `alt + 5` | 5 | L2235HW (右上) |
| `alt + 6` | 6 | L2235HW (右上) |
| `alt + 7` | 7 | L2235HW (右上) |
| `alt + 8` | 8 | L2235HW (右上) |
| `alt + 9` | 9 | L2235HW (右上) |

### ウィンドウをワークスペースに送る

ウィンドウを指定WSに移動し、そのWSにジャンプする。

| キー | 動作 |
|---|---|
| `alt + shift + s/c/b/e` | 用途別WSに送る＋ジャンプ |
| `alt + shift + 1〜9` | 数字WSに送る＋ジャンプ |

```
例: 右上のウィンドウを左上のDIOSに送りたい
  → alt+shift+1 (WS 1はDIOSに固定) でウィンドウが左上に移動
```

### その他

| キー | 動作 |
|---|---|
| `alt + tab` | 前のWSに戻る（行き来） |
| `alt + shift + tab` | 現在のWSを隣のモニターに移動（巡回） |
| `cmd + h` | **無効化済み**（ウィンドウ行方不明を防止） |
| `cmd + alt + h` | **無効化済み**（同上） |
| `ctrl + cmd + ドラッグ` | ウィンドウをどこからでもドラッグ移動 |

---

## 🔧 モード

通常は `main` モード。特殊モードに入ると操作が変わる。

### Resize モード

`alt + r` で入る。ウィンドウサイズを細かく調整できる。

| キー | 動作 |
|---|---|
| `h / l` | 幅を -50 / +50 |
| `j / k` | 高さを +50 / -50 |
| `shift + h / l` | 幅を -200 / +200（粗い調整） |
| `enter` or `esc` | main モードに戻る |

### Service モード

`alt + shift + ;` で入る。設定管理やデバッグ用。

| キー | 動作 |
|---|---|
| `esc` | 設定リロード + main に戻る |
| `r` | ワークスペースツリーをフラット化 + main に戻る |
| `f` | floating⇔tiling切替 + main に戻る |
| `backspace` | 現在のウィンドウ以外を全て閉じる + main に戻る |
| `alt + shift + d` | AeroSpaceを一時無効化/再有効化 + main に戻る |

---

## 🚀 アプリの自動配置

新しいウィンドウが開くと、以下のルールで自動的にワークスペースに配置される。

```
新しいウィンドウ
  │
  ├── Chrome / Firefox / Safari     → WS B (左下: ブラウザ)
  ├── VS Code Insiders / VS Code    → WS E (左上: エディタ)
  ├── Spotify                       → WS S (右下: 音楽)
  ├── Discord                       → WS C (右下: チャット)
  ├── Finder / System Settings 等   → floating (現在のWSに留まる)
  └── それ以外                      → WS 3 (右上: デフォルト柔軟)
```

### 自動配置を無視して手動配置する

アプリが自動配置された後でも、手動で別のWSに移動できる。

```bash
# 例: 今フォーカスしているウィンドウを WS 1 (DIOS/左上) に送る
alt + shift + 1
```

---

## 🖥️ 運用パターン

### パターン1: 通常のコーディング作業

```
[左上: DIOS]              [右上: L2235HW]
 VS Code (WS E)            ドキュメント参照 (WS 3)

[左下: 名前なし]           [右下: MacBook]
 Chrome (WS B)              Spotify (WS S) + Discord (WS C)
```

- `alt+e` でエディタへジャンプ
- `alt+b` でブラウザへジャンプ
- `alt+s` で音楽操作
- `alt+ctrl+k` / `alt+ctrl+j` で上下のモニタ間をサッと移動

### パターン2: 調査・リサーチ中心

```
[左上: DIOS]              [右上: L2235HW]
 資料A (WS 1)              資料B (WS 3)

[左下: 名前なし]           [右下: MacBook]
 Chrome 複数タブ (WS B)     Discord で質問 (WS C)
```

- `alt+1` と `alt+3` で左上・右上の資料を切替
- `alt+b` でブラウザを確認
- `alt+shift+3` でウィンドウを右上に送る

### パターン3: 会議・コミュニケーション中心

```
[左上: DIOS]              [右上: L2235HW]
 VS Code (WS E)            会議メモ (WS 3)

[左下: 名前なし]           [右下: MacBook]
 Chrome 画面共有 (WS B)     Discord (WS C)
```

---

## 🔍 トラブルシューティング

### よく使うCLIコマンド

```bash
# ワークスペース一覧
aerospace list-workspaces --all

# モニター情報の確認
aerospace list-monitors

# 全ウィンドウの一覧 (JSON)
aerospace list-windows --all --json

# フォーカス中のウィンドウ情報
aerospace list-windows --focused --json

# アプリIDの確認 (on-window-detected の設定に使用)
aerospace list-apps --all

# 設定のリロード (service モードの esc と同等)
aerospace reload-config

# CLIからキーバインドをトリガー
aerospace trigger-binding --mode main alt-1
```

### ウィンドウが見つからない

AeroSpaceは非表示ワークスペースのウィンドウを画面の右下隅に隠す。

1. `aerospace list-windows --all --json` で全ウィンドウの場所を確認
2. 該当WSに `alt+数字` or `alt+英字` で切替
3. もし完全に見失った場合: `alt+shift+;` → `r` でワークスペースツリーをリセット

### フォーカスが意図しないウィンドウに飛ぶ

- `Displays have separate Spaces` が OFF になっているか確認
  ```bash
  defaults read com.apple.spaces spans-displays
  # true なら OK (= "Displays have separate Spaces" が OFF)
  ```
- 設定変更後は**ログアウト→ログイン**が必要

### AeroSpaceを一時的に無効化したい

1. `alt + shift + ;` (service モードに入る)
2. `alt + shift + d` (無効化トグル)

### 設定ファイルの場所

```bash
# Nix が生成した設定ファイル (読み取り専用)
# 実際のパスは以下で確認
pgrep -fl AeroSpace
# --config-path の後ろに表示される

# 設定を変更するには
vim ~/dev/dotfiles/modules/desktop/darwin/aerospace.nix
sudo darwin-rebuild switch --flake ~/dev/dotfiles#yuta
```

---

## 📋 macOS システム設定 (自動適用済み)

以下の設定は `darwin-rebuild switch` で自動的に適用されている。

| 設定 | 値 | 効果 |
|---|---|---|
| `Displays have separate Spaces` | **OFF** | マルチモニタの安定性向上。⚠️ macOSネイティブフルスクリーンは他画面が黒くなる |
| `NSWindowShouldDragOnGesture` | ON | `ctrl+cmd+ドラッグ` でどこでもウィンドウ移動 |
| `NSAutomaticWindowAnimationsEnabled` | OFF | ウィンドウのアニメーションを無効化（高速化） |
| `NSWindowResizeTime` | 0.001 | リサイズ速度を最大化 |
| `Dock autohide` | ON | Dock を自動的に隠す |
| `mru-spaces` | OFF | Spaces の自動並べ替えを無効化 |
| `expose-group-apps` | ON | Mission Control でアプリごとにグループ化 |
| `cmd-h / cmd-alt-h` | 無効化 | ウィンドウの "Hide" を防止 |
| JankyBorders | 有効 | フォーカス中ウィンドウに黄色いボーダーを表示 |

---

## 🔑 キーバインド早見表

```
                    main モード
 ┌─────────────────────────────────────────────┐
 │ フォーカス移動    alt + h/j/k/l              │
 │ モニタ間移動      alt + ctrl + h/j/k/l       │
 │ ウィンドウ移動    alt + shift + h/j/k/l      │
 │                                             │
 │ WS切替 (用途)    alt + s/c/b/e              │
 │ WS切替 (数字)    alt + 1〜9                  │
 │ WS送り+ジャンプ   alt + shift + s/c/b/e/1〜9  │
 │                                             │
 │ フルスクリーン    alt + enter                 │
 │ float⇔tile      alt + shift + space         │
 │ レイアウト        alt + / または ,            │
 │ リサイズ          alt + - / =                │
 │                                             │
 │ 前のWSに戻る      alt + tab                   │
 │ WSをモニタ移動    alt + shift + tab           │
 │ resize モード    alt + r                     │
 │ service モード   alt + shift + ;             │
 └─────────────────────────────────────────────┘

             resize モード (alt+r で入る)
 ┌─────────────────────────────────────────────┐
 │ 幅  -50 / +50    h / l                      │
 │ 高さ +50 / -50   j / k                      │
 │ 幅 -200 / +200   shift+h / shift+l          │
 │ 戻る             enter or esc               │
 └─────────────────────────────────────────────┘

           service モード (alt+shift+; で入る)
 ┌─────────────────────────────────────────────┐
 │ 設定リロード      esc                        │
 │ ツリーリセット    r                          │
 │ float切替        f                          │
 │ 全窓閉じ         backspace                  │
 │ AeroSpace無効化   alt + shift + d            │
 └─────────────────────────────────────────────┘
```
