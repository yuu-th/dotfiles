# modules/common/claude-code.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.claudeCode; in {
  options.myConfig.claudeCode.enable = lib.mkEnableOption "Claude Code CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.claude-code ];
    };
  };
}
