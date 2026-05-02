{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.darwin.omniwm;

  # ── 設定ソース ───────────────────────────────────────────────────────────
  common   = import ./common.nix     { inherit pkgs; };
  hotkeys  = import ./hotkeys.nix    { inherit pkgs; };
  appRules = import ./app-rules.nix  { inherit pkgs; };
  helpers  = import ./workspace-builder.nix { inherit pkgs; };

  # ── 選択中のモニタプロファイルを import ─────────────────────────────────
  profilePath = ./monitor-profiles + "/${cfg.monitorProfile}.nix";
  monitorProfile = import profilePath { inherit helpers; };

  tomlFormat = pkgs.formats.toml { };

  settingsToml = tomlFormat.generate "omniwm-settings.toml"
    (lib.recursiveUpdate common (monitorProfile // {
      inherit hotkeys appRules;
    }));

  # ── パス定義 ─────────────────────────────────────────────────────────────
  omniwmApp    = "/Applications/OmniWM.app/Contents/MacOS/OmniWM";
  omniwmctl    = "/opt/homebrew/bin/omniwmctl";
  launchdLabel = "org.nixos.omniwm";

  # ── Script ラッパ ────────────────────────────────────────────────────────
  mkScript = name: src: env:
    let
      envBlock = lib.concatStringsSep "\n"
        (lib.mapAttrsToList (k: v: ''export ${k}="${toString v}"'') env);
    in pkgs.writeShellScriptBin name ''
      ${envBlock}
      ${builtins.readFile src}
    '';

  baseEnv = {
    OMNIWMCTL = omniwmctl;
    JQ        = "${pkgs.jq}/bin/jq";
  };

  deploy = mkScript "omniwm-deploy" ./scripts/deploy.sh (baseEnv // {
    SETTINGS_TOML = settingsToml;
    LAUNCHD_LABEL = launchdLabel;
  });

  focusMonitorDir = mkScript "omniwm-focus-monitor-dir"
    ./scripts/focus-monitor-dir.sh baseEnv;

  setupMedia = mkScript "omniwm-setup-media-workspace"
    ./scripts/setup-media-workspace.sh baseEnv;

  wsLaunch = mkScript "omniwm-ws-launch" ./scripts/ws-launch.sh baseEnv;

  moveWindowToNamedWS = mkScript "omniwm-move-window-to-named-ws"
    ./scripts/move-window-to-named-ws.sh baseEnv;

  karabinerRules = import ./karabiner-rules.nix {
    inherit wsLaunch moveWindowToNamedWS setupMedia focusMonitorDir omniwmctl;
  };
in {
  imports = [ ../homebrew.nix ];

  options.myConfig.darwin.omniwm = {
    enable = lib.mkEnableOption "OmniWM scrollable tiling window manager";

    monitorProfile = lib.mkOption {
      type = lib.types.str;
      default = "default";
      description = ''
        Active monitor profile name.
        Looks up modules/darwin/omniwm/monitor-profiles/<name>.nix.
        新しいプロファイルを足す場合は monitor-profiles/ に <name>.nix を作成して
        この値を切り替えるだけ。default は main/secondary だけのフォールバック。
      '';
      example = "office-3mon";
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = !config.myConfig.darwin.aerospace.enable;
        message = ''
          OmniWM と AeroSpace は同時に有効化できません。
          profiles/darwin.nix で どちらか一方だけ enable = true にしてください。
        '';
      }
      {
        assertion = builtins.pathExists profilePath;
        message = ''
          monitorProfile = "${cfg.monitorProfile}" のファイルが存在しません。
          modules/darwin/omniwm/monitor-profiles/${cfg.monitorProfile}.nix を作成するか、
          別の既存プロファイル名を指定してください。既存: default, office-3mon
        '';
      }
    ];

    homebrew.taps  = [ "BarutSRB/tap" ];
    homebrew.casks = [ "omniwm" ];

    environment.systemPackages = [
      deploy
      focusMonitorDir
      setupMedia
      wsLaunch
      moveWindowToNamedWS
      pkgs.jq
    ];

    # ── OmniWM 本体: launchd で起動・KeepAlive・runtime deploy ─────────────
    launchd.user.agents.omniwm = {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && ${deploy}/bin/omniwm-deploy; exec ${omniwmApp}''
        ];
        KeepAlive = true;
        RunAtLoad = true;
        ThrottleInterval = 10;
      };
    };

    # ── イベント駆動 watcher（モニタ抜き差し検知） ──────────────────────
    # --no-send-initial で起動時の暴発を防ぐ。deploy.sh は idempotent。
    launchd.user.agents.omniwm-display-watcher = {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && exec ${omniwmctl} watch display-changed --no-send-initial --exec ${deploy}/bin/omniwm-deploy''
        ];
        KeepAlive = true;
        RunAtLoad = true;
        ThrottleInterval = 10;
      };
    };

    myConfig.darwin.karabiner.rules = karabinerRules;
  };
}
