# modules/common/cursor-cli.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.cursorCli; in {
  options.myConfig.cursorCli.enable = lib.mkEnableOption "Cursor CLI (cursor-agent)";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.cursor-cli ];
    };
  };
}
