{ config, lib, ... }:
let cfg = config.myConfig.darwin.parsec; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.parsec.enable = lib.mkEnableOption "Parsec (Remote streaming service client)";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "parsec" ];
  };
}
