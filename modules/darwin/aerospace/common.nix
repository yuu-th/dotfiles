# ── AeroSpace 共通設定 ──────────────────────────────────────────────────────
# キーバインド、モード、gaps、マウス追従、JankyBorders、アプリ割り当て など
# モニタ構成に依存しない設定をここに集約する
#
# このファイルは Nix関数として attrset を返す
# default.nix から import してプロファイルとマージされる
{ pkgs, setupMediaWorkspace, focusTool, ... }:
{
  config-version = 2;

  # ── 基本設定 ──────────────────────────────────────────────────────────────
  # start-at-login は nix-darwin が launchd agent 経由で管理するため不要
  automatically-unhide-macos-hidden-apps = true;

  # ── 正規化 ────────────────────────────────────────────────────────────────
  enable-normalization-flatten-containers = true;
  enable-normalization-opposite-orientation-for-nested-containers = true;

  # ── レイアウト基本値 ──────────────────────────────────────────────────────
  default-root-container-layout = "tiles";
  default-root-container-orientation = "auto";
  accordion-padding = 24;

  # ── マウス自動追従 ────────────────────────────────────────────────────────
  # モニタ切替時: マウスがそのモニタ上になければモニタ中央に移動
  on-focused-monitor-changed = [ "move-mouse monitor-lazy-center" ];
  # ウィンドウフォーカス変更時: マウスがそのウィンドウ上になければウィンドウ中央に移動
  on-focus-changed = [ "move-mouse window-lazy-center" ];

  # ── JankyBorders をAeroSpaceと一緒に起動 (Goodies #3) ──────────────────
  # borders はプロセス重複検知内蔵なので AeroSpace 再起動時も安全
  after-startup-command = [
    "exec-and-forget /opt/homebrew/bin/borders active_color=0xFFE8D44D inactive_color=0xFF3C3C3C width=3.0 style=round"
  ];

  # ── Gaps ────────────────────────────────────────────────────────────────
  gaps = {
    inner = { horizontal = 8; vertical = 8; };
    outer = { left = 8; right = 8; top = 8; bottom = 8; };
  };

  # ── Floating (ダイアログ系) ── どのプロファイルでも共通 ──────────────────
  # プロファイル側の on-window-detected の先頭にマージされる想定ではなく、
  # プロファイル側で完全なリストを定義する（TOML の配列は上書き）
  # → floating ルールはプロファイル側にも含める必要がある

  # ── キーバインド ──────────────────────────────────────────────────────────
  mode = {
    main.binding = {
      # ── フォーカス移動 (同一WS内のウィンドウ間) ──
      alt-h = "focus left";
      alt-j = "focus down";
      alt-k = "focus up";
      alt-l = "focus right";

      # ── モニタ間フォーカス (方向ベースで直感的) ──
      alt-ctrl-h = "focus-monitor left";
      alt-ctrl-j = "focus-monitor down";
      alt-ctrl-k = "focus-monitor up";
      alt-ctrl-l = "focus-monitor right";

      # ── ウィンドウ移動 (同一WS内タイル位置) ──
      alt-shift-h = "move left";
      alt-shift-j = "move down";
      alt-shift-k = "move up";
      alt-shift-l = "move right";

      # ── ウィンドウのグループ化（ネスト化） ──
      # 特定のウィンドウだけを合体させてアコーディオン等を作る用
      alt-ctrl-shift-h = "join-with left";
      alt-ctrl-shift-j = "join-with down";
      alt-ctrl-shift-k = "join-with up";
      alt-ctrl-shift-l = "join-with right";

      # ── 用途別WS切替 / アプリ一発呼び出し ──
      # alt-s/c/a: WS M (右下) に移動しつつ、アコーディオンの中からすぐ手前に出したいアプリを呼ぶ
      alt-s = [ "workspace M" "exec-and-forget open -a Spotify" ];
      alt-c = [ "workspace M" "exec-and-forget open -a Discord" ];
      alt-a = [ "workspace M" "exec-and-forget open -a Calendar" ]; # Mac純正カレンダー手前化
      alt-m = "workspace M"; # WS M に移動するだけ
      
      # 🌟 マクロ: 起動時に[Spotify/Discord] | [カレンダー]のレイアウトを一瞬で自動生成する
      # Nixの writeShellScriptBin を通じて生成された絶対パスを埋め込む（ハードコード排除！）
      alt-ctrl-m = "exec-and-forget ${setupMediaWorkspace}/bin/setup-media-workspace";

      alt-b = "workspace B";
      alt-e = "workspace E";

      # ── 数字WS切替 ──
      alt-1 = "workspace 1";
      alt-2 = "workspace 2";
      alt-3 = "workspace 3";
      alt-4 = "workspace 4";
      alt-5 = "workspace 5";
      alt-6 = "workspace 6";
      alt-7 = "workspace 7";
      alt-8 = "workspace 8";
      alt-9 = "workspace 9";

      # ── ウィンドウをWSに送る＋ジャンプ ──
      alt-shift-m = [ "move-node-to-workspace M" "workspace M" ];
      alt-shift-b = [ "move-node-to-workspace B" "workspace B" ];
      alt-shift-e = [ "move-node-to-workspace E" "workspace E" ];

      alt-shift-1 = [ "move-node-to-workspace 1" "workspace 1" ];
      alt-shift-2 = [ "move-node-to-workspace 2" "workspace 2" ];
      alt-shift-3 = [ "move-node-to-workspace 3" "workspace 3" ];
      alt-shift-4 = [ "move-node-to-workspace 4" "workspace 4" ];
      alt-shift-5 = [ "move-node-to-workspace 5" "workspace 5" ];
      alt-shift-6 = [ "move-node-to-workspace 6" "workspace 6" ];
      alt-shift-7 = [ "move-node-to-workspace 7" "workspace 7" ];
      alt-shift-8 = [ "move-node-to-workspace 8" "workspace 8" ];
      alt-shift-9 = [ "move-node-to-workspace 9" "workspace 9" ];

      # ── レイアウト切替 ──
      alt-slash = "layout tiles horizontal vertical";
      alt-comma = "layout accordion horizontal vertical";

      # ── フルスクリーン (AeroSpace管理, macOS native ではない) ──
      alt-enter = "fullscreen --no-outer-gaps";

      # ── floating⇔tiling切替 ──
      alt-shift-space = "layout floating tiling";

      # ── リサイズ (簡易) ──
      alt-minus = "resize smart -50";
      alt-equal = "resize smart +50";

      # ── モード切替 ──
      alt-r = "mode resize";
      alt-shift-semicolon = "mode service";

      # ── WS 行き来 ──
      alt-tab = "workspace-back-and-forth";
      alt-shift-tab = "move-workspace-to-monitor --wrap-around next";

      # ── Goodies #10: cmd-h (Hide) を無効化 ──
      cmd-h = [];
      cmd-alt-h = [];

      # ── Ghostty ウィンドウフォーカス ─────────────────────────────────────────
      # Alt+Ctrl+1/2/3: 現在 WS の Ghostty ウィンドウを X 座標昇順で N 番目にフォーカス
      alt-ctrl-1 = "exec-and-forget ${focusTool}/bin/focus-tool win 1";
      alt-ctrl-2 = "exec-and-forget ${focusTool}/bin/focus-tool win 2";
      alt-ctrl-3 = "exec-and-forget ${focusTool}/bin/focus-tool win 3";
      # 新規ウィンドウは Ghostty ネイティブの Cmd+N を使う（AeroSpace バインド不要）
    };

    # ── リサイズモード ──
    resize.binding = {
      h     = "resize width -50";
      j     = "resize height +50";
      k     = "resize height -50";
      l     = "resize width +50";
      shift-h = "resize width -200";
      shift-l = "resize width +200";
      enter = "mode main";
      esc   = "mode main";
    };

    # ── サービスモード (設定リロード・デバッグ用) ──
    service.binding = {
      esc       = [ "reload-config" "mode main" ];
      r         = [ "flatten-workspace-tree" "mode main" ];
      f         = [ "layout floating tiling" "mode main" ];
      backspace = [ "close-all-windows-but-current" "mode main" ];
      alt-shift-d = [ "enable toggle" "mode main" ];
    };
  };

  # ── アプリをワークスペースに自動割り当て ──────────────────────────────────
  # モニタ構成に依存しないルール。ルール順序が重要: 最初にマッチしたルールのみ実行される。
  on-window-detected = [
    # ── Browser → WS B ──
    { "if".app-id = "com.google.Chrome";            run = [ "move-node-to-workspace B" ]; }
    { "if".app-id = "org.mozilla.firefox";          run = [ "move-node-to-workspace B" ]; }
    { "if".app-id = "com.apple.Safari";             run = [ "move-node-to-workspace B" ]; }
    { "if".app-id = "company.thebrowser.dia";       run = [ "move-node-to-workspace B" ]; }

    # ── Editor → WS E ──
    { "if".app-id = "com.microsoft.VSCodeInsiders"; run = [ "move-node-to-workspace E" ]; }
    { "if".app-id = "com.microsoft.VSCode";         run = [ "move-node-to-workspace E" ]; }

    # ── AI / Agent → WS 1 ──
    { "if".app-id = "com.google.antigravity";       run = [ "move-node-to-workspace 1" ]; }

    # ── Media / 常駐アプリ → WS M ──
    { "if".app-id = "com.spotify.client";           run = [ "move-node-to-workspace M" ]; }
    { "if".app-id = "com.hnc.Discord";              run = [ "move-node-to-workspace M" ]; }
    { "if".app-id = "com.apple.iCal";               run = [ "move-node-to-workspace M" ]; }

    # ── Terminals (その他) → WS 3 ──
    { "if".app-id = "com.googlecode.iterm2";        run = [ "move-node-to-workspace 3" ]; }
    { "if".app-id = "com.apple.Terminal";           run = [ "move-node-to-workspace 3" ]; }

    # ── Floating (ダイアログ系) ── 現在のWSに留まる ──
    { "if".app-name-regex-substring = "^(Finder|System Preferences|System Settings|Calculator|Dictionary)$";
      run = [ "layout floating" ]; }
    { "if".app-id = "com.mojang.minecraftlauncher";
      run = [ "layout floating" ]; }

    # Ghostty は on-window-detected で自動移動しない。
    # Alt+N で開いた新ウィンドウは現在の WS にそのまま配置される。
  ];
}
