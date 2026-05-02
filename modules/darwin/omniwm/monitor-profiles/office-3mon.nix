# ── 例: オフィス 3 モニタ構成 ────────────────────────────────────────────
# Built-in (main) + HP V27ie G5 + 名前なしモニタ
#
# このプロファイルを使うには profiles/darwin.nix で:
#   myConfig.darwin.omniwm.monitorProfile = "office-3mon";
#
# 解決失敗時の挙動:
#   - "HP V27ie G5" が接続されていない → そのワークスペースは secondary に
#     フォールバック（deploy.sh が runtime にチェック）
#   - 名前なしモニタが無い → 同様に secondary フォールバック
#   → どんなモニタ構成でも crash せず動く
{ helpers }:
let inherit (helpers) mkWorkspaces main display unnamedDisplay;
in {
  workspaces = mkWorkspaces {
    monitorMap = {
      # MacBook Built-in: Calendar / 常駐
      "M" = main;

      # 名前なしモニタ (左下): Browser / Editor / Agent
      "B" = unnamedDisplay;
      "E" = unnamedDisplay;
      "1" = unnamedDisplay;
      "2" = unnamedDisplay;

      # HP V27ie G5: catch-all / その他
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
