# modules/common/wrangler.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.wrangler; in {
  options.myConfig.wrangler.enable = lib.mkEnableOption "Cloudflare Workers/Pages CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.wrangler ];
    };
  };
}
