package archive

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

type archivingResult struct {
	count int64
	from  time.Time
	to    time.Time
}

func (a *Service) createRollingArchive(ctx context.Context, conn *pgx.Conn, record sqlc.Archive) (archivingResult, [32]byte, int64, error) {
	if err := os.MkdirAll(a.archiveDir, 0o700); err != nil {
		return archivingResult{}, [32]byte{}, 0, fmt.Errorf("create archive directory: %w", err)
	}
	root, err := os.OpenRoot(a.archiveDir)
	if err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	defer root.Close()
	key := filepath.FromSlash(record.ObjectKey)
	if err := root.MkdirAll(filepath.Dir(key), 0o700); err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	if err := syncDirectory(root, "."); err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	file, err := root.OpenFile(key+".part", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	written, writeErr := writeArchivePages(ctx, sqlc.New(conn), writer, record)
	if err := errors.Join(writeErr, writer.Close()); err != nil {
		return archivingResult{}, [32]byte{}, 0, fmt.Errorf("write archive ZIP: %w", err)
	}
	if err := file.Sync(); err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	if err := file.Close(); err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	if err := ctx.Err(); err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	if err := root.Rename(key+".part", key); err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	if err := syncDirectory(root, filepath.Dir(key)); err != nil {
		return archivingResult{}, [32]byte{}, 0, err
	}
	var sum [32]byte
	copy(sum[:], hash.Sum(nil))
	return written, sum, size, nil
}

func writeArchivePages(ctx context.Context, queries *sqlc.Queries, writer *zip.Writer, record sqlc.Archive) (archivingResult, error) {
	params := sqlc.ListArchiveLogsParams{ClientID: record.ClientID, ArchiveID: &record.ID, PageSize: 5000}
	var result archivingResult
	for index := 0; ; index++ {
		page, err := queries.ListArchiveLogs(ctx, params)
		if err != nil {
			return archivingResult{}, fmt.Errorf("read claimed archive logs: %w", err)
		}
		if len(page) == 0 {
			return result, nil
		}
		entry, err := writer.Create(fmt.Sprintf("logs_%06d.log", index))
		if err != nil {
			return archivingResult{}, err
		}
		buffer := bufio.NewWriter(entry)
		for _, line := range page {
			if err := ctx.Err(); err != nil {
				return archivingResult{}, err
			}
			if _, err := fmt.Fprintf(buffer, "[%s %s] %s\n", line.OccurredAt.Format(time.RFC3339Nano), line.InstanceName, line.Message); err != nil {
				return archivingResult{}, err
			}
			if result.count == 0 {
				result.to = line.OccurredAt
			}
			result.from = line.OccurredAt
			result.count++
		}
		if err := buffer.Flush(); err != nil {
			return archivingResult{}, err
		}
		last := page[len(page)-1]
		params.HasCursor = true
		params.CursorAt = last.OccurredAt
		params.CursorSequence = last.SequenceNumber
		params.CursorID = last.ID
	}
}

func syncDirectory(root *os.Root, name string) error {
	dir, err := root.Open(name)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
