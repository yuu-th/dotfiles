# modules/common/flutter.nix
#
# Flutter SDK (CLI) + JDK17 をシステム全体に提供する。
#
# ⚠️  提供するもの・しないもの:
#   ✅ flutter / dart コマンド
#   ✅ flutter create, flutter build web/linux, flutter run (web/Linux)
#   ✅ JDK17 (Gradle ビルドに必要)
#   ❌ Android エミュレーター  → nixpkgs の androidenv が aarch64-darwin 非対応
#                                Android Studio (darwin/android-studio.nix) に任せる
#   ❌ iOS ビルド              → Xcode は Nix で管理不可。Nix ストアと衝突する
#
# Android 実機デバッグには android-tools.nix (ADB) も有効にすること。
#
# プロジェクト別に Flutter バージョンを固定したい場合は
# プロジェクト内の flake.nix で pkgs.flutter を上書きする。
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.flutter; in {
  options.myConfig.flutter.enable = lib.mkEnableOption "Flutter SDK and JDK17";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [
        pkgs.flutter
        pkgs.jdk17
      ];
    };
  };
}
