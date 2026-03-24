{ pkgs, lib, ... }:
let
  # ── レイアウト自動建築マクロ (scripts/ 内の外部ファイルを読み込んでパッケージ化) ──
  setupMediaWorkspace = pkgs.writeShellScriptBin "setup-media-workspace" (builtins.readFile ./scripts/setup-media-workspace.sh);

  # 共通設定とモニタ別プロファイルをインポート
  common  = import ./common.nix { inherit pkgs setupMediaWorkspace; };
  profile = import ./profiles/quad-monitor.nix;
in
{
  # スクリプトをグローバルコマンドとしても使用可能にする
  environment.systemPackages = [ setupMediaWorkspace ];

  services.aerospace = {
    enable = true;
    settings = lib.recursiveUpdate common profile;
  };
}
