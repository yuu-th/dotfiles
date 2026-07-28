# modules/common/wrangler.nix
{ config, lib, pkgs, inputs, ... }:
let
  cfg = config.myConfig.wrangler;
  # unstable の wrangler 4.93.0 は darwin で EBADF ビルド破損(nixpkgs#423082 系)のため、
  # 動作する旧 rev (nixpkgs-wrangler) の 4.62.0 を使う。上流修正後は inputs を消して
  # pkgs.wrangler に戻す。
  wrangler = inputs.nixpkgs-wrangler.legacyPackages.${pkgs.stdenv.hostPlatform.system}.wrangler;
in {
  options.myConfig.wrangler.enable = lib.mkEnableOption "Cloudflare Workers/Pages CLI";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ wrangler ];
    };
  };
}
