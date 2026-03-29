# modules/darwin/dia.nix
{ config, lib, ... }:
let cfg = config.myConfig.darwin.dia; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.dia.enable = lib.mkEnableOption "Dia browser";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "thebrowsercompany-dia" ];
  };
}
