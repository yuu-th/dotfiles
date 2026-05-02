# ── オフィス 3 モニタ構成 ────────────────────────────────────────────────────
# Built-in (main) + HP V27ie G5 + 名前なしモニタ
#
# match の条件すべて成立する時に auto-select される：
#   - HP V27ie G5 が接続されている
#   - 名前なしモニタが接続されている
#
# 手動で選びたい場合は profiles/darwin.nix で:
#   myConfig.darwin.omniwm.monitorProfile = "office-3mon";
{ helpers }:
let inherit (helpers) mkWorkspaces main display unnamedDisplay;
in {
  match = {
    requiredDisplays = [ "HP V27ie G5" ];
    requireUnnamed = true;
  };

  workspaces = mkWorkspaces {
    monitorMap = {
      "M" = main;
      "B" = unnamedDisplay;
      "E" = unnamedDisplay;
      "1" = unnamedDisplay;
      "2" = unnamedDisplay;
      "3" = display "HP V27ie G5";
      "4" = display "HP V27ie G5";
      "5" = display "HP V27ie G5";
      "6" = display "HP V27ie G5";
      "7" = display "HP V27ie G5";
      "8" = display "HP V27ie G5";
      "9" = display "HP V27ie G5";
    };
    layoutMap = {
      "E" = "dwindle";
    };
  };
}
