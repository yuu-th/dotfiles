# modules/common/github-copilot-cli.nix
{ config, lib, inputs, pkgs, ... }:
let cfg = config.myConfig.githubCopilotCli; in {
  options.myConfig.githubCopilotCli.enable = lib.mkEnableOption "GitHub Copilot CLI";

  # numtide/llm-agents.nix: Darwin/Linux 共通、1日4回自動更新
  # flake-autoupdate.nix の daily job で --update-input llm-agents を実行して追従
  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser}.home.packages =
      [ inputs.llm-agents.packages.${pkgs.stdenv.hostPlatform.system}.copilot-cli ];
  };
}
