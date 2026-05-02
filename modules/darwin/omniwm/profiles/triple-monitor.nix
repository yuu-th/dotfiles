# ── 3枚モニタプロファイル ──────────────────────────────────────────────────
# HP V27ie G5 (左上) + 名前なし (左下、display:2) + Built-in Retina Display (右上)
#
# 物理配置:
#   ┌──────────────────┐  ┌──────────────────────────┐
#   │  HP V27ie G5     │  │ Built-in Retina Display  │
#   │ デフォルト柔軟   │  │  カレンダー/常駐          │
#   │ (3〜9)           │  │  (M)                      │
#   └──────────────────┘  └──────────────────────────┘
#   ┌──────────────────┐
#   │   (名前なし)     │  ← display:2
#   │  ブラウザ/Editor │
#   │  (B, E, 1, 2)    │
#   └──────────────────┘
#
# 名前なしモニタは `unnamedDisplay` で displayId だけで識別する。
{ helpers }:
let
  inherit (helpers) mkWorkspaces display unnamedDisplay;
  mac = display "Built-in Retina Display";
  hp  = display "HP V27ie G5";
in {
  workspaces = mkWorkspaces {
    monitorMap = {
      # MacBook (Built-in) — Calendar / 常駐
      "M" = mac;

      # 名前なしモニタ — Browser / Editor / Agent
      "B" = unnamedDisplay;
      "E" = unnamedDisplay;
      "1" = unnamedDisplay;
      "2" = unnamedDisplay;

      # HP V27ie G5 — デフォルト柔軟
      "3" = hp;
      "4" = hp;
      "5" = hp;
      "6" = hp;
      "7" = hp;
      "8" = hp;
      "9" = hp;
    };
    layoutMap = {
      "E" = "dwindle";
    };
  };
}
