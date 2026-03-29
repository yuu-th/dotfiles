# modules/darwin/spotify.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.spotify; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.spotify.enable = lib.mkEnableOption "Spotify";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "spotify" ];
  };
}
