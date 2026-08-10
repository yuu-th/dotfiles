# ── HP + 内蔵の 2 モニタ構成 ─────────────────────────────────────────────────
#
# 実際の机の配置（上から）:
#   ↑ HP V27ie G5      … メイン作業
#   ↓ Built-in         … ブラウザ + 常駐（名前なしモニタが無いぶんを引き受ける）
#
# 名前なしモニタが繋がっていない時（自宅 / 出張先で HP だけ繋いだ時など）に
# auto-select される。requireUnnamed を持たないので office-3mon より specificity が
# 同じだが、monitorCount = 2 で 3 モニタ環境を弾く。
{ helpers }:
let
  inherit (helpers) mkWorkspaces main display routeAt builtinName;
  hp = "HP V27ie G5";
in {
  match = {
    requiredDisplays = [ hp ];
    monitorCount = 2;
  };

  # ── OmniWM Routing map（実際の机の配置）────────────────────────────────
  routing = { mode = "custom"; };
  monitorRoutingOverrides = [
    (routeAt { name = hp;          row = 0; })   # 上
    (routeAt { name = builtinName; row = 1; })   # 下
  ];

  workspaces = mkWorkspaces {
    monitorMap = {
      # ── 上段: メイン作業 → HP ─────────────────────────────────────────
      "W" = display hp;
      "E" = display hp;
      "R" = display hp;
      "3" = display hp;
      "4" = display hp;
      "5" = display hp;
      "6" = display hp;
      "7" = display hp;
      "8" = display hp;
      "9" = display hp;
      # ── 下段: ブラウザ → Built-in（名前なしモニタの代役）──────────────
      "S" = main;
      "D" = main;
      "F" = main;
      "1" = main;
      "2" = main;
      # ── 最下段: 常駐 → Built-in ───────────────────────────────────────
      "X" = main;
      "C" = main;
      "V" = main;
    };
  };
}
