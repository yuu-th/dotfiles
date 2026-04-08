# modules/darwin/cmux.nix
#
# cmux AI ターミナル
# - Homebrew Cask でインストール管理
# - CLI を PATH に追加（/Applications/cmux.app/Contents/Resources/bin）
# - settings.json / cmux.json を Nix で管理
#
# ⚠️ 初回ビルド前に ~/.config/cmux/settings.json と ~/.config/cmux/cmux.json が
#    存在する場合は手動で削除すること（home-manager の force = true で上書きされる）
{ config, lib, ... }:
let cfg = config.myConfig.darwin.cmux; in {
  imports = [ ./homebrew.nix ];

  options.myConfig.darwin.cmux.enable = lib.mkEnableOption "cmux AI terminal";

  config = lib.mkIf cfg.enable {
    homebrew = {
      taps = [ "manaflow-ai/cmux" ];
      casks = [ "cmux" ];
    };

    # cmux CLI を PATH に追加
    environment.systemPath = [ "/Applications/cmux.app/Contents/Resources/bin" ];

    home-manager.users.${config.myConfig.primaryUser} = {

      # ── fish 関数 ──────────────────────────────────────────────────────
      # aidev: カレントディレクトリ（worktree）を cmux ワークスペースとして展開する
      #        左ペイン: Zellij AI セッション、右ペイン: shell / nvim / browser
      # worktree-new: git worktree 作成 → aidev を自動実行
      programs.fish.functions = {
        aidev = {
          description = "Start AI workspace in cmux (left: Zellij AI, right: shell/nvim/browser)";
          body = ''
            set proj (basename (pwd))
            set cwd (pwd)

            # AI ツール選択（fzf）
            set ai_choice (printf "claude\ncopilot" | fzf --prompt "🤖 AI> " --height 5 --no-info)
            if test -z "$ai_choice"
              echo "aidev: cancelled"
              return 0
            end

            switch $ai_choice
              case claude
                set ai_cmd "claude --dangerous-skip-permissions"
              case copilot
                set ai_cmd "copilot --agent Myソクラテス --allow-all"
            end

            # ワークスペース作成（左ペイン = zellij AI セッションで永続化）
            # 出力形式: "OK surface:N workspace:N"
            # $2 = 左ペインの surface ref（zellij に AI を送り込む際に使用）
            set new_ws_out (cmux new-workspace --name "$proj [$ai_choice]" --cwd $cwd --command "zellij attach $proj-ai --create")
            set ws (echo $new_ws_out | awk '{print $NF}' | string trim)
            set ai_surface (echo $new_ws_out | awk '{print $2}' | string trim)
            if test -z "$ws"
              echo "aidev: failed to create cmux workspace" >&2
              return 1
            end
            sleep 1  # zellij の起動を待つ

            # zellij 内で AI コマンドを起動（surface 明示で確実に左ペインに送信）
            cmux send --surface $ai_surface --workspace $ws "$ai_cmd\n"

            # 右ペイン作成
            cmux new-split right --workspace $ws
            sleep 0.3

            # pane ref を取得（list-panes の * プレフィックス対策: grep -oE で抽出）
            set _panes (cmux list-panes --workspace $ws)
            set left_pane (echo $_panes[1] | grep -oE 'pane:[0-9]+')
            set right_pane (echo $_panes[-1] | grep -oE 'pane:[0-9]+')

            # 右ペイン surface ①: zellij tools セッション
            # new-split で作られた右ペインの既存 surface に送信
            set tools_surface (cmux list-pane-surfaces --workspace $ws --pane $right_pane | head -1 | grep -oE 'surface:[0-9]+')
            if test -n "$tools_surface"
              cmux send --surface $tools_surface --workspace $ws "zellij attach $proj-tools --create\n"
            end

            # 右ペイン surface ②: nvim
            set nvim_out (cmux new-surface --type terminal --pane $right_pane --workspace $ws)
            set nvim_surface (echo $nvim_out | awk '{print $2}' | string trim)
            sleep 0.3
            if test -n "$nvim_surface"
              cmux send --surface $nvim_surface --workspace $ws "nvim .\n"
            end

            # 右ペイン surface ③: browser（localhost:3000）
            cmux new-surface --type browser --url "http://localhost:3000" --pane $right_pane --workspace $ws

            # ワークスペースを選択＆左ペイン（AI）にフォーカスを戻す
            cmux select-workspace --workspace $ws
            if test -n "$left_pane"
              cmux focus-pane --pane $left_pane --workspace $ws
            end

            echo "✓ AI workspace '$proj [$ai_choice]' ready"
          '';
        };

        worktree-new = {
          description = "Create git worktree under ~/dev/ and open AI workspace in cmux";
          body = ''
            if test (count $argv) -lt 1
              echo "Usage: worktree-new <name> [<branch-args>...]" >&2
              echo "  e.g. worktree-new my-feature" >&2
              echo "  e.g. worktree-new my-feature origin/main" >&2
              return 1
            end
            set name $argv[1]
            set branch_args $argv[2..]
            set wt_path ~/dev/$name

            if test (count $branch_args) -gt 0
              git worktree add $wt_path $branch_args; or return 1
            else
              git worktree add $wt_path; or return 1
            end

            cd $wt_path; and aidev
          '';
        };
      };

      # ── settings.json ──────────────────────────────────────────────────
      home.file.".config/cmux/settings.json" = {
        force = true;
        text = ''
          {
            "$schema": "https://raw.githubusercontent.com/manaflow-ai/cmux/main/web/data/cmux-settings.schema.json",
            "schemaVersion": 1,
            "app": {
              "appearance": "dark"
            },
            "automation": {
              "claudeCodeIntegration": true
            }
          }
        '';
      };

      # ── cmux.json（グローバルコマンド定義） ──────────────────────────────
      # "Open AI Viewer": 全 AI セッション俯瞰用（レイアウトは手動で構成）
      # "Start AI Dev" は fish 関数 aidev に移行済み（cmux.json からは削除）
      home.file.".config/cmux/cmux.json" = {
        force = true;
        text = ''
          {
            "commands": [
              {
                "name": "Open AI Viewer",
                "keywords": ["viewer", "watch", "monitor", "all"],
                "restart": "confirm",
                "workspace": {
                  "name": "viewer",
                  "cwd": "~"
                }
              }
            ]
          }
        '';
      };
    };
  };
}
