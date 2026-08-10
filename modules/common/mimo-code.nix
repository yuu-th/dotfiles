# modules/common/mimo-code.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.mimoCode; in {
  options.myConfig.mimoCode.enable = lib.mkEnableOption "MiMo Code CLI (Xiaomi)";

  # @mimo-ai/cli は nixpkgs 未収録のため、vercel.nix と同様に pnpm global で導入。
  # Token Plan の apiKey/baseURL は repo に入れず、opencode と同じく
  # ~/.config/mimocode/mimocode.json にローカル手書きで持たせる（秘密はgit追跡外）。
  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = { lib, ... }: {
      # 新しい pnpm は global bin dir を $PNPM_HOME/bin に置く（古い版は $PNPM_HOME 直下）。
      # 既存の shim（vercel 等）は直下にあるので両方 PATH に通す。
      home.sessionPath = [ "$HOME/Library/pnpm" "$HOME/Library/pnpm/bin" ];

      home.activation.mimoCodeSetup = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
        export PNPM_HOME="$HOME/Library/pnpm"
        # pnpm は「global bin dir が PATH に無い」と `pnpm add -g` を ERROR で
        # 失敗させる。$PNPM_HOME だけでは足りないので $PNPM_HOME/bin も通す。
        export PATH="$PNPM_HOME/bin:$PNPM_HOME:$PATH"
        mkdir -p "$PNPM_HOME/bin"
        # NOTE: バイナリ名は初回インストール後に要確認(mimo / mimocode)。
        # 新旧どちらの shim 置き場も見る（片方しか見ないと毎回 pnpm add が走る）。
        if [ ! -x "$PNPM_HOME/bin/mimo" ] && [ ! -x "$PNPM_HOME/mimo" ]; then
          # ⚠️ 失敗しても activation 全体を落とさない。
          # ここで非ゼロ終了すると darwin-rebuild switch が中断し、
          # /run/current-system が更新されないまま古い世代を指し続けてしまう
          # （= 新しい launchd エージェントの store パスが GC 対象になる）。
          ${pkgs.pnpm}/bin/pnpm add -g @mimo-ai/cli \
            || echo "warning: pnpm add -g @mimo-ai/cli failed; skipping" >&2
        fi
      '';
    };
  };
}
