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

    # ── Karabiner: cmux 内ワークスペース切替 ─────────────────────────────────
    # 固定 workspace（sidebar 0, 1）を ⌘⇧S / ⌘⇧V に割り当て、
    # ⌘1–⌘9 をプロジェクト workspace（sidebar 2〜）に offset して割り当てる。
    myConfig.darwin.karabiner.rules =
      let
        cmuxBin = "/Applications/cmux.app/Contents/Resources/bin/cmux";
        cmuxCond = {
          type = "frontmost_application_if";
          bundle_identifiers = [ "^com\\.cmuxterm\\.app$" ];
        };
        selectWs = idx: {
          shell_command = "${cmuxBin} select-workspace --workspace ${toString idx}";
        };
        fixedKeys = [
          # ⌘⇧S → basic shell (sidebar slot 0)
          { type = "basic"; conditions = [ cmuxCond ];
            from = { key_code = "s"; modifiers.mandatory = [ "command" "shift" ]; };
            to = [ (selectWs 0) ]; }
          # ⌘⇧V → viewer (sidebar slot 1)
          { type = "basic"; conditions = [ cmuxCond ];
            from = { key_code = "v"; modifiers.mandatory = [ "command" "shift" ]; };
            to = [ (selectWs 1) ]; }
        ];
        # ⌘1 → slot 2, ⌘2 → slot 3, ... ⌘9 → slot 10 (offset = 2 fixed workspaces)
        projectKeys = map (n: {
          type = "basic";
          conditions = [ cmuxCond ];
          from = { key_code = toString n; modifiers.mandatory = [ "command" ]; };
          to = [ (selectWs (n + 1)) ];
        }) (lib.range 1 9);
      in [
        {
          description = "cmux: workspace navigation (fixed slots + project offset)";
          manipulators = fixedKeys ++ projectKeys;
        }
      ];

    home-manager.users.${config.myConfig.primaryUser} = {

      # ── fish 関数 ──────────────────────────────────────────────────────
      # aidev: カレントディレクトリ（worktree）を cmux ワークスペースとして展開する
      #        左ペイン: Zellij AI セッション、右ペイン: shell / nvim / browser
      # worktree-new: git worktree 作成 → aidev を自動実行
      programs.fish.functions = {
        # zellij セッション名を最大19文字に安全に短縮するプライベート関数
        # （zellij のセッション名上限は25文字。最長サフィックス -{cp|cl}-ai / -tools = 6文字を考慮して19文字以内）
        # 19文字超の場合: 先頭12文字（末尾ハイフン除去）+ "-" + MD5先頭6文字
        # Dependents: aidev, ai-viewer, aidev-stop, cmux-init
        "_zellij_sname" = {
          description = "Convert project name to zellij-safe base name (max 19 chars)";
          body = ''
            set _p $argv[1]
            if test (string length $_p) -gt 19
              set _h (echo $_p | md5 | string sub -l 6)
              set _pre (string sub -l 12 $_p | string trim --right --chars '-')
              echo "$_pre-$_h"
            else
              echo $_p
            end
          '';
        };

        # AI 名 → 2文字省略形（セッション名に埋め込む用）
        # Dependents: aidev, ai-viewer, aidev-stop, cmux-init
        "_ai_short" = {
          description = "Convert AI name to 2-char abbreviation (claude→cl, copilot→cp)";
          body = ''
            switch $argv[1]
              case claude; echo cl
              case copilot; echo cp
              case '*'; echo $argv[1]
            end
          '';
        };

        # 2文字省略形 → AI 名（表示用）
        "_ai_long" = {
          description = "Convert 2-char AI abbreviation to full name (cl→claude, cp→copilot)";
          body = ''
            switch $argv[1]
              case cl; echo claude
              case cp; echo copilot
              case '*'; echo $argv[1]
            end
          '';
        };

        # 共有 project plan を 1 回で計算して返す
        # 出力形式: project<TAB>ai<TAB>cwd<TAB>session<TAB>source
        # source = workspace | orphan
        "_cmux_project_plan" = {
          description = "List project plan rows sorted by project name";
          body = ''
            set _ws_json (cmux rpc workspace.list 2>/dev/null)
            if test -z "$_ws_json"
              return 1
            end

            set _rows
            set _known_sessions
            set _known_projects
            set _tab (printf '\t')

            for _row in (echo $_ws_json | jq -r '
              .workspaces[]
              | select(.title | test("^.+ \\[(claude|copilot)\\]$"))
              | (.title | capture("^(?<project>.+) \\[(?<ai>claude|copilot)\\]$")) as $m
              | $m.project + "\t" + $m.ai + "\t" + (.current_directory // "")
            ' 2>/dev/null)
              set _parts (string split $_tab $_row)
              set _project $_parts[1]
              set _ai $_parts[2]
              set _cwd $_parts[3]
              set _session (_zellij_sname $_project)"-"(_ai_short $_ai)"-ai"
              set _known_sessions $_known_sessions $_session
              set _known_projects $_known_projects $_project
              set _rows $_rows (printf '%s\t%s\t%s\t%s\t%s' "$_project" "$_ai" "$_cwd" "$_session" workspace)
            end

            set _zellij_sessions (zellij list-sessions --short --no-formatting 2>/dev/null)
            for _sess in (printf '%s\n' $_zellij_sessions | grep -E -- '-(cp|cl)-ai$')
              if contains -- $_sess $_known_sessions
                continue
              end
              set _base (string replace -r -- '-(cp|cl)-ai$' "" $_sess)
              set _tools_sess $_base"-tools"
              if not printf '%s\n' $_zellij_sessions | grep -Fxq -- "$_tools_sess"
                continue
              end
              set _orph_abbr (string match -r '(cp|cl)-ai$' $_sess | tail -1)
              set _orph_ai (_ai_long $_orph_abbr)
              set _orph_cwd (zellij -s $_sess action dump-layout 2>/dev/null | awk -F'"' '/cwd / {print $2; exit}')
              if test -z "$_orph_cwd"
                continue
              end
              set _orph_project (basename $_orph_cwd)
              if contains -- $_orph_project $_known_projects
                continue
              end
              set _known_projects $_known_projects $_orph_project
              set _rows $_rows (printf '%s\t%s\t%s\t%s\t%s' "$_orph_project" "$_orph_ai" "$_orph_cwd" "$_sess" orphan)
            end

            if test (count $_rows) -eq 0
              return 0
            end

            printf '%s\n' $_rows | env LC_ALL=C sort
          '';
        };

        aidev = {
          description = "Start AI workspace in cmux (left: Zellij AI, right: shell/nvim/browser)";
          # Requires: _zellij_sname
          body = ''
            # ── フラグ解析（--ai claude|copilot, --no-focus）──────────────────
            set _ai_flag ""
            set _no_focus 0
            set _i 1
            while test $_i -le (count $argv)
              switch $argv[$_i]
                case --ai
                  set _i (math $_i + 1)
                  if test $_i -le (count $argv)
                    set _ai_flag $argv[$_i]
                  end
                case --no-focus
                  set _no_focus 1
              end
              set _i (math $_i + 1)
            end

            set proj (basename (pwd))
            set cwd (pwd)
            # zellij セッション名上限25文字のため、長い場合はハッシュで短縮（最大19文字）
            set session (_zellij_sname $proj)

            # ── 既存の cmux workspace を確認（"proj [*]" パターンでマッチ）────
            set _existing_ws ""
            set _existing_ai ""
            for _ws_line in (cmux list-workspaces 2>/dev/null)
              set _ws_name (echo $_ws_line | sed 's/^[* ]*workspace:[0-9]* *//' | sed 's/ *\[selected\]$//' | string trim)
              if string match -q "$proj [*]" "$_ws_name"
                set _existing_ws (echo $_ws_line | grep -oE 'workspace:[0-9]+')
                set _existing_ai (string match -r '\[(claude|copilot)\]' "$_ws_name" | tail -1)
                break
              end
            end
            if test -n "$_existing_ws"
              set _ai_abbr (_ai_short $_existing_ai)
              if zellij list-sessions --short --no-formatting 2>/dev/null | grep -q "^$session-$_ai_abbr-ai\$"
                # -ai セッション生存中 → -tools も確認（両方存在が正常稼働の条件）
                if zellij list-sessions --short --no-formatting 2>/dev/null | grep -q "^$session-tools\$"
                  # pgrep でプロセス確認（--create 付き: ai-viewer 誤検知防止）
                  if pgrep -f "zellij attach $session-$_ai_abbr-ai --create" > /dev/null 2>&1
                    # pgrep OK → さらに pane 構造を確認（追加のガード）
                    set _panes_info (cmux list-panes --workspace $_existing_ws 2>/dev/null)
                    if test (count $_panes_info) -ge 2
                      set _right_pane (echo $_panes_info[-1] | grep -oE 'pane:[0-9]+')
                      set _right_surface_count (count (cmux list-pane-surfaces --workspace $_existing_ws --pane $_right_pane 2>/dev/null))
                      if test $_right_surface_count -ge 2
                        # 正常稼働中（-ai ✓ + -tools ✓ + pgrep ✓ + pane 構造 ✓）
                        if test $_no_focus -eq 0
                          cmux select-workspace --workspace $_existing_ws
                        end
                        echo "✓ Already running: '$proj'"
                        return 0
                      end
                    end
                    # pane 構造が壊れている → close して再作成へ
                  end
                  # pgrep NG = ai-viewer のみ or cmux 再起動後 → close して再作成へ
                end
                # -tools セッションなし → close して再作成へ
              end
              # -ai セッションなし or 上記チェック不合格 → workspace を削除して再作成へ
              cmux close-workspace --workspace $_existing_ws 2>/dev/null
            end

            # AI ツール選択（--ai フラグがない場合は fzf）
            if test -n "$_ai_flag"
              set ai_choice "$_ai_flag"
            else
              set ai_choice (printf "claude\ncopilot" | fzf --prompt "🤖 AI> " --height 5 --no-info)
              if test -z "$ai_choice"
                echo "aidev: cancelled"
                return 0
              end
            end

            switch $ai_choice
              case claude
                set ai_cmd "claude --dangerously-skip-permissions"
              case copilot
                set ai_cmd "copilot --agent Myソクラテス --allow-all"
            end
            set _ai_abbr (_ai_short $ai_choice)

            # zellij セッションが既存かどうかを事前に確認
            # 既存 = AI はすでに起動中なので cmux send によるコマンド送信をスキップ
            set _zellij_session_exists 0
            if zellij list-sessions --short --no-formatting 2>/dev/null | grep -q "^$session-$_ai_abbr-ai\$"
              set _zellij_session_exists 1
            end

            # ワークスペース作成（左ペイン = zellij AI セッションで永続化）
            # 出力形式: "OK surface:N workspace:N"  →  $NF = workspace ref
            set ws (cmux new-workspace --name "$proj [$ai_choice]" --cwd $cwd --command "zellij attach $session-$_ai_abbr-ai --create" | awk '{print $NF}' | string trim)
            if test -z "$ws"
              echo "aidev: failed to create cmux workspace" >&2
              return 1
            end
            # 左ペインの surface ref を取得
            set ai_pane (cmux list-panes --workspace $ws | head -1 | grep -oE 'pane:[0-9]+')
            set ai_surface (cmux list-pane-surfaces --workspace $ws --pane $ai_pane | head -1 | grep -oE 'surface:[0-9]+')
            # 新規セッションのみ AI コマンドを送信（既存セッションは AI 起動済みなのでスキップ）
            if test $_zellij_session_exists -eq 0
              if test -n "$ai_surface"
                cmux send --surface $ai_surface --workspace $ws "$ai_cmd\n"
              else
                echo "aidev: warning: could not get AI surface ref" >&2
              end
            end

            # 右ペイン作成
            cmux new-split right --workspace $ws

            # pane ref を取得（list-panes の * プレフィックス対策: grep -oE で抽出）
            set _panes (cmux list-panes --workspace $ws)
            set left_pane (echo $_panes[1] | grep -oE 'pane:[0-9]+')
            set right_pane (echo $_panes[-1] | grep -oE 'pane:[0-9]+')

            # 右ペイン surface ①: zellij tools セッション
            set tools_surface (cmux list-pane-surfaces --workspace $ws --pane $right_pane | head -1 | grep -oE 'surface:[0-9]+')
            if test -n "$tools_surface"
              cmux send --surface $tools_surface --workspace $ws "zellij attach $session-tools --create\n"
            end

            # 右ペイン surface ②: nvim
            set nvim_out (cmux new-surface --type terminal --pane $right_pane --workspace $ws)
            set nvim_surface (echo $nvim_out | awk '{print $2}' | string trim)
            if test -n "$nvim_surface"
              cmux send --surface $nvim_surface --workspace $ws "nvim .\n"
            end

            # 右ペイン surface ③: browser（localhost:3000）
            cmux new-surface --type browser --url "http://localhost:3000" --pane $right_pane --workspace $ws

            # ワークスペースを選択＆左ペイン（AI）にフォーカスを戻す（--no-focus の場合はスキップ）
            if test $_no_focus -eq 0
              cmux select-workspace --workspace $ws
              if test -n "$left_pane"
                cmux focus-pane --pane $left_pane --workspace $ws
              end
            end

            echo "✓ AI workspace '$proj [$ai_choice]' ready"
          '';
        };

        ai-viewer = {
          description = "Open viewer workspace showing all *-ai Zellij sessions in a grid";
          # Requires: _cmux_project_plan
          body = ''
            # ── フラグ解析（--index N / --session SESSION）───────────────────
            # --index なし → 末尾に配置（単体起動時のデフォルト動作）
            set _target_index ""
            set _sessions_from_args
            set _i 1
            while test $_i -le (count $argv)
              switch $argv[$_i]
                case --index
                  set _i (math $_i + 1)
                  if test $_i -le (count $argv)
                    set _target_index $argv[$_i]
                  end
                case --session
                  set _i (math $_i + 1)
                  if test $_i -le (count $argv)
                    set _sessions_from_args $_sessions_from_args $argv[$_i]
                  end
              end
              set _i (math $_i + 1)
            end

            # 明示 session が渡された場合はその順序を優先し、なければ共有 plan 順で収集
            if test (count $_sessions_from_args) -gt 0
              set ai_sessions $_sessions_from_args
            else
              set ai_sessions (zellij list-sessions --short --no-formatting 2>/dev/null | grep -E -- '-(cp|cl)-ai$' | grep -v '^$')
              set ordered_sessions
              set _plan_rows (_cmux_project_plan)
              for _row in $_plan_rows
                set _parts (string split (printf '\t') $_row)
                set _session $_parts[4]
                if contains -- $_session $ai_sessions
                  set ordered_sessions $ordered_sessions $_session
                end
              end
              for s in $ai_sessions
                if not contains -- $s $ordered_sessions
                  set ordered_sessions $ordered_sessions $s
                end
              end
              set ai_sessions $ordered_sessions
            end

            if test (count $ai_sessions) -eq 0
              echo "ai-viewer: no active *-ai sessions found" >&2
              echo "  Run 'aidev' in a project directory first." >&2
              return 1
            end

            # 呼び出し元の workspace を記録（self-close 防止）
            set caller_ws (cmux current-workspace 2>/dev/null | grep -oE 'workspace:[0-9]+')

            # 既存の "ai-viewer" workspace を閉じる（再起動対応、呼び出し元は除外）
            for ws_line in (cmux list-workspaces)
              set ws_ref (echo $ws_line | grep -oE 'workspace:[0-9]+')
              # 呼び出し元は絶対に閉じない
              if test "$ws_ref" = "$caller_ws"
                continue
              end
              # workspace 名が厳密に "ai-viewer" のものだけ閉じる
              set _name (echo $ws_line | sed 's/^[* ]*workspace:[0-9]* *//' | sed 's/ *\[selected\]$//' | string trim)
              if test "$_name" = "ai-viewer"
                cmux close-workspace --workspace $ws_ref 2>/dev/null
              end
            end

            set n (count $ai_sessions)
            set cols (math "min($n, 4)")
            set rows (math "ceil($n / $cols)")
            echo "ai-viewer: $n sessions → $cols col × $rows row"

            # --index 未指定の場合: 末尾に配置するため作成前に最後の ref を記録
            set last_ws_ref ""
            if test -z "$_target_index"
              set last_ws_ref (cmux list-workspaces | grep -oE 'workspace:[0-9]+' | tail -1)
            end

            # viewer ワークスペースを作成
            # 先頭 pane は new-workspace の --command を使い、
            # post-create の外部 send に依存しない経路へ寄せる
            set _viewer_out (cmux new-workspace --name "ai-viewer" --cwd ~ --command "zellij attach $ai_sessions[1]")
            set ws (echo $_viewer_out | awk '{print $NF}' | string trim)
            if test -z "$ws"
              echo "ai-viewer: failed to create ai-viewer workspace" >&2
              return 1
            end
            # 最初のペインに session 1 を送信
            set first_pane (cmux list-panes --workspace $ws | head -1 | grep -oE 'pane:[0-9]+')

            # 1行目: right split を繰り返して cols 列を作成
            # NOTE: macOS seq は seq 2 1 のとき降順で 2,1 を返すため
            #       明示的にステップ 1 を指定（seq first step last）
            set col_panes $first_pane
            for i in (seq 2 1 $cols)
              set _split_out (cmux new-split right --workspace $ws)
              set _panes (cmux list-panes --workspace $ws)
              set new_pane (echo $_panes[-1] | grep -oE 'pane:[0-9]+')
              set col_panes $col_panes $new_pane
              set new_surface (echo $_split_out | grep -oE 'surface:[0-9]+')
              if test -n "$new_surface"
                cmux send --surface $new_surface --workspace $ws "zellij attach $ai_sessions[$i]\n"
              end
            end

            # 列幅の均等化（連続 right-split はバイナリ分割なので不均等になる）
            # cmux のボーダー自動再配分(proportional)を考慮し左→右に順次リサイズ
            # 公式: amount_k = round(pw * (N-1-k) / (2N))  (k=1..N-2)
            if test $cols -gt 2
              set _pw (osascript -e 'tell application "System Events" to tell (first process whose name contains "cmux") to get size of window 1' 2>/dev/null | cut -d',' -f1 | string trim)
              if test -n "$_pw"
                for k in (seq 1 (math "$cols - 2"))
                  set _amt (math "round($_pw * ($cols - 1 - $k) / (2 * $cols))")
                  if test $_amt -gt 0
                    cmux resize-pane --pane $col_panes[(math "$k + 1")] --workspace $ws -L --amount $_amt
                  end
                end
              end
            end

            # 2行目以降: 各列ペインを focus-pane → down split
            if test $rows -gt 1
              for row in (seq 2 1 $rows)
                for col in (seq 1 1 $cols)
                  set session_idx (math "($row - 1) * $cols + $col")
                  if test $session_idx -le $n
                    cmux focus-pane --pane $col_panes[$col] --workspace $ws
                    set _down_out (cmux new-split down --workspace $ws)
                    set bot_surface (echo $_down_out | grep -oE 'surface:[0-9]+')
                    if test -n "$bot_surface"
                      cmux send --surface $bot_surface --workspace $ws "zellij attach $ai_sessions[$session_idx]\n"
                    end
                  end
                end
              end
            end

            cmux select-workspace --workspace $ws
            # reorder: --index 指定なら指定スロットへ、なければ末尾へ
            if test -n "$_target_index"
              cmux reorder-workspace --workspace $ws --index $_target_index 2>/dev/null
            else if test -n "$last_ws_ref"
              cmux reorder-workspace --workspace $ws --after $last_ws_ref 2>/dev/null
            end
            echo "✓ viewer ready ($n AI sessions)"
          '';
        };

        ai-viewer-refresh = {
          description = "Recreate ai-viewer and place it at slot 1";
          body = ''
            set _helper 0
            if test (count $argv) -gt 0
              for _arg in $argv
                switch $_arg
                  case --helper-internal
                    set _helper 1
                end
              end
            end

            set _current_ws (cmux current-workspace 2>/dev/null | grep -oE 'workspace:[0-9]+')
            set _self_ws_id "$CMUX_WORKSPACE_ID"
            set _current_title ""
            if test -n "$_current_ws"
              set _current_title (cmux rpc workspace.list 2>/dev/null | jq -r --arg ref "$_current_ws" '
                first(.workspaces[] | select(.ref == $ref) | .title) // empty
              ' 2>/dev/null)
            end

            if test $_helper -eq 0 -a "$_current_title" = "ai-viewer"
              cmux new-workspace --name "_ai_viewer_refresh" --cwd ~ --command "ai-viewer-refresh --helper-internal" >/dev/null
              echo "ai-viewer-refresh: delegated via helper workspace"
              return 0
            end

            ai-viewer --index 1
            set _status $status

            if test $_helper -eq 1 -a -n "$_self_ws_id"
              cmux close-workspace --workspace "$_self_ws_id" 2>/dev/null
            end

            return $_status
          '';
        };

        aidev-stop = {
          description = "Stop aidev workspace(s): kill zellij sessions and close cmux workspace";
          # Requires: _zellij_sname
          body = ''
            # ── 1. cmux workspace 一覧（"proj [claude/copilot]" パターン）を収集 ──
            set _candidates
            set _ws_refs
            set _known_bases  # cmux で管理中のセッションベース名（重複チェック用）
            for _ws_line in (cmux list-workspaces 2>/dev/null)
              set _ws_name (echo $_ws_line | sed 's/^[* ]*workspace:[0-9]* *//' | sed 's/ *\[selected\]$//' | string trim)
              if string match -qr '.*\[(claude|copilot)\]$' "$_ws_name"
                set _candidates $_candidates $_ws_name
                set _ws_refs $_ws_refs (echo $_ws_line | grep -oE 'workspace:[0-9]+')
                set _proj_base (echo $_ws_name | sed 's/ \[.*\]$//')
                set _known_bases $_known_bases (_zellij_sname $_proj_base)
              end
            end

            # ── 2. 孤立 zellij session を収集（cmux workspace に紐付かない *-(cp|cl)-ai セッション）──
            set _orphan_candidates
            for _sess in (zellij list-sessions --short --no-formatting 2>/dev/null | grep -E -- '-(cp|cl)-ai$')
              set _base (string replace -r -- '-(cp|cl)-ai$' "" $_sess)
              set _orph_abbr (string match -r '(cp|cl)-ai$' $_sess | tail -1)
              if not contains $_base $_known_bases
                set _orph_ai (_ai_long $_orph_abbr)
                set _orphan_candidates $_orphan_candidates "$_base [$_orph_ai] (orphaned)"
              end
            end

            # ── 3. 候補がなければ終了 ──
            set _all_candidates $_candidates $_orphan_candidates
            if test (count $_all_candidates) -eq 0
              echo "aidev-stop: no aidev workspaces found" >&2
              return 1
            end

            # ── 4. fzf で対象を選択（TAB で複数選択）──
            set _selected (printf '%s\n' $_all_candidates | fzf --prompt "🛑 Stop> " --height 40% --multi --no-info)
            if test -z "$_selected"
              echo "aidev-stop: cancelled"
              return 0
            end

            for _name in $_selected
              # proj 名を抽出して zellij セッション名を計算
              set _proj (echo $_name | sed 's/ \[.*$//')
              set _session (_zellij_sname $_proj)
              set _del_ai (string match -r '\[(claude|copilot)\]' "$_name" | tail -1)
              set _del_abbr (_ai_short $_del_ai)

              # zellij sessions を完全削除（EXITED 状態で残さない）
              zellij delete-session --force "$_session-$_del_abbr-ai" 2>/dev/null
              zellij delete-session --force "$_session-tools" 2>/dev/null

              # (orphaned) でない場合のみ cmux workspace を閉じる
              if not string match -q "*(orphaned)" "$_name"
                set _idx 1
                set _target_ws ""
                for c in $_candidates
                  if test "$c" = "$_name"
                    set _target_ws $_ws_refs[$_idx]
                    break
                  end
                  set _idx (math "$_idx + 1")
                end
                if test -n "$_target_ws"
                  cmux close-workspace --workspace $_target_ws 2>/dev/null
                end
              end

              echo "✓ Stopped '$_name'"
            end
          '';
        };

        worktree-new = {
          description = "Create a sibling git worktree from a new branch and open AI workspace in cmux";
          body = ''
            if test (count $argv) -ne 2
              echo "Usage: worktree-new <branch> <base>" >&2
              echo "  e.g. worktree-new feature/login origin/main" >&2
              return 1
            end

            set branch $argv[1]
            set base $argv[2]

            git check-ref-format --branch $branch >/dev/null 2>&1
            or begin
              echo "worktree-new: invalid branch name: $branch" >&2
              return 1
            end

            set repo_root (git rev-parse --show-toplevel 2>/dev/null)
            or begin
              echo "worktree-new: not inside a git repo" >&2
              return 1
            end

            git rev-parse --verify --quiet "$base^{commit}" >/dev/null 2>&1
            or begin
              echo "worktree-new: base '$base' not found" >&2
              return 1
            end

            if git show-ref --verify --quiet "refs/heads/$branch"
              echo "worktree-new: branch '$branch' already exists" >&2
              return 1
            end

            set repo_name (basename $repo_root)
            set parent_dir (dirname $repo_root)
            set branch_slug (string replace -a -- "/" "-" "$branch")
            set branch_slug (string replace -ra -- "[^A-Za-z0-9._-]+" "-" "$branch_slug")
            set branch_slug (string trim --chars "-." "$branch_slug")

            if test -z "$branch_slug"
              echo "worktree-new: branch '$branch' cannot be converted to a directory name" >&2
              return 1
            end

            set wt_path "$parent_dir/$repo_name-$branch_slug"
            if test -e "$wt_path"
              echo "worktree-new: path already exists: $wt_path" >&2
              return 1
            end

            git worktree add -b "$branch" "$wt_path" "$base"; or return 1

            cd "$wt_path"; and aidev
          '';
        };

        cmux-init = {
          description = "Restore all AI workspaces after cmux restart (re-runs aidev for each dead workspace)";
          body = ''
            # 呼び出し元 workspace を記録（viewer 再生成時の self-close 回避用）
            set _caller_ws (cmux current-workspace 2>/dev/null | grep -oE 'workspace:[0-9]+')

            # cmux rpc でワークスペース一覧を取得（cleanup 対象判定に使用）
            set _ws_json (cmux rpc workspace.list 2>/dev/null)
            if test -z "$_ws_json"
              echo "cmux-init: failed to get workspace list" >&2
              return 1
            end

            # 現在の workspace ID（最後に _cmux_restore workspace を close するために使用）
            set _current_ws_id "$CMUX_WORKSPACE_ID"

            # 共有 plan を 1 回だけ計算し、viewer と restore の両方で使い回す
            set _plan_rows (_cmux_project_plan)
            set _plan_sessions
            set _viewer_sessions
            set _needs_viewer_refresh 0
            set _zellij_sessions (zellij list-sessions --short --no-formatting 2>/dev/null)
            for _row in $_plan_rows
              set _parts (string split (printf '\t') $_row)
              set _session $_parts[4]
              set _plan_sessions $_plan_sessions $_session
              if contains -- $_session $_zellij_sessions
                set _viewer_sessions $_viewer_sessions $_session
              else
                set _needs_viewer_refresh 1
              end
            end

            # ── STEP 0: shell workspace を index 0 に配置 ────────────────────
            set _ws_json2 (cmux rpc workspace.list 2>/dev/null)
            set _shell_ref (echo $_ws_json2 | jq -r 'first(.workspaces[] | select(.title == "shell") | .ref) // empty' 2>/dev/null)
            if test -z "$_shell_ref"
              echo "cmux-init: creating shell workspace ..."
              set _shell_ref (cmux new-workspace --name "shell" --cwd ~ 2>/dev/null | awk '{print $NF}' | string trim)
            end
            if test -n "$_shell_ref"
              cmux reorder-workspace --workspace $_shell_ref --index 0 2>/dev/null
              # terminal に cd ~ を送信（既存の場合は cwd を ~ にリセット）
              set _shell_pane (cmux list-panes --workspace $_shell_ref 2>/dev/null | head -1 | grep -oE 'pane:[0-9]+')
              set _shell_surface (cmux list-pane-surfaces --workspace $_shell_ref --pane $_shell_pane 2>/dev/null | head -1 | grep -oE 'surface:[0-9]+')
              if test -n "$_shell_surface"
                cmux send --surface $_shell_surface "cd ~\n" 2>/dev/null
              end
              echo "cmux-init: shell positioned at slot 0"
            end

            # ── STEP 1: 既に存在する AI セッションだけで viewer を早めに出す ───
            if test (count $_viewer_sessions) -gt 0
              if test -n "$_caller_ws"
                cmux select-workspace --workspace $_caller_ws 2>/dev/null
              end
              echo "cmux-init: launching ai-viewer ..."
              set _viewer_args --index 1
              for _session in $_viewer_sessions
                set _viewer_args $_viewer_args --session $_session
              end
              ai-viewer $_viewer_args 2>/dev/null
              or echo "cmux-init: ai-viewer skipped (failed to create early viewer)"
            end

            # ── STEP 2: viewer を残しつつ不要な workspace を close ────────────
            # 残すもの: "shell"、"ai-viewer"、"proj [claude|copilot]"、自分自身 (_cmux_restore)
            echo "cmux-init: cleaning up unrelated workspaces ..."
            set _ws_json_cleanup (cmux rpc workspace.list 2>/dev/null)
            set _to_close (echo $_ws_json_cleanup | jq -r --arg cur_id "$_current_ws_id" '
              .workspaces[]
              | select(
                  .title != "shell"
                  and .title != "ai-viewer"
                  and (.title | (endswith(" [claude]") or endswith(" [copilot]")) | not)
                  and (.id != $cur_id)
                )
              | .ref
            ' 2>/dev/null)
            for _ref in $_to_close
              echo "cmux-init: closing unrelated workspace ($_ref) ..."
              cmux close-workspace --workspace $_ref 2>/dev/null
            end

            # ── STEP 3: 共有 plan 順に project workspace を復元 ───────────────
            if test (count $_plan_rows) -eq 0
              echo "cmux-init: no project workspaces found"
            end

            set _restored 0
            set _prev_cwd (pwd)
            for _entry in $_plan_rows
              set _parts (string split (printf '\t') $_entry)
              set _project $_parts[1]
              set _ai $_parts[2]
              set _cwd $_parts[3]
              set _source $_parts[5]
              if not test -d "$_cwd"
                echo "cmux-init: skipping $_project ($_source; directory not found: $_cwd)"
                continue
              end
              echo "cmux-init: restoring $_project [$_ai] ($_source) ..."
              cd $_cwd
              aidev --ai $_ai --no-focus
              set _restored (math $_restored + 1)
            end

            cd $_prev_cwd
            echo "✓ cmux-init: $_restored workspace(s) restored"

            # ── STEP 4: early viewer に未反映だった session があれば最後に同期 ──
            set _final_viewer_sessions
            set _final_zellij_sessions (zellij list-sessions --short --no-formatting 2>/dev/null)
            for _session in $_plan_sessions
              if contains -- $_session $_final_zellij_sessions
                set _final_viewer_sessions $_final_viewer_sessions $_session
              end
            end
            if test $_needs_viewer_refresh -eq 1 -a (count $_final_viewer_sessions) -gt 0
              if test -n "$_caller_ws"
                cmux select-workspace --workspace $_caller_ws 2>/dev/null
              end
              echo "cmux-init: refreshing ai-viewer ..."
              set _viewer_args --index 1
              for _session in $_final_viewer_sessions
                set _viewer_args $_viewer_args --session $_session
              end
              ai-viewer $_viewer_args 2>/dev/null
              or echo "cmux-init: ai-viewer refresh skipped"
            end

            # ── STEP 5: 自分自身 (_cmux_restore workspace) を close ──────────
            if test -n "$_current_ws_id"
              set _my_title (cmux rpc workspace.list 2>/dev/null | jq -r --arg id "$_current_ws_id" 'first(.workspaces[] | select(.id == $id) | .title) // empty' 2>/dev/null)
              if test "$_my_title" = "_cmux_restore"
                echo "cmux-init: closing restore workspace ..."
                cmux close-workspace --workspace "$_current_ws_id" 2>/dev/null
              end
            end

            echo "✓ cmux-init: done — shell(0) → ai-viewer(1) → cleanup → projects(2+)"
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
              "appearance": "dark",
              "reorderOnNotification": false,
              "newWorkspacePlacement": "end"
            },
            "automation": {
              "claudeCodeIntegration": true,
              "socketControlMode": "allowAll"
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
                  "name": "ai-viewer",
                  "cwd": "~"
                }
              },
              {
                "name": "Refresh AI Viewer",
                "description": "既存 ai-viewer を壊して正しい session で作り直す",
                "keywords": ["viewer", "refresh", "rebuild", "reconnect"],
                "restart": "recreate",
                "workspace": {
                  "name": "_ai_viewer_refresh",
                  "cwd": "~",
                  "layout": {
                    "pane": {
                      "surfaces": [{"type": "terminal", "command": "ai-viewer-refresh"}]
                    }
                  }
                }
              },
              {
                "name": "Restore Workspaces",
                "description": "cmux 再起動後に全 AI workspace を再接続する",
                "keywords": ["init", "restore", "restart", "reconnect"],
                "restart": "recreate",
                "workspace": {
                  "name": "_cmux_restore",
                  "cwd": "~",
                  "layout": {
                    "pane": {
                      "surfaces": [{"type": "terminal", "command": "cmux-init"}]
                    }
                  }
                }
              }
            ]
          }
        '';
      };
    };
  };
}
