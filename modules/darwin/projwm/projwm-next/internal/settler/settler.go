// Package settler waits until adapter observation stabilizes after a mutation.
// For fake/simulator backends, observation is immediate (no async settling).
// design.md §11.
package settler

import (
	"context"

	"github.com/yuu-th/projwm-next/internal/adapter/wm"
	w "github.com/yuu-th/projwm-next/internal/world"
)

type Settler struct {
	Adapter wm.Adapter
}

// Settle returns the latest ObservedWorld. For fake/simulator, returns immediately.
func (s *Settler) Settle(ctx context.Context) (w.ObservedWorld, error) {
	return s.Adapter.Observe(ctx)
}
