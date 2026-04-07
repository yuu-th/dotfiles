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
      ];

      # ── カスタムレイアウト: minimal (タブバー・ステータスバーなし) ────────
      # "minimal" は Zellij 組み込みではないため自前で定義する。
      # 単一ペインのみ。タブバー・ステータスバーは表示しない。
      home.file.".config/zellij/layouts/minimal.kdl".text = ''
        layout {
          pane
        }
      '';

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

    };
  };
}
