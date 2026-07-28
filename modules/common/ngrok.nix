# modules/common/ngrok.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.ngrok; in {
  options.myConfig.ngrok.enable = lib.mkEnableOption "ngrok tunnel CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.ngrok ];
    };
  };
}
