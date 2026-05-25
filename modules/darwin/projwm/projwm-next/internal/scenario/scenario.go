// Package scenario provides a Step-driven scenario harness over a backend.
// design.md §15. Used by scenarios/*_test.go to run the same Steps on fake/simulator.
package scenario

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	"github.com/yuu-th/projwm-next/internal/controller"
	"github.com/yuu-th/projwm-next/internal/event"
	"github.com/yuu-th/projwm-next/internal/intent"
	"github.com/yuu-th/projwm-next/internal/invariant"
	"github.com/yuu-th/projwm-next/internal/store"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Backend kinds.
type BackendKind string

const (
	BackendFake      BackendKind = "fake"
	BackendSimulator BackendKind = "simulator"
	BackendRecorded  BackendKind = "recorded"
	BackendReal      BackendKind = "real"
)

// Step is a single scenario step. design.md §15.
type Step struct {
	Name    string
	Apply   func(t *testing.T, b *Backend) // arbitrary mutation: send intent / event / inject
	Command string                         // optional override; otherwise inferred from intent
	// SkipFinalFocus disables Invariant 10 for this step (used when no command policy applies).
	SkipFinalFocus bool
}

// Scenario is a sequence of Steps; each Step's ExpectedInvariants = §2 全項目.
type Scenario struct {
	Name  string
	Setup func(b *Backend)
	Steps []Step
}

var (
	ErrRealBackendRequiresOptions = errors.New("scenario: BackendReal requires explicit NewBackendWithOptions opt-in")
	ErrRealBackendPreflight       = errors.New("scenario: real backend preflight failed")
)

type RealDriverLifecycle interface {
	Preflight(ctx context.Context, env w.ManagedEnvironment) error
	Teardown(ctx context.Context, env w.ManagedEnvironment) error
}

type BackendOptions struct {
	Real RealBackendOptions
}

type RealBackendOptions struct {
	AllowLive bool
	Adapter   wm.Adapter
	Lifecycle RealDriverLifecycle
}

// Backend wires an adapter + Controller + MemoryStore for one scenario instance.
type Backend struct {
	Kind       BackendKind
	Env        w.ManagedEnvironment
	Adapter    wm.Adapter
	Fake       *wm.Fake
	Store      *store.MemoryStore
	Controller *controller.Controller
	Lifecycle  RealDriverLifecycle
	// LastCommand is the command key used by the last apply call (for invariant 10).
	LastCommand string
}

// NewBackend constructs a non-real backend. BackendReal is intentionally
// fail-closed here so tests cannot accidentally perform live WM mutations.
func NewBackend(kind BackendKind, env w.ManagedEnvironment, desired w.DesiredWorld) *Backend {
	if kind == BackendReal {
		panic(ErrRealBackendRequiresOptions)
	}
	backend, err := NewBackendWithOptions(kind, env, desired, BackendOptions{})
	if err != nil {
		panic(err)
	}
	return backend
}

func NewBackendWithOptions(kind BackendKind, env w.ManagedEnvironment, desired w.DesiredWorld, opts BackendOptions) (*Backend, error) {
	var fake *wm.Fake
	var adapter wm.Adapter
	var lifecycle RealDriverLifecycle
	switch kind {
	case BackendReal:
		if err := validateRealBackendOptions(env, opts.Real); err != nil {
			return nil, err
		}
		adapter = opts.Real.Adapter
		if adapter == nil {
			adapter = wm.NewSigWM(env, nil, nil)
		}
		lifecycle = opts.Real.Lifecycle
		if lifecycle != nil {
			if err := lifecycle.Preflight(context.Background(), env); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrRealBackendPreflight, err)
			}
		}
	default:
		fake = wm.NewFake(env)
		adapter = fake
	}
	st := store.NewMemoryStore(desired)
	ctrl := controller.New(env, desired, adapter, st)
	if kind == BackendSimulator {
		ctrl.SetUseSimulator(true)
	}
	return &Backend{Kind: kind, Env: env, Adapter: adapter, Fake: fake, Store: st, Controller: ctrl, Lifecycle: lifecycle}, nil
}

func validateRealBackendOptions(env w.ManagedEnvironment, opts RealBackendOptions) error {
	if !opts.AllowLive {
		return ErrRealBackendRequiresOptions
	}
	switch env.WindowManager.Backend {
	case "omniwm", "real":
	default:
		return fmt.Errorf("scenario: BackendReal requires real/omniwm environment backend, got %q", env.WindowManager.Backend)
	}
	if env.Authority != "nix" {
		return fmt.Errorf("scenario: BackendReal requires nix-authorized ManagedEnvironment, got %q", env.Authority)
	}
	return nil
}

// ApplyIntent sends an intent through the controller and records the command key.
func (b *Backend) ApplyIntent(in intent.Intent) error {
	b.LastCommand = inferCommand(in)
	_, err := b.Controller.ApplyIntent(context.Background(), in)
	return err
}

// ApplyEvent sends an event.
func (b *Backend) ApplyEvent(ev event.Event) error {
	b.LastCommand = "event:external"
	_, err := b.Controller.ApplyEvent(context.Background(), ev)
	return err
}

func inferCommand(in intent.Intent) string {
	return "intent:" + string(in.Kind())
}

// Run executes the scenario on the given backend, asserting all invariants after each Step.
func (s Scenario) Run(t *testing.T, b *Backend) {
	t.Helper()
	if b.Lifecycle != nil {
		t.Cleanup(func() {
			if err := b.Lifecycle.Teardown(context.Background(), b.Env); err != nil {
				t.Errorf("real backend teardown: %v", err)
			}
		})
	}
	if s.Setup != nil {
		s.Setup(b)
	}
	for _, step := range s.Steps {
		step := step
		t.Run(fmt.Sprintf("[%s] %s/%s", b.Kind, s.Name, step.Name), func(t *testing.T) {
			step.Apply(t, b)
			cmd := step.Command
			if cmd == "" {
				cmd = b.LastCommand
			}
			if step.SkipFinalFocus {
				cmd = ""
			}
			vs := invariant.CheckAll(b.Controller.State(), invariant.CheckOptions{FinalFocusCommandKey: cmd})
			if len(vs) > 0 {
				for _, v := range vs {
					t.Errorf("invariant violation: %s", v)
				}
			}
		})
	}
}

// RunOnAllBackends is a helper that runs the scenario on both fake & simulator backends.
func RunOnAllBackends(t *testing.T, makeEnv func() (w.ManagedEnvironment, w.DesiredWorld), s Scenario) {
	t.Helper()
	for _, kind := range []BackendKind{BackendFake, BackendSimulator} {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			env, desired := makeEnv()
			b := NewBackend(kind, env, desired)
			s.Run(t, b)
		})
	}
}

// RunOnRealBackend is an explicit opt-in entry point for future Human-operation
// scenario runs. It is intentionally separate from RunOnAllBackends so ordinary
// unit tests never touch the live window manager by accident.
func RunOnRealBackend(t *testing.T, makeEnv func() (w.ManagedEnvironment, w.DesiredWorld), s Scenario, opts RealBackendOptions) {
	t.Helper()
	t.Run(string(BackendReal), func(t *testing.T) {
		env, desired := makeEnv()
		b, err := NewBackendWithOptions(BackendReal, env, desired, BackendOptions{Real: opts})
		if err != nil {
			t.Fatalf("NewBackendWithOptions(real): %v", err)
		}
		s.Run(t, b)
	})
}
