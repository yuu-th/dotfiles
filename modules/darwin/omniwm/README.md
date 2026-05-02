# OmniWM モジュール

AeroSpace の代替として OmniWM (scrollable tiling WM) を nix-darwin で管理する。

## ファイル構成

```
modules/darwin/omniwm/
├── default.nix              # myConfig.darwin.omniwm.enable で有効化
├── common.nix               # gaps/borders/focus/niri/dwindle 等の共通 TOML
├── hotkeys.nix              # OmniWM ネイティブ [[hotkeys]] 定義
├── app-rules.nix            # [[appRules]] (AeroSpace の on-window-detected 移植)
├── workspace-builder.nix    # workspaces 配列を生成するヘルパ
├── karabiner-rules.nix      # Karabiner 補助ルール（OmniWM ネイティブで足りない部分）
├── profiles/
│   ├── two-monitor.nix      # 2 枚モニタ用 monitor map
│   ├── triple-monitor.nix   # 3 枚モニタ用
│   └── quad-monitor.nix     # 4 枚モニタ用
└── scripts/
    ├── switch-profile.sh           # モニタ枚数検出 → TOML 差替 → OmniWM 再起動
    ├── focus-monitor-dir.sh        # 方向ベース focus-monitor の自前実装
    ├── setup-media-workspace.sh    # WS M に Spotify+Discord+Calendar 自動配置
    ├── ws-launch.sh                # WS切替+open -a (alt-s/c/a 用)
    └── move-window-to-named-ws.sh  # 名前指定 WS への送り＋ジャンプ
```

## ワークスペース命名規約

OmniWM の workspace `name` (rawID) は数値のみ受理されるため：

| 論理名 (AeroSpace) | rawID (OmniWM) | displayName |
|---|---|---|
| 1〜9 | 1〜9 | 同じ |
| M (Media) | 10 | "M" |
| B (Browser) | 11 | "B" |
| E (Editor) | 12 | "E" |

`appRules.assignToWorkspace` は rawID で指定（例：M なら `"10"`）。
`omniwmctl workspace focus-name M` は displayName / rawName いずれも受け付ける。

## 既知の制約

- **specificDisplay の monitorAssignment**：v0.4.8 で TOML decode が壊れているため、profiles では `main` / `secondary` のみ使用。AeroSpace のように特定モニタへ厳密に固定したい場合は OmniWM GUI で個別設定する。
- **設定のホットリロード無し**：`switch-profile.sh` がモニタ枚数変化を検知して TOML を差替え、`launchctl kickstart -k gui/$UID/org.nixos.omniwm` で OmniWM をプロセス再起動する。一瞬のチラつきあり。
- **focus-monitor の3 モニタ以上での挙動**：`focus-monitor prev/next` は活性 workspace のないモニタを跨げない。実用上は 2 枚モニタ運用で問題なし。

## キーバインド一覧（niri 流に最適化）

### OmniWM ネイティブ（hotkeys.nix）

#### ワークスペース
- `Option+1〜9`：WS 1〜9 切替
- `Option+Shift+1〜9`：ウィンドウを WS 1〜9 へ送る
- `Option+Tab`：直前 WS に戻る

#### ウィンドウフォーカス・移動
- `Option+H/J/K/L`：フォーカス移動（左/下/上/右）
- `Option+Shift+H/J/K/L`：ウィンドウ移動（左/右で隣 column への consume 含む）
- `Option+P`：直前のフォーカスウィンドウに戻る

#### Column 操作（niri 流）
- `Control+Option+1〜9`：現 WS の N 番目の column に直接ジャンプ
- `Option+Home` / `Option+End`：WS の最初/最後の column へ
- `Control+Option+Shift+H/L`：column 単位で左右に動かす
- `Option+,` / `Option+.`：column 幅プリセットを巡回（戻り/進み）
- `Option+T`：column 内のウィンドウをタブ表示にトグル（旧 accordion 相当）
- `Option+Shift+F`：column を全幅化（一時的に集中表示）
- `Option+Shift+B`：全 column の幅を均等化

#### レイアウト・モード
- `Option+/`：WS のレイアウトを niri ⇄ dwindle で切替
- `Option+Return`：OmniWM 管理のフルスクリーン
- `Option+Shift+Space`：floating ⇄ tiling

#### UI / Discoverability
- `Option+Shift+O`：Overview（全 WS 俯瞰）
- `Option+Shift+R`：floating ウィンドウを全部最前面に
- `Control+Option+Space`：Command Palette（全コマンド検索）
- `Control+Option+M`：Open Menu Anywhere（フォーカスウィンドウのメニュー召喚）
- ``Option+` ``：Quake terminal（OmniWM 内蔵 libghostty）

### Karabiner 補助レイヤ（karabiner-rules.nix）

OmniWM ネイティブで実装できないものだけ補完。

- `Option+M/B/E`：WS M/B/E（名前指定）切替
- `Option+Shift+M/B/E`：ウィンドウを WS M/B/E へ送る
- `Option+S`：WS M に行って Spotify 起動
- `Option+C`：WS M に行って Discord 起動
- `Option+A`：WS M に行って Calendar 起動
- `Option+Control+M`：メディアアプリ一括起動（setup-media-workspace、簡略化版）
- `Option+Control+H/J/K/L`：方向ベース focus-monitor（OmniWM の prev/next を補強）
- `Cmd+H` / `Cmd+Option+H`：macOS Hide ブロック（tile WM を保護）

## 切替手順（AeroSpace ↔ OmniWM）

`profiles/darwin.nix` の以下を反転：

```nix
myConfig.darwin.aerospace.enable = false;
myConfig.darwin.omniwm.enable    = true;
```

から

```nix
myConfig.darwin.aerospace.enable = true;
myConfig.darwin.omniwm.enable    = false;
```

そして：

```bash
sudo darwin-rebuild switch --flake .#yuta
```

`borders.enable` は `aerospace.enable` に連動するため自動で切替わる
（OmniWM 時は内蔵 border、AeroSpace 時は JankyBorders）。

## 初回セットアップ（OmniWM 初導入時のみ）

1. `darwin-rebuild switch` で OmniWM が brew cask 経由で /Applications/OmniWM.app に
   インストールされる
2. システム設定 → プライバシーとセキュリティ → アクセシビリティで OmniWM を有効化
3. システム設定 → デスクトップとDock → Mission Control で「ディスプレイごとに別の操作スペース」を OFF
   （nix の `system.activationScripts.systemTweaks` で自動設定済み）
4. メニューバーから OmniWM を Quit → launchd が自動起動を引き継ぐ

## トラブルシュート

### 設定が反映されない
1. `omniwmctl ping` で IPC 疎通確認
2. `~/.config/omniwm/settings.toml.corrupt` の有無で TOML decode 失敗を判定
3. `launchctl kickstart -k gui/$UID/org.nixos.omniwm` で再起動
4. プロファイル状態をリセット：`rm ~/.local/share/omniwm/monitor-count` → 再 kickstart

### OmniWM が起動しない
1. `pgrep -lx OmniWM` で確認
2. `launchctl list | grep omniwm` で launchd エージェント登録確認
3. `/usr/bin/log show --process OmniWM --last 1m` でログ確認

### モニタ抜き差し後にレイアウトが崩れる
- profile-switcher が 10 秒ポーリング → 自動で TOML 差替 + 再起動
- 即座に切替したい場合：`omniwm-switch-profile` を直接実行

### キーバインドが効かない
1. `~/.config/karabiner/karabiner.json` の rules セクションを確認
2. Karabiner-Elements GUI で OmniWM 関連ルールが ON になっているか確認
3. `omniwmctl command focus left` 等で IPC 経由の動作確認

## 参照

- 上位ドキュメント: [/OMNIWM.md](/OMNIWM.md) — OmniWM 仕様の網羅カタログ
- AeroSpace 旧実装: [/modules/darwin/aerospace/](/modules/darwin/aerospace/) — 切替時のリファレンス
- プロジェクトパターン: [/AGENTS.md](/AGENTS.md)
