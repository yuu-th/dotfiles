# modules/darwin/anki.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.anki; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.anki.enable = lib.mkEnableOption "Anki";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "anki" ];
  };
}
