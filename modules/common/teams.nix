{ config, lib, pkgs, ... }:
let cfg = config.myConfig.teams; in {
  options.myConfig.teams.enable = lib.mkEnableOption "Microsoft Teams";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = with pkgs; [ teams ];
    };
  };
}
