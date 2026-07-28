#!/bin/bash
set -u
LOG=/tmp/s2-test-output.log
TRACE=/tmp/s2-daemon-stderr.log
REC=/tmp/s2-omniwm-recorder.jsonl
: > "$LOG"; : > "$TRACE"; : > "$REC"
UID_=$(id -u)
PLIST="$HOME/Library/LaunchAgents/org.nixos.projwmd-next.plist"
LABEL="gui/$UID_/org.nixos.projwmd-next"

log(){ echo "$@" | tee -a "$LOG"; }

log "=== safe session prep $(date +%H:%M:%S) ==="
log "zed-windows(keep, do NOT kill): $(pgrep -fl 'Zed.app/Contents/MacOS/zed' | grep -v -- '--crash-handler' | wc -l | tr -d ' ')"
log "user vivaldi (keep): $(pgrep -f 'MacOS/Vivaldi' | grep -v Helper | wc -l | tr -d ' ')"

# Quiesce production daemon so it does not double-manage windows with the test daemon.
launchctl bootout "$LABEL" 2>&1 | tee -a "$LOG" || log "(bootout: already out)"
sleep 1
if launchctl print "$LABEL" >/dev/null 2>&1; then log "WARN daemon still loaded after bootout"; else log "daemon quiesced OK"; fi

# Kill ONLY leftover managed automation Vivaldi (argv carries vivaldi-data); never the user's.
for pid in $(pgrep -f 'vivaldi-data'); do log "killing leftover managed vivaldi pid=$pid"; kill -9 "$pid" 2>/dev/null; done

# Start external omniwm recorder.
python3 /tmp/s2_recorder.py "$REC" &
RECPID=$!
log "recorder pid=$RECPID -> $REC"

log "=== test start $(date +%H:%M:%S) ==="
cd /Users/yuta/dev/dotfiles/modules/darwin/projwm/projwm-next || exit 1
PROJWM_NEXT_REAL_ACCEPTANCE=1 PROJWM_NEXT_PLANNER_TRACE=1 PROJWM_NEXT_DAEMON_STDERR_FILE="$TRACE" \
  go test -tags integration -run '^(TestHumanE2EArchiveUnarchiveSteps|TestHumanE2EProductionRemovalWithoutCloseWindowSteps)$' ./scenarios/ -v -timeout 600s >> "$LOG" 2>&1
TEST_EXIT=$?
log "=== test exit=$TEST_EXIT $(date +%H:%M:%S) ==="

# Stop recorder.
kill "$RECPID" 2>/dev/null
sleep 0.5

# Ensure production daemon restored (harness t.Cleanup may not run on timeout-kill).
if ! launchctl print "$LABEL" >/dev/null 2>&1; then
  launchctl bootstrap "gui/$UID_" "$PLIST" 2>&1 | tee -a "$LOG"
  sleep 1
  if launchctl print "$LABEL" >/dev/null 2>&1; then log "daemon restored (manual bootstrap)"; else log "ERROR daemon NOT restored"; fi
else
  log "daemon restored (harness)"
fi

log "trace lines: $(wc -l < "$TRACE" | tr -d ' ')  recorder ticks: $(wc -l < "$REC" | tr -d ' ')"
log "=== done $(date +%H:%M:%S) ==="
