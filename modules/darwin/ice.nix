# modules/darwin/ice.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.ice; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.ice.enable = lib.mkEnableOption "Ice menu bar manager";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "jordanbaird-ice" ];
  };
}
