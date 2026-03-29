# modules/darwin/google-calendar.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.googleCalendar; in {
  imports = [ ./pake-webapps ];

  options.myConfig.darwin.googleCalendar = {
    enable = lib.mkEnableOption "Google Calendar as native app";
    version = lib.mkOption {
      type    = lib.types.str;
      default = "1.0.0";
      description = "バージョンを上げると次回 darwin-rebuild switch 時にリビルドされる";
    };
  };

  config = lib.mkIf cfg.enable {
    myConfig.darwin.pake.webapps.calendar = {
      url    = "https://calendar.google.com";
      name   = "GoogleCalendar";
      width  = 1440;
      height = 900;
      version = cfg.version;
    };
  };
}
