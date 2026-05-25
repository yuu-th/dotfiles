//go:build real_ops

package session

import "testing"

func TestTmuxEnsureSession(t *testing.T) {
	TestRealOpsTmuxEnsureSession(t)
}

func TestTmuxEnsureSessionAlreadyExists(t *testing.T) {
	TestRealOpsTmuxEnsureSessionAlreadyExists(t)
}

func TestTmuxEnsureGroupedSession(t *testing.T) {
	TestRealOpsTmuxEnsureGroupedSession(t)
}

func TestTmuxKillSession(t *testing.T) {
	TestRealOpsTmuxKillSession(t)
}
