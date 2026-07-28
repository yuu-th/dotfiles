# modules/darwin/chrome-cli.nix
#
# chrome-cli (https://github.com/prasmussen/chrome-cli)
# - Chromium 系 browser を Scripting Bridge (AppleScript) 経由で CLI 制御
# - 対応: Chrome, Brave, Vivaldi, Edge, Arc, Chromium 等
# - Bundle ID は環境変数 CHROME_BUNDLE_IDENTIFIER で切替可能
#
# focus を奪わない non-intrusive な browser 制御手段として導入
# (AX API 経由の制御は focus を奪うため採用しない)。
{ config, lib, ... }:
let cfg = config.myConfig.darwin.chromeCli; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.chromeCli.enable = lib.mkEnableOption "chrome-cli (Chromium browser CLI control)";

  config = lib.mkIf cfg.enable {
    homebrew.brews = [ "chrome-cli" ];
  };
}
