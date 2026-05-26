# modules/darwin/projwm/default.nix
#
# projwm-next — production-shaped AI workspace manager
#
# 設計: queue/{design.md, implementation-design.md, specs.md}
#
# 構成:
#   - projwmd (production daemon, single writer)
#   - projwmctl / projwmevent / projwmstore-bootstrap (CLI / sidecars)
#   - launchd agents: projwmd-next + 各 event sidecar (windows-changed,
#     display-changed, layout-changed, safety-timer, wake)
#
# 注: legacy `projwm` (cmd/projwm + internal/projwm + 旧 launchd エージェント)
# は handoff §4.7 cutover で削除済み。`launchctl list | grep projwm` で
# `projwm-next-*` と `projwmd-next` のみが running 状態であることを期待する。
{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.darwin.projwm;
  user = config.myConfig.primaryUser;
  userHome = "/Users/${user}";

  # ── 開発・運用用 sudo nopass は ./sudoers.nix を import ─────────────────

  projwmNext = pkgs.buildGoModule {
    pname = "projwm-next";
    version = "0.1.0-dev";
    src = ./projwm-next;
    # bubbletea + bubbles + lipgloss などの依存があるため null から
    # 実 hash に変更。依存変更時は lib.fakeHash に戻して darwin-rebuild
    # 実行 → エラーから正しい hash を取得して再設定。
    vendorHash = "sha256-gPLuRTQ9suGQuDDA39rP6H9MIXnkB6BIzVTfraTeYl4=";
    subPackages = [
      "cmd/projwmd"
      "cmd/projwmctl"
      "cmd/projwmevent"
      "cmd/projwmstore-bootstrap"
      "cmd/projwm"
      "cmd/projwm-cockpit"
    ];
    ldflags = [ "-s" "-w" ];
    doCheck = true;
    meta = with lib; {
      description = "Production-shaped next-generation projwm daemon and CLI";
      platforms = platforms.darwin;
    };
  };

  nextStateDir = cfg.next.stateDir;
  nextStoreDir = cfg.next.storeDir;
  nextPrivatePayloadDir = "${nextStateDir}/private-payloads";
  nextEventQueueDir = "${nextStateDir}/event-queue";
  nextSocketPath = cfg.next.socketPath;
  nextLogDir = "${nextStateDir}/logs";
  nextStartupProvenancePath = "${nextStateDir}/startup-provenance.json";
  nextLaunchdLabel = "org.nixos.projwmd-next";

  nextManagedEnvironment = {
    schemaVersion = 1;
    authority = "nix";
    source = "modules/darwin/projwm/default.nix:next";
    minProjwmdVersion = "0.1.0";
    windowManager = {
      backend = "omniwm";
      layout = {
        defaultColumnWidth = 0.5;
        columnWidthPresets = [ 0.4 0.5 0.66 0.8 0.95 ];
        maxVisibleColumns = 4;
        maxWindowsPerColumn = 4;
        centerFocusedColumn = "never";
        alwaysCenterSingle = true;
      };
      focus = {
        followsMouse = false;
        followsWindowToMonitor = true;
        moveMouseToFocusedWindow = true;
      };
    };
    # SSOT §10.7: top-level array shape for workspaces / slots / apps.
    # Viewer workspace is identified by role="viewer" (no separate field).
    workspaces = [
      { id = "1"; rawName = "1"; displayName = "1"; role = "general"; }
      { id = "2"; rawName = "2"; displayName = "2"; role = "general"; }
      { id = "3"; rawName = "3"; displayName = "3"; role = "general"; }
      { id = "4"; rawName = "4"; displayName = "4"; role = "general"; }
      { id = "5"; rawName = "5"; displayName = "5"; role = "general"; }
      { id = "6"; rawName = "6"; displayName = "6"; role = "general"; }
      { id = "7"; rawName = "7"; displayName = "7"; role = "general"; }
      { id = "8"; rawName = "8"; displayName = "8"; role = "general"; }
      { id = "9"; rawName = "9"; displayName = "9"; role = "general"; }
      { id = "M"; rawName = "10"; displayName = "M"; role = "media"; }
      { id = "B"; rawName = "11"; displayName = "B"; role = "browser"; }
      { id = "A"; rawName = "12"; displayName = "A"; role = "viewer"; }
      { id = "Q"; rawName = "13"; displayName = "Q"; role = "project"; }
      { id = "W"; rawName = "14"; displayName = "W"; role = "project"; }
      { id = "E"; rawName = "15"; displayName = "E"; role = "project"; }
      { id = "R"; rawName = "16"; displayName = "R"; role = "project"; }
      { id = "T"; rawName = "17"; displayName = "T"; role = "project"; }
      { id = "Y"; rawName = "18"; displayName = "Y"; role = "project"; }
      { id = "U"; rawName = "19"; displayName = "U"; role = "project"; }
      { id = "I"; rawName = "20"; displayName = "I"; role = "project"; }
      { id = "O"; rawName = "21"; displayName = "O"; role = "project"; }
      { id = "P"; rawName = "22"; displayName = "P"; role = "project"; }
      # NOTE: CP1-CP6 (cockpit-park workspaces) are defined in omniwm
      # (modules/darwin/omniwm/workspace-builder.nix) but intentionally
      # NOT in projwm's manifest. Per requirements §2 they are
      # "管理外 workspace" (unmanaged): projwm must not classify them
      # as slots, must not plan moves into/out of them, must not own
      # any window placed there.
    ];
    slots = [
      { id = "Q"; workspace = "Q"; order = 1; }
      { id = "W"; workspace = "W"; order = 2; }
      { id = "E"; workspace = "E"; order = 3; }
      { id = "R"; workspace = "R"; order = 4; }
      { id = "T"; workspace = "T"; order = 5; }
      { id = "Y"; workspace = "Y"; order = 6; }
      { id = "U"; workspace = "U"; order = 7; }
      { id = "I"; workspace = "I"; order = 8; }
      { id = "O"; workspace = "O"; order = 9; }
      { id = "P"; workspace = "P"; order = 10; }
    ];
    apps = [
      {
        capability = "terminal";
        bundleId = "com.mitchellh.ghostty";
        appPath = "/Applications/Ghostty.app";
        lifecycleRemoval = {
          allowed = true;
          method = "ax-close-guarded";
          allowedKinds = [ "ai" "shell" "viewer" ];
          requiredEvidence = [ "desired-window-id" "bundle-id" "exact-title" "unique-live-window" ];
        };
      }
      {
        capability = "editor";
        bundleId = "dev.zed.Zed";
        appPath = "/Applications/Zed.app";
        lifecycleRemoval = {
          allowed = true;
          method = "project-scoped-app";
          allowedKinds = [ "editor" ];
          requiredEvidence = [
            "desired-window-id"
            "bundle-id"
            "exact-title"
            "unique-live-window"
            "project-root-correlation"
            "unsaved-change-clean"
          ];
        };
      }
      {
        capability = "browser";
        bundleId = "com.vivaldi.Vivaldi";
        appPath = "/Applications/Vivaldi.app";
        lifecycleRemoval = {
          allowed = true;
          method = "browser-window-close";
          allowedKinds = [ "browser" ];
          requiredEvidence = [
            "desired-window-id"
            "bundle-id"
            "exact-browser-window-id"
            "live-window-correlation"
            "automation-profile"
            "payload-token-correlation"
            "user-profile-isolated"
          ];
        };
      }
    ];
    daemons = {
      controller = nextLaunchdLabel;
      socketPath = nextSocketPath;
      legacyAgents = "remove";
      eventSources = [
        { kind = "windows-changed"; source = "window-manager"; mode = "sidecar"; authority = "hint"; label = "org.nixos.projwm-next-windows-changed"; }
        { kind = "display-changed"; source = "system"; mode = "sidecar"; authority = "hint"; label = "org.nixos.projwm-next-display-changed"; }
        { kind = "layout-changed"; source = "user"; mode = "sidecar"; authority = "hint"; label = "org.nixos.projwm-next-layout-changed"; }
        { kind = "safety-timer"; source = "timer"; mode = "sidecar"; authority = "hint"; label = "org.nixos.projwm-next-safety-timer"; }
        { kind = "wake"; source = "system"; mode = "sidecar"; authority = "hint"; label = "org.nixos.projwm-next-wake"; }
      ];
      # validate-environment が物理的不在を assertion する legacy launchd label。
      # handoff §4.7 cutover 以降、これらは Nix 上の install 定義ごと撤去済み。
      # action="remove" は ManagedEnvironment manifest 上のシグナルとして残す。
      agents = [
        { label = "org.nixos.projwm-reconcile-watch"; action = "remove"; }
        { label = "org.nixos.projwm-reconcile-display"; action = "remove"; }
        { label = "org.nixos.projwm-reconcile-periodic"; action = "remove"; }
        { label = "org.nixos.projwm-reconcile-startup"; action = "remove"; }
        { label = "org.nixos.projwm-reconcile-wake"; action = "remove"; }
        { label = "org.nixos.projwm-layout-watch"; action = "remove"; }
      ];
    };
  };

  nextManifestJson = builtins.toJSON nextManagedEnvironment;
  nextManifestFile = pkgs.writeText "projwm-next-managed-environment.json" nextManifestJson;
  nextManifestDigest = builtins.hashString "sha256" nextManifestJson;

  projwmNextCtl = pkgs.writeShellScriptBin "projwmctl-next" ''
    exec ${projwmNext}/bin/projwmctl \
      --socket-path ${lib.escapeShellArg nextSocketPath} \
      --managed-environment ${lib.escapeShellArg "${nextManifestFile}"} \
      --manifest-digest ${lib.escapeShellArg nextManifestDigest} \
      "$@"
  '';

  # Layer 2 user CLI. Wraps the raw `projwm` binary with the manifest /
  # digest / socket / store-dir env so users can run plain `projwm status`.
  projwmCli = pkgs.writeShellScriptBin "projwm" ''
    export PROJWM_NEXT_SOCKET_PATH=''${PROJWM_NEXT_SOCKET_PATH:-${lib.escapeShellArg nextSocketPath}}
    export PROJWM_NEXT_MANAGED_ENVIRONMENT=''${PROJWM_NEXT_MANAGED_ENVIRONMENT:-${lib.escapeShellArg "${nextManifestFile}"}}
    export PROJWM_NEXT_MANIFEST_DIGEST=''${PROJWM_NEXT_MANIFEST_DIGEST:-${lib.escapeShellArg nextManifestDigest}}
    export PROJWM_NEXT_STORE_DIR=''${PROJWM_NEXT_STORE_DIR:-${lib.escapeShellArg nextStoreDir}}
    exec ${projwmNext}/bin/projwm "$@"
  '';

  # Layer 3 cockpit TUI binary, identical wrapper pattern to projwmCli.
  projwmCockpit = pkgs.writeShellScriptBin "projwm-cockpit" ''
    export PROJWM_NEXT_SOCKET_PATH=''${PROJWM_NEXT_SOCKET_PATH:-${lib.escapeShellArg nextSocketPath}}
    export PROJWM_NEXT_MANAGED_ENVIRONMENT=''${PROJWM_NEXT_MANAGED_ENVIRONMENT:-${lib.escapeShellArg "${nextManifestFile}"}}
    export PROJWM_NEXT_MANIFEST_DIGEST=''${PROJWM_NEXT_MANIFEST_DIGEST:-${lib.escapeShellArg nextManifestDigest}}
    export PROJWM_NEXT_STORE_DIR=''${PROJWM_NEXT_STORE_DIR:-${lib.escapeShellArg nextStoreDir}}
    exec ${projwmNext}/bin/projwm-cockpit "$@"
  '';

  projwmNextEvent = pkgs.writeShellScriptBin "projwmevent-next" ''
    exec ${projwmNext}/bin/projwmevent \
      --socket-path ${lib.escapeShellArg nextSocketPath} \
      --managed-environment ${lib.escapeShellArg "${nextManifestFile}"} \
      --manifest-digest ${lib.escapeShellArg nextManifestDigest} \
      --queue-dir ${lib.escapeShellArg nextEventQueueDir} \
      "$@"
  '';

  projwmNextWindowsChangedEvent = pkgs.writeShellScriptBin "projwm-next-windows-changed-event" ''
    exec ${projwmNextEvent}/bin/projwmevent-next --source window-manager --kind windows-changed
  '';

  projwmNextDisplayChangedEvent = pkgs.writeShellScriptBin "projwm-next-display-changed-event" ''
    exec ${projwmNextEvent}/bin/projwmevent-next --source system --kind display-changed
  '';

  projwmNextLayoutChangedEvent = pkgs.writeShellScriptBin "projwm-next-layout-changed-event" ''
    exec ${projwmNextEvent}/bin/projwmevent-next --source user --kind layout-changed
  '';

  projwmNextBootstrap = pkgs.writeShellScriptBin "projwmstore-bootstrap-next" ''
    if [ "$#" -eq 0 ]; then
      echo "usage: projwmstore-bootstrap-next --desired-world <path> [projwmstore-bootstrap flags...]" >&2
      echo "DesiredWorld is production state/migration input, not Nix topology; generate it from legacy/current state or an explicit admin fixture." >&2
      exit 2
    fi
    exec ${projwmNext}/bin/projwmstore-bootstrap \
      --store-dir ${lib.escapeShellArg nextStoreDir} \
      --managed-environment ${lib.escapeShellArg "${nextManifestFile}"} \
      --manifest-digest ${lib.escapeShellArg nextManifestDigest} \
      "$@"
  '';

  # macOS sleep/wake 監視バイナリ (Swift)。
  # projwm-next の wake sidecar (org.nixos.projwm-next-wake) が
  # PROJWM_WAKE_COMMAND 経由で projwmevent-next --kind wake を発火する。
  wakeWatcher = pkgs.stdenv.mkDerivation {
    pname = "projwm-wake-watcher";
    version = "0.1.0";
    src = ./wake-watcher.swift;
    dontUnpack = true;
    buildPhase = ''
      swiftc $src -o projwm-wake-watcher
    '';
    installPhase = ''
      install -D projwm-wake-watcher $out/bin/projwm-wake-watcher
    '';
    nativeBuildInputs = [ pkgs.swift ];
    meta.platforms = pkgs.lib.platforms.darwin;
  };
in {
  imports = [ ./sudoers.nix ];

  options.myConfig.darwin.projwm = {
    next = {
      enable = lib.mkEnableOption "projwm-next production-shaped daemon and CLI";
      launchd.enable = lib.mkEnableOption "projwm-next LaunchAgent";
      stateDir = lib.mkOption {
        type = lib.types.str;
        default = "${userHome}/.local/state/projwm-next";
        description = "projwm-next production state directory.";
      };
      storeDir = lib.mkOption {
        type = lib.types.str;
        default = "${cfg.next.stateDir}/store";
        description = "projwm-next production PersistentStore directory.";
      };
      socketPath = lib.mkOption {
        type = lib.types.str;
        default = "${cfg.next.stateDir}/projwmd.sock";
        description = "projwm-next production Unix socket path.";
      };
    };
  };

  config = lib.mkIf cfg.next.enable {
    home-manager.users.${user} = { lib, ... }: {
      # We do NOT install projwmNext directly: it ships raw binaries
      # (projwm, projwmctl, projwmevent, projwmd, projwmstore-bootstrap)
      # whose names collide with the user-friendly wrappers below.
      # Wrappers reference projwmNext internally via /nix/store paths
      # so its binaries remain reachable.
      home.packages = [
        projwmNextCtl
        projwmNextEvent
        projwmNextBootstrap
        projwmCli
        projwmCockpit
      ];
      home.sessionVariables = {
        PROJWM_NEXT_SOCKET_PATH = nextSocketPath;
        PROJWM_NEXT_MANIFEST_DIGEST = nextManifestDigest;
        PROJWM_NEXT_MANAGED_ENVIRONMENT = "${nextManifestFile}";
        PROJWM_NEXT_STORE_DIR = nextStoreDir;
      };
      home.activation.projwmNextDirs = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
        mkdir -p ${lib.escapeShellArg nextStateDir} ${lib.escapeShellArg nextLogDir} ${lib.escapeShellArg nextStoreDir} ${lib.escapeShellArg nextEventQueueDir}
      '';
    };

    launchd.user.agents.projwmd-next = lib.mkIf cfg.next.launchd.enable {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''
            set -eu
            /bin/wait4path /nix/store
            # Force PATH explicitly because launchd EnvironmentVariables can
            # be shadowed by /bin/sh's own defaults under launchd. tmux and
            # ghostty live in the user nix profile.
            export PATH="/etc/profiles/per-user/${user}/bin:${userHome}/.nix-profile/bin:/run/current-system/sw/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"
            mkdir -p ${lib.escapeShellArg nextStateDir} ${lib.escapeShellArg nextLogDir}
            exec ${projwmNext}/bin/projwmd \
              --managed-environment ${lib.escapeShellArg "${nextManifestFile}"} \
              --manifest-digest ${lib.escapeShellArg nextManifestDigest} \
              --store-dir ${lib.escapeShellArg nextStoreDir} \
              --store-kind production \
              --private-payload-dir ${lib.escapeShellArg nextPrivatePayloadDir} \
              --socket-path ${lib.escapeShellArg nextSocketPath} \
              --launchd-label ${lib.escapeShellArg nextLaunchdLabel} \
              --require-launchd-runtime-proof \
              --startup-provenance ${lib.escapeShellArg nextStartupProvenancePath}
          ''
        ];
        EnvironmentVariables = {
          # tmux and the cockpit binary live in the user's nix profile
          # (/etc/profiles/per-user/$USER/bin) — CockpitManager needs
          # them on PATH to spawn the cockpit Ghostty + grouped tmux.
          PATH = "/etc/profiles/per-user/${user}/bin:${userHome}/.nix-profile/bin:/run/current-system/sw/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin";
          PROJWM_NEXT_SOCKET_PATH = nextSocketPath;
          PROJWM_NEXT_MANIFEST_DIGEST = nextManifestDigest;
          PROJWM_NEXT_MANAGED_ENVIRONMENT = "${nextManifestFile}";
          PROJWM_NEXT_STARTUP_PROVENANCE = nextStartupProvenancePath;
        };
        RunAtLoad = true;
        KeepAlive = true;
        ThrottleInterval = 10;
        StandardOutPath = "${nextLogDir}/projwmd.out.log";
        StandardErrorPath = "${nextLogDir}/projwmd.err.log";
      };
    };

    launchd.user.agents.projwm-next-windows-changed = lib.mkIf cfg.next.launchd.enable {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && exec /opt/homebrew/bin/omniwmctl watch windows-changed --no-send-initial --exec ${projwmNextWindowsChangedEvent}/bin/projwm-next-windows-changed-event''
        ];
        KeepAlive = true;
        RunAtLoad = true;
        ThrottleInterval = 10;
        StandardOutPath = "${nextLogDir}/windows-changed.out.log";
        StandardErrorPath = "${nextLogDir}/windows-changed.err.log";
      };
    };

    launchd.user.agents.projwm-next-display-changed = lib.mkIf cfg.next.launchd.enable {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && exec /opt/homebrew/bin/omniwmctl watch display-changed --no-send-initial --exec ${projwmNextDisplayChangedEvent}/bin/projwm-next-display-changed-event''
        ];
        KeepAlive = true;
        RunAtLoad = true;
        ThrottleInterval = 10;
        StandardOutPath = "${nextLogDir}/display-changed.out.log";
        StandardErrorPath = "${nextLogDir}/display-changed.err.log";
      };
    };

    launchd.user.agents.projwm-next-layout-changed = lib.mkIf cfg.next.launchd.enable {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && exec /opt/homebrew/bin/omniwmctl watch layout-changed --no-send-initial --exec ${projwmNextLayoutChangedEvent}/bin/projwm-next-layout-changed-event''
        ];
        KeepAlive = true;
        RunAtLoad = true;
        ThrottleInterval = 10;
        StandardOutPath = "${nextLogDir}/layout-changed.out.log";
        StandardErrorPath = "${nextLogDir}/layout-changed.err.log";
      };
    };

    launchd.user.agents.projwm-next-safety-timer = lib.mkIf cfg.next.launchd.enable {
      serviceConfig = {
        ProgramArguments = [
          "${projwmNextEvent}/bin/projwmevent-next"
          "--source" "timer"
          "--kind" "safety-timer"
        ];
        StartInterval = 60;
        RunAtLoad = false;
        StandardOutPath = "${nextLogDir}/safety-timer.out.log";
        StandardErrorPath = "${nextLogDir}/safety-timer.err.log";
      };
    };

    launchd.user.agents.projwm-next-wake = lib.mkIf cfg.next.launchd.enable {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && exec ${wakeWatcher}/bin/projwm-wake-watcher''
        ];
        EnvironmentVariables = {
          PROJWM_WAKE_COMMAND = "${projwmNextEvent}/bin/projwmevent-next --source system --kind wake";
        };
        KeepAlive = true;
        RunAtLoad = true;
        ThrottleInterval = 10;
        StandardOutPath = "${nextLogDir}/wake.out.log";
        StandardErrorPath = "${nextLogDir}/wake.err.log";
      };
    };
  };
}
