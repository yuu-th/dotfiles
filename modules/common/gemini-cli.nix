# modules/common/gemini-cli.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.geminiCli; in {
  options.myConfig.geminiCli.enable = lib.mkEnableOption "Gemini CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.gemini-cli ];
    };
  };
}
