# PersistentStore Safety Critique — Blocking Design Review

**Status**: Design critique for implementation approval  
**Date**: 2025-01-XX  
**Context**: projwm-next PersistentStore safety before implementation

---

## Executive Summary

Current design direction commits to:
- Directory store with multi-file structure (`desired.json`, `checkpoint.json`, browser snapshots)
- Generation-based transaction protocol with atomic `CURRENT` pointer
- Controller-mediated commit authority (no direct writes)
- Single writer enforcement via IPC boundary
- Migration whitelist/quarantine for legacy state
- Offline repair limited to store recovery primitives

**This critique identifies 7 blocking design ambiguities and 12 concrete safety rules that must be fixed before implementation.**

---

## 1. Transaction Protocol — Generation Management

### Current Direction
- Writes to `generations/<txid>/...` then atomic rename `CURRENT` pointer
- `desired.json` and `checkpoint.json` must share same transaction ID
- Replaces existing single-file flock + tmpfile + rename

### ❌ BLOCKING ISSUES

#### 1.1 Generation Write Atomicity Ambiguity
**Problem**: Design says "write generation fully, fsync, then rename CURRENT" but doesn't specify:
- Does `generations/<txid>/` get created atomically or progressively?
- What happens if crash occurs after `desired.json` written but before `checkpoint.json`?
- Is there a `.tmp` staging area for the generation directory itself?

**Risk**: Partial generation commits create inconsistent state that recovery can't distinguish from corruption.

**Recommendation**:
```
generations/<txid>.tmp/           ← stage entire generation here
  desired.json
  checkpoint.json
  browser-snapshots/
  [fsync each file]
  [fsync directory]
rename generations/<txid>.tmp/ → generations/<txid>/
rename CURRENT.tmp → CURRENT     ← atomic pointer update
```

#### 1.2 CURRENT Pointer Race Condition
**Problem**: Design doesn't address concurrent readers during CURRENT update.

**Scenario**:
1. Writer: rename `CURRENT.tmp` → `CURRENT` (now points to gen N)
2. Reader: reads `CURRENT` → gets gen N path
3. Writer: starts gen N+1, deletes old generations including N
4. Reader: opens `generations/N/desired.json` → ENOENT

**Recommendation**: Generation retention policy MUST be specified:
- Option A: Keep last N generations, delete only after new generation validated
- Option B: Copy-on-read: reader opens CURRENT, copies path, holds reference
- Option C: Generation directory never deleted by writer (manual GC only)

**Decision needed**: Which retention strategy?

#### 1.3 fsync Directory Missing
**Problem**: Design says "fsync files" but doesn't mention fsync on directory entries.

**Risk**: On many filesystems, file data synced but directory entry (filename) not durable until directory fsync'd. Crash can lose newly created `generations/<txid>/` directory entry even if file data synced.

**Recommendation**: Explicit fsync protocol:
```
1. Write desired.json, fsync(desired.json)
2. Write checkpoint.json, fsync(checkpoint.json)
3. fsync(generations/<txid>/)        ← directory entry
4. rename generations/<txid>.tmp → generations/<txid>/
5. fsync(generations/)               ← parent directory
6. Write CURRENT.tmp, fsync(CURRENT.tmp)
7. rename CURRENT.tmp → CURRENT
8. fsync(store-root/)                ← CURRENT directory entry
```

#### 1.4 Transaction ID Generation Undefined
**Problem**: Design doesn't specify transaction ID format/source.

**Ambiguities**:
- Monotonic counter? Timestamp? UUID? Epoch-based?
- Where is counter persisted? (In checkpoint? Separate file?)
- What if restart sees same transaction ID already exists?

**Recommendation**:
```
Transaction ID = <epoch>-<monotonic-counter>-<timestamp-ms>

- Epoch: incremented on bootstrap or crash recovery
- Counter: in-memory, reset each epoch
- Timestamp: for debugging/ordering, not authority

Persistence: checkpoint.json contains next_transaction_id
Recovery: if checkpoint.json missing, use max(existing generations) + 1
Collision: if generation dir exists, MUST be corruption (reject)
```

---

## 2. Production/Test Path Separation

### Current Direction
- Test mode requires: test manifest + test store + test socket
- `--test-mode` flag enables test-only intents
- Production store path rejects test mode startup

### ❌ BLOCKING ISSUES

#### 2.1 Path Separation Enforcement Mechanism Undefined
**Problem**: Design says "production store path rejects test mode" but doesn't specify HOW this is enforced.

**Ambiguities**:
- Is production path hardcoded? Environment variable? Manifest-derived?
- Does store directory contain a "production marker" file?
- Can user accidentally run test mode on production store if they override path?

**Recommendation**:
```
Store initialization creates .store-mode file:
  {"mode": "production", "created_at": "...", "manifest_source": "..."}

Daemon startup:
1. Read .store-mode
2. If mode=production and --test-mode: REJECT
3. If mode=test and no --test-mode: REJECT
4. If mode mismatch: explicit error with path and expected mode

Test store path must be:
  ~/.local/state/projwm-test-<session-id>/
NOT:
  ~/.local/state/projwm-next/  (production)
```

#### 2.2 Test Fixture Isolation Incomplete
**Problem**: Design says "fixture load through Controller transaction" but doesn't address:
- Can test fixture access production manifest?
- Can test store reference production Nix store paths?
- What if test fixture has same project CWD as production?

**Recommendation**:
```
Test manifest MUST:
- Have authority = "test" (not "nix")
- Use isolated app paths (mocks or test binaries)
- Workspace topology isolated from production

Test store MUST:
- Different state directory (enforced by .store-mode)
- Different socket path (no production daemon contact)
- Different browser profile directories (no production data)

IntentLoadFixture validation:
- Reject if fixture.authority = "nix"
- Reject if fixture uses production store path
- Warn if fixture.project.cwd overlaps production
```

---

## 3. Migration Whitelist/Quarantine

### Current Direction
- Whitelist: profile name, project name/cwd, window kind/ordinal, browser profile name
- Quarantine: LiveWindowID, frame coords, SavedURLs (privacy check), current focus

### ❌ BLOCKING ISSUES

#### 3.1 SavedURLs Privacy Policy Undefined
**Problem**: Design says "SavedURLs needs privacy policy check" but doesn't define the policy.

**Ambiguities**:
- What constitutes sensitive URL? (localhost? file://? Basic auth in URL?)
- Is migration interactive (user prompt per URL) or automatic with opt-in?
- Where is privacy policy configured? (Manifest? User preference?)

**Recommendation**:
```
Default policy: REJECT all SavedURLs in automatic migration
Opt-in policy: User runs migration with --import-browser-history
  Then apply URL filter:
    - DROP: file://, localhost:*, 127.0.0.1:*, basic auth URLs
    - WARN: http:// (non-https)
    - ALLOW: https:// with no auth in path

Migration report includes:
  - Total URLs found
  - Total URLs dropped (by category)
  - Total URLs imported
  - Suggest manual review of saved-urls-quarantine.json
```

#### 3.2 Window Ordinal Stability Undefined
**Problem**: Whitelist includes "stable ordinal" but doesn't define how ordinal becomes stable.

**Scenario**: Legacy state has:
```
Project "api" Windows: [zed:0, browser:0, terminal:1, terminal:2]
```

At migration time:
- zed:0 → DesiredWindow, but is ordinal preserved?
- terminal:1, terminal:2 → Are these separate windows or just "terminals"?
- If terminal:2 becomes terminal:1 in DesiredWorld, does user see unexpected order?

**Recommendation**:
```
Migration preserves ordinal only if:
1. Window kind is unique in project (zed:0 → only one zed)
2. Multiple same-kind windows → preserve relative order but renumber from 0
3. Layout.Columns preserved as semantic order hint (not pixel coordinates)

Migration report:
  - "Renumbered terminals in project 'api': 2 → 0, 3 → 1"
  - "Preserved ordinal: zed:0, browser:0"
```

#### 3.3 Migration Quarantine Storage Location
**Problem**: Design doesn't specify where quarantined data goes.

**Options**:
- A. Discard entirely (no recovery)
- B. Store in `migration-quarantine.json` in new store
- C. Keep legacy `state.json.bak` untouched as archive
- D. Separate quarantine directory outside store

**Recommendation**:
```
Option C + B hybrid:
1. Copy legacy state.json → state.json.migrated-from (archive)
2. Create migration-report.json with:
   - Whitelist items imported
   - Quarantine items dropped (with reasons)
   - Warning items (e.g. SavedURLs privacy)
3. Store both in generations/<migration-txid>/
4. User can manually review and re-import via admin command
```

---

## 4. Crash Recovery Protocol

### Current Direction
- Restart: load last committed generation, discard predicted state
- Journal is "recovery aid" not truth
- If checkpoint missing, treat as corruption

### ❌ BLOCKING ISSUES

#### 4.1 Journal Semantics Ambiguous
**Problem**: Design says "journal is crash/debug/recovery aid" but doesn't specify:
- What does journal contain?
- When is journal written? (Every transaction? Periodic?)
- How is journal used in recovery? (Replay? Just diagnostic?)

**Recommendation**:
```
Journal is append-only transaction log:

journal.jsonl:
  {"txid": "E1-0001", "timestamp": "...", "intent": "UserIntent{...}", "result": "committed"}
  {"txid": "E1-0002", "timestamp": "...", "intent": "LayoutAccept{...}", "result": "aborted-stale-epoch"}
  {"txid": "E1-0003", "timestamp": "...", "intent": "BrowserArchive{...}", "result": "in-progress"}

Recovery use:
1. Scan journal for last committed transaction
2. If journal txid > store CURRENT txid → crash during commit
3. Recovery: rollback to last committed, report in-progress intents
4. Journal is NOT replayed (not truth, just audit trail)

Journal rotation:
- New journal per epoch
- Archived to traces/ on clean shutdown
```

#### 4.2 Partial Commit Detection Missing
**Problem**: Scenario: crash after `desired.json` written, before `checkpoint.json`.

**Current design says**: "desired.json and checkpoint.json must have same transaction ID"

**But doesn't specify**: How to detect this corruption on startup?

**Recommendation**:
```
Store validation on startup:
1. Read CURRENT → generation path
2. Read generation/desired.json → extract txid_desired
3. Read generation/checkpoint.json → extract txid_checkpoint
4. If txid_desired != txid_checkpoint:
   → CORRUPTION: "Partial commit detected"
   → Fallback: rollback to previous generation
   → If no previous: migration or empty bootstrap

Checkpoint structure MUST include:
  {
    "transaction_id": "E1-0042",
    "epoch": 1,
    "desired_hash": "sha256-of-desired.json",  ← integrity check
    "schema_version": 1
  }
```

#### 4.3 Epoch Bump Criteria Undefined
**Problem**: Design mentions "epoch" but doesn't define when epoch increments.

**Ambiguities**:
- Epoch bumps on every restart?
- Only on crash recovery?
- Only on schema migration?

**Recommendation**:
```
Epoch increments on:
1. Clean bootstrap (first run or empty store)
2. Crash recovery (detected by missing clean-shutdown marker)
3. Schema migration
4. Manual admin rollback-store

Epoch does NOT increment on:
- Clean shutdown + restart
- Daemon reload without store change

Epoch stored in:
- checkpoint.json (current epoch)
- .last-clean-shutdown (marker deleted on startup, written on clean exit)

Recovery logic:
if .last-clean-shutdown missing:
  → Assume crash
  → Increment epoch
  → Log "Recovery: starting epoch N+1 after unclean shutdown"
```

---

## 5. Schema Versioning Strategy

### Current Direction
- Schema version in checkpoint
- Manifest has schema version for environment
- Store has schema version for DesiredWorld

### ❌ BLOCKING ISSUES

#### 5.1 Schema Version Location Ambiguity
**Problem**: Design mentions schema version in checkpoint but doesn't clarify:
- Is schema version per-file (desired.json, checkpoint.json separate versions)?
- Or single store-wide schema version?
- What if desired.json is v1 but checkpoint.json is v2?

**Recommendation**:
```
Single store-wide schema version in checkpoint.json:
  {
    "schema_version": 1,
    "schema_components": {
      "desired_world": 1,
      "checkpoint": 1,
      "browser_snapshot": 1
    }
  }

Validation:
- If any component schema > daemon's max supported: REJECT
- If any component schema < daemon's min supported: MIGRATION REQUIRED

Migration runs before daemon event loop starts.
```

#### 5.2 Forward Compatibility Policy Missing
**Problem**: Design says "unknown field → block" for manifest, but doesn't define policy for store.

**Scenario**: User runs new daemon (adds new field to DesiredWorld), then downgrades daemon.

**Options**:
- A. Old daemon rejects store (strict versioning)
- B. Old daemon ignores unknown fields (forward-compatible)
- C. Old daemon quarantines unknown fields, preserves on write-back

**Recommendation**:
```
Option A: Strict versioning for store (unlike manifest)

Rationale:
- Store is truth, not contract
- Unknown desired-world fields can change semantics (not just hints)
- Downgrade should be explicit: admin rollback-store --to-generation <old-schema>

Schema validation error:
  "Store schema version 2 not supported by daemon (max: 1)"
  "To downgrade: projwmctl admin rollback-store --to-generation <v1-gen>"
```

---

## 6. Journal vs Checkpoint Semantics

### Current Direction
- Journal: append-only transaction log, not truth
- Checkpoint: recovery metadata, truth for restart

### ❌ BLOCKING ISSUES

#### 6.1 Checkpoint Content Minimal Definition
**Problem**: Design lists "epoch / transaction / dirty scope / store version" but doesn't define minimal required fields.

**Recommendation**:
```json
{
  "schema_version": 1,
  "transaction_id": "E1-0042",
  "epoch": 1,
  "timestamp": "2025-01-15T10:30:00Z",
  "desired_hash": "sha256:abc123...",
  "dirty_scopes": ["workspace:viewer", "project:api"],
  "managed_environment": {
    "source": "/nix/store/xxx-manifest.json",
    "schema_version": 1,
    "hash": "sha256:def456..."
  },
  "last_observation_timestamp": "2025-01-15T10:29:50Z",
  "next_transaction_id": "E1-0043"
}
```

#### 6.2 Journal vs Checkpoint Write Order
**Problem**: Design doesn't specify: is journal entry written before or after checkpoint commit?

**Scenario**:
1. Controller decides to commit transaction T
2. Writes generation with desired + checkpoint
3. Renames CURRENT
4. Crash before journal entry written

Result: Store shows committed, but no journal entry. Is this inconsistent?

**Recommendation**:
```
Write order:
1. Append journal entry with result="in-progress"
2. fsync journal
3. Write generation (desired + checkpoint)
4. fsync generation
5. Rename CURRENT
6. Update journal entry result="committed"
7. fsync journal

Recovery:
- Journal entry "in-progress" + store committed → update journal to "committed"
- Journal entry "in-progress" + store not committed → rollback or retry

Journal is diagnostic, not recovery source.
```

---

## 7. Offline Repair Constraints

### Current Direction
- No arbitrary `state edit` command
- Offline repair limited to: validate, rollback, quarantine, rebuild index, retry migration
- Emergency repair: `projwmd --recover` (no event loop, no adapter calls)

### ❌ BLOCKING ISSUES

#### 7.1 Repair Primitive "Rebuild Index" Undefined
**Problem**: Design lists "rebuild index" as repair primitive but store is file-based (no separate index mentioned).

**Clarification needed**:
- Is there a separate index file?
- Or is "rebuild index" just re-validation?

**Recommendation**:
```
If no separate index planned:
- Remove "rebuild index" from repair primitive list
- Replace with "validate and regenerate CURRENT from generations/"

If index planned later (e.g., for fast generation lookup):
- Define index structure now (even if not implemented yet)
- Example: .index.json → {generations: [{id, timestamp, hash}, ...]}
```

#### 7.2 Quarantine Corrupted Generation Ambiguous
**Problem**: Design allows "quarantine corrupted generation" but doesn't define:
- Where does quarantined generation go?
- Is it moved? Renamed? Deleted?
- Can user un-quarantine later?

**Recommendation**:
```
Quarantine operation:
  rename generations/<txid>/ → generations/.quarantined-<txid>-<timestamp>/

Quarantine metadata:
  Create generations/.quarantined-<txid>-<timestamp>/quarantine-reason.txt
  Contains: error, timestamp, daemon version, recovery context

Un-quarantine:
  projwmctl admin unquarantine-generation <txid>
  Validation required before un-quarantine

Automatic cleanup:
  Quarantined generations kept for 30 days, then purged (configurable)
```

#### 7.3 --recover Mode Exit Criteria Undefined
**Problem**: Design says `projwmd --recover` does validate/rollback/quarantine and exits, but doesn't specify success vs failure exit.

**Recommendation**:
```
projwmd --recover exit codes:
  0: Store validated successfully, no repair needed
  1: Store repaired successfully (rollback or quarantine performed)
  2: Store corruption detected, automatic repair failed (manual intervention needed)
  3: No store found (migration or bootstrap required)

Output:
  --recover must write machine-readable recovery-report.json:
  {
    "status": "repaired",
    "actions": ["quarantined generation E1-0042", "rolled back to E1-0041"],
    "safe_to_start": true
  }
```

---

## 8. Consolidated Safety Rules for Generation Protocol

Based on above critique, the following rules MUST be implemented:

### Generation Transaction Protocol
1. **RULE-GEN-1**: Generation directory staged in `.tmp` suffix, then atomically renamed
2. **RULE-GEN-2**: fsync order: file → file-parent-dir → CURRENT.tmp → CURRENT-parent-dir
3. **RULE-GEN-3**: Transaction ID format: `<epoch>-<counter>-<timestamp>`, collision detection mandatory
4. **RULE-GEN-4**: `desired.json` and `checkpoint.json` transaction ID match validated on load
5. **RULE-GEN-5**: Generation retention: minimum 2 generations kept until new generation validated
6. **RULE-GEN-6**: CURRENT pointer read → generation path copy (no dangling reference after GC)

### Recovery Protocol
7. **RULE-REC-1**: `.last-clean-shutdown` marker determines epoch bump on restart
8. **RULE-REC-2**: Partial commit detected by desired/checkpoint txid mismatch → rollback to previous
9. **RULE-REC-3**: Journal entries mark "in-progress" before commit, "committed" after CURRENT rename
10. **RULE-REC-4**: Journal append-only per epoch, rotated to traces/ on clean shutdown

### Schema & Migration
11. **RULE-SCH-1**: Single store-wide schema version in checkpoint, not per-file
12. **RULE-SCH-2**: Store schema mismatch → strict rejection, no forward-compat (unlike manifest)
13. **RULE-MIG-1**: Migration creates `migration-report.json` in generation with whitelist/quarantine details
14. **RULE-MIG-2**: SavedURLs import requires opt-in flag, default privacy filter drops localhost/file/auth

### Test & Repair
15. **RULE-TST-1**: Store directory contains `.store-mode` file (production/test), enforced on startup
16. **RULE-TST-2**: Test store path pattern: `~/.local/state/projwm-test-<session>/` (not production path)
17. **RULE-REP-1**: `projwmd --recover` writes `recovery-report.json`, exit code indicates safe-to-start
18. **RULE-REP-2**: Quarantine: rename to `.quarantined-<txid>-<timestamp>/` with reason file

---

## 9. User Decisions Required

The following design choices require explicit user decision before implementation:

### Decision 1: Generation Retention Policy
**Options**:
- A. Keep last N generations (e.g., 5), auto-delete older
- B. Keep all generations, manual GC only
- C. Keep generations for T days (e.g., 7 days), auto-delete older

**Recommendation**: Option A (last 5 generations) for automatic safety without unbounded growth.

### Decision 2: Test Store Isolation Level
**Options**:
- A. Test store can reference production manifest (share Nix environment)
- B. Test store must use isolated test manifest (no production Nix paths)

**Recommendation**: Option B (full isolation) to prevent test mutations from affecting production-configured apps.

### Decision 3: Migration SavedURLs Handling
**Options**:
- A. Always drop SavedURLs (safest, but loses data)
- B. Import with privacy filter (automated)
- C. Interactive prompt per URL (slow for large history)

**Recommendation**: Option B with opt-in flag `--import-browser-history`.

### Decision 4: Epoch Increment on Clean Restart
**Options**:
- A. Epoch increments on every restart (simple, but large epoch numbers)
- B. Epoch only increments on crash/migration (complex marker file logic)

**Recommendation**: Option B (crash/migration only) to keep epoch meaningful.

### Decision 5: Journal Retention
**Options**:
- A. One journal per epoch, kept forever
- B. Journal rotated to traces/ on clean shutdown, subject to trace retention policy
- C. Journal purged on clean shutdown (minimal storage)

**Recommendation**: Option B (archive to traces/) for post-mortem debugging.

### Decision 6: Schema Migration Timing
**Options**:
- A. Migration runs on daemon startup (blocking startup until migration completes)
- B. Migration is separate tool, must run before daemon start (explicit step)

**Recommendation**: Option A (startup migration) for user convenience, but with timeout and fallback to manual migration tool.

### Decision 7: Repair "Rebuild Index" Scope
**Options**:
- A. Remove "rebuild index" from repair primitives (no index planned)
- B. Define index structure now (generations list cache)
- C. Add "rebuild CURRENT" primitive (scan generations/, select latest valid)

**Recommendation**: Option C (rebuild CURRENT) as practical repair primitive without premature index design.

---

## 10. Implementation Readiness Checklist

Before proceeding to implementation, the following must be resolved:

- [ ] Transaction ID format and collision detection strategy finalized
- [ ] fsync protocol with directory fsync order documented
- [ ] Generation retention policy (count or time-based) decided
- [ ] CURRENT pointer race condition mitigation chosen
- [ ] .store-mode enforcement mechanism implemented
- [ ] Test store path isolation pattern agreed
- [ ] Migration whitelist/quarantine report format specified
- [ ] SavedURLs privacy policy and opt-in mechanism defined
- [ ] Journal write order and recovery semantics clarified
- [ ] Epoch increment criteria (.last-clean-shutdown marker) decided
- [ ] Checkpoint minimal required fields documented
- [ ] Schema version location (per-file vs store-wide) decided
- [ ] Schema forward-compatibility policy (strict reject) confirmed
- [ ] Quarantine operation (rename pattern, metadata) specified
- [ ] `projwmd --recover` exit codes and recovery-report.json format defined
- [ ] Repair primitives list finalized (remove/clarify "rebuild index")
- [ ] Window ordinal stability in migration defined

---

## 11. Suggested Next Steps

1. **User reviews** this critique and makes explicit decisions on 7 decision points
2. **Document** all 18 safety rules in implementation-design.md with rule IDs
3. **Prototype** generation transaction protocol (rules GEN-1 to GEN-6) in isolated test
4. **Validate** fsync ordering on target filesystem (macOS APFS) with crash injection
5. **Design** store API interface that enforces transaction protocol (no direct file writes)
6. **Implement** .store-mode enforcement before any mutation paths
7. **Test** migration quarantine with realistic legacy state.json samples

---

## 12. Final Assessment

**The current design direction is sound but incomplete.**

- ✅ Generation-based transaction protocol is correct approach (better than single-file rename)
- ✅ Single writer enforcement via Controller commit is architecturally correct
- ✅ Migration whitelist/quarantine principle is correct
- ✅ Offline repair constraints are appropriately limited

**However, implementation cannot proceed until**:
- ❌ Transaction protocol ambiguities (7 issues) resolved
- ❌ Safety rules (18 rules) explicitly documented
- ❌ User decisions (7 decisions) explicitly made

**Estimated risk** if implementation proceeds without resolving these:
- **High**: Data loss from partial commits or race conditions
- **High**: Production/test contamination from path isolation gaps
- **Medium**: Migration data loss from undefined quarantine handling
- **Medium**: Unrecoverable corruption from undefined recovery protocol

**Blocking recommendation**: Do not implement PersistentStore mutation paths until all 7 decision points answered and all 18 safety rules documented.
