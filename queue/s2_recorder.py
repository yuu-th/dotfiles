#!/usr/bin/env python3
"""External omniwm time-series recorder for the S2 browser-respawn investigation
(handoff §14.10). Polls `omniwmctl query windows` ~every 400ms and writes one
timestamped line per tick capturing every Vivaldi window: pid, workspace
(rawName/displayName), display, visibility, title, and whether the backing
process is MANAGED (argv carries the projwm vivaldi-data --user-data-dir) vs the
user's own Vivaldi. Read-only; never mutates anything."""
import subprocess, json, time, sys

out_path = sys.argv[1] if len(sys.argv) > 1 else "/tmp/s2-omniwm-recorder.jsonl"
out = open(out_path, "w")

def is_managed(pid):
    if not pid:
        return False
    try:
        r = subprocess.run(["ps", "-p", str(pid), "-o", "command="],
                           capture_output=True, text=True, timeout=2)
        return "vivaldi-data" in r.stdout
    except Exception:
        return False

while True:
    t = time.time()
    ts = time.strftime("%H:%M:%S", time.localtime(t)) + f".{int((t % 1) * 1000):03d}"
    try:
        r = subprocess.run(["omniwmctl", "query", "windows", "--format", "json"],
                           capture_output=True, text=True, timeout=3)
        d = json.loads(r.stdout)
        wins = d.get("result", {}).get("payload", {}).get("windows", [])
        viv = []
        for w in wins:
            app = w.get("app") or {}
            if "vivaldi" not in str(app.get("bundleId", "")).lower():
                continue
            ws = w.get("workspace") or {}
            disp = w.get("display") or {}
            pid = w.get("pid")
            viv.append({
                "id": w.get("id"),
                "pid": pid,
                "managed": is_managed(pid),
                "ws": ws.get("rawName"),
                "wsName": ws.get("displayName"),
                "disp": disp.get("name"),
                "vis": w.get("isVisible"),
                "title": w.get("title"),
            })
        out.write(f"{ts} vivaldi={len(viv)} managed={sum(1 for v in viv if v['managed'])} {json.dumps(viv, ensure_ascii=False)}\n")
    except Exception as e:
        out.write(f"{ts} ERR {e}\n")
    out.flush()
    time.sleep(0.4)
