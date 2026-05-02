# 名前指定の WS にフォーカスウィンドウを移動 + その WS にジャンプ
# (AeroSpace の `alt-shift-m = [ "move-node-to-workspace M" "workspace M" ]` 相当)
#
# Usage: omniwm-move-window-to-named-ws <workspace-name>
# 環境変数 (default.nix から注入):
#   OMNIWMCTL : omniwmctl のフルパス
#   JQ        : jq のフルパス
#
# 設計メモ:
# - フォーカス window-id を最初に capture（後続操作で focus が逃げる前に）
# - move-to-workspace と focus-name の間に 100ms 待ち（IPC の async 反映猶予）
# - 移動後にフォーカスを取り戻す（window focus by id）
set -euo pipefail

WS="${1:-}"
[ -z "$WS" ] && { echo "usage: $0 <workspace>" >&2; exit 1; }

LOG="$HOME/.local/share/omniwm/move-window.log"
mkdir -p "$(dirname "$LOG")"

# ── 1. 現在のフォーカス window-id を確実に取得（move 前に） ────────────────
WID=$("$OMNIWMCTL" query focused-window --format json 2>/dev/null \
  | "$JQ" -r '.result.payload.window.id // empty' 2>/dev/null || true)

# ── 2. WS の rawID を query で動的解決 ────────────────────────────────────
RAWID=$("$OMNIWMCTL" query workspaces --format json 2>/dev/null \
  | "$JQ" -r --arg n "$WS" '
      .result.payload.workspaces[]
      | select(.rawName == $n or .displayName == $n)
      | .rawName
    ' | head -n1)

if [ -z "$RAWID" ]; then
  echo "$(date '+%H:%M:%S') ERROR: workspace '$WS' not found" >> "$LOG"
  exit 1
fi

# ── 3. 移動実行 ──────────────────────────────────────────────────────────
MOVE_OUT=$("$OMNIWMCTL" command move-to-workspace "$RAWID" 2>&1 || true)

# 反映猶予（IPC は async 処理されることがある）
/bin/sleep 0.15

# ── 4. 対象 WS にフォーカス切替（user の視点を移動先に追従） ───────────────
FOCUS_OUT=$("$OMNIWMCTL" workspace focus-name "$WS" 2>&1 || true)

/bin/sleep 0.05

# ── 5. 移動した window 自体をフォーカス（ベストエフォート、失敗しても OK）─
# Karabiner 起動時の focus が違うアプリを指してた場合、移動後もそのウィンドウを
# 再フォーカスして user の操作対象を取り戻す。
WIN_FOCUS_OUT="(skipped)"
if [ -n "$WID" ]; then
  WIN_FOCUS_OUT=$("$OMNIWMCTL" window focus "$WID" 2>&1 || true)
fi

# ── 6. ログ ─────────────────────────────────────────────────────────────
{
  echo "$(date '+%H:%M:%S') target=$WS rawID=$RAWID"
  echo "  before: WID=$WID"
  echo "  move-to-workspace: $MOVE_OUT"
  echo "  focus-name: $FOCUS_OUT"
  echo "  window focus: $WIN_FOCUS_OUT"
} >> "$LOG"
