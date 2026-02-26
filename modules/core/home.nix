{ pkgs, ... }:

{
  home.packages = with pkgs; [
    # Core CLI Tools
    gh
    jq
    ripgrep
    fd
    fzf
    htop
    # curl and git are typically in here too, but git is configured specifically below
  ];

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
