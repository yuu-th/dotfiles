package reconcile

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// Zed spawn lock: 並走する reconcile process（launchd watch / periodic / CLI）が
// 同時に同じ project の Zed window を spawn しようとして無限ループするバグの対策。
// 単一プロセス内 mutex では足りないので flock(2) を使う。
//
// lock 名は title から hash で derive。lock dir は ~/.cache/projwm/.locks/

var heldZedLocks = map[string]*flock.Flock{}

func zedLockPath(title string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".cache", "projwm", ".locks")
	_ = os.MkdirAll(dir, 0o755)
	h := sha1.Sum([]byte(title))
	return filepath.Join(dir, "zed-spawn-"+hex.EncodeToString(h[:6])+".lock")
}

// acquireZedSpawnLock は flock を timeout 付きで取得。取れたら true。
// 成功時は heldZedLocks に登録、releaseZedSpawnLock で解放する。
func acquireZedSpawnLock(title string, timeout time.Duration) bool {
	path := zedLockPath(title)
	lock := flock.New(path)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ok, err := lock.TryLock()
		if err == nil && ok {
			heldZedLocks[title] = lock
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

func releaseZedSpawnLock(title string) {
	if lock, ok := heldZedLocks[title]; ok {
		_ = lock.Unlock()
		delete(heldZedLocks, title)
	}
}
