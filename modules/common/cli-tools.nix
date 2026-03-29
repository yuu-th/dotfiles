# modules/common/cli-tools.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.cliTools; in {
  options.myConfig.cliTools.enable = lib.mkEnableOption "base CLI tools";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.system.primaryUser} = {
      home.packages = with pkgs; [ gh jq ripgrep fd fzf htop ];
    };
  };
}
