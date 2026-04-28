# modules/common/vercel.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.vercel; in {
  options.myConfig.vercel.enable = lib.mkEnableOption "Vercel CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.vercel ];
    };
  };
}
