# modules/nixos/orbstack-vm.nix
#
# OrbStack VM 向けハードウェア設定。
#
# OrbStack は Apple Virtualization Framework でカーネルを直接起動するため、
# NixOS 側の bootloader は不要。filesystem の実マウントも OrbStack (vinit) が行う。
# このモジュールは NixOS の assertion を満たすための最小宣言。
{ ... }: {
  # OrbStack が bootloader を管理する（EFI なし、GRUB なし）
  boot.loader.grub.enable = false;

  # 実体は /dev/vdb1 の btrfs サブボリューム（サブボリューム ID は VM ごとに動的）
  # 実マウントは OrbStack の vinit が行うため、ここは assertion を満たす宣言のみ
  fileSystems."/" = {
    device  = "/dev/vdb1";
    fsType  = "btrfs";
    options = [ "noatime" "nodatasum" "nodatacow" ];
  };
}
