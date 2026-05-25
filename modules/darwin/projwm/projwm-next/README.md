# projwm-next

Implementation is in active hardening. The authoritative design/specification
source remains the queue SSOT; this README is only a local orientation note.

## SSOT (Single Source of Truth)

All design decisions and specifications are documented in:

- `/Users/yuta/dev/dotfiles/queue/design.md`
- `/Users/yuta/dev/dotfiles/queue/implementation-design.md`
- `/Users/yuta/dev/dotfiles/queue/specs.md`

This README does not duplicate SSOT content. Refer to the above documents for:
- Architecture (World Controller, Reducer, Planner, Simulator, Executor, Settler, Verifier)
- Type definitions (WorldState, DesiredWorld, ObservedWorld, PredictedWorld)
- Intent catalog (8 kinds)
- Operation catalog
- Invariant definitions (13 core + 6 controller transaction semantics)
- Scenario specifications (8 scenarios)
- Manifest schema
- PersistentStore contract
- IPC handshake protocol
- Adapter interfaces

## Current status

- Scenario, controller, transaction, IPC, PersistentStore, and real adapter
  contracts are implemented in Go.
- Production daemon startup uses an existing FileStore, managed-environment
  manifest digest, manifest-authorized socket path, launchd provenance plumbing,
  and PrivatePayloadStore wiring for browser payloads.
- Browser URL payloads are private: PersistentStore keeps opaque refs, raw
  payloads live in PrivatePayloadStore, and adapter errors are redacted.
- Final Human-operation authority is not complete until the real launchd/OmniWM/
  Ghostty/Vivaldi acceptance rows are proven on the live host.

## Testing

```bash
cd modules/darwin/projwm/projwm-next
go test ./... -count=1
go test -tags integration ./... -count=1
cd /Users/yuta/dev/dotfiles
nix build .#darwinConfigurations.yuta.config.system.build.toplevel --no-link
```
