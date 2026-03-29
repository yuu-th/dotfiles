# modules/darwin/linearmouse/default.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.linearmouse; in {
  options.myConfig.darwin.linearmouse.enable = lib.mkEnableOption "LinearMouse cursor and scroll settings";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "linearmouse" ];

    launchd.user.agents.linearmouse = {
      serviceConfig = {
        ProgramArguments = [ "/Applications/LinearMouse.app/Contents/MacOS/LinearMouse" ];
        RunAtLoad = true;
        KeepAlive = true;
      };
    };

    home-manager.users.${config.system.primaryUser} = {
      xdg.configFile."linearmouse/linearmouse.json".source = ./linearmouse.json;
    };
  };
}
