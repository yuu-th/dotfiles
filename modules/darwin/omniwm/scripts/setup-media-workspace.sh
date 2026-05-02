# メディアワークスペース自動構築 (AeroSpace の setupMediaWorkspace 相当)
# WS M に Calendar / Spotify / Discord を起動し、Spotify+Discord をタブ統合、
# Calendar を右端に配置する。
#
# 環境変数 (default.nix から注入):
#   OMNIWMCTL : omniwmctl のフルパス
#   JQ        : jq のフルパス
#
# 設計メモ:
# - window opaque-id は OmniWM 再起動を跨ぐと無効化される (stale_window_id)
#   ため、各ステップで bundleId から動的に再取得する。
# - command 失敗時は黙過せず、ログに出して次へ進む（デバッグ性向上）。
set -euo pipefail

# ── 0. ヘルパ ──────────────────────────────────────────────────────────────
ws_focus_M() { "$OMNIWMCTL" workspace focus-name M > /dev/null; }

# bundleId から WS M 内の最新 window opaque-id を取得（stale 回避のため毎回呼ぶ）
get_wid() {
  "$OMNIWMCTL" query windows --workspace M --format json 2>/dev/null \
    | "$JQ" -r --arg b "$1" '.result.payload.windows[]? | select(.bundleId == $b) | .id' \
    | head -n1
}

step() { echo "[setup-media] $*" >&2; }

# ── 1. WS M に移動 + 必要アプリを起動 ─────────────────────────────────────
ws_focus_M
/usr/bin/open -a Calendar
/usr/bin/open -a Spotify
/usr/bin/open -a Discord

# ── 2. 3 つのウィンドウが WS M に揃うまで待機（最大 10 秒）──────────────
step "waiting for windows..."
for _ in $(seq 1 20); do
  WINS=$("$OMNIWMCTL" query windows --workspace M --format json 2>/dev/null) || WINS=""
  if [ -n "$WINS" ] && "$JQ" -e '
    [.result.payload.windows[]?.bundleId] as $b
    | ($b | contains(["com.apple.iCal"]))
      and ($b | contains(["com.spotify.client"]))
      and ($b | contains(["com.hnc.Discord"]))
  ' <<< "$WINS" > /dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
sleep 1

# ── 3. 最初の存在チェック ────────────────────────────────────────────────
WID_CAL=$(get_wid "com.apple.iCal")
WID_SPO=$(get_wid "com.spotify.client")
WID_DIS=$(get_wid "com.hnc.Discord")
if [ -z "$WID_CAL" ] || [ -z "$WID_SPO" ] || [ -z "$WID_DIS" ]; then
  echo "[setup-media] error: not all required apps appeared in workspace M" >&2
  echo "  Calendar=$WID_CAL Spotify=$WID_SPO Discord=$WID_DIS" >&2
  exit 1
fi

# ── 4. Discord を Spotify の column に consume（命名空間の都度再取得）─────
# move <left|right> は OmniWM 内部で .consumeWindow 効果を持ち、隣 column の
# ウィンドウを取り込む。order が起動順依存なので left を先に試す。
WID_DIS=$(get_wid "com.hnc.Discord")
if [ -n "$WID_DIS" ]; then
  step "consume Discord into Spotify's column"
  "$OMNIWMCTL" window focus "$WID_DIS" > /dev/null || true
  sleep 0.3
  "$OMNIWMCTL" command move left > /dev/null 2>&1 || \
    "$OMNIWMCTL" command move right > /dev/null 2>&1 || \
    step "  warn: move left/right both failed"
  sleep 0.3
fi

# ── 5. その column を tabbed 化 (AeroSpace の "layout accordion" 相当) ───
WID_SPO=$(get_wid "com.spotify.client")
if [ -n "$WID_SPO" ]; then
  step "toggle column tabbed (Spotify+Discord)"
  "$OMNIWMCTL" window focus "$WID_SPO" > /dev/null || true
  sleep 0.3
  "$OMNIWMCTL" command toggle-column-tabbed > /dev/null 2>&1 || \
    step "  warn: toggle-column-tabbed failed"
  sleep 0.3
fi

# ── 6. Calendar を右端に寄せて幅を縮小 ───────────────────────────────────
WID_CAL=$(get_wid "com.apple.iCal")
if [ -n "$WID_CAL" ]; then
  step "place Calendar on the right and narrow it"
  "$OMNIWMCTL" window focus "$WID_CAL" > /dev/null || true
  sleep 0.3
  for _ in 1 2 3; do
    "$OMNIWMCTL" command move-column right > /dev/null 2>&1 || true
  done
  sleep 0.3
  "$OMNIWMCTL" command cycle-column-width backward > /dev/null 2>&1 || true
fi

# ── 7. Spotify を最前面（フォーカス）に ──────────────────────────────────
WID_SPO=$(get_wid "com.spotify.client")
if [ -n "$WID_SPO" ]; then
  "$OMNIWMCTL" window focus "$WID_SPO" > /dev/null || true
fi

# ── 8. 最終整合性チェック（stale 黙過の予防） ─────────────────────────────
FINAL=$("$OMNIWMCTL" query windows --workspace M --format json 2>/dev/null \
  | "$JQ" '[.result.payload.windows[]?.bundleId] | length' || echo 0)
if [ "$FINAL" -lt 3 ]; then
  echo "[setup-media] warn: workspace M has only $FINAL windows after layout" >&2
fi

step "done."
