# ── 3枚モニタプロファイル ──────────────────────────────────────────────────
# HP V27ie G5 (左上) + 名前なし (左下) + Built-in Retina Display (右上)
#
# 物理配置:
#   ┌──────────────────┐  ┌──────────────────────────┐
#   │  HP V27ie G5     │  │ Built-in Retina Display  │
#   │ デフォルト柔軟    │  │  カレンダー/常駐 (main)   │
#   │ (secondary)      │  │                           │
#   └──────────────────┘  └──────────────────────────┘
#   ┌──────────────────┐
#   │   (名前なし)     │  ← display:2
#   │  ブラウザ/Editor │
#   │  (secondary)     │
#   └──────────────────┘
#
# NOTE: 3 枚以上のモニタで `secondary` は「main 以外のいずれか」になる。
# AeroSpace のように特定モニタへ固定割当したい場合は OmniWM GUI から
# specificDisplay を選ぶ（TOML 直書きは v0.4.8 で不具合あり）。
{ helpers }:
let inherit (helpers) mkWorkspaces main secondary;
in {
  workspaces = mkWorkspaces {
    # Built-in (main) — Calendar / 常駐
    "M" = main;

    # それ以外のモニタ (secondary) — 全部 secondary でフォールバック
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
