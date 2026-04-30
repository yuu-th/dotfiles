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
# プロジェクト内の flake.nix で特定バージョンを指定する:
#   pkgs.flutter329  # Flutter 3.29.x
#   pkgs.flutter327  # Flutter 3.27.x
#   利用可能なバージョン: https://search.nixos.org/packages?query=flutter
# direnv (nix-direnv) と組み合わせることで cd 時に自動切り替えが可能。
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
