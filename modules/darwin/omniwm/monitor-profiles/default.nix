# ── デフォルトモニタプロファイル ────────────────────────────────────────────
# match を持たない = 常にマッチする最終フォールバック。
# main / secondary だけを使うのでハードウェア構成に一切依存せず crash しない。
#
# routing は macOS の Arrange をそのまま使う（見知らぬ環境で実配置を推測できないため）。
# 1 モニタなら secondary も実質 main に落ちる。
{ helpers }:
let inherit (helpers) mkWorkspaces main secondary;
in {
  # match なし = catch-all（auto detect 時の最後の砦）

  routing = { mode = "macOS"; };
  monitorRoutingOverrides = [ ];

  workspaces = mkWorkspaces {
    monitorMap = {
      # ── 作業系 → main 以外（外部モニタがあればそちら）──────────────────
      "W" = secondary;
      "E" = secondary;
      "R" = secondary;
      "3" = secondary;
      "4" = secondary;
      "5" = secondary;
      "6" = secondary;
      "7" = secondary;
      "8" = secondary;
      "9" = secondary;
      # ── ブラウザ系 → main 以外 ────────────────────────────────────────
      "S" = secondary;
      "D" = secondary;
      "F" = secondary;
      "1" = secondary;
      "2" = secondary;
      # ── 常駐 → main（手元の画面）──────────────────────────────────────
      "X" = main;
      "C" = main;
      "V" = main;
    };
  };
}
