# modules/common/browser-use.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.browserUse; in {
  options.myConfig.browserUse.enable = lib.mkEnableOption "browser-use AI browser automation";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.sessionPath = [
        "$HOME/.browser-use-env/bin"
        "$HOME/.browser-use/bin"
      ];

      home.activation.browserUseSetup = lib.hm.dag.entryAfter ["writeBoundary"] ''
        if [ ! -d "$HOME/.browser-use-env" ]; then
          ${pkgs.uv}/bin/uv venv "$HOME/.browser-use-env"
        fi
        if [ ! -f "$HOME/.browser-use-env/bin/playwright" ]; then
          "$HOME/.browser-use-env/bin/uv" pip install browser-use playwright
          "$HOME/.browser-use-env/bin/playwright" install chromium
        fi
      '';
    };
  };
}
