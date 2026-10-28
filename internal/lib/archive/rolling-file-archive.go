package archive

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
	dirs, err := archiveInstanceDirs(instances)
	if err != nil {
		return archivingResult{}, err
	}
	var result archivingResult
	for _, instance := range instances {
		written, err := writeInstancePages(ctx, queries, writer, record, instance, dirs[instance])
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

func writeInstancePages(ctx context.Context, queries *sqlc.Queries, writer *zip.Writer, record sqlc.Archive, instance, dir string) (archivingResult, error) {
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
		entry, err := writer.Create(fmt.Sprintf("%s/logs_%06d.log", dir, index))
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

func archiveInstanceDirs(instances []string) (map[string]string, error) {
	dirs := make(map[string]string, len(instances))
	folded := make(map[string][]string)
	for _, instance := range instances {
		dir := archiveInstancePath(instance)
		dirs[instance] = dir
		folded[strings.ToLower(dir)] = append(folded[strings.ToLower(dir)], instance)
	}
	for _, group := range folded {
		if len(group) < 2 {
			continue
		}
		for _, instance := range group {
			dirs[instance] = encodedArchiveDir(instance)
		}
	}
	seen := make(map[string]string, len(dirs))
	for instance, dir := range dirs {
		key := strings.ToLower(dir)
		if other, ok := seen[key]; ok {
			return nil, fmt.Errorf("instances %q and %q share archive directory %q", other, instance, dir)
		}
		seen[key] = instance
	}
	return dirs, nil
}

func archiveInstancePath(instance string) string {
	if isIdentityArchiveDir(instance) {
		return instance
	}
	encoded := percentEncodeInstance(instance)
	if isUsableArchiveDir(encoded) && strings.Contains(encoded, "%") {
		return encoded
	}
	return encodedArchiveDir(instance)
}

func encodedArchiveDir(instance string) string {
	return "=" + hex.EncodeToString([]byte(instance))
}

func percentEncodeInstance(instance string) string {
	var builder strings.Builder
	builder.Grow(len(instance))
	for i := 0; i < len(instance); i++ {
		char := instance[i]
		if isUnreservedArchiveByte(char) {
			builder.WriteByte(char)
			continue
		}
		fmt.Fprintf(&builder, "%%%02X", char)
	}
	return builder.String()
}

func isIdentityArchiveDir(name string) bool {
	if !isUsableArchiveDir(name) {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isUnreservedArchiveByte(name[i]) {
			return false
		}
	}
	return true
}

func isUsableArchiveDir(name string) bool {
	if name == "" || name == "." || name == ".." || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	if strings.ContainsAny(name, "/\\") || strings.ContainsRune(name, 0) {
		return false
	}
	return !isWindowsReservedName(name)
}

func isUnreservedArchiveByte(char byte) bool {
	return char == '-' || char == '_' || char == '.' ||
		(char >= '0' && char <= '9') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= 'a' && char <= 'z')
}

func isWindowsReservedName(name string) bool {
	stem := name
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		stem = name[:dot]
	}
	switch strings.ToUpper(stem) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}
