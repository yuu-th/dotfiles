# ── 3枚モニタプロファイル ──────────────────────────────────────────────────
# HP V27ie G5 (左上) + 名前なし (左下) + MacBook (右上)
#
# aerospace list-monitors の出力:
#   1 | HP V27ie G5
#   2 |               (名前なし)
#   3 | Built-in Retina Display
#
# 物理配置:
#   ┌──────────────────┐  ┌──────────────────────────┐
#   │  HP V27ie G5     │  │ Built-in Retina Display  │
#   │ Monitor 1 (左上) │  │  Monitor 3 (右上)        │
#   │  デフォルト柔軟  │  │  カレンダー/常駐          │
#   └──────────────────┘  └──────────────────────────┘
#   ┌──────────────────┐
#   │   (名前なし)     │
#   │ Monitor 2 (左下) │
#   │  ブラウザ/Editor │
#   └──────────────────┘
#
# NOTE: アプリ割り当てルール (on-window-detected) は common.nix に集約済み
{
  # ── 永続ワークスペース ──────────────────────────────────────────────────
  persistent-workspaces = [ "1" "2" "3" "4" "5" "6" "7" "8" "9" "M" "B" "E" ];

  # ── ワークスペース → モニタ固定割り当て ──────────────────────────────────
  workspace-to-monitor-force-assignment = {
    # ── Built-in Retina Display (右上/MacBook) ── カレンダー/常駐アプリ用
    "M" = "Built-in Retina Display";

    # ── 名前なしモニタ (左下, Monitor #2) ── ブラウザ/Editor
    "B" = 2;                 # Browser (名前が空のためモニタ番号で指定)
    "E" = 2;                 # VS Code Insiders
    "1" = 2;
    "2" = 2;

    # ── HP V27ie G5 (左上) ── デフォルト柔軟 (未割当アプリの着地先)
    "3" = "HP V27ie G5";     # catch-all のデフォルト先
    "4" = "HP V27ie G5";
    "5" = "HP V27ie G5";
    "6" = "HP V27ie G5";
    "7" = "HP V27ie G5";
    "8" = "HP V27ie G5";
    "9" = "HP V27ie G5";
  };
}
