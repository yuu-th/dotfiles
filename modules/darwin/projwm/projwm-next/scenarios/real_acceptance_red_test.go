//go:build integration

package scenarios

// Window-content semantics historical red cases previously lived here. Once
// SessionCapabilityAdapter (internal/adapter/session.Client) and Vivaldi
// AppleScript tab inspection (VivaldiAdapter.InspectTabs) landed, the four
// tests (SESS.1, SESS.2, SESS.3, PRIV.6.5b) were promoted into
// real_acceptance_test.go where they reconcile the ideal state and assert
// against the live tmux server / Vivaldi window state.
//
// This file is intentionally retained without TestHumanE2E* functions so
// the integrity tests in internal/scenario have a stable
// redAcceptancePath() target.
