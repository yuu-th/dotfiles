# modules/darwin/karabiner/default.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.karabiner; in {
  options.myConfig.darwin.karabiner = {
    enable = lib.mkEnableOption "Karabiner-Elements key remapping";
    rules = lib.mkOption {
      type = lib.types.listOf lib.types.attrs;
      default = [];
      description = "List of complex modification rules to inject into Karabiner.";
    };
  };

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "karabiner-elements" ];

    home-manager.users.${config.myConfig.primaryUser} = {
      xdg.configFile."karabiner/karabiner.json" = {
        text = let
          baseJson = builtins.fromJSON (builtins.readFile ./karabiner.json);
          baseProfile = builtins.head baseJson.profiles;
          newRules = baseProfile.complex_modifications.rules ++ cfg.rules;
          newProfile = baseProfile // {
            complex_modifications = baseProfile.complex_modifications // {
              rules = newRules;
            };
          };
          newProfiles = [ newProfile ] ++ (builtins.tail baseJson.profiles);
          newJson = baseJson // { profiles = newProfiles; };
        in builtins.toJSON newJson;
        force = true;
      };
    };
  };
}
