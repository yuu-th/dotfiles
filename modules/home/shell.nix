{ ... }:

{
  programs.zsh = {
    enable = true;
    enableCompletion    = true;
    autosuggestion.enable      = true;
    syntaxHighlighting.enable  = true;
    oh-my-zsh = {
      enable  = true;
      theme   = "robbyrussell";
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

  programs.git = {
    enable = true;
    settings.user = {
      name  = "yuu-th";
      email = "yutakato333@gmail.com";
    };
  };
}
