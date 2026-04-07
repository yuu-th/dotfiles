# modules/darwin/raycast.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.raycast; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.raycast.enable = lib.mkEnableOption "Raycast";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "raycast" ];

    # Karabiner: cmd+space → opt+space (Raycast が opt+space を hotkey として使用)
    # Spotlight の cmd+space を Raycast に横取りさせる
    myConfig.darwin.karabiner.rules = [
      {
        description = "cmd+space → opt+space (Raycast)";
        manipulators = [
          {
            type = "basic";
            from = {
              key_code = "spacebar";
              modifiers.mandatory = [ "command" ];
            };
            to = [
              {
                key_code = "spacebar";
                modifiers = [ "option" ];
              }
            ];
          }
        ];
      }
    ];
  };
}
