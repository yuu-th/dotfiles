# modules/common/uv.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.uv; in {
  options.myConfig.uv.enable = lib.mkEnableOption "uv Python package manager";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.uv ];
    };
  };
}
