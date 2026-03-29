# modules/darwin/karabiner/default.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.karabiner; in {
  options.myConfig.darwin.karabiner.enable = lib.mkEnableOption "Karabiner-Elements key remapping";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "karabiner-elements" ];

    home-manager.users.${config.system.primaryUser} = {
      xdg.configFile."karabiner/karabiner.json" = {
        source = ./karabiner.json;
        force = true;
      };
    };
  };
}
