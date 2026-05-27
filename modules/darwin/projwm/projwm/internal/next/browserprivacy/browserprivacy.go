package browserprivacy

import (
	"encoding/json"
	"errors"
	"strings"
)

type PrivatePayloadRef string

type Descriptor struct {
	Class       string  `json:"class"`
	OriginHMAC  *string `json:"originHMAC,omitempty"`
	HasQuery    bool    `json:"hasQuery"`
	HasFragment bool    `json:"hasFragment"`
}

type SensitiveURL struct {
	raw  string
	Ref  *PrivatePayloadRef
	Safe Descriptor
}

type SensitiveTitle struct {
	raw  string
	Ref  *PrivatePayloadRef
	Safe Descriptor
}

type ObservedTab struct {
	ID     string
	URL    SensitiveURL
	Title  SensitiveTitle
	Active bool
}

type PersistentTab struct {
	ID      string             `json:"id"`
	Active  bool               `json:"active"`
	URL     Descriptor         `json:"url"`
	Title   Descriptor         `json:"title"`
	Content *PrivatePayloadRef `json:"content,omitempty"`
}

type SafeLogEvent struct {
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields"`
}

type IPCBrowserTab struct {
	ID     string     `json:"id"`
	Active bool       `json:"active"`
	URL    Descriptor `json:"url"`
	Title  Descriptor `json:"title"`
}

type DiagnosticsTab struct {
	ID      string     `json:"id"`
	URL     Descriptor `json:"url"`
	Title   Descriptor `json:"title"`
	Private bool       `json:"private"`
}

type Policy struct {
	PersistPrivateURLPayload   bool
	PersistPrivateTitlePayload bool
}

var ErrRawBrowserContent = errors.New("raw browser content must not be serialized")

func NewSensitiveURL(raw string, safe Descriptor, ref *PrivatePayloadRef) SensitiveURL {
	return SensitiveURL{raw: raw, Safe: safe, Ref: ref}
}

func NewSensitiveTitle(raw string, safe Descriptor, ref *PrivatePayloadRef) SensitiveTitle {
	return SensitiveTitle{raw: raw, Safe: safe, Ref: ref}
}

func (u SensitiveURL) String() string {
	return "[redacted-url]"
}

func (t SensitiveTitle) String() string {
	return "[redacted-title]"
}

func SnapshotTab(tab ObservedTab, p Policy) PersistentTab {
	var ref *PrivatePayloadRef
	if p.PersistPrivateURLPayload {
		ref = tab.URL.Ref
	}
	return PersistentTab{
		ID:      tab.ID,
		Active:  tab.Active,
		URL:     tab.URL.Safe,
		Title:   tab.Title.Safe,
		Content: ref,
	}
}

func LogTabObserved(tab ObservedTab) SafeLogEvent {
	return SafeLogEvent{
		Message: "browser.tab.observed",
		Fields: map[string]any{
			"tabID":  tab.ID,
			"url":    tab.URL.Safe,
			"title":  tab.Title.Safe,
			"active": tab.Active,
		},
	}
}

func IPCSnapshot(tab ObservedTab) IPCBrowserTab {
	return IPCBrowserTab{
		ID:     tab.ID,
		Active: tab.Active,
		URL:    tab.URL.Safe,
		Title:  tab.Title.Safe,
	}
}

func DiagnosticsSnapshot(tab ObservedTab) DiagnosticsTab {
	return DiagnosticsTab{
		ID:      tab.ID,
		URL:     tab.URL.Safe,
		Title:   tab.Title.Safe,
		Private: true,
	}
}

func MarshalPersistent(v any, forbidden ...string) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	for _, f := range forbidden {
		if f != "" && strings.Contains(string(b), f) {
			return nil, ErrRawBrowserContent
		}
	}
	return b, nil
}
