# modules/common/go.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.go; in {
  options.myConfig.go.enable = lib.mkEnableOption "Go";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.system.primaryUser} = {
      home.packages = [ pkgs.go ];
    };
  };
}
