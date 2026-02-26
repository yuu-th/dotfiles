{ pkgs, lib, config, inputs, ... }:
let
  isDarwin = pkgs.stdenv.isDarwin;
  isAarch64 = pkgs.stdenv.isAarch64;

  # Select the correct input feed based on the current platform
  feedKey = (if isDarwin then "vsci-feed-darwin" else "vsci-feed-linux")
            + (if isAarch64 then "-arm64" else "-x64");

  # builtins.readFile inputs.${feedKey} parses the JSON fetched via the flake input
  feed = builtins.fromJSON (builtins.readFile inputs.${feedKey});

  # Build VS Code Insiders with the URL/Hash from our pure flake inputs
  vscode-insiders = (pkgs.vscode.override { isInsiders = true; }).overrideAttrs (oldAttrs: rec {
    pname = "vscode-insiders";
    version = feed.productVersion or "latest";

    src = pkgs.fetchurl {
      url = feed.url;
      sha256 = feed.sha256hash;
    };

    installPhase = if isDarwin then ''
      mkdir -p "$out/Applications"
      if [ -d Contents ] && [ -f Contents/Info.plist ]; then
        # We are already inside the app bundle (common when zip has only one top-level dir)
        mkdir -p "$out/Applications/Visual Studio Code - Insiders.app"
        cp -r ./* "$out/Applications/Visual Studio Code - Insiders.app/"
      else
        # Not inside, find the .app at the root, but don't look deeper to avoid helper apps
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
in
{
  # Only enable VS Code if the user hasn't explicitly disabled it
  options.dotfiles.vscode.enable = lib.mkOption {
    type = lib.types.bool;
    default = true;
    description = "Enable VS Code Insiders with automated hash tracking";
  };

  config = lib.mkIf config.dotfiles.vscode.enable {
    programs.vscode = {
      enable = true;
      package = vscode-insiders;
    };
  };
}
