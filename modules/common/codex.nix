# modules/common/codex.nix
{ config, lib, inputs, pkgs, ... }:
let cfg = config.myConfig.codex; in {
  options.myConfig.codex.enable = lib.mkEnableOption "Codex CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser}.home.packages =
      [ inputs.llm-agents.packages.${pkgs.stdenv.hostPlatform.system}.codex ];
  };
}
