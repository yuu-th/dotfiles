# OmniWM モジュール

scrollable tiling WM である [OmniWM](https://github.com/BarutSRB/OmniWM) を nix-darwin で
declarative に管理する。対象バージョン **0.5.9 / IPC protocol 8**。

AeroSpace 旧実装は `modules/darwin/aerospace/` に温存されており、`profiles/darwin.nix` の
フラグ反転だけで切り戻せる（両方 enable は assertion で禁止）。

## 前提（0.5.9 で変わったので注意）

| 項目 | 値 |
|---|---|
| OS | **macOS 26+ (Tahoe) / Apple Silicon 限定**（0.5.3 で Intel 対応終了） |
| Displays have separate Spaces | **ON 必須**（0.4.9.9 で必須化。OFF だと OmniWM は window management を停止する） |
| 権限 | Accessibility |
| 設定ファイル | `~/.config/omniwm/settings.toml`（**live reload される**。0.4.8.1 以降） |
| ランタイム状態 | `~/.local/state/omniwm/`（clipboard 履歴・更新チェック状態など） |
| 配布 | Homebrew cask (`BarutSRB/tap`) |

⚠️ 0.4.x 時代の「separate Spaces は OFF」「ホットリロード無し」という前提は**どちらも逆転している**。

## 設計思想

- **Niri のスクロール column 一本**。Dwindle は使わない（`[dwindle]` を出力しない・`toggleWorkspaceLayout` は Unassigned）
- **1 画面 1 窓**。`defaultContainerPrimarySpan = 1.0` / `visibleContainerCount = 1` で、切り替えは横スクロール
- **キー操作は全部 space レイヤ**（Karabiner）。OmniWM native hotkey は全 149 ID を `Unassigned` にし、緊急脱出の 2 つだけ残す
- **キーの上下段 = ディスプレイの上下層**という WS ルール
- **ワークスペースバーは常時表示**（今どの WS にいて何が開いているかを常に見せる）
- モニタ構成変化に堅牢: プロファイル auto 検出 + displayUUID の runtime 解決 + 段階的フォールバック

## ファイル構成

```
modules/darwin/omniwm/
├── default.nix              # myConfig.darwin.omniwm.enable で有効化
├── common.nix               # モニタ構成に依存しない全設定
├── hotkeys.nix              # ★ 149 action ID 全列挙（実機の write-back から機械生成）
├── app-rules.nix            # [[appRules]]（layout / minSize のみ）
├── workspace-assignment.nix # ★ bundleId → WS rawName（startup-sort が読む）
├── workspace-builder.nix    # [[workspaces]] 生成 + monitorAssignment / routing ヘルパ
├── karabiner-rules.nix      # ★ space レイヤの全キーバインド
├── monitor-profiles/
│   ├── default.nix          # catch-all（main/secondary のみ・routing は macOS 任せ）
│   ├── office-3mon.nix      # HP + 名前なし + 内蔵
│   └── hp-2mon.nix          # HP + 内蔵
└── scripts/
    ├── deploy.sh                   # プロファイル選択 + displayUUID 解決 + 配置
    ├── startup-sort.sh             # 起動時 one-shot ウィンドウ整列
    └── move-window-to-named-ws.sh  # 名前指定 WS への送り＋追従
```

## ワークスペース（18 個）

**ルール: キーの上下段 = ディスプレイの上下層**

| キー | rawName | 役割 | 3 台構成での着地 |
|---|---|---|---|
| `space+w` `space+e` `space+r` | 10 / 11 / 12 | メイン作業（プロジェクト 1/2/3） | HP V27ie G5（上） |
| `space+s` `space+d` `space+f` | 13 / 14 / 15 | そのプロジェクトのブラウザ | 名前なしモニタ（中） |
| `space+x` | 16 | Media（Spotify / Music） | 内蔵（下・main） |
| `space+c` | 17 | Chat（Discord / Slack / Teams / メッセージ） | 内蔵（下・main） |
| `space+v` | 18 | 予定・ノート（Calendar / Notion / Obsidian） | 内蔵（下・main） |
| `space+1` `space+2` | 1 / 2 | ad-hoc | 名前なしモニタ（中） |
| `space+3`〜`space+9` | 3–9 | ad-hoc | HP V27ie G5（上） |

`w/s` `e/d` `r/f` が「作業 / そのブラウザ」の縦ペア。`space+shift+<同じ字>` でその WS へ窓を送る。

OmniWM の workspace `name` は数値のみ受理されるため、人間可読ラベルは `displayName` で持つ。
rawName の連番が `space+]` `[` の巡回順になるので「数字 → 作業 → ブラウザ → 常駐」の順に振っている。

## キーバインド完全一覧

すべて Karabiner の space レイヤ（space を modifier のように押しながら）。

### ワークスペース
| キー | 操作 |
|---|---|
| `space+w e r s d f x c v` | その WS へ切替 |
| `space+1`〜`9` | 数字 WS へ切替 |
| `space+shift+<同じ字/数字>` | フォーカス窓をその WS へ送る＋追従 |
| `space+]` / `space+[` | WS next / prev |
| `space+tab` | 直前の WS に戻る |
| `space+shift+]` / `space+shift+[` | 窓を WS down / up へ |

### フォーカス・移動
| キー | 操作 |
|---|---|
| `space+h j k l` | 左/下/上/右にフォーカス（**WS 内で完結**。モニタは跨がない） |
| `space+;` | 直前の窓へ |
| `space+shift+h j k l` | 窓を移動（**端まで行くと隣モニタへ抜ける**） |
| `space+ctrl+tab` / `space+ctrl+shift+tab` | フォーカスモニタ next / prev |

### Niri column
| キー | 操作 |
|---|---|
| `space+ctrl+1`〜`9` | N 番目の column へジャンプ（**1-based**） |
| `space+ctrl+[` / `space+ctrl+]` | 最初 / 最後の column |
| `space+ctrl+shift+h` / `l` | column を左右に移動 |
| `space+ctrl+shift+1`〜`9` | column を WS N へ（**1-based**） |
| `space+ctrl+shift+[` / `]` | column を WS up / down へ |
| `space+shift+t` | column をタブ表示にトグル |
| `space+ctrl+,` / `space+ctrl+.` | 左 / 右の窓を column に取り込む・出す |

### サイズ
| キー | 操作 |
|---|---|
| `space+m` | fullscreen |
| `space+shift+m` | floating ⇄ tiling |
| `space+b` | 全幅トグル |
| `space+,` / `space+.` | 幅プリセット巡回（戻り / 進み） |
| `space+-` / `space+=` | 幅 ±10% |
| `space+shift+-` / `space+shift+=` | 高さ ±10% |
| `space+/` | 全 column の幅を均等化 |

### OmniWM 固有 UI・救済
| キー | 操作 |
|---|---|
| `space+g` | Quake Terminal |
| `space+t` | Command Palette（窓検索 / **クリップボード履歴** / メニュー検索） |
| `space+z` | Overview を開く（**閉じるのは Escape / Enter / 背景クリック**、後述） |
| `space+a` / `space+shift+a` | scratchpad 呼び出し / 今の窓を scratchpad へ |
| `space+n` / `space+shift+n` | floating を全部前面に / 画面外の窓を呼び戻す |
| `space+ctrl+g` | system stats |

### native に残しているもの（緊急脱出用）
Karabiner が落ちたり IPC が死んだときの最後の手段として二重に割り当てている。

| キー | 操作 |
|---|---|
| `Option+Return` | fullscreen 解除 |
| `Control+Option+Shift+R` | 画面外の窓を呼び戻す |

### 使ってはいけないキー
- `space+space` — space-leader が自分自身に再入する
- `space+return` — 文章入力で「行末に space → Enter」を打つと改行が消える
- `space+0` — 「space の後に数字」は入力中に起きるため増やさない

## Overview は modal（重要な制約）

upstream の docs/IPC-CLI.md より:

> Overview is modal with respect to external commands. `omniwmctl command toggle-overview` opens it
> while it is closed, but while Overview is open **every IPC/external command—including another
> `toggle-overview`—returns `ignored_overview`**.

つまり:

- `space+z` は **開く専用**。もう一度押しても閉じない
- 閉じるのは **Escape / Enter / 背景クリック**
- 0.5.6 の「Overview を開いたまま構造操作」は native hotkey 専用機能なので、
  全キーを space レイヤ（= IPC 経由）に置いた本構成では**使えない**。
  Overview は「探して選ぶ」用途に限定する

## マルチモニタ（0.5.0 で仕組みが変わった）

OmniWM は **2 枚の配置マップ**を使う。

1. **macOS の Arrange** — 技術的な配置。公式推奨は「一番大きい画面を下、次を右上に階段状」
2. **OmniWM Routing map** — 実際の机の配置。方向 focus / 窓のモニタ間移動 / mouse warp を駆動

旧 `mouseWarp.axis` / `monitorOrder` は 0.5.0 で**削除**され、routing map + 単一トグル + margin になった。
「モニタを縦一列に並べる」という旧運用ルールも不要。

このリポジトリでは routing map を `monitor-profiles/*.nix` が宣言する。

```nix
routing = { mode = "custom"; };
monitorRoutingOverrides = [
  (routeAt { name = "HP V27ie G5";           row = 0; })   # 上
  (routeAt { name = "";                      row = 1; })   # 中（名前なしモニタ）
  (routeAt { name = "Built-in Retina Display"; row = 2; }) # 下
];
```

接続中モニタのうち 1 枚でも grid に無いと OmniWM は macOS 配置へフォールバックする（安全側の劣化）。
`default.nix` は routing を `macOS` にしている（見知らぬ環境で実配置を推測できないため）。

## displayUUID が必須（0.5.9 の最重要変更）

0.5.9 は workspace / per-monitor override のモニタ解決を **displayUUID でしか行わない**。

```swift
// OutputId.resolveMonitor / MonitorSettingsStore.get
if let displayUUID { /* UUID で一意マッチ */ }
guard let displayId else { return nil }
return uniqueMonitor { $0.displayUUID == nil && $0.displayId == displayId && namesMatch(...) }
```

- 実機の全モニタは UUID を持つので、`displayId` + `name` の経路は**絶対に成立しない**
- `Monitor.namesMatch` は両方が非空文字を要求するので、**名前なしモニタを名前で指定するのは原理的に不可能**
- 公式リリースノートも「UUID は旧数値 ID から安全に導出できない」と明記

そのため nix はトークンだけを出力し、`deploy.sh` が runtime に解決する。

```
nix が出力:   displayUUID = "@@OMNIWM_UUID:HP V27ie G5@@"
deploy.sh が: ColorSync CGDisplayCreateUUIDFromDisplayID で解決して実 UUID に置換
```

selector の意味:

| selector | 解決方法 |
|---|---|
| `"<名前>"` | system_profiler の `_name` 完全一致 |
| `""` | EDID name を持たないモニタ（system_profiler が `spdisplays_display` と返すもの） |
| `"Built-in Retina Display"` | `CGDisplayIsBuiltin` で判定 |

**トークンが 1 つでも残ったまま書き込むと致命的**（`displayUUID` の形式違反は `keyNotFound` と違って
回復されず `settings.toml.corrupt` に退避される）ので、`deploy.sh` は書き込み前に残存チェックをして
残っていれば**書かずに終了する**（前回の良い設定を保持する）。

## モニタプロファイル

### 動作原理

```
profiles/darwin.nix:
  myConfig.darwin.omniwm.monitorProfile = "auto";   ← 既定: 自動検出
                                       = "<name>";  ← 強制指定も可
           ↓
deploy.sh が起動時 / モニタ抜き差し時に:
  1. 接続中ディスプレイを列挙（名前 / CGDirectDisplayID / displayUUID / 内蔵か）
  2. "auto" なら match を評価して最も specific なものを選択
  3. @@OMNIWM_UUID:...@@ / @@OMNIWM_ROUTING_MODE:...@@ を実値に置換
  4. 前回の render 結果と同じなら**何もしない**
  5. 変わっていれば atomic に差し替え → OmniWM が live reload する
  6. OmniWM が落ちている時だけ kickstart
```

### 既存プロファイル

| 名前 | match | 割当 |
|---|---|---|
| `office-3mon` | `requiredDisplays=["HP V27ie G5"]`, `requireUnnamed=true` | W,E,R,3–9→HP / S,D,F,1,2→名前なし / X,C,V→main |
| `hp-2mon` | `requiredDisplays=["HP V27ie G5"]`, `monitorCount=2` | W,E,R,3–9→HP / S,D,F,1,2→main / X,C,V→main |
| `default` | なし（catch-all） | 作業系→secondary / X,C,V→main、routing は macOS |

### フォールバックの段階

| 事象 | 挙動 |
|---|---|
| grid の一部モニタが不在 | `[routing] mode` が `macOS` に降格（workspace 側は main の UUID で代替） |
| ディスプレイ列挙が空 / UUID を持つモニタがゼロ | **書き込まずに終了**（前回の設定を保持） |
| トークンが未置換のまま残った | **書き込まずに終了** |

### 新しいプロファイルを追加する手順

1. `monitor-profiles/<名前>.nix` を作成
2. **`git add` する**（flake は git 管理下のファイルしかストアに取り込まないので、
   未追跡だと `builtins.readDir` から見えず黙って無視される）
3. `sudo darwin-rebuild switch --flake .#yuta`
4. ログで選択結果を確認: `tail ~/.local/share/omniwm/deploy.log`

## アプリ→ワークスペース整列（起動時 one-shot）

### 設計

- **`appRules.assignToWorkspace` は使わない**。0.5.9 では「最初にマッチした窓だけ」に意味が
  変わったが、それは「既に開いている窓をまとめて整列する」用途には使えない
- 代わりに `scripts/startup-sort.sh` が起動完了直後に 1 回だけ走り、
  `workspace-assignment.nix` のマップに従って現存ウィンドウを整列させる
- セッション中の新ウィンドウは**振分しない**。手動移動も**絶対に保持される**

### マッピング

| 着地先 | アプリ |
|---|---|
| S (13) | Helium / Chrome / Firefox / Safari / Dia / Zen |
| W (10) | cmux / iTerm2 / Terminal.app |
| E (11) | Claude / Antigravity |
| X (16) | Spotify / Music |
| C (17) | Discord / Slack / Teams / メッセージ |
| V (18) | Calendar / Notion / Obsidian |

エディタ（VSCode / Cursor / JetBrains / Zed）は意図的に**登録しない**（その日どのペアに置くかが
変わるため。必要なら `space+shift+<letter>` で送る）。

### 窓単位で移動する理由

以前は column 単位の `move-column-to-workspace` を使っていたが、`visibleContainerCount = 1` /
幅 100% にした結果「同じ WS に入った別アプリの窓が同一カラムに同居する」ようになり、
カラム単位で動かすと**無関係な窓を巻き込む**事故が実測で起きた。

```
cmux    3 → 10    … WS 10 に着地
Spotify 10 → 16   … Spotify のカラムに cmux が同居していて cmux も 16 へ流された
```

整列は「この窓をこの WS へ」なので、粒度も窓に合わせて `move-to-workspace` を使う。

### 手動で再実行

```bash
omniwm-startup-sort                          # PATH に通っている
launchctl kickstart -k gui/$UID/org.nixos.omniwm   # OmniWM ごと再起動 → 自動で整列
```

ログ: `~/.local/share/omniwm/startup-sort.log`

## hotkeys.nix は生成物に近い

149 個の action ID は**実機の OmniWM が書き戻した `~/.config/omniwm/settings.toml` から機械抽出**
している。手打ちしない。再抽出はこれで済む:

```bash
grep -A2 '^\[\[hotkeys\]\]' ~/.config/omniwm/settings.toml \
  | grep '^id = ' | sed 's/^id = "\(.*\)"/  { id = "\1"; binding = "Unassigned"; }/'
```

全部書く理由は「upstream が default binding を変えても宣言が勝つ」ため。0.5.9 は 16 個の
action ID をリネームしており、旧 ID に当てた設定が**黙って無視されて upstream default が採用される**
という事故が実際に起きていた（例: `toggleColumnFullWidth` → `toggleContainerFullPrimarySpan`）。

binding の正規形は記号を英語名で書く: `Option+Comma` / `Option+Slash` / `Option+Return` /
`Left Arrow` / `Page Up`。未割当は `"Unassigned"`。

## 欠損キーと形式違反の違い（設定を壊さないための知識）

`SettingsTOMLCodec.decode` は strict デコードを試し、`keyNotFound` を捕まえたら recovering
モードで再デコードして欠損キーに default を入れる。

- **セクションを書かないのは安全**（`[dwindle]` / `[overview]` / `version` / `[state]` を省略している）
- **値の形式違反は回復されない**（`dataCorrupted` / `typeMismatch` → `settings.toml.corrupt` に退避）
  → enum 文字列・UUID・色の形式は厳密に正しくないといけない

## 初回セットアップ

1. `darwin-rebuild switch` で OmniWM が brew cask 経由でインストールされる
2. システム設定 → デスクトップとDock → Mission Control で **「ディスプレイごとに個別の操作スペース」を ON**
   （変更後はログアウト → ログインが必要）
3. システム設定 → プライバシーとセキュリティ → アクセシビリティで OmniWM を有効化
4. メニューバーから OmniWM を Quit → launchd が自動起動を引き継ぐ

## トラブルシュート

### 設定が反映されない
1. `omniwmctl ping` で IPC 疎通確認
2. `ls ~/.config/omniwm/settings.toml.corrupt` の有無で decode 失敗を判定
3. `tail ~/.local/share/omniwm/deploy.log` で deploy の履歴と選択プロファイルを確認
4. live reload が拾わない項目に当たった場合は `launchctl kickstart -k gui/$UID/org.nixos.omniwm`

### ワークスペースがモニタに散らない / 1 枚に集中する
displayUUID の解決に失敗している。

```bash
tail ~/.local/share/omniwm/deploy.log      # unresolved display selectors が出ていないか
omniwmctl query displays --format json | jq -r '.result.payload.displays[] | "\(.id) \(.name)"'
grep -c '@@' ~/.config/omniwm/settings.toml # 0 でなければトークンが残っている（異常）
```

期待どおりか確認する:

```bash
omniwmctl query workspaces --format json \
  | jq -r '.result.payload.workspaces[] | "\(.rawName) \(.displayName) -> \(.display.name)"' | sort -n
```

### キーバインドが効かない
1. `pgrep -fl karabiner` で Karabiner が動いているか
2. **`omniwmctl ping` が `pong` を返すか**（space レイヤは全部 IPC 経由なので、
   IPC が死ぬと**ウィンドウ操作が全滅する**。緊急脱出は `Option+Return` と `Control+Option+Shift+R`）
3. `omniwmctl command focus left` で IPC 経由コマンドの疎通確認
4. 二重定義の検出:
   ```bash
   jq -r '.profiles[].complex_modifications.rules[].manipulators[]
          | select(.conditions[]?.name=="space_held")
          | "\(.from.key_code) \(.from.modifiers.mandatory // [])"' \
     ~/.config/karabiner/karabiner.json | sort | uniq -d
   ```

### 起動時整列が走らない
1. `tail ~/.local/share/omniwm/startup-sort.log`
2. `windows visible: 0` で終わっていたら OmniWM の window discovery が間に合っていない
   （最大 15 秒待つようにしてあるが、それでも足りなければ手動で `omniwm-startup-sort`）
3. `moved` が想定より少ないなら bundleId が `workspace-assignment.nix` に登録されているか確認:
   ```bash
   omniwmctl query windows --format json | jq -r '.result.payload.windows[].app.bundleId' | sort -u
   ```

### floating ウィンドウが行方不明
- `space+n` → floating を全部前面に
- `space+shift+n` → 画面外から呼び戻す
- `space+z` → Overview で視覚的に探す（Escape で閉じる）
- `omniwmctl query windows --floating --format json` で位置確認

### `darwin-rebuild switch` が exit 1 で終わる
home-manager の activation script が失敗すると **`/run/current-system` が更新されないまま**
古い世代を指し続ける。この状態は
「launchd の plist は新しいのに PATH 上のツールは古い」という分かりにくい症状になり、
さらに新しい store パスが GC ルートに入らないため `nix-collect-garbage` で消える危険がある。

```bash
readlink -f /run/current-system
nix eval --raw .#darwinConfigurations.yuta.config.system.build.toplevel   # 一致していること
```

## 参照

- OmniWM 公式: <https://github.com/BarutSRB/OmniWM>
- IPC & CLI: `docs/IPC-CLI.md` / アーキテクチャ: `docs/ARCHITECTURE.md`
- 設定スキーマの正典: `Sources/OmniWM/Core/Config/CanonicalTOMLConfig.swift`
  （0.4.x の `canonical-settings.toml` は upstream から削除済み）
- モニタ解決: `Sources/OmniWM/Core/Monitor/OutputId.swift` / `Monitor.swift`
- 実機の自己記述: `omniwmctl query capabilities --format json`
- AeroSpace 旧実装: [/modules/darwin/aerospace/](/modules/darwin/aerospace/)
