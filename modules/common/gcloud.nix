# modules/common/gcloud.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.gcloud; in {
  options.myConfig.gcloud.enable = lib.mkEnableOption "Google Cloud SDK";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.system.primaryUser} = {
      home.packages = [ pkgs.google-cloud-sdk ];
    };
  };
}
