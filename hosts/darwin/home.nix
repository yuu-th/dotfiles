{ pkgs, lib, inputs, ... }: {
  imports = [
    ../../modules/core/home.nix
    ../../modules/dev/home.nix
    ../../modules/ops/home.nix
    ../../modules/editors/vscode.nix
    ../../modules/editors/antigravity.nix
  ];

  home = {
    username = "yuta";
    homeDirectory = lib.mkForce "/Users/yuta";
    stateVersion = "24.11";
  };

  # VS Code Insiders 手動更新 + 環境適用のヘルパー
  # (サーバの自動更新を待たずに今すぐ更新したい場合に使用)
  home.file."bin/vsci-up" = {
    text = ''
      #!/usr/bin/env bash
      set -euo pipefail
      # まず GitHub の最新（サーバの自動更新分など）を取り込む
      git pull --rebase origin main
      # 手動で最新のハッシュを取得
      nix flake update --update-input vsci-feed-darwin-arm64 --update-input vsci-feed-darwin-x64 \
        --update-input vsci-feed-linux-arm64 --update-input vsci-feed-linux-x64
      # システムに適用
      sudo darwin-rebuild switch --flake .#yuta
    '';
    executable = true;
  };

  programs.zsh.shellAliases = {
    vsci-up = "$HOME/bin/vsci-up";
  };
}
