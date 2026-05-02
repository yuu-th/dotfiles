# ── 1枚モニタプロファイル（MacBook 単独）──────────────────────────────────
# Built-in Retina Display のみ。全 WS が main に集約される。
#
# 想定: ノマドワーク・カフェ・通勤先での最小構成。
{ helpers }:
let inherit (helpers) mkWorkspaces main;
in {
  workspaces = mkWorkspaces {
    monitorMap = {
      "1" = main;
      "2" = main;
      "3" = main;
      "4" = main;
      "5" = main;
      "6" = main;
      "7" = main;
      "8" = main;
      "9" = main;
      "M" = main;
      "B" = main;
      "E" = main;
    };
    layoutMap = {
      "E" = "dwindle";    # 単独モニタでも E は IDE 風 BSP
    };
  };
}
