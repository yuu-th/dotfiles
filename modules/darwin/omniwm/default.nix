{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.darwin.omniwm;

  # ── 設定ソース ───────────────────────────────────────────────────────────
  common   = import ./common.nix     { inherit pkgs; };
  hotkeys  = import ./hotkeys.nix    { inherit pkgs; };
  appRules = import ./app-rules.nix  { inherit pkgs; };
  helpers  = import ./workspace-builder.nix { inherit pkgs; };

  # ── 全プロファイルを自動 discover & ビルド ───────────────────────────────
  profileDir = ./monitor-profiles;
  profileFiles =
    let entries = builtins.readDir profileDir;
    in lib.filter (n: lib.hasSuffix ".nix" n)
      (builtins.attrNames entries);
  profileNames = map (f: lib.removeSuffix ".nix" f) profileFiles;

  loadProfile = name:
    import (profileDir + "/${name}.nix") { inherit helpers; };

  profiles = lib.listToAttrs (map (n: {
    name = n;
    value = loadProfile n;
  }) profileNames);

  tomlFormat = pkgs.formats.toml { };

  mkSettingsToml = profile:
    tomlFormat.generate "omniwm-settings.toml"
      (lib.recursiveUpdate common ({
        inherit hotkeys appRules;
        workspaces = profile.workspaces;
      }));

  # 各プロファイルの TOML パスを作成
  profileTomls = lib.mapAttrs (_: p: mkSettingsToml p) profiles;

  # マニフェスト：deploy.sh が読み込む。プロファイル名 → toml パス + match
  manifest = lib.mapAttrsToList (name: profile: {
    inherit name;
    toml = profileTomls.${name};
    match = profile.match or { };
  }) profiles;

  manifestJson = builtins.toJSON manifest;

  # ── パス定義 ─────────────────────────────────────────────────────────────
  omniwmApp    = "/Applications/OmniWM.app/Contents/MacOS/OmniWM";
  omniwmctl    = "/opt/homebrew/bin/omniwmctl";
  launchdLabel = "org.nixos.omniwm";

  # ── Script ラッパ ────────────────────────────────────────────────────────
  mkScript = name: src: env:
    let
      envBlock = lib.concatStringsSep "\n"
        (lib.mapAttrsToList (k: v: ''export ${k}=${lib.escapeShellArg (toString v)}'') env);
    in pkgs.writeShellScriptBin name ''
      ${envBlock}
      ${builtins.readFile src}
    '';

  baseEnv = {
    OMNIWMCTL = omniwmctl;
    JQ        = "${pkgs.jq}/bin/jq";
  };

  deploy = mkScript "omniwm-deploy" ./scripts/deploy.sh (baseEnv // {
    PROFILE_MANIFEST = manifestJson;
    SELECTED_PROFILE = cfg.monitorProfile;
    LAUNCHD_LABEL    = launchdLabel;
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
      default = "auto";
      description = ''
        Active monitor profile.
          "auto"           = 接続中モニタから自動検出（match 条件で選択）
          "<name>"         = monitor-profiles/<name>.nix を強制使用
        match を持つプロファイルが優先、無ければ default に fallback。
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
        assertion = cfg.monitorProfile == "auto"
          || builtins.elem cfg.monitorProfile profileNames;
        message = ''
          monitorProfile = "${cfg.monitorProfile}" は存在しません。
          利用可能: "auto", ${
            lib.concatMapStringsSep ", " (n: "\"${n}\"") profileNames
          }
          新規プロファイルを足す場合は monitor-profiles/<name>.nix を作成してください。
        '';
      }
      {
        assertion = builtins.elem "default" profileNames;
        message = ''
          monitor-profiles/default.nix が必要です（auto 検出の最終 fallback）。
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
