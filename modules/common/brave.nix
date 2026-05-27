# modules/common/brave.nix
#
# Brave browser (Chromium ベース、独自 Sync v2 持ち)
# - パッケージ供給: pkgs.brave (nixpkgs)
#   macOS は公式 prebuilt zip、Linux はビルド済バイナリを wrap。
#   x86_64/aarch64 × linux/darwin 全対応。
# - 取得時刻を最新化したいときは: nix flake update nixpkgs
#
# プロファイル:
#   - macOS: ~/Library/Application Support/BraveSoftware/Brave-Browser/
#   - Linux: ~/.config/BraveSoftware/Brave-Browser/
#   Nix 管理外。Sync chain 紐付け後はここに sync 状態が保持される。
#
# === 位置づけ ===
# Helium のサブ。Brave Sync v2 (端末間 QR ペアリング, E2E) の検証用。
# 日常使いは Helium、Sync 体験が好印象なら段階的に乗り換え検討。
#
# === Policy の適用方法（OS で全く違う） ===
#
#   ┌────────┬──────────────────────────────────────────────────────────────────┐
#   │ macOS  │ defaults write com.brave.Browser <key> <val>  経由               │
#   │        │ → home-manager の targets.darwin.defaults でやる                 │
#   │        │ ⚠️ Chromium の policies/managed/*.json は macOS では読まれない    │
#   │        │   (これは Linux/Windows のみの規約)                              │
#   ├────────┼──────────────────────────────────────────────────────────────────┤
#   │ Linux  │ ~/.config/BraveSoftware/Brave-Browser/policies/managed/*.json    │
#   │        │ → home.file で JSON 配置 (Chromium 標準パス)                     │
#   └────────┴──────────────────────────────────────────────────────────────────┘
#
# === Brave Origin 相当（収益化系を policy で殺す） ===
# Origin ($60/macOS, Linux 無料) で消える機能を policy 経由で同等に無効化。
# Sync は disable しないので Brave Sync v2 はそのまま使える。
#
# === 既知の問題 ===
# brave-browser#45106 — AI Chat / Rewards / Wallet が policy 設定後も
# 残ってしまうバグが報告されている (2025-2026 にかけて未修正の可能性)。
# brave://policy で "Status: OK" でも UI に残るときは、brave://settings の
# 対応 UI から手動 off にする必要あり。
#
# === 適用確認 ===
# 1. darwin-rebuild / nixos-rebuild switch
# 2. Brave を完全終了 (⌘Q) → 再起動
# 3. brave://policy で全 policy が "Status: OK" になってるか確認
# 4. ダメな policy は brave://settings から手動操作で補う
{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.brave;
  isDarwin = pkgs.stdenv.isDarwin;

  # Brave 独自 policy + Chromium 共通 policy
  # キー名は Brave 公式 Group Policy ドキュメント + brave-core ソース由来
  # https://support.brave.app/hc/en-us/articles/360039248271-Group-Policy
  bravePolicies = {
    # === Brave Origin 相当（収益化機能の無効化） ===
    BraveAIChatEnabled         = false; # Leo (AI assistant)
    BraveRewardsDisabled       = true;  # Rewards / Brave Ads
    BraveWalletDisabled        = true;  # Wallet / Web3 domains
    BraveVPNDisabled           = true;  # 内蔵 VPN
    TorDisabled                = true;  # Private window with Tor
    BraveNewsDisabled          = true;  # News feed
    BraveTalkDisabled          = true;  # Brave Talk (Jitsi)
    BraveWaybackMachineEnabled = false; # 404 時の Wayback 提案
    BraveSpeedreaderEnabled    = false; # Speedreader

    # === テレメトリ ===
    BraveP3AEnabled            = false; # 匿名統計
    BraveStatsPingEnabled      = false; # daily usage ping

    # === Chromium 共通: 不要 UI / 通知 ===
    DefaultBrowserSettingEnabled = false; # 起動時「デフォルトに設定」確認
    MetricsReportingEnabled       = false; # Chromium 側テレメトリ
    SearchSuggestEnabled          = false; # アドレスバー検索候補
    SafeBrowsingProtectionLevel   = 1;     # 0=off 1=standard 2=enhanced
    BookmarkBarEnabled            = true;  # ブックマークバー常時表示
  };

  linuxPolicyRelPath =
    ".config/BraveSoftware/Brave-Browser/policies/managed/origin.json";
in {
  options.myConfig.brave.enable =
    lib.mkEnableOption "Brave browser (Origin-equivalent policies、Sync は有効)";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ pkgs.brave ];

      # macOS: defaults write com.brave.Browser ... を home-manager activation 時に実行
      # Linux ではこの option 自体が no-op になる (targets.darwin.* の前提)
      targets.darwin.defaults."com.brave.Browser" =
        lib.mkIf isDarwin bravePolicies;

      # Linux: Chromium 標準の JSON policy ファイル
      # macOS では空 (lib.optionalAttrs で attribute 自体生やさない)
      home.file = lib.optionalAttrs (!isDarwin) {
        "${linuxPolicyRelPath}".text = builtins.toJSON bravePolicies;
      };
    };
  };
}
