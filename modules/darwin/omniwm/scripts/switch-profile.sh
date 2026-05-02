# OmniWM プロファイル自動切替
# モニタ枚数 + 配置 (geometry) の変化を検知して TOML を差替え、OmniWM を再起動する。
#
# 環境変数（default.nix から注入）:
#   OMNIWMCTL    : omniwmctl のフルパス
#   ONE_TOML     : 1枚モニタ用 settings.toml の nix store パス
#   TWO_TOML     : 2枚モニタ用
#   TRIPLE_TOML  : 3枚モニタ用
#   QUAD_TOML    : 4枚モニタ用
#   LAUNCHD_LABEL: OmniWM launchd ラベル
#   JQ           : jq のフルパス
set -euo pipefail

STATE="$HOME/.local/share/omniwm/monitor-fingerprint"
LOG="$HOME/.local/share/omniwm/switch-profile.log"
mkdir -p "$(dirname "$STATE")" "$HOME/.config/omniwm"

# ── ディスプレイ枚数と配置 fingerprint を取得 ────────────────────────────
# OmniWM 起動中なら IPC で正確な geometry を、起動前なら system_profiler でフォールバック。
if pgrep -x OmniWM > /dev/null 2>&1 \
  && [ -S "$HOME/Library/Caches/com.barut.OmniWM/ipc.sock" ]; then
  DISPLAY_LIST=$("$OMNIWMCTL" query displays --format json 2>/dev/null | "$JQ" -c '
    [.result.payload.displays[]
     | { id, name, x: .frame.x, y: .frame.y, w: .frame.width, h: .frame.height }]
    | sort_by(.id)
  ' 2>/dev/null || echo '[]')
  COUNT=$(echo "$DISPLAY_LIST" | "$JQ" 'length' 2>/dev/null || echo "0")
else
  # OmniWM 起動前: system_profiler ベース。X/Y は取れないが枚数 + 名前 + 解像度を fingerprint に
  SP_JSON=$(/usr/sbin/system_profiler SPDisplaysDataType -json 2>/dev/null || echo '{}')
  DISPLAY_LIST=$(echo "$SP_JSON" | "$JQ" -c '
    [.SPDisplaysDataType[]?.spdisplays_ndrvs[]?
     | { id: ._spdisplays_displayID, name: ._name, res: ._spdisplays_pixels }]
    | sort_by(.id)
  ' 2>/dev/null || echo '[]')
  COUNT=$(echo "$DISPLAY_LIST" | "$JQ" 'length' 2>/dev/null || echo "0")
fi

# 枚数 + 配置を統合した fingerprint で変化検出
FINGERPRINT="${COUNT}:$(echo "$DISPLAY_LIST" | /usr/bin/shasum -a 256 | cut -c1-16)"
PREV=$(cat "$STATE" 2>/dev/null || echo "")
[ "$FINGERPRINT" = "$PREV" ] && exit 0

# ── プロファイル選択 ─────────────────────────────────────────────────────
case "$COUNT" in
  1) PROFILE="$ONE_TOML" ;;
  2) PROFILE="$TWO_TOML" ;;
  3) PROFILE="$TRIPLE_TOML" ;;
  4) PROFILE="$QUAD_TOML" ;;
  [5-9]) PROFILE="$QUAD_TOML" ;;   # 5+ は quad にフォールバック
  *)
    # 0 / 検出失敗: 状態だけ記録して終了
    echo "$FINGERPRINT" > "$STATE"
    exit 0
    ;;
esac

# ── 名前なしモニタの displayId placeholder (999000) を実値に置換 ──────────
SP_JSON=$(/usr/sbin/system_profiler SPDisplaysDataType -json 2>/dev/null || echo '{}')

UNNAMED_ID=$(echo "$SP_JSON" | "$JQ" -r '
  [.SPDisplaysDataType[]?.spdisplays_ndrvs[]?
   | select(._name == "spdisplays_display")
   | ._spdisplays_displayID
   | tonumber]
  | .[0] // empty
' 2>/dev/null || true)

UNNAMED_SOURCE="real"
if [ -z "$UNNAMED_ID" ]; then
  # 名前なしモニタ不在 → main display の ID にフォールバック
  UNNAMED_ID=$(echo "$SP_JSON" | "$JQ" -r '
    [.SPDisplaysDataType[]?.spdisplays_ndrvs[]?
     | select(.spdisplays_main == "spdisplays_yes")
     | ._spdisplays_displayID
     | tonumber]
    | .[0] // 1
  ' 2>/dev/null || echo "1")
  UNNAMED_SOURCE="fallback-main"
fi

# placeholder を実値に置換しながら deploy
/usr/bin/sed "s/displayId = 999000/displayId = $UNNAMED_ID/g" "$PROFILE" \
  > "$HOME/.config/omniwm/settings.toml"
echo "$FINGERPRINT" > "$STATE"

# ── ログ出力 ─────────────────────────────────────────────────────────────
{
  echo "$(date '+%Y-%m-%d %H:%M:%S') count=$COUNT fingerprint=${FINGERPRINT##*:} profile=$PROFILE unnamed_id=$UNNAMED_ID source=$UNNAMED_SOURCE"
  echo "  displays: $DISPLAY_LIST"
} > "$LOG" 2>/dev/null || true

# ── OmniWM プロセス再起動（設定ホットリロード未対応のため）──────────────
if pgrep -x OmniWM > /dev/null 2>&1; then
  /bin/launchctl kickstart -k "gui/$UID/$LAUNCHD_LABEL" 2>/dev/null || true
fi
