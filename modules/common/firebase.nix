# modules/common/firebase.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.firebase; in {
  options.myConfig.firebase.enable = lib.mkEnableOption "Firebase CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.firebase-tools ];
    };
  };
}
