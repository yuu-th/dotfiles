# OmniWM プロファイル自動切替
# モニタ枚数を検出し、変化があったときだけ TOML を差替えて OmniWM を再起動する。
# 環境変数（default.nix から注入）:
#   OMNIWMCTL    : omniwmctl のフルパス
#   TWO_TOML     : 2枚モニタ用 settings.toml の nix store パス
#   TRIPLE_TOML  : 3枚モニタ用
#   QUAD_TOML    : 4枚モニタ用
#   LAUNCHD_LABEL: OmniWM launchd ラベル
#   JQ           : jq のフルパス
set -euo pipefail

# OmniWM 起動中なら IPC で正確に取得、起動前は system_profiler でフォールバック
if pgrep -x OmniWM > /dev/null 2>&1 \
  && [ -S "$HOME/Library/Caches/com.barut.OmniWM/ipc.sock" ]; then
  COUNT=$("$OMNIWMCTL" query displays --format json 2>/dev/null \
    | "$JQ" '.result.payload.displays | length' || echo "0")
else
  COUNT=$(/usr/sbin/system_profiler SPDisplaysDataType 2>/dev/null \
    | grep -c "Resolution:" || true)
fi

STATE="$HOME/.local/share/omniwm/monitor-count"
PREV=$(cat "$STATE" 2>/dev/null || echo "")

[ "$COUNT" = "$PREV" ] && exit 0

case "$COUNT" in
  1) PROFILE="$ONE_TOML" ;;
  2) PROFILE="$TWO_TOML" ;;
  3) PROFILE="$TRIPLE_TOML" ;;
  4) PROFILE="$QUAD_TOML" ;;
  # 5枚以上: 4枚プロファイルにフォールバック（main + secondary 配分）
  [5-9]) PROFILE="$QUAD_TOML" ;;
  *)
    # 0 or detection failure: 何もしない
    exit 0
    ;;
esac

mkdir -p "$(dirname "$STATE")" "$HOME/.config/omniwm"
LOG="$HOME/.local/share/omniwm/switch-profile.log"

# ── 名前なしモニタの displayId placeholder (999000) を実値に置換 ──────────
# OmniWM の specificDisplay は実 displayId が必要だが、Nix ビルド時には不明なため
# system_profiler から runtime に取得して sed で書き換える。
DISPLAYS_JSON=$(/usr/sbin/system_profiler SPDisplaysDataType -json 2>/dev/null || echo '{}')

# まず名前なしモニタを探す（macOS 内部名 "spdisplays_display" = EDID name 無し）
UNNAMED_ID=$(echo "$DISPLAYS_JSON" | "$JQ" -r '
  [.SPDisplaysDataType[]?.spdisplays_ndrvs[]?
   | select(._name == "spdisplays_display")
   | ._spdisplays_displayID
   | tonumber]
  | .[0] // empty
' 2>/dev/null || true)

UNNAMED_SOURCE="real"
if [ -z "$UNNAMED_ID" ]; then
  # 名前なしモニタ不在 → main display の ID にフォールバック
  # （unnamedDisplay 指定の WS が main に集約される。triple/quad プロファイルが
  #  選ばれたが実は全モニタ named な構成、というエッジケース対応）
  UNNAMED_ID=$(echo "$DISPLAYS_JSON" | "$JQ" -r '
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
echo "$COUNT" > "$STATE"

# ログ出力（debug 用、ファイル肥大化を避けるため最新だけ残す）
{
  echo "$(date '+%Y-%m-%d %H:%M:%S') count=$COUNT profile=$PROFILE unnamed_id=$UNNAMED_ID source=$UNNAMED_SOURCE"
} > "$LOG" 2>/dev/null || true

# OmniWM は設定ホットリロード未対応 → プロセス再起動が必要
if pgrep -x OmniWM > /dev/null 2>&1; then
  /bin/launchctl kickstart -k "gui/$UID/$LAUNCHD_LABEL" 2>/dev/null || true
fi
