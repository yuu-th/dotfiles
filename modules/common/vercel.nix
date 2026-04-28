# modules/common/vercel.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.vercel; in {
  options.myConfig.vercel.enable = lib.mkEnableOption "Vercel CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = { lib, ... }: {
      home.sessionPath = [ "$HOME/Library/pnpm" ];

      home.activation.vercelSetup = lib.hm.dag.entryAfter ["writeBoundary"] ''
        export PNPM_HOME="$HOME/Library/pnpm"
        mkdir -p "$PNPM_HOME"
        if [ ! -f "$PNPM_HOME/vercel" ]; then
          ${pkgs.pnpm}/bin/pnpm add -g vercel
        fi
      '';
    };
  };
}
