# modules/darwin/zed.nix
#
# Zed editor (Homebrew Cask)
# - GUI: /Applications/Zed.app  (bundleId: dev.zed.Zed)
# - CLI: zed <path>  → cask が /opt/homebrew/bin/zed shim を配置
#         (binary "#{appdir}/Zed.app/Contents/MacOS/cli", target: "zed")
# - 設計: queue/projwm-design.md §5.2 / §5.3 / §6.3 — projwm が `zed <cwd>` で
#   project 単位の Zed window を spawn、bundleId + title=basename(cwd) で識別
#
# 注: nixpkgs の zed-editor は aarch64-darwin で CLI が壊れている
#     (NixOS/nixpkgs#365465) ため使わない。homebrew cask 一択。
#
# ⚠️ 初回ビルド前に手動 /Applications/Zed.app があれば削除すること（cask 衝突回避）
{ config, lib, ... }:
let cfg = config.myConfig.darwin.zed; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.zed.enable = lib.mkEnableOption "Zed editor (Homebrew cask)";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "zed" ];
  };
}
