# 方向ベース focus-monitor (AeroSpace の `focus-monitor left/right/up/down` 相当)
# OmniWM ネイティブは prev/next のみなので、displays geometry から方向を
# 解決して必要回数 prev/next を発行する。
#
# Usage: omniwm-focus-monitor-dir <left|right|up|down>
# 環境変数 (default.nix から注入):
#   OMNIWMCTL : omniwmctl のフルパス
#   JQ        : jq のフルパス
set -euo pipefail

DIR="${1:-}"
case "$DIR" in
  left|right|up|down) ;;
  *) echo "usage: $0 <left|right|up|down>" >&2; exit 1 ;;
esac

abs() { local n="${1#-}"; echo "$n"; }

DJSON=$("$OMNIWMCTL" query displays --format json 2>/dev/null) || exit 0
COUNT=$("$JQ" '.result.payload.displays | length' <<< "$DJSON")
[ "$COUNT" -le 1 ] && exit 0

# 現在フォーカス中のディスプレイの index を特定
CUR=$("$JQ" -r '
  .result.payload.displays
  | to_entries
  | map(select(.value.isCurrent == true))
  | .[0].key // empty
' <<< "$DJSON")
[ -z "$CUR" ] && exit 0

CX=$("$JQ" -r ".result.payload.displays[$CUR].frame | (.x + .width/2)" <<< "$DJSON")
CY=$("$JQ" -r ".result.payload.displays[$CUR].frame | (.y + .height/2)" <<< "$DJSON")
CX=${CX%.*}; CY=${CY%.*}  # 整数化

best_idx=""
best_dist=""
for i in $(seq 0 $((COUNT - 1))); do
  [ "$i" = "$CUR" ] && continue
  X=$("$JQ" -r ".result.payload.displays[$i].frame | (.x + .width/2)" <<< "$DJSON")
  Y=$("$JQ" -r ".result.payload.displays[$i].frame | (.y + .height/2)" <<< "$DJSON")
  X=${X%.*}; Y=${Y%.*}
  DX=$((X - CX))
  DY=$((Y - CY))
  ADX=$(abs "$DX"); ADY=$(abs "$DY")

  # 方向判定: 水平を優先（モニタが斜めに置かれた場合は左右に解決）
  # left/right は ADX>=ADY でヒット、up/down は ADY>ADX で厳格判定
  match=0
  case "$DIR" in
    left)  [ "$DX" -lt 0 ] && [ "$ADX" -ge "$ADY" ] && match=1 ;;
    right) [ "$DX" -gt 0 ] && [ "$ADX" -ge "$ADY" ] && match=1 ;;
    up)    [ "$DY" -lt 0 ] && [ "$ADY" -gt "$ADX" ] && match=1 ;;
    down)  [ "$DY" -gt 0 ] && [ "$ADY" -gt "$ADX" ] && match=1 ;;
  esac
  [ "$match" = "1" ] || continue

  dist=$((ADX + ADY))
  if [ -z "$best_dist" ] || [ "$dist" -lt "$best_dist" ]; then
    best_idx="$i"
    best_dist="$dist"
  fi
done

[ -z "$best_idx" ] && exit 0

# index 差分だけ prev/next を発行
diff=$((best_idx - CUR))
if [ "$diff" -gt 0 ]; then
  for _ in $(seq 1 "$diff"); do "$OMNIWMCTL" command focus-monitor next > /dev/null; done
elif [ "$diff" -lt 0 ]; then
  ndiff=$((0 - diff))
  for _ in $(seq 1 "$ndiff"); do "$OMNIWMCTL" command focus-monitor prev > /dev/null; done
fi
