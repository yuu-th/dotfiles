{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.darwin.omniwm;

  # ── 共通設定 + プロファイル別 monitorAssignment をマージ ─────────────────
  common  = import ./common.nix     { inherit pkgs; };
  hotkeys = import ./hotkeys.nix    { inherit pkgs; };
  appRules = import ./app-rules.nix { inherit pkgs; };

  helpers = import ./workspace-builder.nix { inherit pkgs; };
  twoProfile    = import ./profiles/two-monitor.nix    { inherit helpers; };
  tripleProfile = import ./profiles/triple-monitor.nix { inherit helpers; };
  quadProfile   = import ./profiles/quad-monitor.nix   { inherit helpers; };

  tomlFormat = pkgs.formats.toml { };

  mkConfig = profile:
    tomlFormat.generate "omniwm-settings.toml" (lib.recursiveUpdate common (profile // {
      inherit hotkeys appRules;
    }));

  twoConfig    = mkConfig twoProfile;
  tripleConfig = mkConfig tripleProfile;
  quadConfig   = mkConfig quadProfile;

  # ── OmniWM バイナリ・ctl のパス ──────────────────────────────────────────
  omniwmApp    = "/Applications/OmniWM.app/Contents/MacOS/OmniWM";
  omniwmctl    = "/opt/homebrew/bin/omniwmctl";
  launchdLabel = "org.nixos.omniwm";

  # ── Script ラッパ：環境変数を埋め込んで writeShellScriptBin ──────────────
  mkScript = name: src: env:
    let
      envBlock = lib.concatStringsSep "\n"
        (lib.mapAttrsToList (k: v: ''export ${k}="${toString v}"'') env);
      bin = pkgs.writeShellScriptBin name ''
        ${envBlock}
        ${builtins.readFile src}
      '';
    in bin;

  baseEnv = {
    OMNIWMCTL = omniwmctl;
    JQ        = "${pkgs.jq}/bin/jq";
  };

  switchProfile = mkScript "omniwm-switch-profile" ./scripts/switch-profile.sh
    (baseEnv // {
      TWO_TOML      = twoConfig;
      TRIPLE_TOML   = tripleConfig;
      QUAD_TOML     = quadConfig;
      LAUNCHD_LABEL = launchdLabel;
    });

  focusMonitorDir = mkScript "omniwm-focus-monitor-dir"
    ./scripts/focus-monitor-dir.sh baseEnv;

  setupMedia = mkScript "omniwm-setup-media-workspace"
    ./scripts/setup-media-workspace.sh baseEnv;

  wsLaunch = mkScript "omniwm-ws-launch" ./scripts/ws-launch.sh baseEnv;

  moveWindowToNamedWS = mkScript "omniwm-move-window-to-named-ws"
    ./scripts/move-window-to-named-ws.sh baseEnv;

  # ── Karabiner ルール（OmniWM enable 時にだけ注入） ──────────────────────
  karabinerRules = import ./karabiner-rules.nix {
    inherit wsLaunch moveWindowToNamedWS setupMedia focusMonitorDir omniwmctl;
  };
in {
  imports = [ ../homebrew.nix ];

  options.myConfig.darwin.omniwm.enable =
    lib.mkEnableOption "OmniWM scrollable tiling window manager";

  config = lib.mkIf cfg.enable {
    # ── 排他制御: AeroSpace と同時 enable は禁止 ─────────────────────────
    assertions = [
      {
        assertion = !config.myConfig.darwin.aerospace.enable;
        message = ''
          OmniWM と AeroSpace は同時に有効化できません。
          profiles/darwin.nix で どちらか一方だけ enable = true にしてください。
        '';
      }
    ];

    # ── Homebrew からのインストール（OmniWM 公式 tap） ─────────────────────
    homebrew.taps  = [ "BarutSRB/tap" ];
    homebrew.casks = [ "omniwm" ];

    # ── ユーザの PATH に置くスクリプト群 ─────────────────────────────────
    environment.systemPackages = [
      switchProfile
      focusMonitorDir
      setupMedia
      wsLaunch
      moveWindowToNamedWS
      pkgs.jq
    ];

    # ── OmniWM 本体: 起動前にプロファイル選択 → exec ──────────────────────
    launchd.user.agents.omniwm = {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && ${switchProfile}/bin/omniwm-switch-profile; exec ${omniwmApp}''
        ];
        KeepAlive = true;
        RunAtLoad = true;
      };
    };

    # ── プロファイル自動切替デーモン（10秒ポーリング、変化時のみ作動） ─────
    launchd.user.agents.omniwm-profile-switcher = {
      serviceConfig = {
        ProgramArguments = [ "${switchProfile}/bin/omniwm-switch-profile" ];
        StartInterval = 10;
        RunAtLoad = true;
      };
    };

    # ── Karabiner レイヤに OmniWM 用ルールを注入 ────────────────────────
    myConfig.darwin.karabiner.rules = karabinerRules;
  };
}
