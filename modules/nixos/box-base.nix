# modules/nixos/box-base.nix
#
# 全 box が共有する OrbStack VM ホストの定型設定。
# 各 boxes/box-*/default.nix はこれを import し、固有部分だけを記述する。
{ config, lib, pkgs, ... }: {
  imports = [
    ./orbstack-vm.nix
    ../common/primary-user.nix
  ];

  nixpkgs.hostPlatform    = "aarch64-linux";
  nixpkgs.config.allowUnfree = true;

  # OrbStack がログインに使うホスト側ユーザー
  users.users.${config.myConfig.primaryUser} = {
    isNormalUser = true;
    extraGroups  = [ "wheel" ];
    shell        = pkgs.zsh;
  };

  programs.zsh.enable = true;

  # 開発用 VM のため wheel グループは passwordless sudo
  security.sudo.wheelNeedsPassword = false;

  environment.systemPackages = with pkgs; [ git ];

  nix.settings = {
    experimental-features = [ "nix-command" "flakes" ];
    trusted-users         = [ "root" "@wheel" ];
  };

  system.stateVersion = "24.11";
}
