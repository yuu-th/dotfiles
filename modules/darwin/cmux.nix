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

            # ワークスペース作成（初期ターミナルは AI セッションとして起動）
            # 出力形式: "OK surface:N workspace:N"
            # $2 = AI surface ref、$NF = workspace ref
            set ws_output (cmux new-workspace --name $proj --cwd $cwd --command "zellij attach $proj-ai --create")
            set ws (echo $ws_output | awk '{print $NF}' | string trim)
            set ai_surface (echo $ws_output | awk '{print $2}' | string trim)
            if test -z "$ws"
              echo "aidev: failed to create cmux workspace" >&2
              return 1
            end
            sleep 0.5

            # 右ペインを作成（split right）
            cmux new-split right --workspace $ws
            sleep 0.3

            # 右ペイン surface ①: Zellij shell（tools）セッション
            cmux send --workspace $ws "zellij attach $proj-tools --create\n"

            # 右ペイン surface ②: nvim
            cmux new-surface --type terminal --workspace $ws
            sleep 0.3
            cmux send --workspace $ws "nvim .\n"

            # 右ペイン surface ③: browser（http://localhost:3000）
            cmux new-surface --type browser --url "http://localhost:3000" --workspace $ws

            # フォーカスを左ペイン AI surface に戻す（new-workspace で取得した surface ref を使用）
            if test -n "$ai_surface"
              cmux focus-surface --surface $ai_surface --workspace $ws
            end

            echo "✓ AI workspace '$proj' ready"
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
