# modules/darwin/android-studio.nix
#
# Android Studio — Android SDK・エミュレーター・Gradle の管理を担う。
# Homebrew cask でインストールする (macOS 専用)。
#
# ❓ なぜ Nix の androidenv を使わないのか:
#   nixpkgs の androidenv.composeAndroidPackages は aarch64-darwin (Apple Silicon) で
#   動作しない。SDK tarball の定義が存在せず、エミュレーターも x86_64 バイナリを
#   生成するため実用不可 (nixpkgs#303968, #359918)。
#
# Android Studio インストール後にやること:
#   1. SDK Manager で "Android SDK Platform-Tools" をインストール
#   2. ANDROID_HOME を設定 (通常 ~/Library/Android/sdk)
#      → プロジェクトの flake.nix の shellHook に記述するのが推奨 (SSOT)
#   3. flutter doctor --android-licenses でライセンス同意
{ config, lib, ... }:
let cfg = config.myConfig.darwin.androidStudio; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.androidStudio.enable = lib.mkEnableOption "Android Studio";

  config = lib.mkIf cfg.enable {
    homebrew.casks = [ "android-studio" ];
  };
}
