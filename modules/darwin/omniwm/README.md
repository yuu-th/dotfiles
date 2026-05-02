# OmniWM モジュール

scrollable tiling WM である [OmniWM](https://github.com/BarutSRB/OmniWM) を nix-darwin で
declarative に管理する。AeroSpace 旧実装は `modules/darwin/aerospace/` に温存され、
profile 層のフラグ反転だけで切り戻せる。

## 設計思想

- **niri のスクロール column 哲学に最適化**：固定タイル分割ではなく、横にスクロールする column 列。
- **ユーザの哲学は維持**：12 ワークスペース構成（1〜9 + M / B / E）と特定アプリへの WS 振分。
- **モニタ構成変化に堅牢**：プロファイル auto 検出 + runtime resolution + フォールバックで crash しない。
- **OmniWM ネイティブ機能を活用**：Quake terminal、Overview、Command Palette、Scratchpad、トラックパッド ジェスチャ、フォーカス column 中央化等。

## ファイル構成

```
modules/darwin/omniwm/
├── default.nix              # myConfig.darwin.omniwm.enable で有効化
├── common.nix               # gaps / borders / focus / niri / dwindle 等の共通 TOML
├── hotkeys.nix              # OmniWM ネイティブ [[hotkeys]] 定義
├── app-rules.nix            # [[appRules]] (layout / minWidth のみ。assignToWorkspace は使わない)
├── workspace-assignment.nix # ★ bundleId → WS rawID マッピング（startup-sort が読む）
├── workspace-builder.nix    # [[workspaces]] 配列を生成するヘルパ
├── karabiner-rules.nix      # Karabiner 補助ルール
├── monitor-profiles/        # ★ モニタ構成プロファイル群（v3）
│   ├── README.md            # プロファイル追加手順
│   ├── default.nix          # main/secondary 汎用フォールバック
│   └── office-3mon.nix      # 例: Built-in + HP V27ie G5 + 名前なしモニタ
└── scripts/
    ├── deploy.sh                   # プロファイル選択 + runtime resolve + deploy
    ├── startup-sort.sh             # ★ OmniWM 起動時の one-shot ウィンドウ整列
    ├── focus-monitor-dir.sh        # 方向ベース focus-monitor
    ├── setup-media-workspace.sh    # WS M に Calendar/Spotify/Discord 起動
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

## モニタプロファイル（v3）

### 動作原理

```
profiles/darwin.nix:
  myConfig.darwin.omniwm.monitorProfile = "auto";  ← 既定: 自動検出
                                       = "<name>"; ← 強制指定も可

           ↓
deploy.sh が起動時 / モニタ抜き差し時に：
  1. system_profiler でモニタ情報取得
  2. monitorProfile が "auto" → 各 profile.match を評価して最も specific なものを選択
                   = "<name>" → そのプロファイルを強制
  3. 選んだ TOML の specificDisplay placeholder (`displayId = 0`) を実 displayId に置換
  4. 解決失敗 (モニタ不在) → `monitorAssignment = "secondary"` フォールバック (crash 回避)
  5. ~/.config/omniwm/settings.toml に書き込み
  6. OmniWM kickstart
```

### プロファイル形式

`monitor-profiles/<name>.nix`：

```nix
{ helpers }:
let inherit (helpers) mkWorkspaces main secondary display unnamedDisplay;
in {
  match = {
    requiredDisplays = [ "HP V27ie G5" ];   # 全部接続必須
    requireUnnamed   = true;                 # 名前なしモニタ必須
    monitorCount     = 3;                    # （任意）モニタ枚数一致必須
  };

  workspaces = mkWorkspaces {
    monitorMap = {
      "M" = main;
      "B" = unnamedDisplay;
      "E" = display "HP V27ie G5";
      # ...
    };
    layoutMap = { "E" = "dwindle"; };
  };
}
```

`match` を持たない profile（`default.nix` 等）は **catch-all = 最終フォールバック**。

### 新しいプロファイルを追加する手順

1. `monitor-profiles/<新名前>.nix` を作成（既存を参考に）
2. `git add monitor-profiles/<新名前>.nix`
3. `sudo darwin-rebuild switch --flake .#yuta`
4. `monitorProfile = "auto"` ならモニタ環境に応じて自動選択される
5. ログ: `cat ~/.local/share/omniwm/deploy.log` で選ばれた profile 名を確認

### 既存プロファイル

| 名前 | 用途 | match |
|---|---|---|
| `default` | 汎用フォールバック (1〜N モニタ全対応) | なし |
| `office-3mon` | Built-in + HP V27ie G5 + 名前なし | `requiredDisplays=["HP V27ie G5"]`, `requireUnnamed=true` |

## マルチモニタ・マウスワープ（重要）

OmniWM の `mouseWarp` は「画面端でカーソルを別モニタに飛ばす」拡張機能。**完全な disable
設定は無く、モニタ数 > 1 で常時 ON**（公式 Issue #167 / #239 参照）。さらに `axis` は
`"horizontal"` か `"vertical"` の **単一軸しか取れない** ため、L 字や 3 モニタ多段配置だと:

- 軸外（例: horizontal 設定で縦方向）に動かすと anti-warp clamp で**カーソルが引き戻される**
- これは upstream commit **`1dd47e2` (2026-05-02)** で修正済だが、まだリリース未反映
  （v0.4.8 が現在 brew cask 最新）。次タグ (v0.4.8.1 / v0.4.9) を待つ

### 暫定運用ルール

`common.nix` で以下を強制している（全プロファイル共通）:

```nix
mouseWarp = {
  axis = "vertical";
  margin = 5;
  monitorOrder = [ ];   # 空 = OmniWM が axis に従って frame.minY 昇順で auto-sort
};
```

→ **macOS Settings → ディスプレイ → 配置タブで 3 モニタを縦一列に並べること**。
上から `HP V27ie G5` → 名前なしモニタ → Built-in。`monitorOrder` は空でも、
macOS Settings 側の Y 座標で OmniWM が正しい順序に並べてくれる。X オフセットも揃えると
warp が滑らか。

#### ⚠️ `monitorOrder` を明示指定すると IPC が壊れる

`monitorOrder = [{ name = "..."; displayId = 0; }, ...]` の形式は OmniWM 側 schema と
非互換で、settings.toml decode 失敗 → IPC server (`~/Library/Caches/com.barut.OmniWM/ipc.sock`)
が作られず → `omniwmctl` 経由のキーバインド（alt+M/B/E など）が全部 silent fail する。
プロセスは生きてるが半起動状態になり気付きにくい。**監視は `omniwmctl ping` で確認**。
1 モニタ時は `shouldUseMouseWarp` が false 返すので無害。

### upstream リリース後にやること

`commit 1dd47e2` を含む release が brew cask に降りたら、以下に戻して horizontal も使えるか
試す:

```nix
mouseWarp = {
  axis = "horizontal";  # or "vertical"
  margin = 5;
  monitorOrder = [ ];   # auto-sort
};
```

issue #167 / PR #231 (`both` axis) も watch すると進展が分かる。

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
| `Option+T` | column 内のウィンドウをタブ表示にトグル |
| `Option+Shift+F` | column を全幅化 |
| `Option+=` | 全 column の幅を均等化（balanceSizes） |

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
| **`Option+Shift+R`** | **floating ウィンドウを全部最前面に (raiseAllFloatingWindows)** |
| **`Control+Option+Shift+R`** | **画面外ウィンドウを呼び戻す (rescueOffscreenWindows)** |
| `Control+Option+Space` | Command Palette |
| `Option+\`` | Quake terminal（OmniWM 内蔵） |

### Karabiner 補助レイヤ

| キー | 操作 |
|---|---|
| `Option+M/B/E` | 名前指定 WS 切替（rawID 10/11/12） |
| **`Option+Shift+M/B/E`** | **フォーカスウィンドウを WS M/B/E へ送る + 追従** |
| `Option+S` | WS M に行って Spotify 起動 |
| `Option+C` | WS M に行って Discord 起動 |
| `Option+A` | WS M に行って Calendar 起動 |
| `Option+Control+M` | メディアアプリ一括起動 |
| `Option+Control+H/J/K/L` | 方向ベース focus-monitor |
| `Option+Space` | OmniWM Quake terminal トグル |
| `Cmd+H` / `Cmd+Option+H` | macOS Hide ブロック |

## アプリ→ワークスペース整列（起動時 one-shot）

### 設計

- **`appRules.assignToWorkspace` は使わない** — OmniWM のルールは window 再検知（タブ昇格・
  Cmd+N・Spaces 復元等）で再発火し、手動移動した window が「引き戻される」問題があるため
- 代わりに `scripts/startup-sort.sh` が **OmniWM 起動完了直後に 1 回だけ走り**、現存
  ウィンドウを `workspace-assignment.nix` のマップに従って整列させる
- セッション中の新ウィンドウは**振分しない**（フォーカス中の WS で開く）。手動移動も**絶対に保持される**

### マッピング（`workspace-assignment.nix`）

| 着地先 | アプリ |
|---|---|
| WS B (rawID 11) | Chrome / Firefox / Safari / Dia / Zen Browser |
| WS E (rawID 12) | VSCode / Insiders / Zed / Cursor / IntelliJ / PyCharm / WebStorm / GoLand |
| WS 1 | Antigravity (Google AI agent) |
| WS M (rawID 10) | Spotify / Discord / Calendar / Music |
| WS 3 | iTerm2 / Terminal.app |
| WS 4 | Slack / Microsoft Teams |
| WS 5 | Notion / Obsidian |
| floating (`app-rules.nix`) | Finder / System Settings / Calculator / Dictionary / Activity Monitor / Console / QuickTime / PhotoBooth / Keynote / Pages / Numbers / Minecraft / Raycast / 1Password / iMessage / UTM |

### 起動時整列のフロー

```
launchd agent (org.nixos.omniwm):
  1. wait4path /nix/store
  2. omniwm-deploy  (settings.toml 配置 / kickstart)
  3. OmniWM.app を background 起動
  4. omniwm-startup-sort を background で発火
       ├─ omniwmctl ping を最大 10s polling
       ├─ 全 window 列挙 → bundleId をマップで lookup
       └─ 該当があれば window focus + command move-to-workspace
  5. wait $OMNIWM_PID  (KeepAlive 用)
```

### 整列を手動で再実行したい時

```bash
omniwm-startup-sort   # PATH に通ってる
# または
launchctl kickstart -k gui/$UID/org.nixos.omniwm   # OmniWM ごと再起動 → 自動で整列
```

ログ: `~/.local/share/omniwm/startup-sort.log`

### マッピングを増やす / 変えるには

1. `workspace-assignment.nix` を編集（`"<bundleId>" = "<rawID>"`）
2. `sudo darwin-rebuild switch --flake .#yuta`
3. 次の OmniWM 起動 / kickstart で反映される

bundleId の調べ方:
```bash
osascript -e 'id of app "Chrome"'
# または起動中の window から
omniwmctl query windows --focused --format json | jq -r '.result.payload.windows[].app.bundleId'
```

### セッション中に特定 WS でアプリを起動したい

`scripts/ws-launch.sh` のパターン（既存の `Option+S/C/A` と同型）。Karabiner ルールに足す:

```nix
(rules1 "OmniWM: alt-w = WS B + Chrome"
  (basicShell "w" [ "option" ] "${wsLaunch}/bin/omniwm-ws-launch B \"Google Chrome\""))
```

## 切替手順（AeroSpace ↔ OmniWM）

`profiles/darwin.nix` を編集：

```nix
# OmniWM
myConfig.darwin.aerospace.enable = false;
myConfig.darwin.omniwm.enable    = true;

# AeroSpace に戻す
myConfig.darwin.aerospace.enable = true;
myConfig.darwin.omniwm.enable    = false;
```

そして：

```bash
sudo darwin-rebuild switch --flake .#yuta
```

## 初回セットアップ

1. `darwin-rebuild switch` で OmniWM が brew cask 経由でインストールされる
2. システム設定 → プライバシーとセキュリティ → アクセシビリティで OmniWM を有効化
3. メニューバーから OmniWM を Quit → launchd が自動起動を引き継ぐ

## トラブルシュート

### 設定が反映されない
1. `omniwmctl ping` で IPC 疎通確認
2. `~/.config/omniwm/settings.toml.corrupt` の有無で TOML decode 失敗を判定
3. `cat ~/.local/share/omniwm/deploy.log` で deploy 実行履歴を確認
4. `launchctl kickstart -k gui/$UID/org.nixos.omniwm` で再起動

### OmniWM が起動しない / クラッシュループ
1. `pgrep -lx OmniWM` で確認
2. `launchctl list | grep omniwm` で launchd 登録確認
3. `/usr/bin/log show --process OmniWM --last 1m --info` でログ
4. クラッシュレポート: `~/Library/Logs/DiagnosticReports/OmniWM*.ips`

### モニタ抜き差し後の挙動
- watcher (`omniwm-display-watcher`) が `display-changed` IPC イベントを subscribe
- イベント発火時に deploy.sh が走り、`monitorProfile = "auto"` ならプロファイル再評価
- ログ: `~/.local/share/omniwm/deploy.log`

### Floating ウィンドウが見えない / 行方不明
- **`Option+Shift+R`** → raiseAllFloatingWindows（全 floating を最前面に）
- **`Control+Option+Shift+R`** → rescueOffscreenWindows（画面外から呼び戻し）
- **`Option+Shift+O`** → Overview（全 WS 俯瞰、視覚的に探す）
- `omniwmctl query windows --floating --format json` で位置確認

App rules で float に指定したアプリ（Finder, System Settings, 1Password 等）が
タイル化されてしまう場合：
- `omniwmctl query focused-window-decision` でルール適用状態を確認
- 該当アプリを再起動（appRules は新しいウィンドウにのみ適用）

### 起動時整列が走らない / 一部 window が振分されない
1. `cat ~/.local/share/omniwm/startup-sort.log` で実行履歴を確認
2. ログ末尾 `done: moved=N skipped=M` の `moved` が想定より少ないなら：
   - bundleId が `workspace-assignment.nix` に登録されてるか
   - 振分されないアプリの bundleId を `omniwmctl query windows ... | jq` で実値確認
3. ログに `IPC not ready after 10s` が出てたら、OmniWM 起動が遅すぎる ⇒
   `omniwm-startup-sort` を手動で実行
4. `omniwm-startup-sort` を直接実行できるので、何度でも整列やり直せる

### Option+Shift+M/B/E で window が動かない
1. `cat ~/.local/share/omniwm/move-window.log` で実行履歴確認
2. ログに「move-to-workspace: executed」の行があれば成功
3. それでも動かない場合、focused-window が null かも：
   `omniwmctl query focused-window --format json` で確認

### 空 WS にウィンドウを送る方法
- `Option+Shift+1〜9` → WS 1〜9 へ送る（OmniWM ネイティブ）
- `Option+Shift+M` → WS M へ送る（Karabiner shell 経由）
- `Option+Shift+B` → WS B へ送る
- `Option+Shift+E` → WS E へ送る

送った後、自動でその WS にフォーカスが追従するので、移動先で確認できる。

### キーバインドが効かない
1. Karabiner-Elements が起動中か（`pgrep -fl karabiner`）
2. `~/.config/karabiner/karabiner.json` の rules を確認
3. **`omniwmctl ping` が `pong` 返すか**（IPC server 起動確認）
4. `omniwmctl command focus left` で IPC 経由コマンド疎通確認
5. `Option+L` で `¬` が出る → OmniWM がクラッシュしてキーが OS に流れている

#### IPC が応答しない（`omniwmctl ping` が `No such file or directory`）

**症状**: OmniWM プロセスは生きてる (`pgrep -lx OmniWM`) のに `omniwmctl` 全コマンドが
ENOENT を返し、alt+M/B/E 等の Karabiner shell rule が無反応になる。

**原因**: settings.toml の schema 不整合で OmniWM の起動シーケンスが半端に停止し、
IPC socket (`~/Library/Caches/com.barut.OmniWM/ipc.sock`) が作られていない状態。

**確認**:
```bash
ls -la ~/Library/Caches/com.barut.OmniWM/ipc.sock   # 無ければ半起動
/opt/homebrew/bin/omniwmctl ping                       # No such file or directory
```

**対処**:
1. 直近 `common.nix` 等で TOML schema を変えた箇所を疑い、最小構成に revert
2. `darwin-rebuild switch` で再 deploy
3. `launchctl kickstart -k gui/$UID/org.nixos.omniwm` で再起動
4. ping が `pong` 返ったら復旧

### モニタ配置が勝手に変わる
- DisplayLink Manager が動作中の場合: 一時停止して挙動確認
  `pkill -9 DisplayLink`
- macOS の System Settings → ディスプレイ で配置を再設定
- OmniWM 自体は macOS の display arrangement を変更しない

### モニタ境界でカーソルが引き戻される
**症状**: 隣のモニタにカーソルを移動すると、少し入った瞬間に元のモニタに戻される。

**原因**: OmniWM の anti-warp clamp ロジック（軸外の cross-monitor 移動を強制 clamp）。
特に 3 モニタ L 字配置で `axis = "horizontal"` だと、縦方向の移動が引き戻される。

**対処**:
1. macOS Settings → ディスプレイ → 配置を**縦一列**に変更
   （上から HP V27ie G5 → 名前なし → Built-in）
2. `common.nix` の `mouseWarp.axis = "vertical"` を確認（既定値）
3. 上記「マルチモニタ・マウスワープ」セクションを参照

**根本 fix**: upstream `commit 1dd47e2` (2026-05-02) で clamp 削除済。次リリース待ち。

## 参照

- 上位仕様: [/OMNIWM.md](/OMNIWM.md) — OmniWM 機能の網羅カタログ
- monitor-profile 詳細: [monitor-profiles/README.md](monitor-profiles/README.md)
- AeroSpace 旧実装: [/modules/darwin/aerospace/](/modules/darwin/aerospace/)
- プロジェクトパターン: [/AGENTS.md](/AGENTS.md)
- OmniWM 公式: <https://github.com/BarutSRB/OmniWM>
