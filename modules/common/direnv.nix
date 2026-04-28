# modules/common/direnv.nix
#
# direnv — プロジェクト別の環境変数・devShellを自動ロードするツール。
# Flutterなどプロジェクトごとに flake.nix + .envrc ("use flake") を置くことで
# cd するだけで `nix develop` 相当の環境が自動で有効になる。
#
# 使い方:
#   各プロジェクトルートに .envrc を作成:
#     echo "use flake" > .envrc
#     direnv allow
#
# fish / zsh 両方のシェル統合は home-manager が自動で設定する。
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.direnv; in {
  options.myConfig.direnv.enable = lib.mkEnableOption "direnv shell integration";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      programs.direnv = {
        enable = true;
        nix-direnv.enable = true; # nix-direnv: devShell のキャッシュを効かせてロードを高速化
      };
    };
  };
}
