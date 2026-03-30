# modules/common/rust.nix
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.rust; in {
  options.myConfig.rust.enable = lib.mkEnableOption "Rust development environment";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = with pkgs; [ rustc cargo ];
      home.sessionPath = [ "$HOME/.cargo/bin" ];
    };
  };
}
