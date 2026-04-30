# modules/darwin/xcode-tools.nix
#
# xcodes — Xcode バージョン管理 CLI (macOS 専用)。
#
# ⚠️  Xcode 本体は Nix で管理不可 (App Store / Apple Developer 経由のみ)。
#     xcodes を使うと複数バージョンのインストール・切り替えが可能になる。
#
# 使い方:
#   xcodes list              # インストール可能なバージョン一覧
#   xcodes install 16.2      # インストール (Apple ID 認証が必要)
#   xcodes select 16.2       # アクティブな Xcode を切り替え
#
# 初回セットアップ:
#   1. xcodes install <latest>
#   2. flutter doctor で確認 (Xcode / CocoaPods のチェック)
#   3. CocoaPods は `gem install cocoapods` または brew install cocoapods
{ config, lib, ... }:
let cfg = config.myConfig.darwin.xcodeTools; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.xcodeTools.enable = lib.mkEnableOption "xcodes CLI for Xcode version management";

  config = lib.mkIf cfg.enable {
    homebrew.brews = [ "xcodes" ];
  };
}
