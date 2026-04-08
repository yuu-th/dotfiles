# modules/common/fish.nix
#
# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  Fish shell 環境 — カスタマイズガイド                                   ║
# ║                                                                          ║
# ║  エイリアス追加  → shellAliases に追記                                  ║
# ║  プラグイン追加  → plugins リストに追記（nixpkgs: pkgs.fishPlugins.*）  ║
# ║  プロンプト変更  → programs.starship.settings を編集                    ║
# ║  履歴の設定      → programs.atuin.settings を編集                       ║
# ║  fzf のデフォルト → programs.fzf.defaultOptions を編集                  ║
# ╚══════════════════════════════════════════════════════════════════════════╝
#
# 含まれるツール:
#   Fish shell, Starship (gruvbox-rainbow), Atuin, zoxide, fzf + fzf-fish
#   eza, bat, fd, ripgrep, dust, yazi, lazygit, git-delta, autopair.fish
#
# git.nix と併用すると git-delta が `git diff` にも自動統合される。
{ config, lib, pkgs, ... }:
let cfg = config.myConfig.fish; in {
  options.myConfig.fish.enable = lib.mkEnableOption "Fish shell with Starship, Atuin, zoxide and modern CLI tools";

  config = lib.mkIf cfg.enable {
    # nix-darwin の programs.fish.enable が /etc/fish/ に Nix 環境変数スクリプトを生成する。
    # ログインシェルは zsh のまま。ターミナル（Ghostty）が fish を直接起動する。
    # 参照: https://discourse.nixos.org/t/best-practices-for-shell-management/63106
    programs.fish.enable = true;

    home-manager.users.${config.myConfig.primaryUser} = {

      # ────────────────────────────────────────────────────────────────
      # Fish shell
      # ────────────────────────────────────────────────────────────────
      programs.fish = {
        enable = true;

        shellInit = ''
          set -U fish_greeting ""
        '';

        plugins = [
          # 括弧・引用符の自動補完（入力中に自動でペアを補完）
          { name = "autopair"; src = pkgs.fishPlugins.autopair.src; }
          # fzf の包括的 fish 統合
          #   Ctrl+R → コマンド履歴検索（Atuin が上書きして高機能版になる）
          #   Ctrl+F → ファイル検索（bat プレビュー付き）
          #   Ctrl+L → 訪問済みディレクトリ検索
          #   Ctrl+G → git log 検索
          #   Ctrl+S → git status のパスで絞り込み
          { name = "fzf-fish"; src = pkgs.fishPlugins.fzf-fish.src; }
          # 他に試したいプラグインの例:
          # { name = "done"; src = pkgs.fishPlugins.done.src; }         # 長時間コマンド完了通知
          # { name = "puffer-fish"; src = pkgs.fishPlugins.puffer-fish.src; } # .. → cd ..
        ];

        shellAliases = {
          # eza (ls 代替: アイコン・Git 状態付き)
          ls  = "eza --icons";
          ll  = "eza -l --git --icons";
          lt  = "eza --tree --icons --git";
          la  = "eza -la --icons";
          # bat (cat 代替: シンタックスハイライト)
          cat = "bat";
          # TUI ツール
          lg  = "lazygit";  # Git TUI（ステージング・ブランチ管理）
          yy  = "yazi";     # ファイルマネージャー（画像・PDFプレビュー対応）
          # Git ショートカット
          gs  = "git status";
          ga  = "git add";
          gc  = "git commit";
          gp  = "git push";
          gd  = "git diff";
          # エイリアスを追加したい場合はここに追記
          copilot = "copilot --alt-screen";
        };

        interactiveShellInit = ''
          # man を bat でシンタックスハイライト表示
          set -gx MANPAGER "sh -c 'col -bx | bat -l man -p'"
          set -gx MANROFFOPT "-c"

          # ── Ghostty 起動時の Zellij セッション選択 ────────────────────────
          # Ghostty で起動 & Zellij 未使用 のインタラクティブシェルで自動表示する。
          # 新規ウィンドウ・Quick Terminal 初回起動すべてで発火する。
          # Ctrl+C または "Zellijなし" を選べばそのまま fish を使える。
          if set -q TERM_PROGRAM; and test "$TERM_PROGRAM" = "ghostty"
            and not set -q ZELLIJ
            and not set -q CMUX_WORKSPACE_ID
            set -l _sessions (zellij list-sessions --short --no-formatting 2>/dev/null | awk '{print ''$1}' | grep -v '^$')
            set -l _choices "Zellijなし (plain fish)" "新規セッション..."
            if test -n "$_sessions"
              set _choices $_choices $_sessions
            end
            set -l _picked (printf '%s\n' $_choices | fzf --prompt="  Terminal: " --height=40% --no-sort --border)
            switch "$_picked"
              case "Zellijなし (plain fish)" ""
                # plain fish のまま続ける
              case "新規セッション..."
                read -P "Session name: " _new_name
                if test -n "$_new_name"
                  zellij -s "$_new_name"
                end
              case '*'
                zellij attach "$_picked"
            end
          end
        '';
      };

      # ────────────────────────────────────────────────────────────────
      # Starship — gruvbox-rainbow プリセットベースのプロンプト
      # ────────────────────────────────────────────────────────────────
      programs.starship = {
        enable = true;
        enableFishIntegration = true;
        settings = {
          format = lib.concatStrings [
            "[](color_orange)"
            "$os"
            "$username"
            "[](bg:color_yellow fg:color_orange)"
            "$directory"
            "[](fg:color_yellow bg:color_aqua)"
            "$git_branch"
            "$git_status"
            "[](fg:color_aqua bg:color_blue)"
            "$python"
            "$rust"
            "$golang"
            "$nodejs"
            "[](fg:color_blue bg:color_bg3)"
            "$cmd_duration"
            "[](fg:color_bg3 bg:color_bg1)"
            "$time"
            "[ ](fg:color_bg1)"
            "$line_break$character"
          ];

          palette = "gruvbox_dark";
          palettes.gruvbox_dark = {
            color_fg0    = "#fbf1c7";
            color_bg1    = "#3c3836";
            color_bg3    = "#665c54";
            color_blue   = "#458588";
            color_aqua   = "#689d6a";
            color_green  = "#98971a";
            color_orange = "#d65d0e";
            color_purple = "#b16286";
            color_red    = "#cc241d";
            color_yellow = "#d79921";
          };

          os = {
            disabled = false;
            style = "bg:color_orange fg:color_fg0";
            symbols = {
              Macos = " ";
              Linux = " ";
            };
          };

          username = {
            show_always = true;
            style_user  = "bg:color_orange fg:color_fg0";
            style_root  = "bg:color_orange fg:color_fg0";
            format      = "[ $user ]($style)";
          };

          directory = {
            style             = "fg:color_fg0 bg:color_yellow";
            format            = "[ $path ]($style)";
            truncation_length = 3;
            truncation_symbol = "…/";
            substitutions = {
              "Documents" = "󰈙 ";
              "Downloads" = " ";
              "Music"     = "󰝚 ";
              "Pictures"  = " ";
              "Developer" = "󰲋 ";
              "dev"       = "󰲋 ";
            };
          };

          git_branch = {
            symbol = "";
            style  = "bg:color_aqua";
            format = "[[ $symbol $branch ](fg:color_fg0 bg:color_aqua)]($style)";
          };

          git_status = {
            style  = "bg:color_aqua";
            format = "[[($all_status$ahead_behind )](fg:color_fg0 bg:color_aqua)]($style)";
          };

          nodejs = {
            symbol = "";
            style  = "bg:color_blue";
            format = "[[ $symbol( $version) ](fg:color_fg0 bg:color_blue)]($style)";
          };

          rust = {
            symbol = "";
            style  = "bg:color_blue";
            format = "[[ $symbol( $version) ](fg:color_fg0 bg:color_blue)]($style)";
          };

          golang = {
            symbol = "";
            style  = "bg:color_blue";
            format = "[[ $symbol( $version) ](fg:color_fg0 bg:color_blue)]($style)";
          };

          python = {
            symbol = "";
            style  = "bg:color_blue";
            format = "[[ $symbol( $version) ](fg:color_fg0 bg:color_blue)]($style)";
          };

          cmd_duration = {
            min_time = 500;
            style    = "fg:color_fg0 bg:color_bg3";
            format   = "[ ⏱ $duration ]($style)";
          };

          time = {
            disabled    = false;
            time_format = "%R";
            style       = "fg:color_fg0 bg:color_bg1";
            format      = "[  $time ]($style)";
          };

          character = {
            disabled                   = false;
            success_symbol             = "[❯](bold fg:color_green)";
            error_symbol               = "[❯](bold fg:color_red)";
            vimcmd_symbol              = "[❮](bold fg:color_green)";
            vimcmd_replace_one_symbol  = "[❮](bold fg:color_purple)";
            vimcmd_replace_symbol      = "[❮](bold fg:color_purple)";
            vimcmd_visual_symbol       = "[❮](bold fg:color_yellow)";
          };
        };
      };

      # ────────────────────────────────────────────────────────────────
      # Atuin — SQLiteベースのシェル履歴 + Ctrl+R
      # 全コマンドに実行時間・終了コード・作業ディレクトリを記録。
      # Ctrl+R で高機能な履歴検索 UI が起動する（fzf-fish の Ctrl+R を上書き）。
      # マシン間の同期が必要な場合: auto_sync = true にして `atuin login` を実行。
      # ────────────────────────────────────────────────────────────────
      programs.atuin = {
        enable = true;
        enableFishIntegration = true;
        settings = {
          auto_sync   = false;       # true にすると E2E 暗号化でマシン間同期
          search_mode = "fuzzy";     # "prefix" / "fulltext" / "fuzzy" / "skim"
          filter_mode_shell_up_arrow_filter = "session"; # ↑キーはセッション内に絞る
          show_preview       = true;
          max_preview_height = 4;
          show_help          = false;
        };
      };

      # ────────────────────────────────────────────────────────────────
      # zoxide — スマートディレクトリジャンプ（z / zi）
      # z <一部のパス名> で訪問履歴から推測してジャンプ。
      # zi で fzf のインタラクティブ選択画面が開く。
      # ────────────────────────────────────────────────────────────────
      programs.zoxide = {
        enable = true;
        enableFishIntegration = true;
      };

      # ────────────────────────────────────────────────────────────────
      # fzf — ファジーファインダー
      # fish 統合は fzf-fish プラグインが担う（enableFishIntegration = false）。
      # defaultCommand: ファイル検索に ripgrep を使用（.gitignore を尊重）
      # defaultOptions: プレビューに bat を使用
      #   --height: ターミナルの何%を占有するか
      #   --layout: "reverse" = 上から下（通常の TUI と同じ方向）
      # ────────────────────────────────────────────────────────────────
      programs.fzf = {
        enable = true;
        enableFishIntegration = false; # fzf-fish プラグインが代わりに統合する
        defaultCommand = "rg --files --hidden --follow --glob '!.git/*'";
        defaultOptions = [
          "--height=80%"
          "--layout=reverse"
          "--border"
          "--preview 'bat --color=always {} 2>/dev/null || echo {}'"
        ];
      };

      # ────────────────────────────────────────────────────────────────
      # bat — cat の置き換え（シンタックスハイライト + man ページ対応）
      # `cat <file>` がそのまま bat になる（shellAliases 経由）
      # テーマ一覧: bat --list-themes
      # 人気テーマ: TwoDark / Dracula / gruvbox-dark / OneHalfDark
      # ────────────────────────────────────────────────────────────────
      programs.bat = {
        enable = true;
        config = {
          theme = "TwoDark"; # テーマを変える場合はここを編集
          style = "numbers,changes,header";
        };
      };

      # ────────────────────────────────────────────────────────────────
      # eza — ls の置き換え（Git 状態・Nerd Font アイコン付き）
      # aliases: ls / ll / lt / la（shellAliases 参照）
      # ────────────────────────────────────────────────────────────────
      programs.eza = {
        enable = true;
        git    = true;   # Git 状態を表示（変更・未追跡等）
        icons  = "auto"; # Nerd Font がインストールされている場合のみ表示
        extraOptions = [ "--group-directories-first" ];
      };

      # ────────────────────────────────────────────────────────────────
      # lazygit — Git TUI（lg コマンドで起動）
      # キーバインド: ? でヘルプ表示、space でステージ、c でコミット
      # git-delta を pager として使用（diff がシンタックスハイライトされる）
      # ────────────────────────────────────────────────────────────────
      programs.lazygit = {
        enable = true;
        settings = {
          gui.theme.lightTheme = false;
          # v0.60.0: git.paging (object) → git.pagers (array of paging configs)
          # 各要素が旧 git.paging オブジェクトと同じ形
          git.pagers = [
            {
              diff  = "delta --dark --paging=never";
              log   = "delta --dark --paging=never";
              stash = "delta --dark --paging=never";
            }
          ];
        };
      };

      # ────────────────────────────────────────────────────────────────
      # git-delta — diff のシンタックスハイライト
      # git.nix と共存: `git diff` でも自動的に delta が使われる。
      # side-by-side = true で左右分割表示（wide ターミナル推奨）
      # ────────────────────────────────────────────────────────────────
      # programs.git.delta は HM 24.11 で programs.delta にリネーム済み
      programs.delta = {
        enable               = true;
        enableGitIntegration = true; # git config に delta を自動設定
        options = {
          navigate     = true;  # n/N で diff ファイル間を移動
          dark         = true;
          side-by-side = true;  # 横並び diff（幅が狭い場合は false に）
          line-numbers = true;
          syntax-theme = "TwoDark";
        };
      };

      # ────────────────────────────────────────────────────────────────
      # その他 Rust 製 CLI ツール群
      #   fd      — find の置き換え（例: fd -e nix → .nix ファイルを検索）
      #   ripgrep — grep の置き換え（fzf-fish の Ctrl+F と連携）
      #   dust    — du の置き換え（ツリー形式でディスク使用量を表示）
      #   yazi    — TUI ファイルマネージャー（yy コマンド / 画像・PDFプレビュー対応）
      #   delta   — programs.git.delta.enable = true で管理
      # ────────────────────────────────────────────────────────────────
      home.packages = with pkgs; [
        fd
        ripgrep
        dust
        yazi
      ];
    };
  };
}
