# ── 2枚モニタプロファイル ──────────────────────────────────────────────────
# Built-in Retina Display + LCD-MF234X
#
# 物理配置:
#   ┌──────────────────────────┐  ┌──────────────────────────┐
#   │      LCD-MF234X          │  │ Built-in Retina Display  │
#   │   Editor + 柔軟           │  │   ブラウザ / 常駐         │
#   │  WS: E, 3〜9             │  │  WS: B, M, 1, 2          │
#   └──────────────────────────┘  └──────────────────────────┘
#
# WS E は dwindle (BSP)、それ以外は niri 横スクロール。
{ helpers }:
let
  inherit (helpers) mkWorkspaces display;
  builtIn = display "Built-in Retina Display";
  ext     = display "LCD-MF234X";
in {
  workspaces = mkWorkspaces {
    monitorMap = {
      "1" = builtIn;
      "2" = builtIn;
      "B" = builtIn;
      "M" = builtIn;
      "E" = ext;
      "3" = ext;
      "4" = ext;
      "5" = ext;
      "6" = ext;
      "7" = ext;
      "8" = ext;
      "9" = ext;
    };
    layoutMap = {
      "E" = "dwindle";
    };
  };
}
