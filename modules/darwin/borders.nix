# modules/darwin/borders.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.borders; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.borders.enable = lib.mkEnableOption "JankyBorders focus border";

  config = lib.mkIf cfg.enable {
    homebrew.brews = [ "borders" ];
  };
}
