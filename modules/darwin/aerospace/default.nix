{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.darwin.aerospace;

  setupMediaWorkspace = pkgs.writeShellScriptBin "setup-media-workspace"
    (builtins.readFile ./scripts/setup-media-workspace.sh);

  common = import ./common.nix { inherit pkgs setupMediaWorkspace; };
  tomlFormat = pkgs.formats.toml { };

  mkConfig = profile:
    tomlFormat.generate "aerospace.toml" (lib.recursiveUpdate common profile);

  twoConfig    = mkConfig (import ./profiles/two-monitor.nix);
  tripleConfig = mkConfig (import ./profiles/triple-monitor.nix);
  quadConfig   = mkConfig (import ./profiles/quad-monitor.nix);

  # モニター数を検出し、変化があったときだけ設定を切り替える
  switchProfile = pkgs.writeShellScriptBin "aerospace-switch-profile" ''
    # AeroSpace起動前は system_profiler で検出（起動後は aerospace list-monitors が速い）
    if pgrep -x AeroSpace > /dev/null 2>&1; then
      COUNT=$(aerospace list-monitors 2>/dev/null | wc -l | tr -d ' ')
    else
      COUNT=$(system_profiler SPDisplaysDataType 2>/dev/null | grep -c "Resolution:")
    fi

    STATE="$HOME/.local/share/aerospace/monitor-count"
    PREV=$(cat "$STATE" 2>/dev/null || echo "")

    [ "$COUNT" = "$PREV" ] && exit 0

    case "$COUNT" in
      2) PROFILE="${twoConfig}"    ;;
      3) PROFILE="${tripleConfig}" ;;
      4) PROFILE="${quadConfig}"   ;;
      *) exit 0 ;;
    esac

    mkdir -p "$(dirname "$STATE")" "$HOME/.config/aerospace"
    cp "$PROFILE" "$HOME/.config/aerospace/aerospace.toml"
    echo "$COUNT" > "$STATE"
    pgrep -x AeroSpace > /dev/null 2>&1 && aerospace reload-config
  '';

  aerospaceBin =
    "${pkgs.aerospace}/Applications/AeroSpace.app/Contents/MacOS/AeroSpace";

in {
  options.myConfig.darwin.aerospace.enable =
    lib.mkEnableOption "AeroSpace tiling window manager";

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [
      setupMediaWorkspace
      switchProfile
      pkgs.aerospace
    ];

    # AeroSpace 本体 — 起動前にプロファイル選択、可変パスで起動
    launchd.user.agents.aerospace = {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          "/bin/wait4path /nix/store && ${switchProfile}/bin/aerospace-switch-profile; exec ${aerospaceBin} --config-path $HOME/.config/aerospace/aerospace.toml"
        ];
        KeepAlive = true;
        RunAtLoad = true;
      };
    };

    # プロファイル自動切替デーモン — 10 秒ポーリング、変化時のみ動作
    launchd.user.agents.aerospace-profile-switcher = {
      serviceConfig = {
        ProgramArguments = [
          "${switchProfile}/bin/aerospace-switch-profile"
        ];
        StartInterval = 10;
        RunAtLoad = true;
      };
    };
  };
}
