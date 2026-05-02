# ── 4枚モニタプロファイル ──────────────────────────────────────────────────
# DIOS-MF241X + L2235HW + 名前なし + Built-in Retina Display (main)
#
# 物理配置:
#   ┌──────────────────┐  ┌──────────────┐
#   │ DIOS-MF241X      │  │   L2235HW    │
#   │ Editor+柔軟      │  │ デフォルト柔軟 │
#   │ (secondary)      │  │ (secondary)   │
#   └──────────────────┘  └──────────────┘
#   ┌──────────────────┐  ┌──────────────────────────┐
#   │ (名前なし)       │  │ Built-in Retina Display  │
#   │ ブラウザ専用     │  │ Spotify/Chat (main)      │
#   │ (secondary)      │  │                           │
#   └──────────────────┘  └──────────────────────────┘
#
# NOTE: 4 枚モニタでは `secondary` だけだと特定モニタへの固定割当ができない。
# 必要なら OmniWM GUI から個別に specificDisplay を設定する運用とする。
{ helpers }:
let inherit (helpers) mkWorkspaces main secondary;
in {
  workspaces = mkWorkspaces {
    # Built-in (main) — 常駐
    "M" = main;

    # それ以外 (secondary) — 全部 secondary
    "B" = secondary;
    "E" = secondary;
    "1" = secondary;
    "2" = secondary;
    "3" = secondary;
    "4" = secondary;
    "5" = secondary;
    "6" = secondary;
    "7" = secondary;
    "8" = secondary;
    "9" = secondary;
  };
}
