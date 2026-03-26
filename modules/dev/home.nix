{ pkgs, ... }:

{
  home.packages = with pkgs; [
    rustc
    cargo
    go
    python3
    nodejs_22
  ];
  
  # npm prefix を Nix store 外に向ける
  home.file.".npmrc".text = ''
    prefix=''${HOME}/.npm-global
  '';

  # ~/.npm-global/bin を PATH に追加
  home.sessionPath = [
    "$HOME/.npm-global/bin"
  ];
}
