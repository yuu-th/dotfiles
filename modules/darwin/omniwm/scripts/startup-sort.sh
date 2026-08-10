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

# ── 1b. window discovery 待ち ────────────────────────────────────────────
# ping が通っても OmniWM の window discovery が終わっているとは限らない。
# 実測で「IPC は応答するが query windows が空」の状態で走ってしまい、
# moved=0 skipped=0 で何もせず終了する事故が起きた。
# 最大 15 秒、窓が 1 つ見えるまで待つ。
# （本当に窓が無い状態での起動もあり得るので、見えなくても失敗にはしない）
for _ in $(seq 1 75); do
  COUNT=$("$OMNIWMCTL" query windows --format json 2>/dev/null \
    | "$JQ" -r '.result.payload.windows | length' 2>/dev/null || echo 0)
  case "$COUNT" in
    ''|*[!0-9]*) COUNT=0 ;;
  esac
  [ "$COUNT" -gt 0 ] && break
  /bin/sleep 0.2
done
log "  windows visible: ${COUNT:-0}"

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

  # focus → 確認 → move (race detection)
  #
  # 2 step (`window focus` → `command move-to-workspace`) の間に focus が
  # 変わると **別 window が move されてしまう** race を、 focus 確認 step で
  # 防ぐ。 (user 報告: 「関係 window が違う ws に移動」)
  #
  # cmd 選定: **window 単位**の `move-to-workspace` を使う。
  #
  # 以前は column 単位の `move-column-to-workspace` を使っていたが、
  # niri.visibleContainerCount = 1 / defaultContainerPrimarySpan = 1.0 にした結果
  # 「同じ WS に入った別アプリの窓が同一カラムに同居する」ようになり、
  # カラム単位で動かすと **無関係な窓を巻き込む** 事故が実測で発生した:
  #
  #   cmux   3 -> 10   … WS 10 に着地
  #   Spotify 10 -> 16  … Spotify のカラムに cmux が同居していて cmux も 16 へ流された
  #
  # 整列はあくまで「この窓をこの WS へ」なので、粒度も window に合わせる。
  # 意図して作った stack は ws sort の対象外構造とみなす（対象アプリは
  # workspace-assignment.nix に列挙したものだけ）。
  # 早期 success polling: focus が早く反映されれば即 move (平均 ~30-60ms)、
  # 遅延が出た場合のみ wait。 上限 0.5s に達したら focus race として skip。
  # 高速化のため query は --format json | jq の二重 process spawn を避けて短縮。
  if "$OMNIWMCTL" window focus "$WID" > /dev/null 2>&1; then
    MOVE_DONE=0
    DEADLINE=$(($(/bin/date +%s%N) + 500000000)) # +500ms (ns)
    while [ $(/bin/date +%s%N) -lt $DEADLINE ]; do
      /bin/sleep 0.03
      ACTUAL_FOCUSED=$("$OMNIWMCTL" query focused-window --format json 2>/dev/null \
        | "$JQ" -r '.result.payload.window.id // empty' 2>/dev/null || true)
      if [ "$ACTUAL_FOCUSED" = "$WID" ]; then
        if "$OMNIWMCTL" command move-to-workspace "$TARGET" > /dev/null 2>&1; then
          MOVED=$((MOVED + 1))
          log "  moved: $BUNDLE [$WID] $CUR_WS -> $TARGET"
        else
          log "  ERROR: move-to-workspace failed: $BUNDLE -> $TARGET"
        fi
        MOVE_DONE=1
        break
      fi
    done
    if [ $MOVE_DONE -eq 0 ]; then
      SKIPPED=$((SKIPPED + 1))
      log "  SKIP focus-race: $BUNDLE [$WID] (focus not stabilized in 500ms)"
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
