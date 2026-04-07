# Terminal Workflow — 操作体験設計書

> **設計思想**: VSCodeなし。キー操作だけで全て完結する、Zellij+Ghosttyベースのターミナル完結スタック。  
> **最終更新**: 設計議論を経て確定したバージョン

---

## スタック全体像

```
【Layer 1】 AeroSpace   ... ウィンドウ配置・WS切替・ツール直接テレポート
【Layer 2】 Ghostty     ... ターミナル基盤 (Quick Terminal 含む)
【Layer 3】 Zellij      ... セッション永続化・タブ管理 (プロジェクト=セッション)
【Layer 4】 ツール      ... yazi / neovim / ki / lazygit / claude-code / fish
```

---

## ウィンドウ構成の基本設計

### メインウィンドウ (常に 1 つ)

```
Ghostty: メインウィンドウ
  タイトル: "[project]: main"  (例: "dotfiles: main")
  Zellijセッション: "dotfiles"

  タブ構成 (3つ固定):
  Cmd+E  🔧 Editor   (yazi + neovim/ki の左右分割)
  Cmd+G  📦 Git      (lazygit フルスクリーン)
  Cmd+S  🐚 Shell    (fish シェル)
```

### AI ウィンドウ (ai コマンドで動的生成)

```
Ghostty: AI ウィンドウ (1つ目)
  タイトル: "[project]: AI"  (例: "dotfiles: AI")  ← 数字なし・固定
  Zellijセッション: "dotfiles-ai-1" (内部ID。ユーザーは意識しない)
  表示: claude-code がフルスクリーンで起動

Ghostty: AI ウィンドウ (2つ目)
  タイトル: "dotfiles: AI"  ← 同じタイトル。数字は「現在WSの左から何番目か」で決まる
  Zellijセッション: "dotfiles-ai-2" (内部ID)
  表示: claude-code がフルスクリーンで起動
```

> **位置が番号を決める設計思想**  
> 「AI-1」「AI-2」はウィンドウに名前として付くものではなく、  
> 「現在WSの左から何番目のAIウィンドウか」がそのままショートカットの番号になる。  
> ウィンドウを移動すれば番号も変わる。タイトルに数字は埋め込まない。  
> Zellijセッション名（内部ID）とユーザー体験上の番号は完全に切り離されている。

### WS E のレイアウト (例: 4モニタ環境)

```
【DISO モニター (left-top) - WS E】
┌─────────────────────────────────────┐
│   メインウィンドウ                    │
│   Zellij: dotfiles                  │
│   現在: 🔧 Editor (yazi | neovim)   │
└─────────────────────────────────────┘

【L2235 モニター (right-top) - WS E】
┌──────────────────┬──────────────────┐
│ AI ウィンドウ [左] │ AI ウィンドウ [右] │
│ dotfiles: AI     │ dotfiles: AI     │
│ ← Alt+Ctrl+1     │ ← Alt+Ctrl+2     │
│ claude-code 稼働  │ claude-code 稼働  │
└──────────────────┴──────────────────┘

左から何番目か = Alt+Ctrl+N の N。AeroSpace が自動タイリング。
```

---

## キーバインド体系

### AeroSpace: ワークスペース切替 (既存)

| キー | 動作 |
|------|------|
| `Alt+E` | WS E (Editor/作業スペース) へ |
| `Alt+B` | WS B (Browser) へ |
| `Alt+M` | WS M (Media: Spotify/Discord) へ |
| `Alt+1~9` | 数字 WS へ |
| `Alt+Tab` | 前の WS と往復 |

### AeroSpace: ウィンドウ操作 (既存)

| キー | 動作 |
|------|------|
| `Alt+h/j/k/l` | WS内のウィンドウフォーカス (方向) |
| `Alt+Ctrl+h/j/k/l` | モニター間フォーカス (方向) |
| `Alt+Shift+h/j/k/l` | ウィンドウ移動 (タイリング位置) |
| `Alt+Minus / Equal` | ウィンドウリサイズ |
| `Alt+Enter` | フルスクリーン切替 |
| `Alt+Shift+Space` | フロート/タイル切替 |
| `Alt+R` | リサイズモード |

### AeroSpace: ワークフロー インフラ層 (新規追加)

> `Alt+Ctrl` = focus-tool 専用空間。プロジェクト・AI・ツールへの瞬間移動をすべてここに集約。  
> 具体的なウィンドウ位置・Zellijセッション名を意識させない。

| キー | 動作 | 体験 |
|------|------|------|
| `Alt+Ctrl+P` | プロジェクトピッカー | fzf で既存セッション＋~/dev/ 一覧 → 選択 → 自動で開く/切替 |
| `Alt+Ctrl+A` | 現在プロジェクトの新AI起動 | プロジェクト自動検出 → AIウィンドウを現在WSにタイリング |

> **インフラ層の思想**: 「今どのZellijセッションか」「どのWSか」を意識しなくて良い。  
> `Alt+Ctrl+P` でプロジェクトを、`Alt+Ctrl+A` でAIを、それだけで全てが動く。

### AeroSpace: ツール直接テレポート (新規追加)

> `Alt+Ctrl` = 「指定ツールにどこからでも瞬間移動」（A・P を含め Alt+Ctrl 空間に統一）  
> 既存 `Alt+Ctrl+hjkl` (モニター移動) と `Alt+Ctrl+M` (メディアWS) に統一した意味論。  
> **文字キーはツール名と一致** (E=Editor, G=Git, S=Shell, A=AI new, P=Picker)、**数字はAI位置** (1=左から1番目, 2=左から2番目)。  
> **AI は必ず Alt+Ctrl+1/2/3 で到達**: Zellij メインセッションに AI タブは存在しない。

| キー | 動作 | 仕組み |
|------|------|--------|
| `Alt+Ctrl+E` | Editor に瞬間移動 | "main" ウィンドウをフォーカス + Cmd+E 送信 |
| `Alt+Ctrl+G` | Git に瞬間移動 | "main" ウィンドウをフォーカス + Cmd+G 送信 |
| `Alt+Ctrl+S` | Shell に瞬間移動 | "main" ウィンドウをフォーカス + Cmd+S 送信 |
| `Alt+Ctrl+A` | 現在プロジェクトの新AI起動 | タイトル ": main" からプロジェクト検出 → AI ウィンドウ生成 |
| `Alt+Ctrl+P` | プロジェクトピッカー | fzf で既存セッション + ~/dev/ → 選択 → 自動で開く/切替 |
| `Alt+Ctrl+1` | 左から1番目のAIに移動 | 現在WSのAIウィンドウをX座標昇順で並べて1番目をフォーカス |
| `Alt+Ctrl+2` | 左から2番目のAIに移動 | 現在WSのAIウィンドウをX座標昇順で並べて2番目をフォーカス |
| `Alt+Ctrl+3` | 左から3番目のAIに移動 | 現在WSのAIウィンドウをX座標昇順で並べて3番目をフォーカス |

> **一貫性**: E=Editor・G=Git・S=Shell の意味は AeroSpace 側 (Alt+Ctrl) でも Zellij 側 (Cmd) でも同じ。  
> **Alt+Ctrl で全 focus-tool 操作が完結**: Alt+Shift は AeroSpace のウィンドウ/WS 操作専用。

### Ghostty: ターミナル基盤

| キー | 動作 |
|------|------|
| `Opt+Space` | Quick Terminal トグル (ドロップダウン) — **global:** プレフィックスでGhostty非アクティブ時も動作 |

**Ghostty の設定変更内容 (`modules/darwin/ghostty.nix`)**:

```
追加:
  keybind = global:opt+space=toggle_quick_terminal
    → システム全体でトグル可能なドロップダウン端末
  quick-terminal-position = bottom
  quick-terminal-screen = main

変更 (unbind → Zellij に委譲):
  keybind = super+t=unbind       (既存: new_tab → Zellij に渡す)
  keybind = super+w=unbind       (既存: close_surface → Zellij に渡す)
  keybind = super+shift+t=unbind (既存: new_window → 不要)
  keybind = super+d=unbind       (既存: new_split:right → Zellij に渡す)
  keybind = super+shift+d=unbind (既存: new_split:down → Zellij に渡す)
  keybind = super+[=unbind       (デフォルト: previous_split → Zellij に渡す)
  keybind = super+]=unbind       (デフォルト: next_split → Zellij に渡す)

不要:
  macos-option-as-alt → Zellij の Alt キーバインドを使わない設計にしたため不要
```

> **Cmd+E/G/S は Ghostty のデフォルトバインドではない**ため unbind 不要。そのままZellijに渡る。  
> **Cmd+Shift+T/P も Ghostty のデフォルトバインドではない**ため unbind 不要。

### Zellij: タブ・ペイン操作

| キー | 動作 |
|------|------|
| `Cmd+E` | Editor タブへ |
| `Cmd+G` | Git タブへ |
| `Cmd+S` | Shell タブへ |
| `Cmd+T` | 新タブ作成 |
| `Cmd+Shift+T` | タブ名変更 |
| `Cmd+W` | 現在のタブ/ペイン閉じる |
| `Cmd+D` | ペインを右に分割 |
| `Cmd+Shift+D` | ペインを下に分割 |
| `Cmd+[` | 前のペインへ |
| `Cmd+]` | 次のペインへ |
| `Cmd+Shift+P` | ペイン名変更 |

> **Cmd 系で統一**: 全Zellij操作を Cmd+* に集約（Alt 系バインドなし）。  
> `macos-option-as-alt` の設定は不要。  
> **AI タブなし**: AI は常に専用 Ghostty ウィンドウ (別 Zellij セッション) にあるため、  
> メインウィンドウには Editor / Git / Shell の 3 タブのみ。AI は Alt+Ctrl+1/2/3 で。

---

## Zellijセッション内タブ詳細

### Tab 1: 🔧 Editor

```
┌──────────────────┬────────────────────────────────────────┐
│  yazi (30%)      │  neovim または ki (70%)                  │
│                  │                                         │
│  📁 src/         │  < ファイルの中身がここに表示 >            │
│  📁 modules/     │                                         │
│  📄 flake.nix ◀  │  コード閲覧・軽い編集                     │
│                  │                                         │
│  Enter: 開く     │  q: yazi に戻る                          │
└──────────────────┴────────────────────────────────────────┘

yazi でファイルを選択 → Enter → neovim/ki でそのファイルが開く
```

### Tab 2: 📦 Git

```
┌──────────────────────────────────────────────────────────┐
│                        lazygit                           │
│                                                          │
│  ブランチ管理 / ステージング / コミット / プッシュ / PR     │
│  Space: ステージ  c: コミット  P: プッシュ  b: ブランチ    │
└──────────────────────────────────────────────────────────┘
```

### Tab 3: 🐚 Shell

```
┌──────────────────────────────────────────────────────────┐
│                      fish shell                          │
│                                                          │
│  Ctrl+R : atuin 履歴検索 (fuzzy)                         │
│  Ctrl+F : fzf ファイル検索                               │
│  z <名前>: zoxide でスマートディレクトリ移動              │
│  ll     : eza (git status 付きリスト)                    │
└──────────────────────────────────────────────────────────┘
```

> メインセッションのタブは以上 3 つのみ。AI は独立セッション (Alt+Ctrl+1/2/3 で到達)。

---

## 典型的な作業フロー (具体的キー操作付き)

### シナリオ 1: 作業開始

```
1. Ghostty を起動 → WS E に自動配置される
   fish $ zj dotfiles
   → Zellij セッション "dotfiles" が存在しなければ新規作成
   → Editor / Git / Shell の 3 タブが自動生成される
   → Editor タブ にフォーカス
   → yazi (左) + neovim (右) が既に分割配置されている

   (または Alt+Ctrl+P から "dotfiles" を選択しても同じ結果)

2. ファイル構造を把握する
   yazi でカーソル移動 → Enter でプレビュー/開く
   q で yazi ペインに戻る

3. lazygit で現在の状態確認
   Cmd+G → Git タブへ
   ブランチ状態、未コミット変更をひと目で確認
   Cmd+E → Editor タブに戻る
```

### シナリオ 2: AI コーディングセッション開始

```
1. どこからでも Alt+Ctrl+A を押す (Shell タブへの移動不要)
   → 以下が自動実行される:
     a. 現在WSのメインウィンドウタイトルからプロジェクト名を自動検出
        ("[dotfiles]: main" → project = "dotfiles")
     b. 新しい独立 Zellij セッション "dotfiles-ai-1" を作成
     c. そのセッション内で claude-code を起動
     d. 新しい Ghostty ウィンドウが開き、現在WSにタイリング配置される
     e. ウィンドウタイトルが "[dotfiles]: AI" に設定される

2. AI に指示を出す
   (新しいウィンドウに自動でフォーカスが移動)
   → claude-code のプロンプトにタスクを入力

3. AI 作業中、自分は別の作業をする
   Alt+Ctrl+E → メインウィンドウの Editor タブに移動
   → yazi / neovim で別のファイルを確認

4. AI の進捗を確認したい
   Alt+Ctrl+1 → 現在WSの左から1番目のAIウィンドウに移動
   → 出力を確認、必要なら追加指示
```

### シナリオ 3: 複数 AI を同時管理

```
1. AI が1つ稼働中、さらに2つ目を起動
   Alt+Ctrl+A (どこからでも、2回目)
   → "dotfiles-ai-2" セッション + "[dotfiles]: AI" ウィンドウが生成
   → 現在WSに AI ウィンドウが2つ並ぶ (左=Alt+Ctrl+1, 右=Alt+Ctrl+2)

2. ブラウザで調査しながら両 AI を監視
   Alt+B → WS B (ブラウザ) で情報収集
   Alt+Ctrl+1 → 左のAI に移動 (進捗確認・追加指示)
   Alt+Ctrl+2 → 右のAI に移動 (進捗確認・追加指示)
   Alt+Ctrl+E → Editor に戻る (自分の作業)

3. AI セッションの一覧確認 (必要な場合のみ)
   fish $ zj
   → fzf で全セッション (dotfiles / dotfiles-ai-1 / dotfiles-ai-2) が表示
   ※ セッション名の数字は内部IDであり、ウィンドウの左右位置とは無関係
```

### シナリオ 4: ブラウザ作業中のクイック確認

```
WS B (ブラウザ) で調べ物中...

方法 A: 特定ツールに直接テレポート
  Alt+Ctrl+1 → 現在WSの左から1番目のAIに移動
  Alt+Ctrl+E → Editor に移動
  Alt+Ctrl+G → Git に移動

方法 B: WS E に戻ってからタブ選択
  Alt+E → WS E (メインウィンドウにフォーカスが戻る)
  Cmd+N → 目的のタブへ

方法 C: Quick Terminal で一瞬だけ確認
  Opt+Space → ドロップダウン表示 (どの WS からでも)
  → 素 fish シェルで素早くコマンド実行
  Opt+Space → 閉じる (元の WS/アプリに戻る)
```

### シナリオ 5: プロジェクト切替 & 新規開始

```
【どこからでも Alt+Ctrl+P 一発】

Alt+Ctrl+P → プロジェクトピッカーが開く

  ┌──────────────────────────────────────────────┐
  │ > _                                          │
  │  ⚡ dotfiles   [WS E]   (アクティブ)          │  ← 既存セッション+WS表示
  │  ⚡ my-app     [WS 1]   (アクティブ)          │
  │  📁 new-lib   (~/dev/new-lib)                │  ← 未開封ディレクトリ
  │  📁 side-proj (~/dev/side-proj)              │
  └──────────────────────────────────────────────┘

選択肢ごとの動作:

  ⚡ 既存セッション (Ghosttyウィンドウあり):
    → そのWSに切替 + メインウィンドウにフォーカス

  ⚡ 既存セッション (Ghosttyウィンドウなし/閉じた後):
    → 新Ghosttyウィンドウを開きセッションにアタッチ + そのWSに配置

  📁 未開封ディレクトリ (新プロジェクト):
    → WS E が空: WS E に配置
    → WS E 使用中: WS 1 に配置
    → WS 1 も使用中: WS 2 に配置
    → WS 2 も使用中: 警告 (最大3プロジェクト)
    → 新Zellijセッション (Editor/Git/Shell タブ自動生成) + 新Ghosttyウィンドウ

【WS割り当てルール】
  WS E → WS 1 → WS 2 の順に自動割り当て (最大3プロジェクト同時並走)
  プロジェクトとWSの対応はZellijセッションのウィンドウタイトルから逆引き可能

戻り方:
  Alt+Ctrl+P → "dotfiles" を選択 → WS E に即座に戻る (状態保持)
```

### シナリオ 6: Git 作業 (コミット → PR)

```
コードのステージング・コミット・PR 作成:

Alt+Ctrl+G → Git タブに瞬間移動 (どこからでも)
lazygit が全画面で起動

→ j/k  : ファイル選択
→ Space: ステージング
→ c    : コミットメッセージ入力
→ P    : プッシュ
→ q    : lazygit 終了

PR 作成:
Alt+Ctrl+S → Shell タブへ
fish $ gh pr create --web
```

---

## カスタムコマンド仕様

### `zj` コマンド

```fish
# 使い方
zj              # fzf でセッション一覧から選択
zj dotfiles     # "dotfiles" セッションに attach (なければ新規作成)
zj my-app       # "my-app" セッションに attach (なければ新規作成)

# セッション新規作成時に自動生成されるタブ:
# - 🔧 Editor (yazi + neovim の左右分割レイアウト)
# - 📦 Git    (lazygit)
# - 🐚 Shell  (fish)
# - カレントディレクトリ: ~/dev/<セッション名> に自動 cd
```

### `ai` コマンド

```fish
# Zellijセッション内の Shell タブから実行
ai              # 次の番号で AI セッションを開始

# 内部動作:
# 1. $ZELLIJ_SESSION_NAME から現在のプロジェクト名を取得 (例: "dotfiles")
# 2. 既存の "dotfiles-ai-N" セッションをカウントして次の内部番号N を決定
# 3. 新しい独立 Zellij セッション "dotfiles-ai-N" を作成し claude-code を起動
# 4. 新しい Ghostty ウィンドウを開く (--title "[dotfiles]: AI")  ← 数字なし
# 5. そのウィンドウを "dotfiles-ai-N" セッションにアタッチ
# 6. AeroSpace が現在のWSにタイリング配置する
#
# ユーザーが体験する番号はウィンドウの左右位置で決まる (タイトルには含まない)
# 停止: AI ウィンドウで claude-code を終了 → zellij kill-session dotfiles-ai-N
```

---

## AeroSpace テレポートマクロの実装方針

```
スクリプト: focus-tool.sh <target>

引数と動作:
  "editor"   → 現在WSのメインウィンドウをフォーカス + Cmd+E 送信 (Editor タブへ)
  "git"      → 現在WSのメインウィンドウをフォーカス + Cmd+G 送信 (Git タブへ)
  "shell"    → 現在WSのメインウィンドウをフォーカス + Cmd+S 送信 (Shell タブへ)
  "ai 1"     → 現在WSの左から1番目のAIウィンドウをフォーカス
  "ai 2"     → 現在WSの左から2番目のAIウィンドウをフォーカス
  "ai 3"     → 現在WSの左から3番目のAIウィンドウをフォーカス
  "ai-new"   → 現在WSのプロジェクトを自動検出し、新しいAIウィンドウを起動

【メインウィンドウの識別】
  タイトルが ": main" で終わる Ghostty ウィンドウ = メインウィンドウ
  例: "dotfiles: main"

【AI ウィンドウの識別と位置順ソート】
  タイトルが ": AI" で終わる Ghostty ウィンドウ = AIウィンドウ
  aerospace list-windows --focused-workspace で現在WSに絞り込んだ後、
  osascript (macOS Accessibility API) で各ウィンドウのX座標を取得し昇順ソート。
  N番目のウィンドウIDを aerospace focus --window-id で選択。

【なぜ位置基準か】
  タイトルに数字を埋め込むと、ウィンドウを移動したとき番号と位置がズレる。
  位置基準なら「左にあるやつが1」が常に成立し、どんな配置でも直感的に使える。
  異なるプロジェクトのAIウィンドウが同じWSに混在しても問題なし。

  例:
    L2235左: [dotfiles]: AI → Alt+Ctrl+1 でフォーカス
    L2235右: [my-app]: AI  → Alt+Ctrl+2 でフォーカス
    (ウィンドウを左右入れ替えても Alt+Ctrl+1 は常に「左のやつ」)

aerospace.nix での設定:
  "alt-ctrl-e"   = "exec-and-forget /path/to/focus-tool.sh editor";
  "alt-ctrl-g"   = "exec-and-forget /path/to/focus-tool.sh git";
  "alt-ctrl-s"   = "exec-and-forget /path/to/focus-tool.sh shell";
  "alt-ctrl-1"   = "exec-and-forget /path/to/focus-tool.sh ai 1";
  "alt-ctrl-2"   = "exec-and-forget /path/to/focus-tool.sh ai 2";
  "alt-ctrl-3"   = "exec-and-forget /path/to/focus-tool.sh ai 3";
  "alt-ctrl-a"  = "exec-and-forget /path/to/focus-tool.sh ai-new";
  "alt-ctrl-p"  = "exec-and-forget /path/to/focus-tool.sh project-picker";

【project-picker の動作詳細】
  1. Zellij セッション一覧から AI セッション (*-ai-N) を除外してプロジェクト一覧を取得
  2. ~/dev/ 以下のディレクトリのうち Zellij セッションがないものを追加 (📁 アイコン)
  3. fzf ピッカーを Ghostty 一時ウィンドウで表示
  4. 選択:
     - ⚡ 既存(ウィンドウあり): `aerospace workspace {WS}` でそのWSに切替 + フォーカス
     - ⚡ 既存(ウィンドウなし): 新Ghosttyウィンドウ作成 → セッションにアタッチ → WSに配置
     - 📁 未開封ディレクトリ:
         a. 空きWS検索 (E → 1 → 2の順)、3つ全て埋まっていれば警告して終了
         b. 新Zellijセッション作成 (Editor/Git/Shell タブ自動生成)
         c. 新Ghosttyウィンドウ作成 → セッションにアタッチ → 空きWSに `move-node-to-workspace` で配置
         d. `aerospace workspace {WS}` でそのWSに切替
```

---

## 技術構成

| ツール | 役割 | 設定箇所 |
|--------|------|---------|
| AeroSpace | WS管理・テレポートマクロ | `modules/darwin/aerospace/` |
| Ghostty | ターミナル基盤・Quick Terminal | `modules/darwin/ghostty.nix` |
| Zellij | セッション永続化・タブ/ペイン | `modules/common/zellij.nix` **[新規]** |
| yazi | ファイルブラウザ (VSCode Explorer 代替) | 既存 |
| Neovim | コードビューア/エディタ | `modules/common/neovim.nix` **[新規]** |
| Ki Editor | コードビューア/エディタ (試験) | `modules/common/ki-editor.nix` **[新規]** |
| lazygit | Git 管理 (VSCode Git 代替) | 既存 |
| claude-code | AI Coding エージェント | 既存 |
| fish | インタラクティブシェル | 既存 |

---

## 実装チェックリスト

### `modules/darwin/ghostty.nix` (既存ファイル変更)
- [ ] `keybind = global:opt+space=toggle_quick_terminal` 追加
- [ ] `quick-terminal-position = bottom` 追加
- [ ] `super+t/w/shift+t/d/shift+d` を unbind に変更
- [ ] `super+[` / `super+]` を unbind 追加
- ~~`macos-option-as-alt`~~ → Zellij の Alt キーバインドを使わない設計にしたため不要

### `modules/darwin/aerospace/common.nix` (既存ファイル変更)
- [ ] `alt-ctrl-e/g/s` + `alt-ctrl-1/2/3` テレポートマクロ追加
- [ ] `alt-ctrl-a` (新AI起動) バインド追加
- [ ] `alt-ctrl-p` (プロジェクトピッカー) バインド追加
- [ ] Ghostty を WS E に自動配置するルール追加 (`on-window-detected`)

### `modules/darwin/aerospace/focus-tool.sh` (新規作成)
- [ ] `editor/git/shell` 引数: メインウィンドウ検索 + Cmd キー送信
- [ ] `ai N` 引数: 現在WSのAIウィンドウをX座標でソートしてN番目フォーカス
- [ ] `ai-new` 引数: 現在WSのプロジェクト検出 → 新AIウィンドウ起動
- [ ] `project-picker` 引数: fzf ピッカー + Zellijセッション管理 + WS自動割り当て

### `modules/common/zellij.nix` (新規作成)
- [ ] Zellij 基本設定 (テーマ、タブバー、ウィンドウタイトルフォーマット)
- [ ] デフォルト Ctrl-* / Alt-* キーバインドを全 unbind (Emacs/fish との競合回避)
- [ ] `Cmd+E/G/S` タブ切替バインド設定
- [ ] `Cmd+T/W/D/Shift+D/[/]` ペイン操作バインド設定
- [ ] `Cmd+Shift+T` タブ名変更 / `Cmd+Shift+P` ペイン名変更バインド設定
- [ ] `zj` コマンド (fish function): セッション選択・作成
- [ ] セッション新規作成時の Editor/Git/Shell タブ自動生成ロジック

### `modules/common/neovim.nix` (新規作成)
- [ ] Neovim 基本設定 (tokyonight テーマ)

### `modules/common/ki-editor.nix` (新規作成)
- [ ] Ki Editor (flake input `github:ki-editor/ki-editor` 追加)
- [ ] aarch64-darwin 対応確認

### `flake.nix` (既存ファイル変更)
- [ ] `ki-editor` input 追加

### `profiles/darwin.nix` (既存ファイル変更)
- [ ] zellij / neovim / ki-editor の imports と enable フラグ追加
