# 名前指定の WS にフォーカスウィンドウを移動 + その WS にジャンプ
# (AeroSpace の `alt-shift-m = [ "move-node-to-workspace M" "workspace M" ]` 相当)
#
# Usage: omniwm-move-window-to-named-ws <workspace-name>
# 環境変数 (default.nix から注入):
#   OMNIWMCTL : omniwmctl のフルパス
#   JQ        : jq のフルパス
set -euo pipefail

WS="${1:-}"
[ -z "$WS" ] && { echo "usage: $0 <workspace>" >&2; exit 1; }

# WS の rawID を displayName / rawName のいずれか一致で動的解決。
# AeroSpace 由来の名前付き WS (M/B/E) は OmniWM 内部では rawID=10/11/12 に
# マップされ displayName が "M"/"B"/"E" になる。両方サポートする。
RAWID=$("$OMNIWMCTL" query workspaces --format json 2>/dev/null \
  | "$JQ" -r --arg n "$WS" '
      .result.payload.workspaces[]
      | select(.rawName == $n or .displayName == $n)
      | .rawName
    ' | head -n1)

if [ -z "$RAWID" ]; then
  echo "Error: workspace '$WS' not found" >&2
  exit 1
fi

"$OMNIWMCTL" command move-to-workspace "$RAWID" > /dev/null
"$OMNIWMCTL" workspace focus-name "$WS" > /dev/null
