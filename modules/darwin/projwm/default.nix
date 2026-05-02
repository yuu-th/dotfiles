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
in {
  options.myConfig.darwin.projwm.enable = lib.mkEnableOption "projwm AI workspace manager";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = [ projwm ];

      # config.toml は projwm が無くてもデフォルトで動くため Nix で固定する必要は無いが、
      # ベースラインとして配置（手動編集も可、§6.2.1 fallback）
      home.file.".config/projwm/config.toml".text = ''
        # projwm config (managed by Nix; safe to edit, projwm will pick up changes on restart)
        viewer_workspace = "A"
        slot_names = ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"]
      '';
    };
  };
}
