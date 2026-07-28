# modules/darwin/vivaldi.nix
#
# Vivaldi browser (Homebrew Cask)
# - GUI: /Applications/Vivaldi.app  (bundleId: com.vivaldi.Vivaldi)
# - 用途: projwm v12 の browser 統合専用 secondary browser
#         （通常運用は Zen、Vivaldi は project workspace 切替の AppleScript 制御対象）
# - 設計: queue/projwm-roadmap.md "v12 Browser 統合" 節 — Quick Commands (⌘E) 経由で
#   workspace 名タイプで切替、または ⌘⇧1..9 で index 切替
#
# ⚠️ 初回ビルド前に手動 /Applications/Vivaldi.app があれば削除すること（cask 衝突回避）
{ config, lib, ... }:
let cfg = config.myConfig.darwin.vivaldi; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.vivaldi.enable = lib.mkEnableOption "Vivaldi browser (Homebrew cask)";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "vivaldi" ];
  };
}
