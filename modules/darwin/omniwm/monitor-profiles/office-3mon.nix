# ── オフィス 3 モニタ構成 ────────────────────────────────────────────────────
#
# 実際の机の配置（上から）:
#   ↑ HP V27ie G5      … メイン作業
#     名前なしモニタ      … ブラウザ
#   ↓ Built-in         … ながら見 / 常駐（macOS の main display）
#
# macOS の Arrange は公式推奨の階段配置にしておけばよい（上下が実配置と逆でも構わない）。
# 実配置は下の routing grid が OmniWM に教える。
#
# match の条件すべて成立する時に auto-select される。
# 手動指定する場合は profiles/darwin.nix で:
#   myConfig.darwin.omniwm.monitorProfile = "office-3mon";
{ helpers }:
let
  inherit (helpers) mkWorkspaces main display unnamedDisplay routeAt builtinName;
  hp = "HP V27ie G5";
in {
  match = {
    requiredDisplays = [ hp ];
    requireUnnamed = true;
  };

  # ── OmniWM Routing map（実際の机の配置）────────────────────────────────
  routing = { mode = "custom"; };
  monitorRoutingOverrides = [
    (routeAt { name = hp;          row = 0; })   # 上
    (routeAt { name = "";          row = 1; })   # 中（名前なしモニタ）
    (routeAt { name = builtinName; row = 2; })   # 下
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
      # ── 下段: ブラウザ → 名前なしモニタ ───────────────────────────────
      "S" = unnamedDisplay;
      "D" = unnamedDisplay;
      "F" = unnamedDisplay;
      "1" = unnamedDisplay;
      "2" = unnamedDisplay;
      # ── 最下段: 常駐 → Built-in（main）────────────────────────────────
      "X" = main;
      "C" = main;
      "V" = main;
    };
  };
}
