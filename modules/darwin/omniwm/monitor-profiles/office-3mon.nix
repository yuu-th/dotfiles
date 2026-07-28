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
      # ── projwm slots: 全て unnamedDisplay に集約（main を埋めない方針）─
      "A" = unnamedDisplay;
      "Q" = unnamedDisplay;
      "W" = unnamedDisplay;
      "R" = unnamedDisplay;
      "T" = unnamedDisplay;
      "Y" = unnamedDisplay;
      "U" = unnamedDisplay;
      "I" = unnamedDisplay;
      "O" = unnamedDisplay;
      "P" = unnamedDisplay;
      # ── cockpit park workspace (requirements v2.4 §8.1) ──────────────────
      # CP1 goes to unnamedDisplay — same monitor as workspace A (projwm-managed).
      # CP2-CP6 removed (requirements v2.4: 1 cockpit only).
      "CP1" = unnamedDisplay;
    };
    layoutMap = {
      # projwm では E は niri、旧 Editor の dwindle 設定は不要
    };
  };
}
