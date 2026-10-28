package archive

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type DiskStore struct {
	archiveDir string
}

var _ Store = (*DiskStore)(nil)

// NewDiskStore returns nil for an empty directory, leaving storage unconfigured.
func NewDiskStore(archiveDir string) Store {
	if archiveDir == "" {
		return nil
	}
	return &DiskStore{archiveDir: archiveDir}
}

func (s *DiskStore) Backend() string {
	return "disk"
}

func (s *DiskStore) Put(ctx context.Context, key string, write func(io.Writer) error) (StoredObject, error) {
	if err := ctx.Err(); err != nil {
		return StoredObject{}, err
	}
	if err := os.MkdirAll(s.archiveDir, 0o700); err != nil {
		return StoredObject{}, fmt.Errorf("create archive directory: %w", err)
	}
	root, err := os.OpenRoot(s.archiveDir)
	if err != nil {
		return StoredObject{}, err
	}
	defer root.Close()
	key = filepath.FromSlash(key)
	if err := root.MkdirAll(filepath.Dir(key), 0o700); err != nil {
		return StoredObject{}, err
	}
	if err := syncDirectory(root, "."); err != nil {
		return StoredObject{}, err
	}
	file, err := root.OpenFile(key+".part", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return StoredObject{}, err
	}
	defer file.Close()
	if err := write(file); err != nil {
		return StoredObject{}, err
	}
	if err := file.Sync(); err != nil {
		return StoredObject{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return StoredObject{}, err
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return StoredObject{}, err
	}
	if err := file.Close(); err != nil {
		return StoredObject{}, err
	}
	if err := ctx.Err(); err != nil {
		return StoredObject{}, err
	}
	if err := root.Rename(key+".part", key); err != nil {
		return StoredObject{}, err
	}
	if err := syncDirectory(root, filepath.Dir(key)); err != nil {
		return StoredObject{}, err
	}
	return StoredObject{SizeBytes: size, SHA256: [32]byte(hash.Sum(nil))}, nil
}

func (s *DiskStore) Open(ctx context.Context, key string, sizeBytes int64) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.archiveDir)
	if err != nil {
		return nil, fmt.Errorf("open archive directory: %w", err)
	}
	defer root.Close()
	file, err := root.Open(filepath.FromSlash(key))
	if err != nil {
		return nil, fmt.Errorf("open archive content: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != sizeBytes {
		_ = file.Close()
		return nil, fmt.Errorf("invalid archive file")
	}
	return file, nil
}

func (s *DiskStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.archiveDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open archive directory for cleanup: %w", err)
	}
	defer root.Close()
	var cleanupErr error
	for _, name := range []string{key + ".part", key} {
		if err := root.Remove(filepath.FromSlash(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func syncDirectory(root *os.Root, name string) error {
	dir, err := root.Open(name)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
