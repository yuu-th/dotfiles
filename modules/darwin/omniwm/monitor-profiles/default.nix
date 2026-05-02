# ── デフォルトモニタプロファイル ────────────────────────────────────────────
# main/secondary だけを使う、ハードウェア構成に依存しない堅牢な fallback。
# OmniWM のデフォルト挙動: main = macOS の primary display、secondary = それ以外。
# モニタ枚数 1〜N どれでも crash せず動く。
#
# 「main を MacBook 内蔵」「secondary を任意の外部モニタ」という前提で配置。
{ helpers }:
let inherit (helpers) mkWorkspaces main secondary;
in {
  workspaces = mkWorkspaces {
    monitorMap = {
      "M" = main;          # Calendar / 常駐
      "1" = main;          # AI agent
      "2" = main;
      "B" = main;          # 1 monitor 時に内蔵で見たい

      "E" = secondary;     # Editor (BSP)
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
