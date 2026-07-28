# modules/common/tmux.nix
#
# tmux — AI/shell セッションの永続化に使う terminal multiplexer
#   - set-titles off / allow-rename off  → ghostty --title= を踏まない
#   - window-size latest / aggressive-resize on → grouped session のリサイズ衝突回避
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.tmux; in {
  options.myConfig.tmux.enable = lib.mkEnableOption "tmux multiplexer";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {
      home.packages = with pkgs; [ tmux ];

      home.file.".tmux.conf".text = ''
        # ── title 制御 ──────────────────────────────────────────────────
        # ghostty --title= を踏まない（外から付けた window title を維持）
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

        # status は出さない（ghostty の window title 側で識別する）
        set -g status off
      '';
    };
  };
}
