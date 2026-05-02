# OmniWM 設定 deploy（v3.1: モニタ自動検出 + 名前付きプロファイル対応）
#
# 役割:
# 1. 現在のモニタ構成を system_profiler から取得
# 2. SELECTED_PROFILE が "auto" なら、PROFILE_MANIFEST から match 条件を満たす
#    プロファイルを選択（複数マッチしたら最も specific=requiredDisplays 多いやつ）
# 3. SELECTED_PROFILE が具体名なら、それを強制使用
# 4. 選んだプロファイルの TOML を読み込み、specificDisplay の displayId を
#    実際の値に解決（解決失敗 → secondary フォールバック）
# 5. ~/.config/omniwm/settings.toml に書き込み（前回と同一なら no-op）
# 6. 必要時のみ OmniWM kickstart
#
# 環境変数:
#   PROFILE_MANIFEST : プロファイル一覧 JSON（[{name, toml, match}]）
#   SELECTED_PROFILE : "auto" or プロファイル名
#   LAUNCHD_LABEL    : OmniWM launchd ラベル
#   JQ               : jq のフルパス
set -euo pipefail

DEPLOYED="$HOME/.config/omniwm/settings.toml"
SOURCE_STAMP="$HOME/.local/share/omniwm/last-deployed-source"
LOG="$HOME/.local/share/omniwm/deploy.log"
mkdir -p "$(dirname "$DEPLOYED")" "$(dirname "$LOG")"

# ── 1. 現在のモニタ一覧を取得 ────────────────────────────────────────────
SP_JSON=$(/usr/sbin/system_profiler SPDisplaysDataType -json 2>/dev/null || echo '{}')
DISPLAYS=$(/usr/bin/python3 -c '
import json, sys
data = json.loads(sys.argv[1])
out = []
for sp in data.get("SPDisplaysDataType", []):
    for d in sp.get("spdisplays_ndrvs", []):
        name = d.get("_name", "")
        raw = d.get("_spdisplays_displayID", "0")
        try: did = int(raw)
        except (TypeError, ValueError):
            try: did = int(str(raw), 16)
            except (TypeError, ValueError): did = 0
        out.append({"name": name, "id": did})
print(json.dumps(out))
' "$SP_JSON" 2>/dev/null || echo '[]')

# ── 2. プロファイル選択 ──────────────────────────────────────────────────
# SELECTED_PROFILE = "auto" の場合: PROFILE_MANIFEST から match 評価
# 具体名の場合: そのプロファイルを直接使用
SELECTED_NAME=""
SELECTED_TOML=""

SELECTED_NAME=$(/usr/bin/python3 -c '
import json, sys
manifest = json.loads(sys.argv[1])
selected = sys.argv[2]
displays = json.loads(sys.argv[3])

named = {d["name"] for d in displays if d["name"] != "spdisplays_display"}
has_unnamed = any(d["name"] == "spdisplays_display" for d in displays)
count = len(displays)

if selected != "auto":
    # 強制指定
    for p in manifest:
        if p["name"] == selected:
            print(p["name"] + "\t" + p["toml"])
            sys.exit(0)
    # 見つからない → default に fallback
    for p in manifest:
        if p["name"] == "default":
            print(p["name"] + "\t" + p["toml"])
            sys.exit(0)
    sys.exit(1)

# auto: match 条件評価。specific（requiredDisplays が多い）順に評価
def specificity(p):
    m = p.get("match") or {}
    return len(m.get("requiredDisplays", [])) + (1 if m.get("requireUnnamed") else 0) + (1 if m.get("monitorCount") else 0)

def matches(p):
    m = p.get("match") or {}
    if not m:
        # match なし = catch-all、最後に評価される
        return True
    for d in m.get("requiredDisplays", []):
        if d not in named:
            return False
    if m.get("requireUnnamed") and not has_unnamed:
        return False
    if m.get("monitorCount") and m["monitorCount"] != count:
        return False
    return True

# 順序: specificity 降順 → catch-all (default) は最後
sorted_profiles = sorted(manifest, key=lambda p: (-specificity(p), p["name"]))
for p in sorted_profiles:
    if matches(p):
        print(p["name"] + "\t" + p["toml"])
        sys.exit(0)

# match なしも fallback として default を選ぶ
for p in manifest:
    if p["name"] == "default":
        print(p["name"] + "\t" + p["toml"])
        sys.exit(0)
sys.exit(1)
' "$PROFILE_MANIFEST" "$SELECTED_PROFILE" "$DISPLAYS" 2>/dev/null || echo "default	/dev/null")

SELECTED_NAME=$(echo "$SELECTED_NAME" | cut -f1)
SELECTED_TOML=$(echo "$SELECTED_NAME" | cut -f1)
# 改めてタブ区切りで取得
LINE=$(/usr/bin/python3 -c '
import json, sys
manifest = json.loads(sys.argv[1])
selected = sys.argv[2]
displays = json.loads(sys.argv[3])
named = {d["name"] for d in displays if d["name"] != "spdisplays_display"}
has_unnamed = any(d["name"] == "spdisplays_display" for d in displays)
count = len(displays)

def specificity(p):
    m = p.get("match") or {}
    return len(m.get("requiredDisplays", [])) + (1 if m.get("requireUnnamed") else 0) + (1 if m.get("monitorCount") else 0)
def matches(p):
    m = p.get("match") or {}
    if not m: return True
    for d in m.get("requiredDisplays", []):
        if d not in named: return False
    if m.get("requireUnnamed") and not has_unnamed: return False
    if m.get("monitorCount") and m["monitorCount"] != count: return False
    return True

if selected != "auto":
    for p in manifest:
        if p["name"] == selected:
            print(p["name"] + "\t" + p["toml"]); sys.exit(0)
sorted_profiles = sorted(manifest, key=lambda p: (-specificity(p), p["name"]))
for p in sorted_profiles:
    if matches(p):
        print(p["name"] + "\t" + p["toml"]); sys.exit(0)
for p in manifest:
    if p["name"] == "default":
        print(p["name"] + "\t" + p["toml"]); sys.exit(0)
' "$PROFILE_MANIFEST" "$SELECTED_PROFILE" "$DISPLAYS" 2>/dev/null || echo -e "default\t")

SELECTED_NAME=$(echo "$LINE" | cut -f1)
SELECTED_TOML=$(echo "$LINE" | cut -f2)

if [ -z "$SELECTED_TOML" ] || [ ! -f "$SELECTED_TOML" ]; then
  echo "$(date '+%H:%M:%S') ERROR: profile selection failed (selected=$SELECTED_PROFILE)" >> "$LOG"
  exit 1
fi

# ── 3. 早期 exit: 同じ profile で前回 deploy 済みなら noop ────────────────
PREV_SOURCE=$(cat "$SOURCE_STAMP" 2>/dev/null || echo "")
if [ "$SELECTED_TOML" = "$PREV_SOURCE" ] && [ -f "$DEPLOYED" ]; then
  if ! pgrep -x OmniWM > /dev/null 2>&1; then
    /bin/launchctl kickstart "gui/$UID/$LAUNCHD_LABEL" 2>/dev/null || true
    echo "$(date '+%H:%M:%S') noop: kickstart (OmniWM was down) profile=$SELECTED_NAME" >> "$LOG"
  fi
  exit 0
fi

# ── 4. TOML を読み込み、specificDisplay の displayId を runtime resolve ───
/bin/rm -f "$DEPLOYED"
/usr/bin/python3 << PYEOF > "$DEPLOYED"
import json, sys, re

with open("$SELECTED_TOML") as f:
    text = f.read()

displays = json.loads('''$DISPLAYS''')
named_ids = {}
unnamed_id = None
for d in displays:
    if d.get("name") == "spdisplays_display":
        if unnamed_id is None: unnamed_id = d["id"]
    else:
        named_ids[d["name"]] = d["id"]

# Built-in は system_profiler では "Color LCD" と返るので明示マップ
ALIASES = {
    "Built-in Retina Display": ["Color LCD"],
    "Color LCD": ["Built-in Retina Display"],
}

def resolve_named(name):
    if name in named_ids: return named_ids[name]
    for alt in ALIASES.get(name, []):
        if alt in named_ids: return named_ids[alt]
    return None

out_lines = []
i = 0
lines = text.split("\n")
while i < len(lines):
    line = lines[i]
    if line.strip() == "[workspaces.monitorAssignment]" and i+1 < len(lines) \
       and lines[i+1].strip() == 'type = "specificDisplay"':
        # specificDisplay ブロックを丸ごと処理
        j = i + 2
        block_pre = [line, lines[i+1]]
        while j < len(lines) and lines[j].strip() != "[workspaces.monitorAssignment.output]":
            if lines[j].strip().startswith("[[") or (lines[j].strip().startswith("[") and not lines[j].strip().startswith("[workspaces.monitorAssignment")):
                break
            block_pre.append(lines[j])
            j += 1

        if j >= len(lines) or lines[j].strip() != "[workspaces.monitorAssignment.output]":
            # output ブロック無し → そのまま
            out_lines.extend(block_pre)
            i = j
            continue

        # output ブロック解析
        k = j + 1
        output_displayid = None
        output_name = None
        output_displayuuid = None
        while k < len(lines):
            s = lines[k].strip()
            if s.startswith("[["): break
            if s.startswith("[") and not s.startswith("[workspaces.monitorAssignment.output"):
                break
            m = re.match(r'displayId\s*=\s*(-?\d+)', s)
            if m: output_displayid = int(m.group(1))
            m = re.match(r'name\s*=\s*"(.*)"', s)
            if m: output_name = m.group(1)
            m = re.match(r'displayUUID\s*=\s*"(.*)"', s)
            if m: output_displayuuid = m.group(1)
            k += 1

        # 解決
        resolved_id = None
        if output_name == "":
            if unnamed_id is not None: resolved_id = unnamed_id
        else:
            resolved_id = resolve_named(output_name)

        if resolved_id is not None:
            out_lines.extend(block_pre)
            out_lines.append("")
            out_lines.append("[workspaces.monitorAssignment.output]")
            if output_displayuuid:
                out_lines.append(f'displayUUID = "{output_displayuuid}"')
            out_lines.append(f"displayId = {resolved_id}")
            out_lines.append(f'name = "{output_name}"')
        else:
            # 解決失敗 → secondary フォールバック
            out_lines.append("[workspaces.monitorAssignment]")
            out_lines.append('type = "secondary"')
        i = k
    else:
        out_lines.append(line)
        i += 1

sys.stdout.write("\n".join(out_lines))
PYEOF

/bin/chmod 644 "$DEPLOYED"
echo "$SELECTED_TOML" > "$SOURCE_STAMP"

# ログ
NAMES=$(/usr/bin/python3 -c 'import json,sys; d=json.loads(sys.argv[1]); print(json.dumps([x["name"] for x in d]))' "$DISPLAYS" 2>/dev/null || echo '[]')
echo "$(date '+%H:%M:%S') deployed: profile=$SELECTED_NAME source=$SELECTED_TOML displays=$NAMES" >> "$LOG"

if pgrep -x OmniWM > /dev/null 2>&1; then
  /bin/launchctl kickstart -k "gui/$UID/$LAUNCHD_LABEL" 2>/dev/null || true
  echo "$(date '+%H:%M:%S') kickstart -k (config changed)" >> "$LOG"
fi
