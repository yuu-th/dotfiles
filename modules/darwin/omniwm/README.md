# OmniWM モジュール

scrollable tiling WM である [OmniWM](https://github.com/BarutSRB/OmniWM) を nix-darwin で
declarative に管理する。AeroSpace 旧実装は `modules/darwin/aerospace/` に温存され、
profile 層のフラグ反転だけで切り戻せる。

## 設計思想

- **niri のスクロール column 哲学に最適化**：固定タイル分割ではなく、横にスクロールする column 列。
- **ユーザーの哲学は維持**：12 ワークスペース構成（1〜9 + M / B / E）と特定アプリへの WS 振分。
- **AeroSpace 由来の擬似機能は削除**：resize モード・toggle-floating の重複バインド・大量の Unassigned ホットキー等。
- **OmniWM ネイティブ機能を活用**：Quake terminal、Overview、Command Palette、Scratchpad、トラックパッド ジェスチャ、フォーカス column 中央化等。

## ファイル構成

```
modules/darwin/omniwm/
├── default.nix              # myConfig.darwin.omniwm.enable で有効化
├── common.nix               # gaps / borders / focus / niri / dwindle 等の共通 TOML
├── hotkeys.nix              # OmniWM ネイティブ [[hotkeys]] 定義
├── app-rules.nix            # [[appRules]] (アプリ→WS の自動割当)
├── workspace-builder.nix    # [[workspaces]] 配列を生成するヘルパ
├── karabiner-rules.nix      # Karabiner 補助ルール（OmniWM ネイティブで足りない部分）
├── profiles/
│   ├── one-monitor.nix      # 1 枚モニタ（MacBook 単独）
│   ├── two-monitor.nix      # 2 枚モニタ（Built-in + 外部 1）
│   ├── triple-monitor.nix   # 3 枚モニタ（Built-in + HP V27ie + 名前なし）
│   └── quad-monitor.nix     # 4 枚モニタ（5 枚以上のフォールバックも兼用）
└── scripts/
    ├── switch-profile.sh           # モニタ枚数検出 → TOML 差替 + displayId 置換 → OmniWM 再起動
    ├── focus-monitor-dir.sh        # 方向ベース focus-monitor の自前実装
    ├── setup-media-workspace.sh    # WS M に Calendar/Spotify/Discord を起動（簡略版）
    ├── ws-launch.sh                # WS切替+open -a (alt-s/c/a 用)
    └── move-window-to-named-ws.sh  # 名前指定 WS への送り＋ジャンプ
```

## ワークスペース命名

OmniWM の workspace `name` (rawID) は **数値のみ受理**されるため、AeroSpace の名前付き WS は
内部的に rawID へ変換し displayName で人間可読ラベルを保つ。

| 論理名 | rawID | displayName | 用途 |
|---|---|---|---|
| 1〜9 | 1〜9 | （rawName と同じ） | 数値 WS（自由用途） |
| M (Media) | 10 | "M" | Calendar / Spotify / Discord |
| B (Browser) | 11 | "B" | ブラウザ各種 |
| E (Editor) | 12 | "E" | VSCode / Cursor / IntelliJ 等（Dwindle BSP） |

`appRules.assignToWorkspace` は **rawID** で指定（M なら `"10"`）。
`omniwmctl workspace focus-name` は rawName / displayName 両方を受理。

## モニタ構成プロファイル

`switch-profile.sh` がモニタ枚数を検出し、対応する TOML を `~/.config/omniwm/settings.toml` に
コピーして OmniWM をプロセス再起動する。10 秒ポーリングのデーモンが抜き差しを自動検知する。

| 枚数 | プロファイル | 想定構成 |
|---|---|---|
| 1 | `one-monitor.nix` | MacBook 単独（ノマド・カフェ・通勤先） |
| 2 | `two-monitor.nix` | Built-in + LCD-MF234X 等 |
| 3 | `triple-monitor.nix` | Built-in + HP V27ie + 名前なしモニタ |
| 4 | `quad-monitor.nix` | Built-in + DIOS-MF241X + L2235HW + 名前なし |
| 5+ | (fallback) `quad-monitor.nix` | 4 枚プロファイルにフォールバック |

### 厳密モニタピン留めの仕組み

OmniWM の `monitorAssignment.specificDisplay` は **`name` だけでは decode に失敗** するため、
`displayId` (CGDirectDisplayID) を付与する必要がある。displayId はハードウェア依存で
nix ビルド時には不明なので、以下の二段構えで対処：

1. **名前付きモニタ**：Nix が `displayId = 0` + `name = "X"` を出力。OmniWM は displayId が 0 (= noRuntimeDisplayId) のとき name で fallback マッチして resolve する。
2. **名前なしモニタ**：Nix が placeholder `displayId = 999000` + `name = ""` を出力。`switch-profile.sh` が `system_profiler SPDisplaysDataType -json` から実 displayId を取得して `sed` で置換。

これにより、AeroSpace の `workspace-to-monitor-force-assignment` と同等の厳密ピン留めが実現される。

### 既知の制約

- OmniWM v0.4.8 は `monitorAssignment.specificDisplay` の TOML decode に
  `displayId > 0` を要求する。`name` 単独では rejected → `~/.config/omniwm/settings.toml.corrupt` 行き。
- `displayId = -1` などの負値は OmniWM をクラッシュさせる。
- 設定ホットリロード無し → `switch-profile.sh` が `launchctl kickstart -k gui/$UID/org.nixos.omniwm` で
  プロセス再起動する。一瞬チラつくがモニタ抜き差し時にしか発生しない。

## キーバインド完全一覧

### OmniWM ネイティブ（hotkeys.nix）

#### ワークスペース
| キー | 操作 |
|---|---|
| `Option+1〜9` | WS 1〜9 切替 |
| `Option+Shift+1〜9` | フォーカスウィンドウを WS 1〜9 へ送る |
| `Option+Tab` | 直前 WS に戻る |

#### ウィンドウフォーカス・移動
| キー | 操作 |
|---|---|
| `Option+H/J/K/L` | 左/下/上/右 にフォーカス移動 |
| `Option+Shift+H/J/K/L` | 左/下/上/右 にウィンドウ移動（左右は隣 column への consume 含む） |
| `Option+P` | 直前のフォーカスウィンドウへ |

#### Column 操作（niri 流の主役）
| キー | 操作 |
|---|---|
| `Control+Option+1〜9` | 現 WS の N 番目の column に直接ジャンプ |
| `Option+Home` / `Option+End` | WS の最初/最後の column |
| `Control+Option+Shift+H/L` | column 単位で左右に動かす |
| `Option+,` / `Option+.` | column 幅プリセットを巡回（戻り/進み） |
| `Option+T` | column 内のウィンドウをタブ表示にトグル（旧 accordion 相当） |
| `Option+Shift+F` | column を全幅化 |
| `Option+=` | 全 column の幅を均等化（balanceSizes、"=" は均等のニーモニック） |

#### レイアウト
| キー | 操作 |
|---|---|
| `Option+/` | WS のレイアウトを niri ⇄ dwindle で切替 |
| `Option+Return` | OmniWM 管理のフルスクリーン |
| `Option+Shift+Space` | floating ⇄ tiling |

#### UI / Discoverability / 救済
| キー | 操作 |
|---|---|
| `Option+Shift+O` | Overview（全 WS 俯瞰） |
| `Option+Shift+R` | floating ウィンドウを全部最前面に |
| `Control+Option+Space` | Command Palette（全コマンド検索） |
| ``Option+` `` | Quake terminal（OmniWM 内蔵 libghostty） |
| `Control+Option+Shift+R` | rescueOffscreenWindows（画面外ウィンドウ呼び戻し） |

### Karabiner 補助レイヤ（karabiner-rules.nix）

OmniWM ネイティブで実装できないものだけ補完。

| キー | 操作 |
|---|---|
| `Option+M/B/E` | 名前指定 WS 切替（rawID 10/11/12 への shortcut） |
| `Option+Shift+M/B/E` | フォーカスウィンドウを WS M/B/E へ送る |
| `Option+S` | WS M に行って Spotify 起動 |
| `Option+C` | WS M に行って Discord 起動 |
| `Option+A` | WS M に行って Calendar 起動 |
| `Option+Control+M` | メディアアプリ一括起動（簡略版） |
| `Option+Control+H/J/K/L` | 方向ベース focus-monitor（OmniWM の prev/next を補強） |
| `Cmd+H` / `Cmd+Option+H` | macOS Hide ブロック（tile WM を保護） |

## アプリ→ワークスペース自動振分（appRules）

| 着地先 | アプリ |
|---|---|
| WS B (rawID 11) | Chrome / Firefox / Safari / Dia / Zen Browser |
| WS E (rawID 12) | VSCode / VSCode Insiders / Zed / Cursor / IntelliJ / PyCharm / WebStorm / GoLand |
| WS 1 | Antigravity (Google AI agent) |
| WS M (rawID 10) | Spotify / Discord / Calendar / Music |
| WS 3 | iTerm2 / Terminal.app |
| WS 4 | Slack / Microsoft Teams |
| WS 5 | Notion / Obsidian |
| floating | Finder / System Settings / Calculator / Dictionary / Activity Monitor / Console / QuickTime / PhotoBooth / Keynote / Pages / Numbers / Minecraft / Raycast / 1Password / iMessage / UTM |

`Ghostty` は自動移動なし（現在の WS にそのまま配置）、最小サイズだけ強制。

## 切替手順（AeroSpace ↔ OmniWM）

`profiles/darwin.nix` を編集：

```nix
# OmniWM を使う場合
myConfig.darwin.aerospace.enable = false;
myConfig.darwin.omniwm.enable    = true;

# AeroSpace に戻す場合
myConfig.darwin.aerospace.enable = true;
myConfig.darwin.omniwm.enable    = false;
```

そして：

```bash
sudo darwin-rebuild switch --flake .#yuta
```

`borders.enable` は `aerospace.enable` に連動するため自動で切替わる
（OmniWM 時は内蔵 border、AeroSpace 時は JankyBorders）。
profile 層の assertion で同時 enable は禁止されている。

## 初回セットアップ（OmniWM 初導入時のみ）

1. `darwin-rebuild switch` で OmniWM が brew cask 経由でインストールされる
2. システム設定 → プライバシーとセキュリティ → アクセシビリティで OmniWM を有効化
3. システム設定 → デスクトップとDock → Mission Control で「ディスプレイごとに別の操作スペース」を OFF
   （`profiles/darwin.nix` の `system.activationScripts.systemTweaks` で自動設定済み）
4. メニューバーから OmniWM を Quit → launchd が自動起動を引き継ぐ

## トラブルシュート

### 設定が反映されない
1. `omniwmctl ping` で IPC 疎通確認
2. `~/.config/omniwm/settings.toml.corrupt` の有無で TOML decode 失敗を判定
   （存在すれば直近の load で何かが rejected）
3. `launchctl kickstart -k gui/$UID/org.nixos.omniwm` で再起動
4. プロファイル状態をリセット：`rm ~/.local/share/omniwm/monitor-count` → 再 kickstart

### OmniWM が起動しない / クラッシュループ
1. `pgrep -lx OmniWM` で確認
2. `launchctl list | grep omniwm` で launchd エージェント登録確認
3. `/usr/bin/log show --process OmniWM --last 1m --info` でログ確認
4. **クラッシュ多発時**：`displayId = -1` 等の不正値が settings.toml に残ってる可能性 → corrupt 退避済の可能性が高い

### モニタ抜き差し後にレイアウトが崩れる
- 二段構えで自動検知される：
  - **イベント駆動**: `omniwm-display-watcher` (launchd) が `omniwmctl watch display-changed` で
    モニタ変化を即座に subscribe → switch-profile を発火
  - **10 秒ポーリング (fallback)**: `omniwm-profile-switcher` (launchd) が定期監視。
    watcher が落ちた場合のセーフティネット
- 即座に手動切替したい場合：`omniwm-switch-profile` を直接実行
- 動作ログ：`~/.local/share/omniwm/switch-profile.log`

### 名前なしモニタ（display:N で name=""）の挙動
- `switch-profile.sh` が `system_profiler` から実 displayId を取得し placeholder を置換
- 接続順や USB-C dock の構成で displayId が変わる可能性 → 抜き差ししても基本追従するが、
  特定モニタへ厳密に固定したい場合は GUI で specificDisplay を再設定すると永続化される

### キーバインドが効かない
1. `~/.config/karabiner/karabiner.json` の rules セクションを確認
2. Karabiner-Elements GUI で OmniWM 関連ルールが ON になっているか確認
3. `omniwmctl command focus left` 等で IPC 経由の動作確認
4. `omniwmctl query commands` でホットキー登録状態を確認
5. `Option+L` を押すと `¬` が出る → OmniWM がクラッシュしてホットキーが OS に流れている。
   `pgrep -lx OmniWM` で確認、無ければ `launchctl kickstart -k gui/$UID/org.nixos.omniwm`

### Floating ウィンドウが見えない / 行方不明
OmniWM が floating ウィンドウを画面外や別 WS に置いてしまった場合：
- **`Option+Shift+R`** → raiseAllFloatingWindows（全 floating を最前面に）
- **`Control+Option+Shift+R`** → rescueOffscreenWindows（画面外のウィンドウを呼び戻す）
- **`Option+Shift+O`** → Overview（全 WS を俯瞰、視覚的に探す）
- それでも見つからない → `omniwmctl query windows --floating --format json` で位置確認

App rules で float に指定したアプリ（Finder, System Settings, 1Password 等）が
タイル化されてしまう場合：
- `omniwmctl query focused-window-decision` でルール適用状態を確認
- 該当アプリを再起動（appRules は新しいウィンドウにのみ適用）

### モニタプロファイルの切替
出張先で外部モニタが違う等の場合：
1. `monitor-profiles/<新しい名前>.nix` を作成（既存を参考に）
2. `profiles/darwin.nix` で `myConfig.darwin.omniwm.monitorProfile = "<新しい名前>";`
3. `git add` → `sudo darwin-rebuild switch --flake .#yuta`

プロファイルが現状のモニタと完全には一致しなくても、`deploy.sh` が
matched しないモニタは `secondary` にフォールバックするため crash しない。

### Display 名の確認方法
```bash
# OmniWM 起動中
omniwmctl query displays --format json | jq -r '.result.payload.displays[].name'
# OmniWM 起動前
system_profiler SPDisplaysDataType -json | jq -r '.SPDisplaysDataType[].spdisplays_ndrvs[]?._name'
```
注意: macOS の Built-in は system_profiler では "Color LCD"、OmniWM IPC では
"Built-in Retina Display" と異なる名前で見える。プロファイルでは **`main`** を
使うのが確実（macOS が primary display として resolve）。

### omniwmctl: Connection refused
1. `general.ipcEnabled = true` が settings.toml にあるか
2. OmniWM が起動中か（`pgrep -lx OmniWM`）
3. ipc.sock 存在確認：`ls ~/Library/Caches/com.barut.OmniWM/ipc.sock`
4. 上記すべて OK なら IPC が壊れている可能性 → kickstart で再起動

### WS M (rawID 10) の挙動が変
- rawID は数値のみだが OmniWM の switchWorkspace.0..8 は最大 .8 まで
  → M/B/E (rawID 10/11/12) はネイティブ hotkey で直接呼べない
  → Karabiner の `Option+M/B/E` 経由で `omniwmctl workspace focus-name M` を発行する設計

## 高度な使い方

### WS ごとに layout を切替
WS E は dwindle (BSP)、それ以外は niri (scroll) で起動する。
動的に変えたい場合は `Option+/` でその WS のレイアウトを toggle。

### Quake terminal
``Option+` `` でフォーカス中のモニタ中央にスライド表示。OmniWM 内蔵 libghostty。
既存の Ghostty.app と併存可能。

### Command Palette
`Control+Option+Space` で全コマンドを検索可能。バインドしていない action も
ここから探せる（scratchpad 系等）。

### Scratchpad
「ウィンドウを一時的に隠す」OmniWM 独自機能。
Command Palette から `assign focused window to scratchpad` / `toggle scratchpad window` を呼べる。
頻用するならキーバインドを `hotkeys.nix` に追加：
```nix
{ binding = "Option+Shift+Backslash"; id = "assignFocusedWindowToScratchpad"; }
{ binding = "Option+Backslash";       id = "toggleScratchpadWindow"; }
```

### IPC 経由の自動化
`omniwmctl watch <channel>` でイベント駆動スクリプトが書ける：
```bash
omniwmctl watch focus -- /path/to/on-focus-change.sh
omniwmctl watch display-changed -- /path/to/on-monitor-change.sh
```
チャネル: `focus` / `workspace-bar` / `active-workspace` / `focused-monitor` /
`windows-changed` / `display-changed` / `layout-changed`

## 参照

- 上位仕様ドキュメント: [/OMNIWM.md](/OMNIWM.md) — OmniWM 機能の網羅カタログ
- AeroSpace 旧実装: [/modules/darwin/aerospace/](/modules/darwin/aerospace/) — 切替時のリファレンス
- プロジェクトパターン: [/AGENTS.md](/AGENTS.md)
- OmniWM 公式: [BarutSRB/OmniWM](https://github.com/BarutSRB/OmniWM)
