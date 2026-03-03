package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

const lockFileName = ".clarion.lock"

// AcquireLock creates an advisory lock file in outputDir to prevent concurrent
// writes from multiple clarion processes. Returns an unlock function that
// removes the lock file when deferred. Calls Fatalf (exit 2) if the lock
// already exists.
func AcquireLock(outputDir string) (unlock func(), err error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	lockPath := filepath.Join(outputDir, lockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			Fatalf("Another clarion process is running against %s. If no process is running, delete .clarion.lock and retry.", outputDir)
		}
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	_ = f.Close()

	unlock = func() { os.Remove(lockPath) }
	return unlock, nil
}
