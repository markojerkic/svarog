package archive

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

const archivePageSize = 5000

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
	instances, err := queries.ListArchiveInstances(ctx, sqlc.ListArchiveInstancesParams{
		ClientID: record.ClientID, ArchiveID: &record.ID,
	})
	if err != nil {
		return archivingResult{}, fmt.Errorf("read claimed archive instances: %w", err)
	}
	var result archivingResult
	for _, instance := range instances {
		written, err := writeInstancePages(ctx, queries, writer, record, instance)
		if err != nil {
			return archivingResult{}, err
		}
		result.count += written.count
		if result.to.IsZero() || written.to.After(result.to) {
			result.to = written.to
		}
		if result.from.IsZero() || written.from.Before(result.from) {
			result.from = written.from
		}
	}
	return result, nil
}

func writeInstancePages(ctx context.Context, queries *sqlc.Queries, writer *zip.Writer, record sqlc.Archive, instance string) (archivingResult, error) {
	params := sqlc.ListArchiveLogsParams{
		ClientID: record.ClientID, ArchiveID: &record.ID, InstanceName: instance, PageSize: archivePageSize,
	}
	var result archivingResult
	for index := 0; ; index++ {
		page, err := queries.ListArchiveLogs(ctx, params)
		if err != nil {
			return archivingResult{}, fmt.Errorf("read claimed archive logs: %w", err)
		}
		if len(page) == 0 {
			if result.count == 0 {
				return archivingResult{}, fmt.Errorf("claimed instance %q has no archive logs", instance)
			}
			return result, nil
		}
		entry, err := writer.Create(fmt.Sprintf("%s/logs_%06d.log", archiveInstancePath(instance), index))
		if err != nil {
			return archivingResult{}, err
		}
		buffer := bufio.NewWriter(entry)
		for _, line := range page {
			if err := ctx.Err(); err != nil {
				return archivingResult{}, err
			}
			if line.InstanceName != instance {
				return archivingResult{}, fmt.Errorf("archive page for %q contains instance %q", instance, line.InstanceName)
			}
			if _, err := fmt.Fprintf(buffer, "[%s %s] %s\n", line.OccurredAt.Format(time.RFC3339Nano), line.InstanceName, line.Message); err != nil {
				return archivingResult{}, err
			}
			if result.count == 0 || line.OccurredAt.After(result.to) {
				result.to = line.OccurredAt
			}
			if result.count == 0 || line.OccurredAt.Before(result.from) {
				result.from = line.OccurredAt
			}
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

func archiveInstancePath(instance string) string {
	var builder strings.Builder
	builder.Grow(len(instance))
	for _, char := range instance {
		switch {
		case char == '/' || char == '\\' || char < 0x20 || char == 0x7f:
			builder.WriteByte('_')
		default:
			builder.WriteRune(char)
		}
	}
	name := strings.Trim(builder.String(), ". ")
	if name == "" || name == ".." {
		return "_unnamed"
	}
	return path.Clean(name)
}
