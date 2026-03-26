# ── 4枚モニタプロファイル ──────────────────────────────────────────────────
# DIOS-MF241X (左上) + L2235HW (右上) + 名前なし (左下) + MacBook (右下)
#
# aerospace list-monitors の出力:
#   1 | DIOS-MF241X
#   2 |                   (名前なし)
#   3 | L2235HW
#   4 | Built-in Retina Display
#
# 物理配置:
#   ┌──────────────────┐  ┌──────────────┐
#   │ DIOS-MF241X      │  │   L2235HW    │
#   │ Monitor 1 (左上) │  │ Monitor 3(右上)│
#   │ Editor+柔軟      │  │ デフォルト柔軟│
#   └──────────────────┘  └──────────────┘
#   ┌──────────────────┐  ┌──────────────────────────┐
#   │ (名前なし)       │  │ Built-in Retina Display  │
#   │ Monitor 2 (左下) │  │ Monitor 4 (右下/MacBook) │
#   │ ブラウザ専用     │  │ Spotify/Chat             │
#   └──────────────────┘  └──────────────────────────┘
{
  # ── 永続ワークスペース ──────────────────────────────────────────────────
  persistent-workspaces = [ "1" "2" "3" "4" "5" "6" "7" "8" "9" "M" "B" "E" ];

  # ── ワークスペース → モニタ固定割り当て ──────────────────────────────────
  workspace-to-monitor-force-assignment = {
    # ── Built-in Retina Display (右下/MacBook) ── カレンダー/常駐アプリ用
    "M" = "Built-in Retina Display";

    # ── 名前なしモニタ (左下, Monitor #2) ── ブラウザ専用
    "B" = 2;                 # Browser (名前が空のためモニタ番号で指定)

    # ── DIOS-MF241X (左上) ── Editor + 柔軟
    "E" = "DIOS";            # VS Code Insiders
    "1" = "DIOS";            # 柔軟
    "2" = "DIOS";            # 柔軟

    # ── L2235HW (右上) ── デフォルト柔軟 (未割当アプリの着地先)
    "3" = "L2235";           # catch-all のデフォルト先
    "4" = "L2235";
    "5" = "L2235";
    "6" = "L2235";
    "7" = "L2235";
    "8" = "L2235";
    "9" = "L2235";
  };

  # ── アプリをワークスペースに自動割り当て ──────────────────────────────────
  # ルール順序が重要: 最初にマッチしたルールのみ実行される
  on-window-detected = [
    # ── Browser → WS B (左下) ──
    { "if".app-id = "com.google.Chrome";           run = [ "move-node-to-workspace B" ]; }
    { "if".app-id = "org.mozilla.firefox";         run = [ "move-node-to-workspace B" ]; }
    { "if".app-id = "com.apple.Safari";            run = [ "move-node-to-workspace B" ]; }
    { "if".app-id = "company.thebrowser.dia"; run = [ "move-node-to-workspace B" ]; }

    # ── Editor → WS E (左上) ──
    { "if".app-id = "com.microsoft.VSCodeInsiders"; run = [ "move-node-to-workspace E" ]; }
    { "if".app-id = "com.microsoft.VSCode";         run = [ "move-node-to-workspace E" ]; }

    # ── Media / 常駐アプリ → WS M (右下) ──
    { "if".app-id = "com.spotify.client";          run = [ "move-node-to-workspace M" ]; }
    { "if".app-id = "com.hnc.Discord";             run = [ "move-node-to-workspace M" ]; }
    { "if".app-id = "com.apple.iCal";              run = [ "move-node-to-workspace M" ]; }

    # ── Floating (ダイアログ系) ── 現在のWSに留まる
    { "if".app-name-regex-substring = "^(Finder|System Preferences|System Settings|Calculator|Dictionary)$";
      run = [ "layout floating" ]; }

    # ── Catch-all: 上記に該当しないアプリ → WS 3 (L2235HW/右上) ──
    { run = [ "move-node-to-workspace 3" ]; }
  ];
}
