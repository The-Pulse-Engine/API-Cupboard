//go:build !windows

package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// On Linux, permission denied means a read-only folder, so the lock must fail at once.
func TestLockReadOnlyFolderFailsFast(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to a read-only folder")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	start := time.Now()
	if _, err := LockShelves(filepath.Join(dir, "shelves.json")); err == nil {
		t.Fatal("no error for a read-only folder")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("took %v to fail", time.Since(start))
	}
}
