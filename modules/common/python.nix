# modules/common/python.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.python; in {
  options.myConfig.python.enable = lib.mkEnableOption "Python 3";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.python3 ];
    };
  };
}
