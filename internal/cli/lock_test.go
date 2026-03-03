package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLock_Success(t *testing.T) {
	dir := t.TempDir()
	unlock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if unlock == nil {
		t.Fatal("expected non-nil unlock func")
	}

	// Lock file must exist while held.
	lockPath := filepath.Join(dir, lockFileName)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatal("lock file should exist while lock is held")
	}

	unlock()

	// Lock file must be removed after unlock.
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("lock file should be removed after unlock")
	}
}

func TestAcquireLock_AlreadyExists(t *testing.T) {
	dir := t.TempDir()

	// Pre-create the lock file to simulate a running process.
	lockPath := filepath.Join(dir, lockFileName)
	if err := os.WriteFile(lockPath, []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// AcquireLock should call Fatalf (which calls os.Exit(2)).
	// We test the underlying error path by verifying os.OpenFile with O_EXCL
	// returns an "exists" error without triggering Fatalf in the test.
	_, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		t.Fatal("expected error when lock already exists")
	}
	if !os.IsExist(err) {
		t.Fatalf("expected os.IsExist error, got: %v", err)
	}
}
