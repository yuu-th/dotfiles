{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.darwin.aerospace;
  setupMediaWorkspace = pkgs.writeShellScriptBin "setup-media-workspace" (builtins.readFile ./scripts/setup-media-workspace.sh);
  common  = import ./common.nix { inherit pkgs setupMediaWorkspace; };
  profile = import ./profiles/triple-monitor.nix;
in {
  options.myConfig.darwin.aerospace.enable = lib.mkEnableOption "AeroSpace tiling window manager";

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ setupMediaWorkspace ];

    services.aerospace = {
      enable = true;
      settings = lib.recursiveUpdate common profile;
    };
  };
}
