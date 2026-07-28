# modules/common/opencode.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.opencode; in {
  options.myConfig.opencode.enable = lib.mkEnableOption "OpenCode CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.opencode ];
    };
  };
}
