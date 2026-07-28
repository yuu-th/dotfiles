{ config, lib, pkgs, ... }:
let cfg = config.myConfig.pearDesktop; in {
  options.myConfig.pearDesktop.enable =
    lib.mkEnableOption "Pear Desktop (旧 th-ch/youtube-music: 広告ブロック / SponsorBlock / 歌詞 / Discord RPC 付き YouTube Music ラッパ)";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.pear-desktop ];
    };
  };
}
