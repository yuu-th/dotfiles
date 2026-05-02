# WS切替 + アプリ起動（AeroSpace の `[ "workspace M" "exec-and-forget open -a X" ]` 相当）
# alt-s/c/a 等のキーから Karabiner 経由で呼ばれる。
#
# Usage: omniwm-ws-launch <workspace-name> <app-name>
# 環境変数 (default.nix から注入):
#   OMNIWMCTL : omniwmctl のフルパス
set -euo pipefail

WS="${1:-}"
APP="${2:-}"
if [ -z "$WS" ] || [ -z "$APP" ]; then
  echo "usage: $0 <workspace> <app>" >&2
  exit 1
fi

"$OMNIWMCTL" workspace focus-name "$WS" > /dev/null
/usr/bin/open -a "$APP"
