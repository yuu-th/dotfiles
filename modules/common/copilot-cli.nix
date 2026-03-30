# modules/common/copilot-cli.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.copilotCli; in {
  options.myConfig.copilotCli.enable = lib.mkEnableOption "GitHub Copilot CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.copilot-cli ];
    };
  };
}
