{ pkgs, ... }:

{
  home.packages = with pkgs; [
    rustc
    cargo
    go
    python3
    nodejs_22
  ];
}
