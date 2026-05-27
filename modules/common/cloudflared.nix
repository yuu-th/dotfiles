# modules/common/cloudflared.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.cloudflared; in {
  options.myConfig.cloudflared.enable = lib.mkEnableOption "Cloudflare Tunnel CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.cloudflared ];
    };
  };
}
