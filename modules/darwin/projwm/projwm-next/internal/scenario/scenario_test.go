package scenario

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestNewBackendRealRequiresExplicitOptions(t *testing.T) {
	defer func() {
		got := recover()
		err, ok := got.(error)
		if got == nil || !ok || !errors.Is(err, ErrRealBackendRequiresOptions) {
			t.Fatalf("panic = %v, want ErrRealBackendRequiresOptions", got)
		}
	}()
	_ = NewBackend(BackendReal, realTestEnv(), w.DesiredWorld{})
}

func TestNewBackendWithOptionsRealUsesSigWMAdapter(t *testing.T) {
	backend, err := NewBackendWithOptions(BackendReal, realTestEnv(), w.DesiredWorld{}, BackendOptions{Real: RealBackendOptions{AllowLive: true}})
	if err != nil {
		t.Fatalf("NewBackendWithOptions: %v", err)
	}
	if backend.Kind != BackendReal {
		t.Fatalf("kind = %s, want real", backend.Kind)
	}
	if backend.Fake != nil {
		t.Fatalf("real backend must not expose fake adapter: %+v", backend.Fake)
	}
	if _, ok := backend.Adapter.(*wm.SigWM); !ok {
		t.Fatalf("adapter = %T, want *wm.SigWM", backend.Adapter)
	}
}

func TestNewBackendWithOptionsRealRejectsFakeFixtureEnvironment(t *testing.T) {
	env := realTestEnv()
	env.WindowManager.Backend = "fake"
	_, err := NewBackendWithOptions(BackendReal, env, w.DesiredWorld{}, BackendOptions{Real: RealBackendOptions{AllowLive: true}})
	if err == nil || !strings.Contains(err.Error(), "requires real/omniwm") {
		t.Fatalf("expected fake fixture env rejection, got %v", err)
	}
}

func TestNewBackendWithOptionsRealRequiresAllowLive(t *testing.T) {
	_, err := NewBackendWithOptions(BackendReal, realTestEnv(), w.DesiredWorld{}, BackendOptions{})
	if !errors.Is(err, ErrRealBackendRequiresOptions) {
		t.Fatalf("error = %v, want ErrRealBackendRequiresOptions", err)
	}
}

func TestNewBackendWithOptionsRealRejectsNonNixAuthority(t *testing.T) {
	env := realTestEnv()
	env.Authority = "test"
	_, err := NewBackendWithOptions(BackendReal, env, w.DesiredWorld{}, BackendOptions{Real: RealBackendOptions{AllowLive: true}})
	if err == nil || !strings.Contains(err.Error(), "requires nix-authorized") {
		t.Fatalf("expected non-nix authority rejection, got %v", err)
	}
}

func TestNewBackendWithOptionsRealRunsPreflight(t *testing.T) {
	lifecycle := &recordingLifecycle{preflightErr: errors.New("not safe")}
	_, err := NewBackendWithOptions(BackendReal, realTestEnv(), w.DesiredWorld{}, BackendOptions{Real: RealBackendOptions{AllowLive: true, Lifecycle: lifecycle}})
	if err == nil || !errors.Is(err, ErrRealBackendPreflight) || !lifecycle.preflightRan {
		t.Fatalf("expected preflight error, ran=%v err=%v", lifecycle.preflightRan, err)
	}
}

func TestRunOnRealBackendUsesInjectedAdapter(t *testing.T) {
	env, desired := realRunnableFixture()
	fake := wm.NewFake(env)
	lifecycle := &recordingLifecycle{}
	ran := false
	RunOnRealBackend(t, func() (w.ManagedEnvironment, w.DesiredWorld) {
		return env, desired
	}, Scenario{
		Name: "noop-real-entry",
		Steps: []Step{{
			Name:           "noop",
			SkipFinalFocus: true,
			Apply: func(t *testing.T, b *Backend) {
				ran = true
				if b.Kind != BackendReal {
					t.Fatalf("backend kind = %s, want real", b.Kind)
				}
				if b.Adapter != fake {
					t.Fatalf("adapter = %T, want injected fake adapter", b.Adapter)
				}
				if b.Fake != nil {
					t.Fatalf("real backend must not expose scenario fake: %+v", b.Fake)
				}
			},
		}},
	}, RealBackendOptions{AllowLive: true, Adapter: fake, Lifecycle: lifecycle})
	if !ran {
		t.Fatal("scenario step did not run")
	}
	if !lifecycle.preflightRan || !lifecycle.teardownRan {
		t.Fatalf("lifecycle preflight=%v teardown=%v, want both", lifecycle.preflightRan, lifecycle.teardownRan)
	}
}

func TestRunOnAllBackendsExcludesRealBackend(t *testing.T) {
	seen := map[BackendKind]bool{}
	RunOnAllBackends(t, fakeRunnableFixture, Scenario{
		Name: "ordinary-unit-entry",
		Steps: []Step{{
			Name:           "noop",
			SkipFinalFocus: true,
			Apply: func(t *testing.T, b *Backend) {
				seen[b.Kind] = true
				if b.Kind == BackendReal {
					t.Fatal("ordinary scenario runner must not execute real backend")
				}
			},
		}},
	})
	if !seen[BackendFake] || !seen[BackendSimulator] {
		t.Fatalf("seen backends = %v, want fake and simulator", seen)
	}
}

func realTestEnv() w.ManagedEnvironment {
	return w.ManagedEnvironment{
		SchemaVersion: 1,
		Authority:     "nix",
		WindowManager: w.WindowManagerEnvironment{Backend: "omniwm"},
	}
}

func realRunnableFixture() (w.ManagedEnvironment, w.DesiredWorld) {
	env := realTestEnv()
	env.Workspaces.Workspaces = []w.WorkspaceSpec{{ID: "ws-empty", Role: w.WorkspaceGeneral}}
	desired := w.DesiredWorld{
		ActiveProfile: "empty",
		Profiles: map[w.ProfileID]w.DesiredProfile{
			"empty": {ID: "empty", Assignments: map[w.SlotID]w.ProjectID{}, InactivePolicy: w.InactivePolicyRemove},
		},
		Projects: map[w.ProjectID]w.DesiredProject{},
	}
	return env, desired
}

func fakeRunnableFixture() (w.ManagedEnvironment, w.DesiredWorld) {
	env, desired := realRunnableFixture()
	env.Authority = "test-fixture"
	env.WindowManager.Backend = "fake"
	return env, desired
}

type recordingLifecycle struct {
	preflightRan bool
	teardownRan  bool
	preflightErr error
}

func (l *recordingLifecycle) Preflight(ctx context.Context, env w.ManagedEnvironment) error {
	l.preflightRan = true
	return l.preflightErr
}

func (l *recordingLifecycle) Teardown(ctx context.Context, env w.ManagedEnvironment) error {
	l.teardownRan = true
	return nil
}
