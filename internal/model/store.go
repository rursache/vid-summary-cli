package model

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rursache/vid-summary-cli/internal/config"
)

// Local reports whether a model file already exists and its path.
func Local(name string) (path string, exists bool, err error) {
	dir, err := config.ModelsDir()
	if err != nil {
		return "", false, err
	}
	path = filepath.Join(dir, Filename(name))
	st, err := os.Stat(path)
	if os.IsNotExist(err) {
		return path, false, nil
	}
	if err != nil {
		return path, false, err
	}
	return path, !st.IsDir(), nil
}

// Ensure resolves the model file, downloading it on first use. The download is
// written atomically (*.partial then rename) and verified before use.
func Ensure(name string) (string, error) {
	path, exists, err := Local(name)
	if err != nil {
		return "", err
	}
	if exists {
		if err := verify(path, name); err != nil {
			return "", fmt.Errorf("cached model %s failed verification: %w\n  Remove it and retry: vid-summary-cli models rm %s", name, err, name)
		}
		return path, nil
	}

	fmt.Fprintf(os.Stderr, "Downloading model %s...\n", name)
	if err := download(DownloadURL(name), path); err != nil {
		return "", err
	}
	if err := verify(path, name); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("downloaded model %s failed verification: %w", name, err)
	}
	return path, nil
}

// Remove deletes a local model file.
func Remove(name string) error {
	path, exists, err := Local(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("model %s is not downloaded", name)
	}
	return os.Remove(path)
}
