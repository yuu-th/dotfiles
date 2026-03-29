# modules/darwin/alt-tab.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.altTab; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.altTab.enable = lib.mkEnableOption "AltTab window switcher";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "alt-tab" ];
  };
}
