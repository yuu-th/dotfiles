# OmniWM 設定 deploy（v4: OmniWM 0.5.9 / displayUUID 解決 + live reload）
#
# 役割:
#   1. 接続中ディスプレイを列挙（名前 / CGDirectDisplayID / displayUUID / 内蔵か）
#   2. プロファイルを選択（"auto" なら match 条件で、具体名ならそれを強制）
#   3. 選んだ TOML の `@@OMNIWM_UUID:<selector>@@` と
#      `@@OMNIWM_ROUTING_MODE:<mode>@@` トークンを実値に置換
#   4. 前回と同一内容なら**何も書かない**（live reload を無駄に叩かないため）
#   5. 変わっていれば atomic に差し替え → OmniWM が自分で live reload する
#   6. OmniWM が落ちている時だけ kickstart
#
# ── なぜ displayUUID なのか ──
# OmniWM 0.5.9 の `OutputId.resolveMonitor` / `MonitorSettingsStore.get` は
#   - displayUUID があれば UUID で一意マッチ
#   - なければ「候補モニタの displayUUID が nil」かつ displayId 一致かつ名前一致
# を要求する。実機の全モニタは UUID を持つので後者は絶対に成立しない。さらに
# `Monitor.namesMatch` は両方が非空文字を要求するため、名前なしモニタを名前で
# 指定することも原理的に不可能。よって UUID を埋めるしかない。
#
# ── なぜ kickstart -k をやめたのか ──
# 0.4.8.1 以降 OmniWM は settings.toml をファイル監視して live reload する。
# 再起動するとウィンドウが全部再配置されてチラつくので、書き換えだけで済ませる。
#
# 環境変数（default.nix から注入）:
#   PROFILE_MANIFEST : プロファイル一覧 JSON（[{name, toml, match}]）
#   SELECTED_PROFILE : "auto" or プロファイル名
#   LAUNCHD_LABEL    : OmniWM launchd ラベル
set -euo pipefail

DEPLOYED="$HOME/.config/omniwm/settings.toml"
LOG="$HOME/.local/share/omniwm/deploy.log"
# 前回 deploy した「render 結果そのもの」を保存しておく。
#
# ⚠️ deployed (~/.config/omniwm/settings.toml) と直接比較してはいけない。
# OmniWM は起動時に settings.toml を書き戻して未知キーを温存しつつ
# 自分の既定セクション（[overview] / [hiddenBar] / 全 hotkey 等）を足すので、
# deployed は常に nix の生成物と一致しない。deployed と比べると毎回「差分あり」に
# なって live reload を無駄に叩くことになる。
RENDER_STAMP="$HOME/.local/share/omniwm/last-rendered.toml"
mkdir -p "$(dirname "$DEPLOYED")" "$(dirname "$LOG")"

TMP="$(mktemp "${TMPDIR:-/tmp}/omniwm-settings.XXXXXX")"
trap 'rm -f "$TMP"' EXIT

log() { echo "$(date '+%H:%M:%S') $*" >> "$LOG"; }

# ── 1〜3 をまとめて Python で処理 ─────────────────────────────────────────
# system python (3.9) で完結させる。tomllib が無いので TOML はパースせず、
# nix が埋めたトークンの文字列置換だけを行う。
#
# stdout = 完成した settings.toml
# stderr = 診断メッセージ（ログに落とす）
# 終了コード 0 以外なら書き込まない（前回の良い設定を保持する）
if ! /usr/bin/python3 - "$PROFILE_MANIFEST" "$SELECTED_PROFILE" > "$TMP" 2>>"$LOG" <<'PYEOF'
import ctypes
import json
import re
import subprocess
import sys

manifest = json.loads(sys.argv[1])
selected = sys.argv[2]


def warn(msg):
    print(f"  deploy: {msg}", file=sys.stderr)


# ── 接続中ディスプレイの列挙 ──────────────────────────────────────────────
# 名前は system_profiler から、UUID は ColorSync から、内蔵判定は CoreGraphics から。
# CGGetActiveDisplayList の ID と system_profiler の _spdisplays_displayID は一致する。
def enumerate_displays():
    cg = ctypes.CDLL("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
    cs = ctypes.CDLL("/System/Library/Frameworks/ColorSync.framework/ColorSync")
    cf = ctypes.CDLL("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")

    cs.CGDisplayCreateUUIDFromDisplayID.restype = ctypes.c_void_p
    cs.CGDisplayCreateUUIDFromDisplayID.argtypes = [ctypes.c_uint32]
    cf.CFUUIDCreateString.restype = ctypes.c_void_p
    cf.CFUUIDCreateString.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
    cf.CFStringGetCString.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_long, ctypes.c_uint32]
    cf.CFRelease.argtypes = [ctypes.c_void_p]

    count = ctypes.c_uint32()
    ids = (ctypes.c_uint32 * 32)()
    if cg.CGGetActiveDisplayList(32, ids, ctypes.byref(count)) != 0:
        return []

    def uuid_for(display_id):
        raw = cs.CGDisplayCreateUUIDFromDisplayID(display_id)
        if not raw:
            return None
        text = None
        cfstr = cf.CFUUIDCreateString(None, raw)
        if cfstr:
            buf = ctypes.create_string_buffer(256)
            # 0x08000100 = kCFStringEncodingUTF8
            if cf.CFStringGetCString(cfstr, buf, 256, 0x08000100):
                text = buf.value.decode()
            cf.CFRelease(cfstr)
        cf.CFRelease(raw)
        return text

    for fn in ("CGDisplayVendorNumber", "CGDisplayModelNumber", "CGDisplaySerialNumber"):
        getattr(cg, fn).restype = ctypes.c_uint32
        getattr(cg, fn).argtypes = [ctypes.c_uint32]

    # ── system_profiler から「名前 + EDID 識別子」を集める ──────────────────
    #
    # ⚠️ system_profiler の数値フィールドは **16進文字列**（0x なし）。
    # `_spdisplays_displayID = "16"` は 10進 16 ではなく 0x16 = 22 を意味する。
    # 10進で読むと ID が 10 以上のモニタで静かに取り違える（実際に踏んだ:
    # HP の CGDirectDisplayID が 22 なのに 16 と読んで名前解決が失敗し、
    # プロファイルが default に落ちて 3 枚構成が 2 枚扱いになった）。
    def parse_hex(value):
        if value is None:
            return None
        text = str(value).strip().lower()
        if text.startswith("0x"):
            text = text[2:]
        for base in (16, 10):
            try:
                return int(text, base)
            except ValueError:
                continue
        return None

    sp_entries = []
    try:
        raw = subprocess.run(
            ["/usr/sbin/system_profiler", "SPDisplaysDataType", "-json"],
            capture_output=True, text=True, timeout=30,
        ).stdout
        for section in json.loads(raw).get("SPDisplaysDataType", []):
            for entry in section.get("spdisplays_ndrvs", []):
                sp_entries.append({
                    "name": entry.get("_name", "") or "",
                    "did": parse_hex(entry.get("_spdisplays_displayID")),
                    "vendor": parse_hex(entry.get("_spdisplays_display-vendor-id")),
                    "model": parse_hex(entry.get("_spdisplays_display-product-id")),
                    "serial": parse_hex(entry.get("_spdisplays_display-serial-number")),
                })
    except Exception as exc:  # noqa: BLE001 - 名前が取れなくても UUID だけで進める
        warn(f"system_profiler failed: {exc}")

    # ── CG のディスプレイと system_profiler のエントリを突き合わせる ─────────
    #
    # 主キーは EDID の (vendor, model, serial)。これは CoreGraphics の
    # CGDisplayVendorNumber / ModelNumber / SerialNumber と一致し、
    # **CGDirectDisplayID が変わっても不変**（実測でこのセッション中に
    # HP の ID が 7 → 9 → 22 と変わった）。
    # 一意に決まらない場合だけ displayID による突き合わせに落とす。
    def name_for(did):
        key = (
            cg.CGDisplayVendorNumber(did),
            cg.CGDisplayModelNumber(did),
            cg.CGDisplaySerialNumber(did),
        )
        cands = [e for e in sp_entries
                 if (e["vendor"], e["model"], e["serial"]) == key]
        if len(cands) != 1:
            cands = [e for e in sp_entries if e["did"] == did]
        if len(cands) != 1:
            return None
        return cands[0]["name"]

    out = []
    for i in range(count.value):
        did = ids[i]
        name = name_for(did)
        if name is None:
            warn(f"could not match displayId={did} to a system_profiler entry (name unknown)")
            name = ""
        out.append({
            "id": did,
            "name": name,
            "uuid": uuid_for(did),
            "builtin": bool(cg.CGDisplayIsBuiltin(did)),
        })
    return out


displays = enumerate_displays()
if not displays:
    warn("ERROR: no active displays found; refusing to deploy")
    sys.exit(1)

# system_profiler は EDID name を持たないモニタを "spdisplays_display" と返す
UNNAMED_SENTINEL = "spdisplays_display"
BUILTIN_ALIASES = {"built-in retina display", "color lcd", "built-in display"}

named = {d["name"] for d in displays if d["name"] and d["name"] != UNNAMED_SENTINEL}
has_unnamed = any(d["name"] == UNNAMED_SENTINEL for d in displays)


# ── プロファイル選択 ──────────────────────────────────────────────────────
def specificity(profile):
    m = profile.get("match") or {}
    return (
        len(m.get("requiredDisplays", []))
        + (1 if m.get("requireUnnamed") else 0)
        + (1 if m.get("monitorCount") else 0)
    )


def matches(profile):
    m = profile.get("match") or {}
    if not m:
        return True  # match なし = catch-all
    for want in m.get("requiredDisplays", []):
        if want not in named:
            return False
    if m.get("requireUnnamed") and not has_unnamed:
        return False
    if m.get("monitorCount") and m["monitorCount"] != len(displays):
        return False
    return True


def pick_profile():
    if selected != "auto":
        for p in manifest:
            if p["name"] == selected:
                return p
        warn(f"profile '{selected}' not found; falling back to default")
    # specificity 降順 → 同点は名前順 → catch-all(default) は specificity 0 で最後
    for p in sorted(manifest, key=lambda p: (-specificity(p), p["name"])):
        if matches(p):
            return p
    for p in manifest:
        if p["name"] == "default":
            return p
    return None


profile = pick_profile()
if profile is None:
    warn("ERROR: profile selection failed")
    sys.exit(1)

with open(profile["toml"]) as handle:
    text = handle.read()


# ── トークン解決 ──────────────────────────────────────────────────────────
def resolve(selector):
    """selector に対応する displayUUID を返す。解決できなければ None。"""
    if selector == "":
        cands = [d for d in displays if d["name"] == UNNAMED_SENTINEL]
    elif selector.strip().lower() in BUILTIN_ALIASES:
        cands = [d for d in displays if d["builtin"]]
    else:
        cands = [d for d in displays if d["name"] == selector]
        if not cands:
            # 内蔵の名前は system_profiler と OmniWM で表記が違うので保険をかける
            if selector.strip().lower() in BUILTIN_ALIASES:
                cands = [d for d in displays if d["builtin"]]
    cands = [d for d in cands if d["uuid"]]
    if len(cands) != 1:
        return None
    return cands[0]["uuid"]


main_display = next((d for d in displays if d["builtin"] and d["uuid"]), None)
if main_display is None:
    main_display = next((d for d in displays if d["uuid"]), None)
if main_display is None:
    warn("ERROR: no display exposes a UUID; refusing to deploy")
    sys.exit(1)

unresolved = []


def substitute_uuid(match):
    selector = match.group(1)
    uuid = resolve(selector)
    if uuid is None:
        unresolved.append(selector or "<unnamed>")
        # 有効な UUID を入れて TOML を壊さないことを優先する。
        # routing は下で mode = "macOS" に落ちるので、この値は無害になる。
        return main_display["uuid"]
    return uuid


text = re.sub(r"@@OMNIWM_UUID:([^@]*)@@", substitute_uuid, text)


def substitute_routing_mode(match):
    intended = match.group(1)
    if intended != "custom":
        return intended
    if unresolved:
        warn(f"routing: falling back to macOS arrangement (unresolved: {sorted(set(unresolved))})")
        return "macOS"
    return "custom"


text = re.sub(r"@@OMNIWM_ROUTING_MODE:([^@]*)@@", substitute_routing_mode, text)

# ── 最終ガード ────────────────────────────────────────────────────────────
# トークンが残ったまま書き込むと displayUUID の形式違反になり、OmniWM は
# dataCorrupted として settings.toml を .corrupt に退避してしまう（keyNotFound と
# 違って回復されない）。残っていたら書き込まずに落ちる。
leftover = re.findall(r"@@[A-Z_]+:[^@]*@@", text)
if leftover:
    warn(f"ERROR: unsubstituted tokens remain: {sorted(set(leftover))}")
    sys.exit(1)

if unresolved:
    warn(f"unresolved display selectors (fell back to main): {sorted(set(unresolved))}")

warn(
    "selected profile={} displays={}".format(
        profile["name"],
        [(d["name"] or "<unnamed>", d["uuid"]) for d in displays],
    )
)
sys.stdout.write(text)
PYEOF
then
  log "ERROR: render failed (selected=$SELECTED_PROFILE); keeping previous settings.toml"
  exit 1
fi

# ── 4. render 結果が前回と同じで、かつ deployed が実在するなら書かない ────
# live reload を無駄に発火させないため。OmniWM が落ちている時だけ起こす。
if [ -f "$RENDER_STAMP" ] && [ -f "$DEPLOYED" ] && cmp -s "$TMP" "$RENDER_STAMP"; then
  if ! pgrep -x OmniWM > /dev/null 2>&1; then
    /bin/launchctl kickstart "gui/$UID/$LAUNCHD_LABEL" 2>/dev/null || true
    log "noop: render unchanged, kickstart (OmniWM was down)"
  fi
  exit 0
fi

# ── 5. atomic に差し替え → OmniWM が live reload する ─────────────────────
# mv による inode 差し替えはエディタの保存と同じパターンで、OmniWM の
# ディレクトリ監視 + ファイル監視がこれを拾って reload する。
/bin/chmod 644 "$TMP"
/bin/cp -f "$TMP" "$RENDER_STAMP"
/bin/mv -f "$TMP" "$DEPLOYED"
trap - EXIT
log "deployed: settings.toml updated (live reload)"

# ── 6. 落ちている時だけ起こす ─────────────────────────────────────────────
if ! pgrep -x OmniWM > /dev/null 2>&1; then
  /bin/launchctl kickstart "gui/$UID/$LAUNCHD_LABEL" 2>/dev/null || true
  log "kickstart (OmniWM was down)"
fi
