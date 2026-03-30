# modules/common/git.nix
{ config, lib, ... }:
let cfg = config.myConfig.git; in {
  options.myConfig.git.enable = lib.mkEnableOption "git with user config";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      programs.git = {
        enable = true;
        settings.user = {
          name  = "yuu-th";
          email = "88813495+yuu-th@users.noreply.github.com";
        };
      };
    };
  };
}
