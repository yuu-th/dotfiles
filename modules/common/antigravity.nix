# modules/common/antigravity.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.antigravity; in {
  options.myConfig.antigravity.enable = lib.mkEnableOption "antigravity";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.system.primaryUser} = {
      home.packages = [ pkgs.antigravity ];
    };
  };
}
