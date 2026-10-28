package archive

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

type archivingResult struct {
	count int64
	from  time.Time
	to    time.Time
}

func (a *Service) createRollingArchive(ctx context.Context, conn *pgx.Conn, record sqlc.Archive) (archivingResult, StoredObject, error) {
	var written archivingResult
	object, err := a.store.Put(ctx, record.ObjectKey, func(output io.Writer) error {
		writer := zip.NewWriter(output)
		var writeErr error
		written, writeErr = writeArchivePages(ctx, sqlc.New(conn), writer, record)
		if err := errors.Join(writeErr, writer.Close()); err != nil {
			return fmt.Errorf("write archive ZIP: %w", err)
		}
		return nil
	})
	return written, object, err
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
