# modules/common/zsh.nix
{ config, lib, ... }:
let cfg = config.myConfig.zsh; in {
  options.myConfig.zsh.enable = lib.mkEnableOption "zsh with oh-my-zsh";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      programs.zsh = {
        enable = true;
        enableCompletion = true;
        autosuggestion.enable = true;
        syntaxHighlighting.enable = true;
        oh-my-zsh = {
          enable = true;
          theme = "robbyrussell";
          plugins = [ "git" "fzf" ];
        };
        shellAliases = {
          ll = "ls -alF";
          gs = "git status";
          ga = "git add";
          gc = "git commit";
          gp = "git push";
        };
      };
    };
  };
}
