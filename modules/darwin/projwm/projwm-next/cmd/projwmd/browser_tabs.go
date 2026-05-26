package main

import (
	"context"

	"github.com/yuu-th/projwm-next/internal/adapter/browser"
	"github.com/yuu-th/projwm-next/internal/adapter/observer"
	"github.com/yuu-th/projwm-next/internal/controller"
	"github.com/yuu-th/projwm-next/internal/intent"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// controllerIntentAdapter wraps controller.Controller so the observer
// package (which doesn't import cmd/projwmd types) can submit internal
// intents.
type controllerIntentAdapter struct {
	ctrl *controller.Controller
}

func (c *controllerIntentAdapter) ApplyIntent(ctx context.Context, in intent.Intent) error {
	_, err := c.ctrl.ApplyIntent(ctx, in)
	return err
}

// controllerWorldAdapter wraps controller for the observer to learn
// which project is active.
type controllerWorldAdapter struct {
	ctrl *controller.Controller
}

func (c *controllerWorldAdapter) ActiveProject() (w.ProjectID, bool) {
	st := c.ctrl.State()
	prof, ok := st.Desired.Profiles[st.Desired.ActiveProfile]
	if !ok {
		return "", false
	}
	// Pick the first assigned project under slot-order (deterministic).
	for _, sl := range st.Environment.SlotOrder() {
		if pid, ok := prof.Assignments[sl]; ok && pid != "" {
			return pid, true
		}
	}
	return "", false
}

// privatePayloadAllocator wraps PrivatePayloadStore.Put so the observer
// can allocate a new token for each URL set.
type privatePayloadAllocator struct {
	store browser.PrivatePayloadStore
}

func (a *privatePayloadAllocator) Allocate(project w.ProjectID, urls []string) (w.PrivatePayloadRef, int, int, error) {
	invalid := 0
	clean := make([]string, 0, len(urls))
	for _, u := range urls {
		if u == "" {
			invalid++
			continue
		}
		clean = append(clean, u)
	}
	token, err := a.store.Put(context.Background(), browser.PrivatePayload{URLs: clean})
	if err != nil {
		return "", 0, 0, err
	}
	return w.PrivatePayloadRef(token), len(clean), invalid, nil
}

// vivaldiInspectorAdapter bridges browser.VivaldiAdapter's
// InspectTabsByWindow (which returns []browser.WindowTabs) to the
// observer-package interface (which returns []observer.WindowSnapshot
// to avoid an import cycle).
type vivaldiInspectorAdapter struct {
	v *browser.VivaldiAdapter
}

func (vi *vivaldiInspectorAdapter) InspectTabsByWindow(ctx context.Context) ([]observer.WindowSnapshot, error) {
	wins, err := vi.v.InspectTabsByWindow(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]observer.WindowSnapshot, len(wins))
	for i, w := range wins {
		out[i] = observer.WindowSnapshot{Title: w.Title, URLs: w.URLs}
	}
	return out, nil
}

// newBrowserTabsObserver constructs the Tier 3 observer with all the
// daemon-side glue ready. Returns nil if pre-reqs are missing.
func newBrowserTabsObserver(ctrl *controller.Controller, vivaldi *browser.VivaldiAdapter, privateStore browser.PrivatePayloadStore) *observer.BrowserTabsSync {
	if ctrl == nil || vivaldi == nil || privateStore == nil {
		return nil
	}
	return &observer.BrowserTabsSync{
		Vivaldi:   &vivaldiInspectorAdapter{v: vivaldi},
		Submitter: &controllerIntentAdapter{ctrl: ctrl},
		World:     &controllerWorldAdapter{ctrl: ctrl},
		Allocator: &privatePayloadAllocator{store: privateStore},
	}
}
