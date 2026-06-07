//go:build integration

package scenarios

import (
	"os/exec"
	"strconv"
	"strings"
)

// Zed main-process helpers for the SSOT §6.9.1 ATTR-F2 single-process safety
// PRECONDITION, used by the legitimate L4 crash-recovery scenario S10
// (TestHumanE2ESSOTCrashRecoverySteps): the Zed-crash step only proceeds when
// the test's editor is the SOLE Zed main process, so a user editing session is
// never destroyed (Zed is single-instance — every window shares one process).
//
// ── Why there are no L4 ATTR-attribution tests here (SSOT layer齟齬 fix) ──
//
// The ATTR-* window-attribution edge cases (§6.9.1 A/B/C/D/E/G) are assigned by
// the SSOT to the DETERMINISTIC layers, and only ATTR-F1 (process survival) to
// L3 real_ops. This is deliberate, stated by the spec itself:
//
//   - §6.9.1 layer column: A2=L0, B1/B3=L0/L2, C1=L0/L2, D5/E2/E3=L2,
//     G1-3=L2, F1=L3実機, F2=L3/L4 precond. No row is assigned to L4.
//   - §6.9.1 ⚠️ (A5/B5/C2): complete attribution is "原理的に不可能" under
//     single-process Zed + basename-title; the residual risk is "防御せず受容".
//   - §10.2: "L2/L3 で単一操作 contract が独立に検証できれば、L4 は組合せとして
//     保証される" — a single contract verified at L0/L2/L3 is NOT re-tested at L4.
//
// Earlier work added L4 humanE2E duplicates (TestZedAttr_B1/B3/A2/F1) that
// re-tested this L0/L2 contract on the real machine. They could not pass: they
// required spawning a *distinguishable* real "user" Zed window — exactly the
// single-process limitation §6.9.1 receives as impossible. That was a drift
// from the SSOT layer strategy, so the L4 duplicates were removed. The
// authoritative owners remain:
//
//   - L0/L2: internal/controller/ssot_attr_test.go,
//            internal/controller/ssot_attr_lifecycle_recovery_test.go,
//            internal/identity/ssot_attr_test.go,
//            internal/invariant/ssot_attr_l1_test.go
//   - L3   : internal/adapter/wm/ssot_attr_real_ops_test.go (F1/A1 + D2/D3)
//
// See ledger row ZED-ATTR.

// zedMainPIDs returns the PIDs of Zed MAIN processes — those whose argv[0] is
// exactly the Zed main binary — excluding the always-present --crash-handler
// subprocess (and any worker subprocesses, which have a different argv[0]).
// Matching argv[0] exactly avoids the substring/self-match pitfalls of
// `pgrep -fl` (a bare pgrep can also catch helper processes whose cmdline merely
// contains the path).
func zedMainPIDs() []int {
	out, _ := exec.Command("ps", "-axo", "pid=,args=").Output()
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] != "/Applications/Zed.app/Contents/MacOS/zed" {
			continue
		}
		if strings.Contains(line, "--crash-handler") {
			continue
		}
		if pid, err := strconv.Atoi(fields[0]); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// attrZedMainProcCount counts Zed MAIN processes (see zedMainPIDs).
func attrZedMainProcCount() int { return len(zedMainPIDs()) }
