# modules/common/android-tools.nix
#
# android-tools — ADB (Android Debug Bridge) などの実機デバッグツール群。
# aarch64-darwin / x86_64-linux 両対応。
#
# 含まれる主なコマンド: adb, fastboot, mkbootimg, etc.
#
# Android SDK 全体 (sdkmanager, エミュレーターなど) は含まない。
# SDK 管理は Android Studio (darwin/android-studio.nix) に任せる。
#
# 実機デバッグの手順:
#   1. Android Studio でプラットフォームツールをインストール済みであること
#   2. デバイスの USB デバッグを有効にする
#   3. adb devices で認識確認
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.androidTools; in {
  options.myConfig.androidTools.enable = lib.mkEnableOption "Android Debug Bridge (adb, fastboot)";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.android-tools ];
    };
  };
}
