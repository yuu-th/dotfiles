# modules/common/zellij.nix
#
# ╔══════════════════════════════════════════════════════════════════╗
# ║  Zellij — セッション永続性のみ（タブ・ステータスバーなし）       ║
# ║                                                                  ║
# ║  使い方:                                                          ║
# ║    zj <name>  → ~/dev/<name> で Zellij セッション起動/復帰        ║
# ║    zj         → 既存セッション一覧から fzf 選択                   ║
# ║                                                                  ║
# ║  ウィンドウ管理は Ghostty ネイティブ（タブ・ペイン不使用）:       ║
# ║    Cmd+N = 新規ウィンドウ  Cmd+W = ウィンドウを閉じる            ║
# ║    Zellij は Ctrl+D でデタッチ、Alt+S でスクロール               ║
# ╚══════════════════════════════════════════════════════════════════╝
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.zellij; in {
  options.myConfig.zellij.enable = lib.mkEnableOption "Zellij terminal multiplexer (session persistence only)";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {

      # ── パッケージ ─────────────────────────────────────────────────────────
      home.packages = with pkgs; [
        zellij
        lazygit
        yazi
      ];

      programs.zellij.enable = true;

      # ── Zellij 設定 (config.kdl) ─────────────────────────────────────────
      home.file.".config/zellij/config.kdl".text = ''
        mouse_mode true
        scroll_buffer_size 10000
        copy_on_select false
        pane_frames false
        default_shell "fish"
        default_layout "minimal"
        theme "tokyonight-night"

        keybinds clear-defaults=true {
          normal {
            bind "Ctrl d" { Detach; }
            bind "Alt s" { SwitchToMode "Scroll"; }
          }

          scroll {
            bind "Esc" { SwitchToMode "Normal"; }
            bind "Alt s" { SwitchToMode "Normal"; }
            bind "j" "Down" { ScrollDown; }
            bind "k" "Up" { ScrollUp; }
            bind "f" "PageDown" { PageScrollDown; }
            bind "b" "PageUp" { PageScrollUp; }
            bind "d" { HalfPageScrollDown; }
            bind "u" { HalfPageScrollUp; }
            bind "s" { SwitchToMode "EnterSearch"; SearchInput 0; }
          }

          search {
            bind "Esc" { SwitchToMode "Normal"; }
            bind "n" { Search "down"; }
            bind "p" { Search "up"; }
          }
        }

        themes {
          tokyonight-night {
            fg "#c0caf5"
            bg "#1a1b26"
            black "#15161e"
            red "#f7768e"
            green "#9ece6a"
            yellow "#e0af68"
            blue "#7aa2f7"
            magenta "#bb9af7"
            cyan "#7dcfff"
            white "#a9b1d6"
            orange "#ff9e64"
          }
        }
      '';

      # ── zj.fish (Zellij セッションマネージャー) ───────────────────────────
      home.file.".config/fish/functions/zj.fish".text = ''
function zj --description "Zellij session manager (zj [name])"
    if set -q ZELLIJ_SESSION_NAME
        echo "Already in Zellij session '$ZELLIJ_SESSION_NAME'."
        echo "Detach first (Ctrl+D) or open a new Ghostty window (Cmd+N)."
        return 1
    end

    if test (count $argv) -eq 0
        set -l sessions (zellij list-sessions --short --no-formatting 2>/dev/null | awk '{print $1}' | grep -v '^$')
        if test -z "$sessions"
            echo "No Zellij sessions. Usage: zj <name>"
            return 1
        end
        set -l picked (printf '%s\n' $sessions | fzf --prompt="Attach to: " --height=40%)
        test -z "$picked"; and return 0
        zellij attach "$picked"
        return
    end

    set -l name $argv[1]

    if test -d "$name"
        cd (realpath "$name")
        set name (basename "$name")
    else if test -d "$HOME/dev/$name"
        cd "$HOME/dev/$name"
    end

    if zellij list-sessions --short --no-formatting 2>/dev/null | awk '{print $1}' | grep -qx "$name"
        zellij attach "$name"
    else
        zellij -s "$name"
    end
end
      '';

    };
  };
}
