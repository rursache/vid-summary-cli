package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// verify enforces strict integrity for known models (size + sha256). Unknown
// models cannot be verified, so we only guard against an empty file and warn.
func verify(path, name string) error {
	info, known := knownModels[name]
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !known {
		fmt.Fprintf(os.Stderr, "warning: %q is not in the known-models map; integrity cannot be verified\n", name)
		if st.Size() == 0 {
			return fmt.Errorf("file is empty")
		}
		return nil
	}
	if st.Size() != info.Size {
		return fmt.Errorf("size mismatch: got %d, want %d", st.Size(), info.Size)
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, info.SHA256) {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", sum, info.SHA256)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
