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
  2) PROFILE="$TWO_TOML" ;;
  3) PROFILE="$TRIPLE_TOML" ;;
  4) PROFILE="$QUAD_TOML" ;;
  *) exit 0 ;;
esac

mkdir -p "$(dirname "$STATE")" "$HOME/.config/omniwm"
cp "$PROFILE" "$HOME/.config/omniwm/settings.toml"
echo "$COUNT" > "$STATE"

# OmniWM は設定ホットリロード未対応 → プロセス再起動が必要
if pgrep -x OmniWM > /dev/null 2>&1; then
  /bin/launchctl kickstart -k "gui/$UID/$LAUNCHD_LABEL" 2>/dev/null || true
fi
