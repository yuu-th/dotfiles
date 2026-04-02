#!/usr/bin/env bash

# focus-tool.sh — AeroSpace テレポートマクロ
# 使い方: focus-tool.sh editor | git | shell | ai N | ai-new | project-picker
#
# 【ウィンドウ識別】
#   メインウィンドウ: zj コマンドが ~/.local/share/zellij-aerospace/sessions.tsv に
#                    起動時のウィンドウ ID を登録する（レジストリ方式）
#   AI ウィンドウ:   Zellij レイアウトで pane name="[project]: AI" を固定し
#                    macOS Accessibility API でタイトルマッチ + X 座標ソート

set -euo pipefail

AEROSPACE="/run/current-system/sw/bin/aerospace"
GHOSTTY_BUNDLE="com.mitchellh.ghostty"
REGISTRY="$HOME/.local/share/zellij-aerospace/sessions.tsv"

# ── メインウィンドウ取得（レジストリベース）────────────────────────────────────
# 現在フォーカス WS にある登録済みメインウィンドウの ID を返す
get_main_window_id() {
    [ -f "$REGISTRY" ] || return 1
    local focused_ids
    focused_ids=$("$AEROSPACE" list-windows --focused --format "%{window-id}" 2>/dev/null)
    while IFS=$'\t' read -r session win_id type _rest; do
        if [ "$type" = "main" ] && echo "$focused_ids" | grep -qxF "$win_id"; then
            echo "$win_id"
            return 0
        fi
    done < "$REGISTRY"
    return 1
}

# ── AI ウィンドウの N 番目をフォーカス（X 座標昇順）───────────────────────────
# macOS Accessibility API で ": AI" で終わるウィンドウを X 座標昇順ソートし
# N 番目を AXRaise + activate する（AeroSpace window-id と独立）
focus_ai_window() {
    local n=$1
    osascript 2>/dev/null << APPLESCRIPT
set targetN to $n
set aiWins to {}

tell application "System Events"
    tell process "Ghostty"
        repeat with w in every window
            if title of w ends with ": AI" then
                set wpos to position of w
                set end of aiWins to {title of w, item 1 of wpos as real, id of w}
            end if
        end repeat
    end tell
end tell

if (count of aiWins) = 0 then
    display notification "No AI windows in current workspace." with title "focus-tool"
    return
end if

-- Bubble sort by X position (昇順)
set cnt to count of aiWins
repeat with i from 1 to cnt - 1
    repeat with j from 1 to cnt - i
        if item 2 of item j of aiWins > item 2 of item (j + 1) of aiWins then
            set tmp to item j of aiWins
            set item j of aiWins to item (j + 1) of aiWins
            set item (j + 1) of aiWins to tmp
        end if
    end repeat
end repeat

if cnt < targetN then
    display notification "AI window #" & targetN & " not found (" & cnt & " windows)." with title "focus-tool"
    return
end if

set targetId to item 3 of item targetN of aiWins
tell application "System Events"
    tell process "Ghostty"
        repeat with w in every window
            if id of w is targetId then
                perform action "AXRaise" of w
                exit repeat
            end if
        end repeat
    end tell
end tell
tell application "Ghostty" to activate
APPLESCRIPT
}

# ── Zellij タブ切替キー送信 ────────────────────────────────────────────────────
send_cmd_key() {
    local key=$1
    osascript -e "
tell application \"System Events\"
    keystroke \"$key\" using {command down}
end tell"
}

# ── Editor / Git / Shell テレポート ──────────────────────────────────────────
teleport_to_tool() {
    local tool=$1
    local win_id
    if ! win_id=$(get_main_window_id); then
        osascript -e 'display notification "No main window registered. Run: zj <project>" with title "focus-tool"'
        return 1
    fi
    "$AEROSPACE" focus --window-id "$win_id"
    sleep 0.1
    case "$tool" in
        editor) send_cmd_key "e" ;;
        git)    send_cmd_key "g" ;;
        shell)  send_cmd_key "s" ;;
    esac
}

# ── 新しい AI ウィンドウを起動 ────────────────────────────────────────────────
launch_new_ai() {
    local main_win_id
    if ! main_win_id=$(get_main_window_id); then
        osascript -e 'display notification "Cannot detect current project. Run: zj <project>" with title "focus-tool"'
        return 1
    fi

    # レジストリからプロジェクト名を取得
    local project
    project=$(awk -F'\t' -v id="$main_win_id" '$2 == id && $3 == "main" { print $1; exit }' "$REGISTRY" 2>/dev/null)
    if [ -z "$project" ]; then
        osascript -e 'display notification "Could not determine project from main window." with title "focus-tool"'
        return 1
    fi

    # プロジェクトディレクトリ（~/dev/project が優先）
    local project_dir="$HOME/dev/$project"
    [ -d "$project_dir" ] || project_dir="$HOME"

    # 現在 WS を記録
    local current_ws
    current_ws=$("$AEROSPACE" list-workspaces --focused 2>/dev/null | head -1)

    # 重複しない AI セッション名を決定
    local ai_session i=1
    while true; do
        ai_session="${project}-ai-${i}"
        if ! zellij list-sessions --short 2>/dev/null | tr ' ' '\n' | grep -qxF "$ai_session"; then
            break
        fi
        i=$((i + 1))
    done

    # Zellij レイアウト（pane name でタイトルを固定 → AeroSpace がタイトルマッチできる）
    local layout_file="/tmp/zellij-ai-layout-${ai_session}.kdl"
    cat > "$layout_file" << LAYOUT
layout {
    pane name="${project}: AI" cwd="${project_dir}" {
    }
}
LAYOUT

    # Ghostty 起動スクリプト
    local launcher="/tmp/ghostty-ai-launcher-${ai_session}.sh"
    cat > "$launcher" << LAUNCHER
#!/bin/bash
export PATH="/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:\$PATH"
exec /etc/profiles/per-user/${USER}/bin/fish -c "
    zellij --session '${ai_session}' --layout '${layout_file}'"
LAUNCHER
    chmod +x "$launcher"

    # 既存 Ghostty ウィンドウ ID を記録（差分で新ウィンドウを特定）
    local before_ids
    before_ids=$("$AEROSPACE" list-windows --all \
        --format "%{window-id}\t%{app-bundle-id}" 2>/dev/null \
        | awk -F'\t' -v b="$GHOSTTY_BUNDLE" '$2 == b { print $1 }' | sort)

    # 新しい Ghostty ウィンドウを開く（open -na は新インスタンス起動）
    open -na Ghostty.app --args --command="$launcher"

    # 新ウィンドウが AeroSpace に認識されるまでポーリング（最大 8 秒）
    local new_win_id=""
    for _i in {1..16}; do
        sleep 0.5
        local after_ids
        after_ids=$("$AEROSPACE" list-windows --all \
            --format "%{window-id}\t%{app-bundle-id}" 2>/dev/null \
            | awk -F'\t' -v b="$GHOSTTY_BUNDLE" '$2 == b { print $1 }' | sort)
        new_win_id=$(comm -13 <(echo "$before_ids") <(echo "$after_ids") | head -1)
        [ -n "$new_win_id" ] && break
    done

    if [ -z "$new_win_id" ]; then
        osascript -e 'display notification "Could not detect new Ghostty window." with title "focus-tool"'
        return 1
    fi

    # 現在 WS に移動してフォーカス
    "$AEROSPACE" move-node-to-workspace --window-id "$new_win_id" "$current_ws"
    "$AEROSPACE" focus --window-id "$new_win_id"

    # テンポラリファイルを遅延クリーンアップ（Zellij 起動後）
    ( sleep 10 && rm -f "$launcher" "$layout_file" ) &
}

# ── プロジェクトピッカー ────────────────────────────────────────────────────────
project_picker() {
    # 既存 Zellij セッション（AI セッションを除外）
    local sessions
    sessions=$(zellij list-sessions --short 2>/dev/null \
        | tr ' ' '\n' | grep -v -- '-ai-[0-9]' | grep -v '^$' | sort || true)

    # ~/dev/ 以下のディレクトリでセッションのないもの
    local dev_dirs=""
    if [ -d "$HOME/dev" ]; then
        dev_dirs=$(ls -d "$HOME/dev"/*/ 2>/dev/null \
            | xargs -I{} basename {} \
            | grep -vxF "$sessions" \
            | sed 's/^/📁 /' || true)
    fi

    local session_items
    session_items=$(echo "$sessions" | grep -v '^$' | sed 's/^/⚡ /' || true)

    local all_items
    all_items=$(printf "%s\n%s" "$session_items" "$dev_dirs" | grep -v '^$')

    if [ -z "$all_items" ]; then
        osascript -e 'display notification "No projects found in ~/dev/." with title "project-picker"'
        return 0
    fi

    # osascript ダイアログ（TTY 不要・exec-and-forget から動作可能）
    local choice
    choice=$(osascript << APPLESCRIPT 2>/dev/null
set allItems to paragraphs of "$all_items"
set choice to choose from list allItems with prompt "プロジェクトを選択:" default items {item 1 of allItems} OK button name "開く" cancel button name "キャンセル"
if choice is false then return ""
return item 1 of choice
APPLESCRIPT
    )
    [ -z "$choice" ] && return 0

    if [[ "$choice" == ⚡* ]]; then
        local session_name="${choice#⚡ }"
        _open_existing_session "$session_name"
    else
        local dir_name="${choice#📁 }"
        _open_new_project_dir "$dir_name"
    fi
}

# 既存セッションを持つウィンドウを探してフォーカス（なければ新ウィンドウ）
_open_existing_session() {
    local session_name=$1
    local win_id
    win_id=$(awk -F'\t' -v s="$session_name" '$1 == s && $3 == "main" { print $2; exit }' "$REGISTRY" 2>/dev/null)
    if [ -n "$win_id" ]; then
        local ws
        ws=$("$AEROSPACE" list-windows --all \
            --format "%{window-id}\t%{workspace}" 2>/dev/null \
            | awk -F'\t' -v id="$win_id" '$1 == id { print $2; exit }')
        if [ -n "$ws" ]; then
            "$AEROSPACE" workspace "$ws"
            "$AEROSPACE" focus --window-id "$win_id"
            return 0
        fi
    fi
    # ウィンドウなし: 現在 WS に新ウィンドウを開いてセッションにアタッチ
    _spawn_window_for_session "$session_name"
}

# 新ウィンドウを開いて既存 Zellij セッションにアタッチ
_spawn_window_for_session() {
    local session_name=$1
    local current_ws
    current_ws=$("$AEROSPACE" list-workspaces --focused 2>/dev/null | head -1)

    local launcher="/tmp/ghostty-session-${session_name}.sh"
    cat > "$launcher" << LAUNCHER
#!/bin/bash
export PATH="/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:\$PATH"
exec /etc/profiles/per-user/${USER}/bin/fish -c "zj '${session_name}'"
LAUNCHER
    chmod +x "$launcher"

    local before_ids
    before_ids=$("$AEROSPACE" list-windows --all \
        --format "%{window-id}\t%{app-bundle-id}" 2>/dev/null \
        | awk -F'\t' -v b="$GHOSTTY_BUNDLE" '$2 == b { print $1 }' | sort)

    open -na Ghostty.app --args --command="$launcher"

    local new_win_id=""
    for _i in {1..16}; do
        sleep 0.5
        local after_ids
        after_ids=$("$AEROSPACE" list-windows --all \
            --format "%{window-id}\t%{app-bundle-id}" 2>/dev/null \
            | awk -F'\t' -v b="$GHOSTTY_BUNDLE" '$2 == b { print $1 }' | sort)
        new_win_id=$(comm -13 <(echo "$before_ids") <(echo "$after_ids") | head -1)
        [ -n "$new_win_id" ] && break
    done

    [ -z "$new_win_id" ] && return 1
    "$AEROSPACE" move-node-to-workspace --window-id "$new_win_id" "$current_ws"
    "$AEROSPACE" focus --window-id "$new_win_id"
    ( sleep 5 && rm -f "$launcher" ) &
}

# 未開封ディレクトリを空き WS で開く
_open_new_project_dir() {
    local dir_name=$1
    local project_dir="$HOME/dev/$dir_name"
    [ -d "$project_dir" ] || {
        osascript -e "display notification \"Directory not found: ~/dev/$dir_name\" with title \"project-picker\""
        return 1
    }

    # 空き WS を探す（E → 1~9 の順）
    local target_ws=""
    for ws in E 1 2 3 4 5 6 7 8 9; do
        local ws_wins
        ws_wins=$("$AEROSPACE" list-windows --workspace "$ws" 2>/dev/null | wc -l | tr -d ' ')
        if [ "$ws_wins" = "0" ]; then
            target_ws="$ws"
            break
        fi
    done

    if [ -z "$target_ws" ]; then
        osascript -e 'display notification "No empty workspace available (E, 1-9 are all occupied)." with title "project-picker"'
        return 1
    fi

    local launcher="/tmp/ghostty-new-project-${dir_name}.sh"
    cat > "$launcher" << LAUNCHER
#!/bin/bash
export PATH="/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:\$PATH"
exec /etc/profiles/per-user/${USER}/bin/fish -c "zj '${dir_name}'"
LAUNCHER
    chmod +x "$launcher"

    local before_ids
    before_ids=$("$AEROSPACE" list-windows --all \
        --format "%{window-id}\t%{app-bundle-id}" 2>/dev/null \
        | awk -F'\t' -v b="$GHOSTTY_BUNDLE" '$2 == b { print $1 }' | sort)

    open -na Ghostty.app --args --command="$launcher"

    local new_win_id=""
    for _i in {1..16}; do
        sleep 0.5
        local after_ids
        after_ids=$("$AEROSPACE" list-windows --all \
            --format "%{window-id}\t%{app-bundle-id}" 2>/dev/null \
            | awk -F'\t' -v b="$GHOSTTY_BUNDLE" '$2 == b { print $1 }' | sort)
        new_win_id=$(comm -13 <(echo "$before_ids") <(echo "$after_ids") | head -1)
        [ -n "$new_win_id" ] && break
    done

    [ -z "$new_win_id" ] && {
        osascript -e 'display notification "Could not detect new Ghostty window." with title "project-picker"'
        return 1
    }
    "$AEROSPACE" move-node-to-workspace --window-id "$new_win_id" "$target_ws"
    "$AEROSPACE" workspace "$target_ws"
    "$AEROSPACE" focus --window-id "$new_win_id"
    ( sleep 5 && rm -f "$launcher" ) &
}

# ── メイン ─────────────────────────────────────────────────────────────────────
cmd="${1:-}"
case "$cmd" in
    editor) teleport_to_tool editor ;;
    git)    teleport_to_tool git ;;
    shell)  teleport_to_tool shell ;;
    ai)
        n="${2:-1}"
        focus_ai_window "$n"
        ;;
    ai-new)
        launch_new_ai
        ;;
    project-picker)
        project_picker
        ;;
    *)
        echo "Usage: focus-tool editor|git|shell|ai N|ai-new|project-picker" >&2
        exit 1
        ;;
esac
