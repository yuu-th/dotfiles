package ghosttywrap

import (
	"context"
	"testing"
)

// fakeSpawner はテスト用 Spawner（インターフェース実装の確認）。
type fakeSpawner struct{ calls []string }

func (f *fakeSpawner) Spawn(_ context.Context, title, cwd, session string) error {
	f.calls = append(f.calls, title+"|"+cwd+"|"+session)
	return nil
}

// インターフェースを満たすことのコンパイル時保証。
var _ Spawner = (*fakeSpawner)(nil)
var _ Spawner = CmdSpawner{}

func TestSpawnerInterfaceCompiles(t *testing.T) {
	var s Spawner = &fakeSpawner{}
	if err := s.Spawn(context.Background(), "ai-1:x", "/tmp", "ai-1/x"); err != nil {
		t.Fatal(err)
	}
}
