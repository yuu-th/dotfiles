# OmniWM 起動時 one-shot ウィンドウ整列スクリプト
#
# 役割:
#   起動完了直後に既存ウィンドウを WS_MAP_JSON に従って正しい WS にパパパッと整列。
#   起動時 1 回だけ走る（その後の新ウィンドウや手動移動には介入しない）。
#
# 環境変数 (default.nix から注入):
#   OMNIWMCTL    : omniwmctl のフルパス
#   JQ           : jq のフルパス
#   WS_MAP_JSON  : { "<bundleId>": "<rawID>", ... } の JSON 文字列
#
# 動作:
#   1. IPC ready 待ち（caller も既に待ってるが二重保険）
#   2. 整列前のフォーカスを記憶
#   3. 全 window 列挙 → bundleId をマップで lookup → 該当があれば
#      window focus + command move-to-workspace で移動
#   4. 元のフォーカスを復元
#
# ログ: ~/.local/share/omniwm/startup-sort.log
set -euo pipefail

LOG="$HOME/.local/share/omniwm/startup-sort.log"
mkdir -p "$(dirname "$LOG")"

log() { echo "$(date '+%H:%M:%S') $*" >> "$LOG"; }

# ── 1. IPC ready 待ち ────────────────────────────────────────────────────
for _ in $(seq 1 50); do
  if "$OMNIWMCTL" ping > /dev/null 2>&1; then break; fi
  /bin/sleep 0.2
done

if ! "$OMNIWMCTL" ping > /dev/null 2>&1; then
  log "ERROR: IPC not ready after 10s; aborting"
  exit 1
fi

log "started"

# ── 2. 元のフォーカス記憶 ──────────────────────────────────────────────
ORIG_WID=$("$OMNIWMCTL" query focused-window --format json 2>/dev/null \
  | "$JQ" -r '.result.payload.window.id // empty' 2>/dev/null || true)

# ── 3. 全 window を列挙 → 整列 ─────────────────────────────────────────
WINDOWS_JSON=$("$OMNIWMCTL" query windows --format json 2>/dev/null || echo '{}')

MOVED=0
SKIPPED=0

# bundleId / id / current rawName を 1 行ずつ TSV で吐く（変数を while 内で保持するため
# プロセス置換 < <(...) を使う。pipe だと subshell でカウンタが消える）
while IFS=$'\t' read -r BUNDLE WID CUR_WS; do
  [ -z "$BUNDLE" ] || [ -z "$WID" ] && continue

  TARGET=$(printf '%s' "$WS_MAP_JSON" | "$JQ" -r --arg b "$BUNDLE" '.[$b] // empty')

  if [ -z "$TARGET" ] || [ "$TARGET" = "$CUR_WS" ]; then
    SKIPPED=$((SKIPPED + 1))
    continue
  fi

  # focus → move
  if "$OMNIWMCTL" window focus "$WID" > /dev/null 2>&1; then
    /bin/sleep 0.05
    if "$OMNIWMCTL" command move-to-workspace "$TARGET" > /dev/null 2>&1; then
      MOVED=$((MOVED + 1))
      log "  moved: $BUNDLE [$WID] $CUR_WS -> $TARGET"
    else
      log "  ERROR: move-to-workspace failed: $BUNDLE -> $TARGET"
    fi
  else
    log "  ERROR: window focus failed: $BUNDLE [$WID]"
  fi
done < <(echo "$WINDOWS_JSON" | "$JQ" -r '
  .result.payload.windows[]?
  | [
      (.app.bundleId // ""),
      (.id // ""),
      (.workspace.rawName // "")
    ]
  | @tsv
')

# ── 4. 元のフォーカス復元 ──────────────────────────────────────────────
if [ -n "$ORIG_WID" ]; then
  "$OMNIWMCTL" window focus "$ORIG_WID" > /dev/null 2>&1 || true
fi

log "done: moved=$MOVED skipped=$SKIPPED"
