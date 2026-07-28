# modules/darwin/vivaldi.nix
#
# Vivaldi browser (Homebrew Cask)
# - GUI: /Applications/Vivaldi.app  (bundleId: com.vivaldi.Vivaldi)
# - 用途: secondary browser（通常運用は Zen）
# - Quick Commands (⌘E) で workspace 名タイプ切替、⌘⇧1..9 で index 切替
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
