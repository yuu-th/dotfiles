// Package browser declares the browser-capability adapter contract.
//
// design.md §6 (capability-based apps) / §7 (adapters) で
// browser は `CapabilityBrowser` を持つ別 adapter として分離される。
// implementation-design.md §6 でも browser は「window/tab/profile observation
// and snapshot」を最初に必要とする adapter として列挙されている。
//
// Phase D の browser privacy boundary / real adapter 実装の入口。
package browser

import (
	"context"

	w "github.com/yuu-th/projwm-next/internal/world"
)

// WindowSnapshot は browser window の observation。
// Title / URL は private payload に該当しうるため WindowSnapshot 自体には
// URL を持たせず、PrivatePayloadStore 側で opaque token と共に管理する。
type WindowSnapshot struct {
	WindowID        w.LiveWindowID
	BrowserWindowID string
	Workspace       w.WorkspaceID
	ProfileID       string
	TabCount        int
	// PayloadToken は PrivatePayloadStore で URL/tab list を引くための opaque key。
	PayloadToken string
}

type OpenResult struct {
	BrowserWindowID string
	// LiveWindow は WindowManagerAdapter 側で観測される LiveWindowID。
	// browser-window-close 契約では BrowserWindowID と LiveWindow が同一値
	// （omniwm の window id を共有）の場合もあるが、概念は別物として保持する。
	LiveWindow w.LiveWindowID
}

// BrowserCapabilityAdapter は CapabilityBrowser を持つ ManagedApp の操作面。
//
// design.md §7 の通り adapter は外部 system の癖を隠すが truth を捏造しない。
// Phase D で Chromium / Safari / Arc 等を別実装で satisfy する想定。
type BrowserCapabilityAdapter interface {
	// ObserveWindows は現在の browser windows を返す。mutation 禁止。
	ObserveWindows(ctx context.Context) ([]WindowSnapshot, error)

	// FocusWindow は browser 側に focus を依頼する。
	// WM 側 focus と同期するのは Controller の責任。
	FocusWindow(ctx context.Context, id w.LiveWindowID) error

	// OpenInProfile は profile を指定して URL set を開く。
	// URL は PrivatePayloadStore の token で渡し、API 引数に直接出さない。
	// 戻り値は browser 側 ID であり、WM の LiveWindowID ではない。空の
	// BrowserWindowID は「未相関」を意味し、成功した WM 相関として扱ってはいけない。
	OpenInProfile(ctx context.Context, profile string, payloadToken string) (OpenResult, error)

	// CloseWindow は browser window を閉じる。
	CloseWindow(ctx context.Context, id w.LiveWindowID) error
}

// PrivatePayloadStore は URL / cookie / form data 等 PII になりうる payload を
// daemon process 内のメモリだけに保持し、log / store / IPC envelope に流さないための
// 隔離層。
//
// design.md §6 (browser private payload) と implementation-design.md §6
// (browser snapshot の取り扱い) で「URL を直接 store / log に書かない」要件があり、
// Phase D で実装する。
type PrivatePayloadStore interface {
	// Put は payload を保存し、opaque token を返す。
	// token は browser adapter API / IPC では渡せるが、
	// store には opaque ref としてだけ置ける。log / telemetry には流してはいけない。
	Put(ctx context.Context, payload PrivatePayload) (token string, err error)

	// Get は token から payload を取り出す。
	// 取り出された payload は呼出側が短命に扱うこと。
	Get(ctx context.Context, token string) (PrivatePayload, error)

	// Forget は token を破棄する。session 終了 / profile switch で必須。
	Forget(ctx context.Context, token string) error
}

// PrivatePayload は browser に渡す機微データ。fields は Phase D で拡張する。
type PrivatePayload struct {
	URLs []string
	// Cookies / form data などは Phase D で追加。
}
