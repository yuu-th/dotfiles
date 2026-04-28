# modules/common/vercel.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.vercel; in {
  options.myConfig.vercel.enable = lib.mkEnableOption "Vercel CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = { lib, ... }: {
      home.sessionPath = [ "$HOME/Library/pnpm" ];

      home.activation.vercelSetup = lib.hm.dag.entryAfter ["writeBoundary"] ''
        if ! command -v vercel &>/dev/null; then
          ${pkgs.pnpm}/bin/pnpm add -g vercel
        fi
      '';
    };
  };
}
