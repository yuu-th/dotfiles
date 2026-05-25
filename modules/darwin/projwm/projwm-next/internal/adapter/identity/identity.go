// Package identity declares the identity-resolver adapter contract.
//
// 既存の `internal/identity` package は pure resolver (TitleContract + match
// hints から DesiredWindow → LiveWindow を分類する関数) を提供する。
// このパッケージはその外側、すなわち「resolver を Observer や session/browser
// adapter から得た observation に bind する」境界を切り分ける。
//
// design.md §4–§7 / §11 (identity-driven planning) に対応。
//
// fake / real を marker で分離し、production path に fake resolver が紛れ込む
// ことを防ぐ。
package identity

import (
	"context"

	pid "github.com/yuu-th/projwm-next/internal/identity"
	w "github.com/yuu-th/projwm-next/internal/world"
)

// Resolver は1つの DesiredWindow を ObservedWorld の中で分類する。
// The pure internal/identity resolver satisfies this shape.
type Resolver interface {
	Resolve(ctx context.Context, desired w.DesiredWindow, observed w.ObservedWorld) (pid.Resolution, error)
}

// FakeResolver は test/simulator 用。pure 関数として `internal/identity.Resolve`
// を直接呼ぶだけのため、separate interface tag を付けて real adapter と区別する。
type FakeResolver interface {
	Resolver
	// fakeResolverMarker は real adapter から fake を取り違えないための marker。
	fakeResolverMarker()
}

// RealResolver は OS 側 observation (mac WindowServer, x11 properties など) と
// session/browser adapter からの hint を組み合わせる。Phase D 以降に実装する。
type RealResolver interface {
	Resolver
	// realResolverMarker は test 用 fake を production path に紛れ込ませない marker。
	realResolverMarker()
}

type fakeResolver struct{}

func NewFakeResolver() FakeResolver { return fakeResolver{} }

func (fakeResolver) Resolve(ctx context.Context, desired w.DesiredWindow, observed w.ObservedWorld) (pid.Resolution, error) {
	if err := ctx.Err(); err != nil {
		return pid.Resolution{}, err
	}
	return pid.Resolve(desired, observed), nil
}

func (fakeResolver) fakeResolverMarker() {}

type realResolver struct{}

func NewRealResolver() RealResolver { return realResolver{} }

func (realResolver) Resolve(ctx context.Context, desired w.DesiredWindow, observed w.ObservedWorld) (pid.Resolution, error) {
	if err := ctx.Err(); err != nil {
		return pid.Resolution{}, err
	}
	return pid.Resolve(desired, observed), nil
}

func (realResolver) realResolverMarker() {}
