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

  # ── 開発・運用用 sudo nopass は ./sudoers.nix を import ─────────────────

  projwm = pkgs.buildGoModule {
    pname = "projwm";
    version = "0.1.0-dev";
    src = ./projwm;
    vendorHash = "sha256-J4sCq4f9jDU81WTGtX0jZcmPQrE7sJWvOiyBwJpfvWg=";
    proxyVendor = true;
    subPackages = [ "." ];
    ldflags = [ "-s" "-w" "-X" "main.version=0.1.0-dev" ];
    doCheck = true;           # `go test ./...` を実行
    meta = with lib; {
      description = "OmniWM + tmux + Zed ベースの AI workspace manager";
      platforms = platforms.darwin;
    };
  };

  # kitty.app をユーザ空間にコピーして OmniWM 互換にする setup スクリプト。
  # 詳細はスクリプトのコメント参照（queue/projwm-design.md v11.3）。
  setupKittyProjwm = pkgs.writeShellScriptBin "projwm-setup-kitty"
    (builtins.readFile ./scripts/setup-kitty-projwm.sh);

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
  imports = [ ./sudoers.nix ];

  options.myConfig.darwin.projwm.enable = lib.mkEnableOption "projwm AI workspace manager";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = { lib, ... }: {
      home.packages = [ projwm reconcileDebounced setupKittyProjwm ];

      # config.toml は projwm が無くてもデフォルトで動くため Nix で固定する必要は無いが、
      # ベースラインとして配置（手動編集も可、§6.2.1 fallback）
      home.file.".config/projwm/config.toml".text = ''
        # projwm config (managed by Nix; safe to edit, projwm will pick up changes on restart)
        viewer_workspace = "A"
        slot_names = ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"]
        # terminal driver: kitty を ~/Applications/kitty-projwm.app に user-space copy
        # して NSPrincipalClass 注入で OmniWM 互換にする (v11.3、setup-kitty-projwm.sh)
        terminal_app_path = "~/Applications/kitty-projwm.app"
        terminal_bundle_id = "net.kovidgoyal.kitty.projwm"
      '';

      # 注: home.activation 経由の setup は macOS のセキュリティ制約 (codesign が
      # builderEnv で動かないケース) で壊れた bundle を生成する。代わりに projwm Go
      # 自身が reconcile 開始時に setup-kitty-projwm を呼ぶ方式に切替（v11.3 以降）。
      # ユーザは何もしなくて良い、最初の `projwm up` 実行時に自動構築される。
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
