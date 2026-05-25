// Package observer declares the Observer contract.
//
// design.md §3.6 / §13: Observer は実世界を読むだけで、mutation してはいけない。
// implementation-design.md §6 で「最初に必要な責務」として
// windows/workspaces/focus/layout/display observation が挙げられている。
//
// The current real path observes through wm.Adapter.Observe; this interface
// remains the seam for future read-only observer extraction.
package observer

import (
	"context"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// Observer collects ObservedWorld from the live system without mutation.
//
// 実装契約:
//
//   - mutation を一切行わない（adapter を mutate 経路で呼ばない）
//   - キャッシュしてもよいが、ObserveWorld 結果は最新エポック分だけを返す
//   - context cancellation で速やかに中断する
type Observer interface {
	ObserveWorld(ctx context.Context) (w.ObservedWorld, error)
}
