package browser

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FilePrivatePayloadStore struct {
	root string
}

func NewFilePrivatePayloadStore(root string) (*FilePrivatePayloadStore, error) {
	if root == "" {
		return nil, errors.New("browser/private-store: root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("browser/private-store: resolve root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("browser/private-store: create root: %w", err)
	}
	if err := os.Chmod(abs, 0o700); err != nil {
		return nil, fmt.Errorf("browser/private-store: secure root: %w", err)
	}
	return &FilePrivatePayloadStore{root: abs}, nil
}

func (s *FilePrivatePayloadStore) Put(ctx context.Context, payload PrivatePayload) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	token, err := newPayloadToken()
	if err != nil {
		return "", err
	}
	if err := writePrivatePayloadFile(s.pathForToken(token), payload); err != nil {
		return "", err
	}
	return token, nil
}

func (s *FilePrivatePayloadStore) Get(ctx context.Context, token string) (PrivatePayload, error) {
	if err := ctx.Err(); err != nil {
		return PrivatePayload{}, err
	}
	var payload PrivatePayload
	if err := readPrivatePayloadFile(s.pathForToken(token), &payload); err != nil {
		return PrivatePayload{}, err
	}
	return payload, nil
}

func (s *FilePrivatePayloadStore) Forget(ctx context.Context, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := s.pathForToken(token)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("browser/private-store: forget payload: %w", err)
	}
	return nil
}

func (s *FilePrivatePayloadStore) pathForToken(token string) string {
	if !validPayloadToken(token) {
		return filepath.Join(s.root, "__invalid_token__")
	}
	return filepath.Join(s.root, token+".json")
}

func newPayloadToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("browser/private-store: generate token: %w", err)
	}
	return "browser-payload-v1-" + hex.EncodeToString(b[:]), nil
}

func validPayloadToken(token string) bool {
	const prefix = "browser-payload-v1-"
	if !strings.HasPrefix(token, prefix) || len(token) != len(prefix)+32 {
		return false
	}
	for _, r := range token[len(prefix):] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func writePrivatePayloadFile(path string, payload PrivatePayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("browser/private-store: marshal payload: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("browser/private-store: write payload: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("browser/private-store: commit payload: %w", err)
	}
	return nil
}

func readPrivatePayloadFile(path string, payload *PrivatePayload) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("browser/private-store: read payload: %w", err)
	}
	if err := json.Unmarshal(data, payload); err != nil {
		return fmt.Errorf("browser/private-store: parse payload: %w", err)
	}
	return nil
}

var _ PrivatePayloadStore = (*FilePrivatePayloadStore)(nil)
