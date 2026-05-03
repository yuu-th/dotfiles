package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/yuu-th/projwm/internal/config"
	"github.com/yuu-th/projwm/internal/reconcile"
)

// runReconcileOnce は state を読み直して reconcile を 1 回実行する。
// profile switch / up / archive 等の mutate 後に呼ぶ共通ヘルパ。
func runReconcileOnce() error {
	cfgRes, err := config.LoadFromDefaultPath()
	if err != nil {
		return err
	}
	_, st, err := loadStore()
	if err != nil {
		return err
	}
	r := reconcile.New(cfgRes.Config)
	acts, err := r.Run(context.Background(), st, reconcile.Options{Logger: os.Stderr})
	if err != nil {
		return err
	}
	errCount := 0
	for _, a := range acts {
		if a.OnError != nil {
			errCount++
			fmt.Fprintf(os.Stderr, "  ERROR %s %s: %v\n", a.Op, a.Target, a.OnError)
		}
	}
	if errCount > 0 {
		return fmt.Errorf("%d action(s) failed during reconcile", errCount)
	}
	fmt.Fprintf(os.Stderr, "reconcile: %d action(s) executed\n", len(acts))
	return nil
}
