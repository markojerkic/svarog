//go:build linux || darwin

package embedded

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockStore holds an exclusive lock on dir until the returned func is called.
func lockStore(dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create NATS_STORE_DIR: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(dir, "svarog.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open NATS_STORE_DIR lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("NATS_STORE_DIR %s is in use by another embedded broker", dir)
		}
		return nil, fmt.Errorf("lock NATS_STORE_DIR: %w", err)
	}
	return func() { _ = file.Close() }, nil
}
