# ── デフォルトモニタプロファイル ────────────────────────────────────────────
# match を持たない = 常にマッチする最終フォールバック。
# main/secondary だけ使うので、ハードウェア構成に一切依存せず crash しない。
{ helpers }:
let inherit (helpers) mkWorkspaces main secondary;
in {
  # match なし = catch-all（auto detect 時の最後の砦）

  workspaces = mkWorkspaces {
    monitorMap = {
      "M" = main;
      "1" = main;
      "2" = main;
      "B" = main;
      "E" = secondary;
      "3" = secondary;
      "4" = secondary;
      "5" = secondary;
      "6" = secondary;
      "7" = secondary;
      "8" = secondary;
      "9" = secondary;
    };
    layoutMap = {
      "E" = "dwindle";
    };
  };
}
