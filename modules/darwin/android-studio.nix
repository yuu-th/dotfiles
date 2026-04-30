# modules/darwin/android-studio.nix
#
# Android Studio — Android SDK・エミュレーター・Gradle の管理を担う。
# Homebrew cask でインストールする (macOS 専用)。
#
# ❓ なぜ Nix の androidenv を使わないのか:
#   nixpkgs の androidenv.composeAndroidPackages は aarch64-darwin (Apple Silicon) で
#   動作しない。SDK tarball の定義が存在せず、エミュレーターも x86_64 バイナリを
#   生成するため実用不可 (nixpkgs#303968, #359918)。
#   Android Studio は Apple Silicon ネイティブの ARM64 エミュレーターを提供するため、
#   これが aarch64-darwin での唯一の実用的な選択肢。
#
# Android Studio インストール後にやること:
#   1. SDK Manager で "Android SDK Platform-Tools" をインストール
#   2. flutter doctor --android-licenses でライセンス同意
#
# ANDROID_HOME は ~/Library/Android/sdk に自動設定される (sessionVariables)。
{ config, lib, ... }:
let cfg = config.myConfig.darwin.androidStudio; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.androidStudio.enable = lib.mkEnableOption "Android Studio";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "android-studio" ];

    home-manager.users.${config.myConfig.primaryUser} = {
      home.sessionVariables = {
        ANDROID_HOME = "$HOME/Library/Android/sdk";
        ANDROID_SDK_ROOT = "$HOME/Library/Android/sdk";
      };
      home.sessionPath = [
        "$HOME/Library/Android/sdk/platform-tools"
        "$HOME/Library/Android/sdk/emulator"
      ];
    };
  };
}
