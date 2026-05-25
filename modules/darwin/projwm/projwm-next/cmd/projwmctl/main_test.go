package main

import (
	"strings"
	"testing"

	"github.com/yuu-th/projwm-next/internal/ipc"
	w "github.com/yuu-th/projwm-next/internal/world"
)

func TestFormatIntentResponseRedactsToRequestAndCommitEvidence(t *testing.T) {
	tx := w.TransactionID("txn-privacy")
	gen := w.GenerationID("G000002")
	out := formatIntentResponse(ipc.IntentResponse{
		RequestID:           "req-privacy",
		AcceptedTransaction: &tx,
		CommittedGeneration: &gen,
	})
	if out != "ok request=req-privacy acceptedTransaction=txn-privacy committedGeneration=G000002\n" {
		t.Fatalf("unexpected response: %q", out)
	}
	for _, secret := range []string{"https://secret.example", "browser-payload-v1-", "raw-browser-secret"} {
		if strings.Contains(out, secret) {
			t.Fatalf("projwmctl output leaked browser secret material %q: %s", secret, out)
		}
	}
}
