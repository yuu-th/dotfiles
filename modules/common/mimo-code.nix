# modules/common/mimo-code.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.mimoCode; in {
  options.myConfig.mimoCode.enable = lib.mkEnableOption "MiMo Code CLI (Xiaomi)";

  # @mimo-ai/cli は nixpkgs 未収録のため、vercel.nix と同様に pnpm global で導入。
  # Token Plan の apiKey/baseURL は repo に入れず、opencode と同じく
  # ~/.config/mimocode/mimocode.json にローカル手書きで持たせる（秘密はgit追跡外）。
  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = { lib, ... }: {
      home.sessionPath = [ "$HOME/Library/pnpm" ];

      home.activation.mimoCodeSetup = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
        export PNPM_HOME="$HOME/Library/pnpm"
        export PATH="$PNPM_HOME:$PATH"
        mkdir -p "$PNPM_HOME"
        # NOTE: バイナリ名は初回インストール後に要確認(mimo / mimocode)。
        # 違う場合はこの guard を直す（効かなくても毎回 pnpm add が走るだけで害はない）。
        if [ ! -f "$PNPM_HOME/mimo" ]; then
          ${pkgs.pnpm}/bin/pnpm add -g @mimo-ai/cli
        fi
      '';
    };
  };
}
