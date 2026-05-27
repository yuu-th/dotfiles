# modules/common/devbox.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.devbox; in {
  options.myConfig.devbox.enable = lib.mkEnableOption "Devbox CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.devbox ];
    };
  };
}
