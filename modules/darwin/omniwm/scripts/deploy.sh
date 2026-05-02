# OmniWM 設定 deploy & runtime resolution（v3）
#
# 役割:
# 1. nix が生成した settings.toml を runtime にロード
# 2. specificDisplay の placeholder を **実モニタ ID** に解決
#    - displayId = 0 + name = "X"  → 名前 X のモニタの実 ID を system_profiler から取得
#    - displayId = 0 + name = ""   → 名前なしモニタの実 ID を取得
#    - 解決失敗（モニタ未接続等）→ monitorAssignment を "secondary" に置換（crash 回避）
# 3. ~/.config/omniwm/settings.toml に書き込み（前回と同一なら no-op）
# 4. 必要時のみ OmniWM kickstart
#
# 環境変数:
#   SETTINGS_TOML : nix が生成した settings.toml の store パス
#   LAUNCHD_LABEL : OmniWM launchd ラベル
#   JQ            : jq のフルパス
set -euo pipefail

DEPLOYED="$HOME/.config/omniwm/settings.toml"
SOURCE_STAMP="$HOME/.local/share/omniwm/last-deployed-source"
LOG="$HOME/.local/share/omniwm/deploy.log"
mkdir -p "$(dirname "$DEPLOYED")" "$(dirname "$LOG")"

# ── 早期 exit: nix store パスが同じなら何もしない（true idempotence）─────
PREV_SOURCE=$(cat "$SOURCE_STAMP" 2>/dev/null || echo "")

if [ "$SETTINGS_TOML" = "$PREV_SOURCE" ] && [ -f "$DEPLOYED" ]; then
  if ! pgrep -x OmniWM > /dev/null 2>&1; then
    /bin/launchctl kickstart "gui/$UID/$LAUNCHD_LABEL" 2>/dev/null || true
    echo "$(date '+%H:%M:%S') noop: kickstart (OmniWM was down)" >> "$LOG"
  fi
  exit 0
fi

# ── runtime にディスプレイ name → displayId マップを構築 ─────────────────
# system_profiler は OmniWM 起動前でも使える
# displayID は decimal の場合と hex 文字列の場合があるので Python でパースする
SP_JSON=$(/usr/sbin/system_profiler SPDisplaysDataType -json 2>/dev/null || echo '{}')

DISPLAYS=$(/usr/bin/python3 -c '
import json, sys
data = json.loads(sys.argv[1])
out = []
for sp in data.get("SPDisplaysDataType", []):
    for d in sp.get("spdisplays_ndrvs", []):
        name = d.get("_name", "")
        raw = d.get("_spdisplays_displayID", "0")
        # 数値 / 16進文字列どちらにも対応
        try:
            did = int(raw)
        except (TypeError, ValueError):
            try:
                did = int(str(raw), 16)
            except (TypeError, ValueError):
                did = 0
        out.append({"name": name, "id": did})
print(json.dumps(out))
' "$SP_JSON" 2>/dev/null || echo '[]')

# ── Python で TOML を resolve / fallback 書き換え ────────────────────────
# Python の標準 tomllib（読み）は使えるが書きが無い。手動 line-based 編集。
# nix store からコピーした settings.toml は read-only になるため、書き込み前に
# 既存ファイルを削除して新規作成する。
/bin/rm -f "$DEPLOYED"
/usr/bin/python3 << PYEOF > "$DEPLOYED"
import json, sys, re

with open("$SETTINGS_TOML") as f:
    text = f.read()

displays = json.loads('''$DISPLAYS''')

# 接続中ディスプレイ: name → id（macOS の "spdisplays_display" は EDID name 無し）
named_ids = {}      # 名前付き: "HP V27ie G5" → 105
unnamed_id = None   # 名前なし: 最初の "spdisplays_display" の id
for d in displays:
    if d.get("name") == "spdisplays_display":
        if unnamed_id is None:
            unnamed_id = d["id"]
    else:
        named_ids[d["name"]] = d["id"]

# 行ベースで [workspaces.monitorAssignment] / [...output] ブロックを処理
out_lines = []
i = 0
lines = text.split("\n")
while i < len(lines):
    line = lines[i]
    # workspaces.monitorAssignment ブロック検出
    if line.strip() == "[workspaces.monitorAssignment]":
        # 次行 type = "...."
        if i+1 < len(lines) and lines[i+1].strip().startswith("type ="):
            type_line = lines[i+1]
            type_val = type_line.split('=',1)[1].strip().strip('"')
            if type_val == "specificDisplay":
                # output ブロックを探す（数行後にある）
                # ヘッダ + output ブロックを丸ごと再構成
                # まず後続を読み出す
                block_lines = [line, type_line]
                j = i+2
                # 空行・他のキー・[workspaces.monitorAssignment.output] を探す
                output_block_start = None
                while j < len(lines) and not lines[j].strip().startswith("[[") \
                        and not (lines[j].strip().startswith("[") and not lines[j].strip().startswith("[workspaces.monitorAssignment")):
                    if lines[j].strip() == "[workspaces.monitorAssignment.output]":
                        output_block_start = j
                        break
                    block_lines.append(lines[j])
                    j += 1

                if output_block_start is None:
                    # output ブロック無し（不正形）→ そのまま出力
                    out_lines.extend(block_lines)
                    i = j
                    continue

                # output ブロックから displayId / name を読む
                output_displayid = None
                output_name = None
                k = output_block_start + 1
                output_extra = []
                while k < len(lines):
                    s = lines[k].strip()
                    if s.startswith("[[") or (s.startswith("[") and not s.startswith("[workspaces.monitorAssignment.output")):
                        break
                    if s.startswith("displayId"):
                        m = re.match(r'displayId\s*=\s*(-?\d+)', s)
                        if m: output_displayid = int(m.group(1))
                    elif s.startswith("name"):
                        m = re.match(r'name\s*=\s*"(.*)"', s)
                        if m: output_name = m.group(1)
                    elif s.startswith("displayUUID"):
                        output_extra.append(lines[k])
                    k += 1

                # 解決
                resolved_id = None
                if output_name == "":
                    # 名前なしモニタ
                    if unnamed_id is not None:
                        resolved_id = unnamed_id
                else:
                    if output_name in named_ids:
                        resolved_id = named_ids[output_name]

                if resolved_id is not None:
                    # 実 ID で specificDisplay を維持
                    out_lines.extend(block_lines)
                    out_lines.append("")
                    out_lines.append("[workspaces.monitorAssignment.output]")
                    for e in output_extra:
                        out_lines.append(e)
                    out_lines.append(f"displayId = {resolved_id}")
                    out_lines.append(f'name = "{output_name}"')
                else:
                    # 解決失敗 → secondary にフォールバック（crash 回避）
                    out_lines.append(line)
                    out_lines.append('type = "secondary"')
                    # block_lines 内の type 以外（コメント等）は保持
                    for bl in block_lines[2:]:
                        if bl.strip() and not bl.strip().startswith("type"):
                            out_lines.append(bl)
                # output ブロック以降にスキップ
                i = k
                continue
        # specificDisplay じゃない or パース失敗 → そのまま出力
        out_lines.append(line)
        i += 1
    else:
        out_lines.append(line)
        i += 1

sys.stdout.write("\n".join(out_lines))
PYEOF

# 書き込んだファイルを user-writable に（OmniWM が canonicalize で書き戻すため）
/bin/chmod 644 "$DEPLOYED"

# stamp 更新
echo "$SETTINGS_TOML" > "$SOURCE_STAMP"
NAMES=$(echo "$DISPLAYS" | "$JQ" -c '[.[].name]' 2>/dev/null || echo '[]')
echo "$(date '+%H:%M:%S') deployed: source=$SETTINGS_TOML displays=$NAMES" >> "$LOG"

# OmniWM 再起動（hot-reload 未対応）
if pgrep -x OmniWM > /dev/null 2>&1; then
  /bin/launchctl kickstart -k "gui/$UID/$LAUNCHD_LABEL" 2>/dev/null || true
  echo "$(date '+%H:%M:%S') kickstart -k (config source changed)" >> "$LOG"
fi
