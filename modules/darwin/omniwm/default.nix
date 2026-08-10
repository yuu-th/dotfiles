{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.darwin.omniwm;

  # ── 設定ソース ───────────────────────────────────────────────────────────
  common       = import ./common.nix     { inherit pkgs; };
  hotkeys      = import ./hotkeys.nix    { inherit pkgs; };
  appRules     = import ./app-rules.nix  { inherit pkgs lib; };
  helpers      = import ./workspace-builder.nix { inherit pkgs; };
  wsAssignment = import ./workspace-assignment.nix;

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

  # プロファイルは routing の意図（"custom" / "macOS"）を宣言する。
  # deploy.sh が `@@OMNIWM_ROUTING_MODE:<意図>@@` を見て、grid 内の全モニタの
  # displayUUID を解決できた時だけ "custom" を採用し、1 枚でも解決できなければ
  # "macOS" に落とす（`MonitorRouting.completeLayout` が不完全な grid で
  # macOS 配置にフォールバックする挙動と揃えるため）。
  mkSettingsToml = profile:
    tomlFormat.generate "omniwm-settings.toml"
      (lib.recursiveUpdate common {
        inherit hotkeys appRules;
        workspaces = profile.workspaces;
        routing = {
          mode = "@@OMNIWM_ROUTING_MODE:${profile.routing.mode or "macOS"}@@";
        };
        monitorRoutingOverrides = profile.monitorRoutingOverrides or [ ];
      });

  profileTomls = lib.mapAttrs (_: p: mkSettingsToml p) profiles;

  # マニフェスト：deploy.sh が読む。プロファイル名 → toml パス + match 条件
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

  # deploy.sh は system python だけで完結するので jq は要らない。
  deploy = mkScript "omniwm-deploy" ./scripts/deploy.sh {
    PROFILE_MANIFEST = manifestJson;
    SELECTED_PROFILE = cfg.monitorProfile;
    LAUNCHD_LABEL    = launchdLabel;
  };

  moveWindowToNamedWS = mkScript "omniwm-move-window-to-named-ws"
    ./scripts/move-window-to-named-ws.sh baseEnv;

  startupSort = mkScript "omniwm-startup-sort" ./scripts/startup-sort.sh
    (baseEnv // { WS_MAP_JSON = builtins.toJSON wsAssignment; });

  karabinerRules = import ./karabiner-rules.nix {
    inherit moveWindowToNamedWS omniwmctl;
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
      moveWindowToNamedWS
      startupSort
      pkgs.jq
    ];

    launchd.user.agents.omniwm = {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''
            /bin/wait4path /nix/store
            ${deploy}/bin/omniwm-deploy
            ${omniwmApp} &
            OMNIWM_PID=$!
            # OmniWM IPC が立ち上がったら startup-sort を 1 回だけ走らせる
            ( ${startupSort}/bin/omniwm-startup-sort ) &
            wait $OMNIWM_PID
          ''
        ];
        KeepAlive = true;
        RunAtLoad = true;
        ThrottleInterval = 10;
      };
    };

    # モニタ抜き差しで deploy を再実行してプロファイルを再評価する。
    # `watch <channel> --exec <argv...>` が 0.5.9 の正しい形（旧 `-- <child>` ではない）。
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
