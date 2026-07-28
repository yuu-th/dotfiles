# modules/common/tmux.nix
#
# tmux — projwm が AI/shell の永続化に使う terminal multiplexer
# 設計: queue/projwm-design.md §5.1 / §5.3
#   - set-titles off / allow-rename off  → ghostty --title= を踏まない
#   - window-size latest / aggressive-resize on → grouped session のリサイズ衝突回避
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.tmux; in {
  options.myConfig.tmux.enable = lib.mkEnableOption "tmux multiplexer (projwm 用)";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = with pkgs; [ tmux ];

      home.file.".tmux.conf".text = ''
        # ── projwm 必須項目 ─────────────────────────────────────────────
        # ghostty --title= を踏まない（projwm の title 規約 <kind>-<id>:<proj> 維持）
        set -g set-titles off
        set -g allow-rename off

        # grouped session 複数 client 同時アタッチ時のサイズ衝突回避
        set -g window-size latest
        set -g aggressive-resize on

        # 256 色＋truecolor
        set -g default-terminal "screen-256color"
        set -ga terminal-overrides ",xterm-256color:Tc,xterm-ghostty:Tc"

        # ── 基本 ────────────────────────────────────────────────────────
        set -g mouse on
        set -g history-limit 50000
        set -g escape-time 10
        set -g focus-events on
        set -g base-index 1
        setw -g pane-base-index 1

        # default shell
        set -g default-shell ${pkgs.fish}/bin/fish
        set -g default-command ${pkgs.fish}/bin/fish

        # status は最小（projwm は viewer/launcher で集約表示するので tmux 内 status は不要）
        set -g status off
      '';
    };
  };
}
