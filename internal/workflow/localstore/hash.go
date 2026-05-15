package localstore

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

// HashWorkflowDoc returns "sha256:<hex>" for the file at path.
func HashWorkflowDoc(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", festerrors.Wrap(err, "hashing workflow doc")
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
