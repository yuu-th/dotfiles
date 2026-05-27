package ipc

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/yuu-th/projwm/internal/next/store"
)

func TestMutationThroughWriterSucceeds(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(st)
	resp, err := srv.Handle(context.Background(), Request{
		Method:             MethodMutateDesiredWorld,
		ExpectedGeneration: store.InitialGeneration,
		Actor:              "projwmctl",
		Handshake:          validHandshake(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Generation != "E0001-G000001" {
		t.Fatalf("generation = %q", resp.Generation)
	}
}

func TestDirectMutationRejected(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(st)
	_, err = srv.Handle(context.Background(), Request{
		Method:             MethodMutateDesiredWorld,
		ExpectedGeneration: store.InitialGeneration,
		Actor:              "test",
		Direct:             true,
		Handshake:          validHandshake(),
	})
	if !errors.Is(err, ErrDirectMutation) {
		t.Fatalf("err = %v, want ErrDirectMutation", err)
	}
}

func TestStaleGenerationRejected(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(st)
	if _, err := srv.Handle(context.Background(), Request{
		Method:             MethodMutateDesiredWorld,
		ExpectedGeneration: store.InitialGeneration,
		Actor:              "first",
		Handshake:          validHandshake(),
	}); err != nil {
		t.Fatal(err)
	}
	_, err = srv.Handle(context.Background(), Request{
		Method:             MethodMutateDesiredWorld,
		ExpectedGeneration: store.InitialGeneration,
		Actor:              "stale",
		Handshake:          validHandshake(),
	})
	if !errors.Is(err, store.ErrStaleGeneration) {
		t.Fatalf("err = %v, want ErrStaleGeneration", err)
	}
}

func TestUnknownMethodRejected(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(st)
	if _, err := srv.Handle(context.Background(), Request{Method: "repairState", Handshake: validHandshake()}); !errors.Is(err, ErrUnknownMethod) {
		t.Fatalf("err = %v, want ErrUnknownMethod", err)
	}
}

func TestProtocolMismatchRejected(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(st)
	_, err = srv.Handle(context.Background(), Request{
		Method:             MethodReadWorld,
		ExpectedGeneration: store.InitialGeneration,
		Actor:              "projwmctl",
		Handshake: Handshake{
			ProtocolVersion:              2,
			DaemonVersion:                "projwmctl",
			ManagedEnvironmentGeneration: "env-test",
			StoreSchemaVersion:           1,
		},
	})
	if !errors.Is(err, ErrProtocolMismatch) {
		t.Fatalf("err = %v, want ErrProtocolMismatch", err)
	}
}

func TestSidecarCanOnlySendEventHint(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(st)
	if _, err := srv.Handle(context.Background(), Request{
		Method:             MethodEventHint,
		ExpectedGeneration: store.InitialGeneration,
		Actor:              "sidecar",
		Handshake:          validHandshake(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Handle(context.Background(), Request{
		Method:             MethodMutateDesiredWorld,
		ExpectedGeneration: store.InitialGeneration,
		Actor:              "sidecar",
		Handshake:          validHandshake(),
	}); !errors.Is(err, ErrSidecarWrite) {
		t.Fatalf("err = %v, want ErrSidecarWrite", err)
	}
}

func TestErrorTaxonomyIncludesRequiredKinds(t *testing.T) {
	required := []ErrorKind{
		ErrorSocketAbsent,
		ErrorConnectionRefused,
		ErrorTimeout,
		ErrorDaemonBusy,
		ErrorProtocolMismatch,
		ErrorIntentRejected,
		ErrorTransactionFailed,
		ErrorUnsupported,
	}
	for _, kind := range required {
		if kind == "" {
			t.Fatal("empty error kind")
		}
	}
}

func TestConcurrentMutationsAreSerializedByGeneration(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(st)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, g := range []store.Generation{store.InitialGeneration, "E0001-G000001"} {
		wg.Add(1)
		go func(expected store.Generation) {
			defer wg.Done()
			_, err := srv.Handle(context.Background(), Request{
				Method:             MethodMutateDesiredWorld,
				ExpectedGeneration: expected,
				Actor:              string(expected),
				Handshake:          validHandshake(),
			})
			errs <- err
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, store.ErrStaleGeneration) {
			t.Fatalf("unexpected err = %v", err)
		}
	}
	current, err := st.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current != "E0001-G000002" && current != "E0001-G000001" {
		t.Fatalf("unexpected current generation %q", current)
	}
}

func validHandshake() Handshake {
	return Handshake{
		ProtocolVersion:              1,
		DaemonVersion:                "projwmctl-test",
		ManagedEnvironmentGeneration: "env-test",
		StoreSchemaVersion:           1,
	}
}
