# modules/darwin/orbstack.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.orbstack; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.orbstack.enable = lib.mkEnableOption "OrbStack";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "orbstack" ];
  };
}
