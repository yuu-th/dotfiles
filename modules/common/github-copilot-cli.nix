# modules/common/github-copilot-cli.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.githubCopilotCli; in {
  options.myConfig.githubCopilotCli.enable = lib.mkEnableOption "GitHub Copilot CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.github-copilot-cli ];
    };
  };
}
