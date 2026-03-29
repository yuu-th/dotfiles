# modules/darwin/discord.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.discord; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.discord.enable = lib.mkEnableOption "Discord";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "discord" ];
  };
}
