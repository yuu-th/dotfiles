# modules/darwin/raycast.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.raycast; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.raycast.enable = lib.mkEnableOption "Raycast";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "raycast" ];
  };
}
