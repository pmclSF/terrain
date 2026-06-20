package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile writes data to path via a same-directory temporary file
// and rename. Callers are responsible for creating filepath.Dir(path).
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("atomic write temp %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("atomic write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("atomic sync %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("atomic close %s: %w", path, err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("atomic chmod %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("atomic rename %s: %w", path, err)
	}
	cleanup = false
	return nil
}
