#!/usr/bin/env bash

# focus-tool.sh — Ghostty ウィンドウフォーカスツール
#
# 使い方:
#   focus-tool win N   現在の WS の Ghostty ウィンドウを X 座標昇順で並べ N 番目をフォーカス

set -uo pipefail

export PATH="/etc/profiles/per-user/${USER}/bin:/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:$PATH"

AEROSPACE="/run/current-system/sw/bin/aerospace"

focus_ghostty_window() {
    local n=$1

    # 現在 WS の全ウィンドウ ID を取得
    local current_ws ws_ids
    current_ws=$("$AEROSPACE" list-workspaces --focused 2>/dev/null | head -1)
    ws_ids=$("$AEROSPACE" list-windows --workspace "$current_ws" \
        --format "%{window-id}" 2>/dev/null)

    if [ -z "$ws_ids" ]; then
        osascript -e 'display notification "No windows in current workspace." with title "focus-tool"'
        return 0
    fi

    # AppleScript 用にカンマ区切りへ変換
    local ids_csv
    ids_csv=$(echo "$ws_ids" | tr '\n' ',' | sed 's/,$//')

    osascript 2>/dev/null <<APPLESCRIPT
set targetN to $n
set allowedIdList to {"${ids_csv//,/\",\"}"}
set allowedIds to {}
repeat with s in allowedIdList
    try
        set end of allowedIds to (s as integer)
    end try
end repeat

set ghosttyWins to {}
tell application "System Events"
    tell process "Ghostty"
        repeat with w in every window
            try
                set axId to (value of attribute "AXWindowID" of w) as integer
                set isAllowed to false
                repeat with aId in allowedIds
                    if axId = aId then
                        set isAllowed to true
                        exit repeat
                    end if
                end repeat
                if isAllowed then
                    set wpos to position of w
                    set end of ghosttyWins to {axId, item 1 of wpos as real, id of w}
                end if
            end try
        end repeat
    end tell
end tell

if (count of ghosttyWins) = 0 then
    display notification "No Ghostty windows in current workspace." with title "focus-tool"
    return
end if

-- X 座標昇順でソート（バブルソート）
set cnt to count of ghosttyWins
repeat with i from 1 to cnt - 1
    repeat with j from 1 to cnt - i
        if item 2 of item j of ghosttyWins > item 2 of item (j + 1) of ghosttyWins then
            set tmp to item j of ghosttyWins
            set item j of ghosttyWins to item (j + 1) of ghosttyWins
            set item (j + 1) of ghosttyWins to tmp
        end if
    end repeat
end repeat

if cnt < targetN then
    display notification "Window #" & targetN & " not found (" & cnt & " windows)." with title "focus-tool"
    return
end if

set targetId to item 3 of item targetN of ghosttyWins
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



# ── メイン ─────────────────────────────────────────────────────────────────────
cmd="${1:-}"
case "$cmd" in
    win|w|ai)
        n="${2:-1}"
        focus_ghostty_window "$n"
        ;;
    *)
        echo "Usage: focus-tool win N" >&2
        exit 1
        ;;
esac
