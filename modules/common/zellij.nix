# modules/common/zellij.nix
#
# ╔══════════════════════════════════════════════════════════════════╗
# ║  Zellij — プロジェクト単位のセッション管理                        ║
# ║                                                                  ║
# ║  使い方:                                                          ║
# ║    zj <name>  → ~/dev/<name> で Zellij セッション起動/復帰        ║
# ║    zj         → 既存セッション一覧から fzf 選択                   ║
# ║                                                                  ║
# ║  タブ構成（project.kdl レイアウト）:                              ║
# ║    Cmd+E  🔧 Editor  (yazi 左 | 空シェル 右)                     ║
# ║    Cmd+G  📦 Git     (lazygit フルスクリーン)                    ║
# ║    Cmd+S  🐚 Shell   (fish フリーシェル)                         ║
# ╚══════════════════════════════════════════════════════════════════╝
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.zellij; in {
  options.myConfig.zellij.enable = lib.mkEnableOption "Zellij terminal multiplexer with project session management";

  config = lib.mkIf cfg.enable {
    home-manager.users.${config.myConfig.primaryUser} = {

      # ── パッケージ ─────────────────────────────────────────────────────────
      home.packages = with pkgs; [
        zellij
        yazi      # ファイルブラウザ (Editor タブ左ペイン)
        lazygit   # Git TUI (Git タブ)
      ];

      # ── Zellij 設定 (config.kdl) ─────────────────────────────────────────
      # enableFishIntegration は使わない（zj コマンドで手動起動する設計）
      home.file.".config/zellij/config.kdl".text = ''
        // ── 基本設定 ───────────────────────────────────────────────────────
        mouse_mode true
        scroll_buffer_size 10000
        copy_on_select false
        pane_frames false          // ペイン枠を非表示（Zellij のタブバーのみ表示）
        default_shell "fish"

        // ── テーマ ─────────────────────────────────────────────────────────
        theme "tokyo-night"

        // ── キーバインド（Cmd 系で統一、デフォルトは全クリア）─────────────
        // デフォルトキーをクリアして Ghostty から透過した Cmd+* キーを使う
        keybinds clear-defaults=true {
          normal {
            // ── タブ移動（固定: 1=Editor, 2=Git, 3=Shell）──
            bind "Super e" { GoToTab 1; }
            bind "Super g" { GoToTab 2; }
            bind "Super s" { GoToTab 3; }

            // ── タブ操作 ──
            bind "Super t" { NewTab; }
            bind "Super Shift t" { SwitchToMode "RenameTab"; }
            bind "Super w" { ClosePane; }

            // ── ペイン操作 ──
            bind "Super d"       { NewPane "right"; }
            bind "Super Shift d" { NewPane "down"; }
            bind "Super ["       { FocusPreviousPane; }
            bind "Super ]"       { FocusNextPane; }
            bind "Super Shift p" { SwitchToMode "RenamePane"; }

            // ── スクロール ──
            bind "PageUp"   { PageScrollUp; }
            bind "PageDown" { PageScrollDown; }
            bind "Alt Up"   { HalfPageScrollUp; }
            bind "Alt Down" { HalfPageScrollDown; }

            // ── リサイズモード ──
            bind "Ctrl r" { SwitchToMode "Resize"; }

            // ── セッション操作 ──
            bind "Ctrl Shift d" { Detach; }   // Zellij をデタッチしてセッションを保持
          }

          resize {
            bind "h"     { Resize "Left"; }
            bind "j"     { Resize "Down"; }
            bind "k"     { Resize "Up"; }
            bind "l"     { Resize "Right"; }
            bind "Esc"   { SwitchToMode "Normal"; }
            bind "Enter" { SwitchToMode "Normal"; }
          }

          rename_tab {
            bind "Enter" { SwitchToMode "Normal"; }
            bind "Esc"   { SwitchToMode "Normal"; UndoRenameTab; }
          }

          rename_pane {
            bind "Enter" { SwitchToMode "Normal"; }
            bind "Esc"   { SwitchToMode "Normal"; UndoRenamePane; }
          }
        }
      '';

      # ── プロジェクトレイアウト (Editor/Git/Shell 3 タブ) ────────────────
      home.file.".config/zellij/layouts/project.kdl".text = ''
        layout {
          tab name="🔧 Editor" focus=true {
            pane split_direction="vertical" {
              pane command="yazi" size="35%"
              pane
            }
          }
          tab name="📦 Git" {
            pane command="lazygit"
          }
          tab name="🐚 Shell" {
            pane
          }
        }
      '';

      # ── zj コマンド（Zellij セッションマネージャー）────────────────────────
      # fish function として定義
      programs.fish.functions.zj = {
        description = "Zellij session manager (zj <name> or zj to pick)";
        body = ''
          # ── 引数なし: 既存セッション一覧から fzf 選択してアタッチ ──────────
          if test (count $argv) -eq 0
            set -l sessions (zellij list-sessions --short 2>/dev/null \
              | string split ' ' | string trim | string join \n | grep -v '^$' || true)
            if test -z "$sessions"
              echo "No Zellij sessions. Usage: zj <name>"
              return 1
            end
            set -l picked (echo $sessions | fzf --prompt="Zellij session: " --height=40%)
            if test -z "$picked"
              return 0
            end
            zellij attach $picked
            return 0
          end

          set -l name $argv[1]

          # ── Zellij 内からの二重起動を防止 ────────────────────────────────
          if set -q ZELLIJ_SESSION_NAME
            if test "$ZELLIJ_SESSION_NAME" = "$name"
              echo "Already in Zellij session '$name'."
              return 0
            else
              echo "Already inside Zellij session '$ZELLIJ_SESSION_NAME'."
              echo "Detach first (Ctrl+Shift+D) or open a new Ghostty window."
              return 1
            end
          end

          # ── プロジェクトディレクトリを決定 ───────────────────────────────
          set -l project_dir
          if test -d "$name"
            set project_dir (realpath $name)
            set name (basename $project_dir)
          else if test -d "$HOME/dev/$name"
            set project_dir "$HOME/dev/$name"
          else
            set project_dir (pwd)
          end

          # ── レジストリにウィンドウ ID を登録 ─────────────────────────────
          # AeroSpace の focus-tool.sh がメインウィンドウを識別するために使用
          set -l registry "$HOME/.local/share/zellij-aerospace/sessions.tsv"
          set -l win_id (aerospace list-windows --focused --format "%{window-id}" 2>/dev/null \
            | head -1 | string trim)
          if test -n "$win_id"
            mkdir -p (dirname $registry)
            # 既存エントリを削除して新規追加
            if test -f $registry
              awk -F'\t' -v s="$name" '$1 != s' $registry > /tmp/zj-reg-tmp.tsv 2>/dev/null
              and mv /tmp/zj-reg-tmp.tsv $registry
              or rm -f /tmp/zj-reg-tmp.tsv
            end
            printf "%s\t%s\tmain\t%s\n" $name $win_id $project_dir >> $registry
          end

          # ── プロジェクトディレクトリに移動 ───────────────────────────────
          if test -d "$project_dir"
            cd $project_dir
          end

          # ── Zellij セッション起動 or アタッチ ────────────────────────────
          set -l existing (zellij list-sessions --short 2>/dev/null \
            | string split ' ' | string trim)
          if contains -- $name $existing
            zellij attach $name
          else
            zellij --session $name --layout project
          end
        '';
      };

    };
  };
}
