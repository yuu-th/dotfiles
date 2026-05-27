//go:build integration

package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/yuu-th/projwm/internal/omniwm"
)

func TestIntegrationOmniWMQuerySmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wins, err := omniwm.New(nil).QueryWindows(ctx)
	if err != nil {
		t.Fatalf("OmniWM query smoke failed: %v", err)
	}
	t.Logf("OmniWM query smoke observed %d windows", len(wins))
}
