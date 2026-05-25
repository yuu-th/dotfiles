package clientauth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yuu-th/projwm-next/internal/manifest"
)

// VerifyManagedSocket proves a raw client is targeting the socket authorized by
// the same ManagedEnvironment manifest whose digest it will present to projwmd.
func VerifyManagedSocket(client, socketPath, manifestPath, manifestDigest string) error {
	if socketPath == "" {
		return fmt.Errorf("%s: socket path is required", client)
	}
	if manifestPath == "" {
		return fmt.Errorf("%s: --managed-environment or PROJWM_NEXT_MANAGED_ENVIRONMENT is required to authorize socket path", client)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("%s: read managed environment %s: %w", client, manifestPath, err)
	}
	sum := sha256.Sum256(data)
	gotDigest := hex.EncodeToString(sum[:])
	if manifestDigest == "" {
		return fmt.Errorf("%s: manifest digest is required", client)
	}
	if gotDigest != manifestDigest {
		return fmt.Errorf("%s: manifest digest mismatch for %s: computed %s, provided %s", client, manifestPath, gotDigest, manifestDigest)
	}
	env, err := manifest.Parse(data, "0.1.0")
	if err != nil {
		return fmt.Errorf("%s: managed environment invalid: %w", client, err)
	}
	if filepath.Clean(env.Daemons.SocketPath) != filepath.Clean(socketPath) {
		return fmt.Errorf("%s: socket path %s is not authorized by managed environment socketPath %s", client, socketPath, env.Daemons.SocketPath)
	}
	return nil
}
