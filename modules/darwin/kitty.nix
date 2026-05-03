# modules/darwin/kitty.nix
#
# kitty terminal emulator (Homebrew Cask)
# - GUI: /Applications/kitty.app (bundleId: net.kovidgoyal.kitty)
# - CLI: /opt/homebrew/bin/kitty (cask が shim を配置)
# - 設計: queue/projwm-design.md v11.3 — projwm の terminal driver として
#   ghostty 代わりに採用（OmniWM 0.4.8 が SwiftUI ベースの Ghostty 1.3 の
#   window を AX で列挙できないバグが macOS 26.x Tahoe + Ghostty 1.3 で発覚、
#   queue/projwm-report.md D-006 参照）
#
# kitty 採用理由:
#   - Cocoa/AppKit ベースで OmniWM 互換性確認済（NSWorkspace.runningApplications で正常列挙）
#   - `kitty --single-instance` で多 window を 1 プロセスから生成できる効率
#   - tiling WM 親和性が yabai/aerospace コミュニティで確立
{ config, lib, ... }:
let cfg = config.myConfig.darwin.kitty; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.kitty.enable = lib.mkEnableOption "kitty terminal (Homebrew cask)";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "kitty" ];
  };
}
