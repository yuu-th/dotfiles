# modules/darwin/pake-webapps/default.nix
#
# Pake webapp マネージャー。
# - pake CLI 自体は homebrew.brews で管理
# - webapp はここで宣言するだけでビルド・インストール・GC が自動化される
# - version フィールドを上げることがリビルドの唯一のトリガー
{ config, lib, ... }:

with lib;

let
  cfg = config.myConfig.darwin.pake;

  webappModule = types.submodule {
    options = {
      url = mkOption {
        type = types.str;
        description = "パッケージ化する Web アプリの URL";
      };
      name = mkOption {
        type = types.str;
        description = "アプリ名 (スペースなし推奨、.app のファイル名になる)";
      };
      width = mkOption {
        type = types.int;
        default = 1200;
        description = "ウィンドウ幅 (px)";
      };
      height = mkOption {
        type = types.int;
        default = 780;
        description = "ウィンドウ高さ (px)";
      };
      icon = mkOption {
        type = types.nullOr types.str;
        default = null;
        description = "アイコンの URL or ローカルパス (.icns/.png/.ico)";
      };
      hideTitleBar = mkOption {
        type = types.bool;
        default = false;
        description = "macOS のタイトルバーを隠す (immersive モード)";
      };
      version = mkOption {
        type = types.str;
        description = "アプリバージョン。これを上げると次回 darwin-rebuild switch 時にリビルドされる";
        example = "1.0.0";
      };
    };
  };

  stateDir = "/Users/${config.system.primaryUser}/.local/state/pake-webapps";

  installScript = app:
    let
      appPath   = "/Applications/${app.name}.app";
      stateFile = "${stateDir}/${app.name}.ver";
      iconFlag  = optionalString (app.icon != null) " --icon ${escapeShellArg app.icon}";
      titleFlag = optionalString app.hideTitleBar " --hide-title-bar";
      pakeCmd   = "pake ${escapeShellArg app.url}"
                + " --name ${escapeShellArg app.name}"
                + " --width ${toString app.width}"
                + " --height ${toString app.height}"
                + iconFlag
                + titleFlag
                + " --app-version ${escapeShellArg app.version}"
                + " --install";
    in ''
      # ── ${app.name} ──────────────────────────────────────────────────
      _installed_ver=""
      [ -f ${escapeShellArg stateFile} ] && _installed_ver=$(cat ${escapeShellArg stateFile})

      if [ "$_installed_ver" != ${escapeShellArg app.version} ] || [ ! -d ${escapeShellArg appPath} ]; then
        echo "[pake-webapps] Building ${app.name} v${app.version}..."
        _tmp=$(mktemp -d)
        (cd "$_tmp" && ${pakeCmd})
        rm -rf "$_tmp"
        echo ${escapeShellArg app.version} > ${escapeShellArg stateFile}
        echo "[pake-webapps] Installed ${app.name}.app -> /Applications/"
      else
        echo "[pake-webapps] ${app.name} v${app.version} is up-to-date, skipping."
      fi
    '';

  gcScript =
    let
      declaredNames = concatStringsSep " " (
        map (a: escapeShellArg a.name) (attrValues cfg.webapps)
      );
    in ''
      # GC: 宣言されていない pake webapp を削除
      [ -d ${escapeShellArg stateDir} ] || exit 0
      _declared=(${declaredNames})
      for _sf in "${stateDir}"/*.ver; do
        [ -f "$_sf" ] || continue
        _n=$(basename "$_sf" .ver)
        _found=0
        for _d in "''${_declared[@]}"; do
          [ "$_d" = "$_n" ] && { _found=1; break; }
        done
        if [ "$_found" = "0" ]; then
          echo "[pake-webapps] GC: removing $_n (no longer declared)"
          rm -f "$_sf"
          rm -rf "/Applications/$_n.app"
        fi
      done
    '';

in {
  options.myConfig.darwin.pake = {
    enable = mkEnableOption "Pake webapp manager";

    webapps = mkOption {
      type    = types.attrsOf webappModule;
      default = {};
      description = "Pake で管理する webapp の宣言的定義。リストから消すだけでアンインストールされる。";
    };
  };

  config = mkIf cfg.enable {
    system.activationScripts.pakeWebapps.text = ''
      set -euo pipefail
      export PATH="/opt/homebrew/bin:/usr/bin:/bin:$PATH"

      if ! command -v pake >/dev/null 2>&1; then
        echo "[pake-webapps] pake not found, skipping"
      else
        mkdir -p ${escapeShellArg stateDir}
        ${concatMapStrings installScript (attrValues cfg.webapps)}
        ${gcScript}
      fi
    '';
  };
}
