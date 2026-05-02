# ── 4枚モニタプロファイル ──────────────────────────────────────────────────
# DIOS-MF241X (左上) + L2235HW (右上) + 名前なし (左下) + Built-in Retina Display (右下)
#
#   ┌──────────────────┐  ┌──────────────┐
#   │ DIOS-MF241X      │  │   L2235HW    │
#   │ Editor+柔軟      │  │ デフォルト柔軟 │
#   │ (E, 1, 2)        │  │ (3〜9)        │
#   └──────────────────┘  └──────────────┘
#   ┌──────────────────┐  ┌──────────────────────────┐
#   │ (名前なし)       │  │ Built-in Retina Display  │
#   │ ブラウザ専用     │  │ Spotify/Chat (M)         │
#   │ (B)              │  │                           │
#   └──────────────────┘  └──────────────────────────┘
{ helpers }:
let
  inherit (helpers) mkWorkspaces display unnamedDisplay;
  mac   = display "Built-in Retina Display";
  dios  = display "DIOS-MF241X";
  l2235 = display "L2235HW";
in {
  workspaces = mkWorkspaces {
    monitorMap = {
      "M" = mac;
      "B" = unnamedDisplay;
      "E" = dios;
      "1" = dios;
      "2" = dios;
      "3" = l2235;
      "4" = l2235;
      "5" = l2235;
      "6" = l2235;
      "7" = l2235;
      "8" = l2235;
      "9" = l2235;
    };
    layoutMap = {
      "E" = "dwindle";
    };
  };
}
