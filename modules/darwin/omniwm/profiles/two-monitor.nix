# ── 2枚モニタプロファイル ──────────────────────────────────────────────────
# Built-in Retina Display (main) + LCD-MF234X (secondary)
#
# 物理配置:
#   ┌──────────────────────────┐  ┌──────────────────────────┐
#   │      LCD-MF234X          │  │ Built-in Retina Display  │
#   │   Editor + 柔軟           │  │   ブラウザ / 常駐         │
#   │  WS: E, 3〜9 (secondary) │  │  WS: B, M, 1, 2 (main)   │
#   └──────────────────────────┘  └──────────────────────────┘
#
# NOTE: macOS の "main" は基本的に Built-in Retina Display。
# 個別モニタ名で固定したい場合は OmniWM GUI で specificDisplay を使う
# （TOML だと v0.4.8 時点で decode に問題あり）。
{ helpers }:
let inherit (helpers) mkWorkspaces main secondary;
in {
  workspaces = mkWorkspaces {
    # Built-in Retina Display (main) — ブラウザ/常駐
    "1" = main;
    "2" = main;
    "B" = main;
    "M" = main;

    # 外部モニタ (secondary) — Editor + 柔軟
    "E" = secondary;
    "3" = secondary;
    "4" = secondary;
    "5" = secondary;
    "6" = secondary;
    "7" = secondary;
    "8" = secondary;
    "9" = secondary;
  };
}
