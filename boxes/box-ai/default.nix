{ inputs, ... }: {
  imports = [ ../../modules/nixos/box-base.nix ];

  myConfig.primaryUser = "yuta";

  # この box が所有する永続ディレクトリ
  systemd.tmpfiles.rules = [ "d /data/boxes/ai 0755 root root -" ];

  # ── box-ai コンテナ（隔離された作業空間）─────────────────────────────────────
  containers."ai" = {
    autoStart      = true;   # VM 起動時にコンテナも自動で立ち上がる
    privateNetwork = false;  # AI API への通信が必要なためネット共有

    # /root/ にマウント → root-login で着地した瞬間に永続領域にいる
    bindMounts."/root" = {
      hostPath   = "/data/boxes/ai";
      isReadOnly = false;
    };

    config = { pkgs, ... }: {
      system.stateVersion        = "24.11";
      nixpkgs.config.allowUnfree = true;

      networking.nameservers = [ "1.1.1.1" "8.8.8.8" ];

      # root から使えるようにシステムレベルで入れる
      environment.systemPackages = with pkgs; [
        claude-code
        gemini-cli
        github-copilot-cli
        gh git jq ripgrep fd fzf htop
      ];

      programs.zsh = {
        enable = true;
        interactiveShellInit = ''
          PROMPT='%F{magenta}[box-ai]%f %~ %# '
        '';
      };

      programs.git = {
        enable = true;
        config = {
          user.name  = "yuu-th";
          user.email = "88813495+yuu-th@users.noreply.github.com";
        };
      };

      users.users.root.shell = pkgs.zsh;
    };
  };
}
