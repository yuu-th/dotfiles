# modules/common/helium.nix
#
# Helium browser (Chromium ベース、imput 製、プライバシー重視)
# - パッケージ供給: inputs.helium-flake (github:amaanq/helium-flake)
#   x86_64-linux / aarch64-linux / x86_64-darwin / aarch64-darwin 全対応。
#   上流リリース後 15 分以内に flake 内 hash が自動更新される。
# - 取得時刻を最新化したいときは: nix flake update helium-flake
#
# プロファイル / 機密データ:
#   - macOS: ~/Library/Application Support/net.imput.helium/
#   - Linux: ~/.config/helium/
#   Nix 管理外。ログイン状態・パスワード・Cookie・履歴・拡張機能はここに溜まる。
#
# ⚠️ macOS で過去に手動 install した /Applications/Helium.app がある場合、
#   Nix store 経由の .app と二重になる。初回 darwin-rebuild 前に削除推奨:
#     rm -rf /Applications/Helium.app
#   プロファイル（~/Library/Application Support/net.imput.helium）は触らない。
#
# === 拡張機能は手動 install で運用（Helium 設計上の制約） ===
# ExtensionInstallForcelist policy 経由の auto-install は試した結果、
# 個人マシン (MDM 非管理) では **Helium 独自 web store の curated subset
# にホストされている拡張機能のみ** policy install が許可される仕様だった。
# Bitwarden / Vimium / Floccus はこの subset に含まれず、URL を正しく
# 設定 (clients2.9oo91e.qjz9zk) しても install されない。
#
# Helium の手動 install は CWS 全件を proxy 経由で匿名取得できるので、
# 「手動 install + プロファイルで保持」が公式想定運用。
# 拡張を入れる回数はマシン更新時のみなので運用コスト低い。
#
# 推奨拡張:
#   - Bitwarden:  https://chromewebstore.google.com/detail/nngceckbapebfimnlniiiahkandclblb
#   - Vimium:     https://chromewebstore.google.com/detail/dbepggeogbaibhgnhhndojpepiihcmeb
#   - Floccus:    https://chromewebstore.google.com/detail/fnaicdffflnofjppbagibeoednhnbjhg
#     Floccus → Add account → GitHub Gist
#       トークン / Gist ID は Bitwarden 内 `floccus-gist-*` を参照
#       Floccus の "Encryption" option を ON にして E2E にする
#
# === Chrome からの一回限り import（必要なら） ===
# Settings → Import data from another browser → Chrome
# ブックマーク・履歴は取れる。パスワード/Cookie は Helium 仕様で取れない。
{ config, lib, pkgs, inputs, ... }:
let
  cfg = config.myConfig.helium;
  heliumPkg = inputs.helium-flake.packages.${pkgs.stdenv.hostPlatform.system}.default;
in {
  options.myConfig.helium.enable = lib.mkEnableOption "Helium browser";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ heliumPkg ];
    };
  };
}
