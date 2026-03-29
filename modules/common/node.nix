# modules/common/node.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.node; in {
  options.myConfig.node.enable = lib.mkEnableOption "Node.js development environment";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.system.primaryUser} = {
      home.packages = [ pkgs.nodejs_22 ];
      home.file.".npmrc".text = ''
        prefix=''${HOME}/.npm-global
      '';
      home.sessionPath = [ "$HOME/.npm-global/bin" ];
    };
  };
}
