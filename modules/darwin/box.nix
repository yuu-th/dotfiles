# modules/darwin/box.nix
#
# macOS から boxes/ 環境に透過的にアクセスするコマンド。
# OrbStack VM や nixos-container の存在を意識せず使える。
#
# 使い方:
#   box <name>          コンテナのシェルに入る
#   box switch <name>   環境を再定義して再起動（darwin-rebuild switch と同じ感覚）
#   box stop <name>     コンテナを停止
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.darwin.box; in {
  imports = [ ./orbstack.nix ];  # orb コマンドの提供元

  options.myConfig.darwin.box = {
    enable = lib.mkEnableOption "box command";
    flakePath = lib.mkOption {
      type        = lib.types.str;
      default     = "$HOME/dev/dotfiles";
      description = "dotfiles flake へのパス（box switch で使用）";
    };
  };

  config = lib.mkIf cfg.enable {
    myConfig.darwin.orbstack.enable = true;

    environment.systemPackages = [
      (pkgs.writeShellScriptBin "box" ''
        set -euo pipefail

        usage() {
          echo "Usage:"
          echo "  box <name>          コンテナのシェルに入る"
          echo "  box switch <name>   環境を再定義して適用"
          echo "  box stop <name>     コンテナを停止"
        }

        case "''${1:-}" in
          switch)
            [ -z "''${2:-}" ] && { usage; exit 1; }
            echo "→ box-$2 を再構築中..."
            orb run -m "box-$2" \
              sudo nixos-rebuild switch --flake "${cfg.flakePath}#box-$2"
            echo "✓ box-$2 適用完了"
            ;;
          stop)
            [ -z "''${2:-}" ] && { usage; exit 1; }
            orb run -m "box-$2" sudo nixos-container stop "$2"
            ;;
          ""|--help|-h)
            usage
            ;;
          *)
            # コンテナが未設定（初回）なら自動でセットアップ
            if ! orb run -m "box-$1" systemctl is-enabled "container@$1" >/dev/null 2>&1; then
              echo "→ 初回セットアップ: box-$1 を構築中..."
              orb run -m "box-$1" sudo nixos-rebuild switch --flake "${cfg.flakePath}#box-$1"
            fi
            orb run -m "box-$1" sudo nixos-container root-login "$1"
            ;;
        esac
      '')
    ];
  };
}
