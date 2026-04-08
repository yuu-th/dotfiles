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
                set ai_cmd "claude --dangerously-skip-permissions"
              case copilot
                set ai_cmd "copilot --agent Myソクラテス --allow-all"
            end

            # ワークスペース作成（左ペイン = zellij AI セッションで永続化）
            # 出力形式: "OK surface:N workspace:N"  →  $NF = workspace ref
            set ws (cmux new-workspace --name "$proj [$ai_choice]" --cwd $cwd --command "zellij attach $proj-ai --create" | awk '{print $NF}' | string trim)
            if test -z "$ws"
              echo "aidev: failed to create cmux workspace" >&2
              return 1
            end
            sleep 0.5

            # 左ペインの surface ref を取得し AI コマンドを送信
            set ai_pane (cmux list-panes --workspace $ws | head -1 | grep -oE 'pane:[0-9]+')
            set ai_surface (cmux list-pane-surfaces --workspace $ws --pane $ai_pane | head -1 | grep -oE 'surface:[0-9]+')
            if test -n "$ai_surface"
              cmux send --surface $ai_surface --workspace $ws "$ai_cmd\n"
            else
              echo "aidev: warning: could not get AI surface ref" >&2
            end

            # 右ペイン作成
            cmux new-split right --workspace $ws
            sleep 0.3

            # pane ref を取得（list-panes の * プレフィックス対策: grep -oE で抽出）
            set _panes (cmux list-panes --workspace $ws)
            set left_pane (echo $_panes[1] | grep -oE 'pane:[0-9]+')
            set right_pane (echo $_panes[-1] | grep -oE 'pane:[0-9]+')

            # 右ペイン surface ①: zellij tools セッション
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

        ai-viewer = {
          description = "Open viewer workspace showing all *-ai Zellij sessions in a grid";
          body = ''
            # *-ai セッション一覧を取得
            set ai_sessions (zellij list-sessions --short --no-formatting 2>/dev/null | grep -- '-ai$' | grep -v '^$')
            if test (count $ai_sessions) -eq 0
              echo "ai-viewer: no active *-ai sessions found" >&2
              echo "  Run 'aidev' in a project directory first." >&2
              return 1
            end

            set n (count $ai_sessions)
            set cols (math "min($n, 4)")
            set rows (math "ceil($n / $cols)")
            echo "ai-viewer: $n sessions → $cols col × $rows row"

            # 呼び出し元 surface を記録（new-workspace が自動的に引き込む余分ペインを後で閉じる）
            set my_surface_id $CMUX_SURFACE_ID

            # viewer ワークスペースを作成
            set ws (cmux new-workspace --name "viewer" --cwd ~ | awk '{print $NF}' | string trim)
            if test -z "$ws"
              echo "ai-viewer: failed to create viewer workspace" >&2
              return 1
            end
            sleep 0.3

            # workspace 作成直後のペイン一覧を記録
            # new-workspace が呼び出し元ペインを引き込む場合があるため
            # 意図したペイン（first_pane 以外）を後で閉じる
            set initial_panes
            for p in (cmux list-panes --workspace $ws)
              set ref (echo $p | grep -oE 'pane:[0-9]+')
              if test -n "$ref"
                set initial_panes $initial_panes $ref
              end
            end
            set first_pane $initial_panes[1]

            # 1セッション目を最初のペインに送信
            set first_surface (cmux list-pane-surfaces --workspace $ws --pane $first_pane | head -1 | grep -oE 'surface:[0-9]+')
            if test -n "$first_surface"
              cmux send --surface $first_surface --workspace $ws "zellij attach $ai_sessions[1]\n"
            end

            # 意図せず引き込まれた余分ペインを閉じる（呼び出し元 surface を含むペイン）
            # first_pane 以外で my_surface_id を持つ surface を close
            if test -n "$my_surface_id"
              for ep in $initial_panes[2..]
                set ep_surfaces (cmux list-pane-surfaces --workspace $ws --pane $ep 2>/dev/null | grep -oE 'surface:[0-9A-Fa-f-]+')
                for s in $ep_surfaces
                  cmux close-surface --surface $s --workspace $ws 2>/dev/null
                end
              end
            end

            # 1行目: right split を繰り返して cols 列を作成
            set col_panes $first_pane
            for i in (seq 2 $cols)
              cmux new-split right --workspace $ws
              sleep 0.3
              set _panes (cmux list-panes --workspace $ws)
              set new_pane (echo $_panes[-1] | grep -oE 'pane:[0-9]+')
              set col_panes $col_panes $new_pane
              set new_surface (cmux list-pane-surfaces --workspace $ws --pane $new_pane | head -1 | grep -oE 'surface:[0-9]+')
              if test -n "$new_surface"
                cmux send --surface $new_surface --workspace $ws "zellij attach $ai_sessions[$i]\n"
              end
            end

            # 2行目以降: 各列ペインを focus-pane → down split
            if test $rows -gt 1
              for row in (seq 2 $rows)
                for col in (seq 1 $cols)
                  set session_idx (math "($row - 1) * $cols + $col")
                  if test $session_idx -le $n
                    cmux focus-pane --pane $col_panes[$col] --workspace $ws
                    sleep 0.2
                    cmux new-split down --workspace $ws
                    sleep 0.3
                    set _panes2 (cmux list-panes --workspace $ws)
                    set bot_pane (echo $_panes2[-1] | grep -oE 'pane:[0-9]+')
                    set bot_surface (cmux list-pane-surfaces --workspace $ws --pane $bot_pane | head -1 | grep -oE 'surface:[0-9]+')
                    if test -n "$bot_surface"
                      cmux send --surface $bot_surface --workspace $ws "zellij attach $ai_sessions[$session_idx]\n"
                    end
                  end
                end
              end
            end

            cmux select-workspace --workspace $ws
            echo "✓ viewer ready ($n AI sessions)"
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
