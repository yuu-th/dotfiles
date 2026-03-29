# modules/darwin/obsidian.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.obsidian; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.obsidian.enable = lib.mkEnableOption "Obsidian";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "obsidian" ];
  };
}
