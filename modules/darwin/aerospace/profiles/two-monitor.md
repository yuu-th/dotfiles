# AeroSpace 2枚モニター操作マニュアル

> macOS (Apple Silicon) × nix-darwin × 2ディスプレイ構成

---

## 📺 モニター配置

```
┌──────────────────────────┐  ┌──────────────────────────┐
│      LCD-MF234X          │  │ Built-in Retina Display  │
│      Monitor 1 (外部)    │  │   Monitor 2 (MacBook)    │
│                          │  │                          │
│  Editor + 柔軟スペース   │  │  ブラウザ / 常駐アプリ   │
│                          │  │                          │
│  WS: E, 3, 4, 5, 6, 7, 8, 9│  WS: B, M, 1, 2         │
└──────────────────────────┘  └──────────────────────────┘
```

## 🖱️ マウス自動追従

| トリガー | マウスの動き |
|---|---|
| `alt + h/j/k/l` でウィンドウフォーカス移動 | フォーカス先ウィンドウの中央に移動 |
| `alt + 1〜9` / `alt + b/e/m` でWS切替 | 切替先WSのフォーカスウィンドウの中央に移動 |
| `alt + ctrl + h/l` でモニタ間移動 | 移動先モニタの中央に移動 |

---

## ⌨️ ワークスペース切替

### 用途別ワークスペース（固定）

| キー | WS | モニター | 用途 |
|---|---|---|---|
| `alt + b` | B | MacBook | ブラウザ |
| `alt + m` | M | MacBook | Spotify / Discord / カレンダー |
| `alt + e` | E | LCD-MF234X | VS Code Insiders |

### 数字ワークスペース

| キー | WS | モニター |
|---|---|---|
| `alt + 1` | 1 | MacBook |
| `alt + 2` | 2 | MacBook |
| `alt + 3` | 3 | LCD-MF234X |
| `alt + 4` | 4 | LCD-MF234X |
| `alt + 5` | 5 | LCD-MF234X |
| `alt + 6` | 6 | LCD-MF234X |
| `alt + 7` | 7 | LCD-MF234X |
| `alt + 8` | 8 | LCD-MF234X |
| `alt + 9` | 9 | LCD-MF234X |

---

## 🚀 アプリの自動配置

```
新しいウィンドウ
  │
  ├── Chrome / Firefox / Safari / Dia  → WS B (MacBook: ブラウザ)
  ├── VS Code Insiders / VS Code       → WS E (LCD-MF234X: エディタ)
  ├── Gemini                           → WS 1 (MacBook: AI)
  ├── Spotify / Discord / カレンダー   → WS M (MacBook: 常駐)
  ├── iTerm2 / Terminal                → WS 3 (LCD-MF234X: ターミナル)
  ├── Finder / System Settings 等      → floating (現在のWSに留まる)
  └── それ以外                         → WS 3 (LCD-MF234X: デフォルト柔軟)
```

---

## 🖥️ 運用パターン

### 通常のコーディング作業

```
[LCD-MF234X (外部)]          [Built-in Retina Display]
 VS Code (WS E)               Chrome (WS B)
 ターミナル (WS 3)             Spotify + Discord (WS M)
```

- `alt+e` でエディタへ
- `alt+b` でブラウザへ
- `alt+ctrl+h / l` でモニタ間移動

---

## 🔍 トラブルシューティング

```bash
# モニター情報の確認
aerospace list-monitors

# ワークスペース一覧
aerospace list-workspaces --all

# 設定のリロード
aerospace reload-config
```

### プロファイル切替方法

`modules/darwin/aerospace/default.nix` の import 行を変更して `darwin-rebuild switch`:

```nix
profile = import ./profiles/two-monitor.nix;    # 現在
# profile = import ./profiles/triple-monitor.nix;
# profile = import ./profiles/quad-monitor.nix;
```
