# modules/common/vscode.nix
{ config, lib, pkgs, inputs, ... }:
let
  cfg      = config.myConfig.vscode;
  isDarwin = pkgs.stdenv.isDarwin;
  isAarch64 = pkgs.stdenv.isAarch64;

  feedKey = (if isDarwin then "vsci-feed-darwin" else "vsci-feed-linux")
            + (if isAarch64 then "-arm64" else "-x64");
  feed = builtins.fromJSON (builtins.readFile inputs.${feedKey});

  vscode-insiders = (pkgs.vscode.override { isInsiders = true; }).overrideAttrs (oldAttrs: rec {
    pname   = "vscode-insiders";
    version = feed.productVersion or "latest";
    src = pkgs.fetchurl {
      url    = feed.url;
      sha256 = feed.sha256hash;
    };
    installPhase = if isDarwin then ''
      mkdir -p "$out/Applications"
      if [ -d Contents ] && [ -f Contents/Info.plist ]; then
        mkdir -p "$out/Applications/Visual Studio Code - Insiders.app"
        cp -r ./* "$out/Applications/Visual Studio Code - Insiders.app/"
      else
        app=$(find . -maxdepth 1 -type d -name "*.app" -print -quit)
        if [ -n "$app" ]; then
          cp -r "$app" "$out/Applications/"
        else
          echo "error: no .app found after unpacking; listing unpacked tree:" >&2
          ls -laR . >&2 || true
          exit 1
        fi
      fi
    '' else oldAttrs.installPhase;
  });
in {
  options.myConfig.vscode.enable = lib.mkEnableOption "VS Code Insiders with automated hash tracking";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      programs.vscode = {
        enable  = true;
        package = vscode-insiders;
      };

      home.file."bin/vsci-up" = lib.mkIf isDarwin {
        text = ''
          #!/usr/bin/env bash
          set -euo pipefail
          git pull --rebase origin main
          nix flake update --update-input vsci-feed-darwin-arm64 --update-input vsci-feed-darwin-x64 \
            --update-input vsci-feed-linux-arm64 --update-input vsci-feed-linux-x64
          sudo darwin-rebuild switch --flake .#yuta
        '';
        executable = true;
      };

      programs.zsh.shellAliases = lib.mkIf isDarwin {
        vsci-up = "$HOME/bin/vsci-up";
      };
    };
  };
}
