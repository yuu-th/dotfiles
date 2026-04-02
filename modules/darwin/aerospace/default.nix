{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.darwin.aerospace;
  setupMediaWorkspace = pkgs.writeShellScriptBin "setup-media-workspace" (builtins.readFile ./scripts/setup-media-workspace.sh);
  focusTool           = pkgs.writeShellScriptBin "focus-tool"           (builtins.readFile ./scripts/focus-tool.sh);
  common  = import ./common.nix { inherit pkgs setupMediaWorkspace focusTool; };
  profile = import ./profiles/triple-monitor.nix;
in {
  options.myConfig.darwin.aerospace.enable = lib.mkEnableOption "AeroSpace tiling window manager";

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ setupMediaWorkspace focusTool ];

    services.aerospace = {
      enable = true;
      settings = lib.recursiveUpdate common profile;
    };
  };
}
