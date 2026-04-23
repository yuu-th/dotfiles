# ── 2枚モニタプロファイル ──────────────────────────────────────────────────
# LCD-MF234X (外部) + Built-in Retina Display (MacBook)
#
# aerospace list-monitors の出力:
#   1 | LCD-MF234X
#   2 | Built-in Retina Display
#
# 物理配置:
#   ┌──────────────────────────┐  ┌──────────────────────────┐
#   │      LCD-MF234X          │  │ Built-in Retina Display  │
#   │    Monitor 1 (外部)      │  │   Monitor 2 (MacBook)    │
#   │   Editor + 柔軟          │  │   ブラウザ / 常駐         │
#   │                          │  │                          │
#   │  WS: E, 3〜9             │  │  WS: B, M, 1, 2          │
#   └──────────────────────────┘  └──────────────────────────┘
#
# NOTE: アプリ割り当てルール (on-window-detected) は common.nix に集約済み
{
  # ── 永続ワークスペース ──────────────────────────────────────────────────
  persistent-workspaces = [ "1" "2" "3" "4" "5" "6" "7" "8" "9" "M" "B" "E" ];

  # ── ワークスペース → モニタ固定割り当て ──────────────────────────────────
  workspace-to-monitor-force-assignment = {
    # ── Built-in Retina Display (MacBook) ── ブラウザ / 常駐
    "B" = "Built-in Retina Display";
    "M" = "Built-in Retina Display";
    "1" = "Built-in Retina Display";
    "2" = "Built-in Retina Display";

    # ── LCD-MF234X (外部モニター) ── Editor + 柔軟 (未割当アプリの着地先)
    "E" = "LCD-MF234X";
    "3" = "LCD-MF234X";    # catch-all のデフォルト先
    "4" = "LCD-MF234X";
    "5" = "LCD-MF234X";
    "6" = "LCD-MF234X";
    "7" = "LCD-MF234X";
    "8" = "LCD-MF234X";
    "9" = "LCD-MF234X";
  };
}
