# modules/darwin/projwm/default.nix
#
# projwm — AI workspace manager (Go バイナリ + 周辺設定)
#
# 設計: queue/projwm-design.md（v11.2）
#
# 構成:
#   - Go バイナリ (buildGoModule)
#   - OmniWM workspace 追加 (Phase 5 で omniwm-workspaces.nix に分離予定)
#   - hotkeys 追加 (Phase 5)
#   - launchd reconcile-watcher (Phase 6)
{ config, lib, pkgs, ... }:
let
  cfg = config.myConfig.darwin.projwm;

  projwm = pkgs.buildGoModule {
    pname = "projwm";
    version = "0.1.0-dev";
    src = ./projwm;
    vendorHash = "sha256-N1XhOg3KUpWjUuuCNVKFvXXdqGgLgEYvMhNVkJGbi7Y=";
    proxyVendor = true;
    subPackages = [ "." ];
    ldflags = [ "-s" "-w" "-X" "main.version=0.1.0-dev" ];
    doCheck = true;           # `go test ./...` を実行
    meta = with lib; {
      description = "OmniWM + tmux + Zed ベースの AI workspace manager";
      platforms = platforms.darwin;
    };
  };

  # 500ms debounce ラッパ。omniwmctl watch から大量に発火する
  # windows-changed イベントを 1 つの reconcile に集約する。
  reconcileDebounced = pkgs.writeShellScriptBin "projwm-reconcile-debounced" ''
    set -eu
    LOCK="''${TMPDIR:-/tmp}/projwm-reconcile.lock"
    MARK="''${TMPDIR:-/tmp}/projwm-reconcile.pending"

    # 連続発火時は marker を作り、既に走っている debounce があれば任せて exit
    touch "$MARK"
    if ! ${pkgs.flock}/bin/flock -n "$LOCK" -c true; then
      exit 0
    fi

    ${pkgs.flock}/bin/flock "$LOCK" -c '
      sleep 0.5
      while [ -f "'$MARK'" ]; do
        rm -f "'$MARK'"
        ${projwm}/bin/projwm reconcile >> "$HOME/.local/state/projwm/logs/reconcile.log" 2>&1 || true
      done
    '
  '';
in {
  options.myConfig.darwin.projwm.enable = lib.mkEnableOption "projwm AI workspace manager";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ projwm reconcileDebounced ];

      # config.toml は projwm が無くてもデフォルトで動くため Nix で固定する必要は無いが、
      # ベースラインとして配置（手動編集も可、§6.2.1 fallback）
      home.file.".config/projwm/config.toml".text = ''
        # projwm config (managed by Nix; safe to edit, projwm will pick up changes on restart)
        viewer_workspace = "A"
        slot_names = ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"]
      '';
    };

    # ── launchd: 自動 reconcile 2 系統 (projwm-design.md §7.4) ────────────
    # 1) omniwmctl watch windows-changed → debounce 500ms → projwm reconcile
    # 2) 60 秒定期 backstop (watch 取りこぼし対策)
    launchd.user.agents.projwm-reconcile-watch = {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && exec /opt/homebrew/bin/omniwmctl watch windows-changed --no-send-initial --exec ${reconcileDebounced}/bin/projwm-reconcile-debounced''
        ];
        KeepAlive = true;
        RunAtLoad = true;
        ThrottleInterval = 10;
        StandardOutPath = "/tmp/projwm-reconcile-watch.log";
        StandardErrorPath = "/tmp/projwm-reconcile-watch.err";
      };
    };

    launchd.user.agents.projwm-reconcile-display = {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && exec /opt/homebrew/bin/omniwmctl watch display-changed --no-send-initial --exec ${reconcileDebounced}/bin/projwm-reconcile-debounced''
        ];
        KeepAlive = true;
        RunAtLoad = true;
        ThrottleInterval = 10;
      };
    };

    launchd.user.agents.projwm-reconcile-periodic = {
      serviceConfig = {
        ProgramArguments = [
          "/bin/sh" "-c"
          ''/bin/wait4path /nix/store && exec ${projwm}/bin/projwm reconcile''
        ];
        StartInterval = 60;
        RunAtLoad = false;
        StandardOutPath = "/tmp/projwm-reconcile-periodic.log";
        StandardErrorPath = "/tmp/projwm-reconcile-periodic.err";
      };
    };
  };
}
