package archive

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/db"
)

type archivingResult struct {
	count int64
	from  time.Time
	to    time.Time
}

func (a *ArchiveServiceImpl) createRollingArchive(ctx context.Context, tempDir string, destZip string, projectID string, clientID string, cutoff time.Time) (archivingResult, error) {
	cursor := &db.LastCursor{
		Timestamp:  cutoff,
		IsBackward: true,
	}
	var result archivingResult
	fileIndex := 0
	for {
		page, err := a.logsService.GetLogs(ctx, db.LogPageRequest{
			ProjectId: projectID,
			ClientId:  clientID,
			PageSize:  5000,
			Cursor:    cursor,
		})
		if err != nil {
			slog.Error("Error getting logs", "error", err)
			return archivingResult{}, err
		}
		if len(page.Logs) == 0 {
			break
		}
		content := make([]byte, 0, 1024*1024)
		for _, log := range page.Logs {
			content = append(content, fmt.Appendf(nil, "[%s %s] %s\n", log.Timestamp.Format(time.RFC3339), log.Client.InstanceId, log.LogLine)...)
			if result.count == 0 || log.Timestamp.Before(result.from) {
				result.from = log.Timestamp
			}
			if log.Timestamp.After(result.to) {
				result.to = log.Timestamp
			}
			result.count++
		}
		tempFile := filepath.Join(tempDir, fmt.Sprintf("archive_%s_%s_%d.log", projectID, clientID, fileIndex))
		if err := os.WriteFile(tempFile, content, 0o644); err != nil {
			slog.Error("Error writing file", "error", err)
			return archivingResult{}, err
		}
		last := page.Logs[len(page.Logs)-1]
		cursor = &db.LastCursor{
			Timestamp:      last.Timestamp,
			SequenceNumber: last.SequenceNumber,
			ID:             last.ID,
			IsBackward:     true,
		}
		fileIndex++
	}
	if result.count == 0 {
		return result, nil
	}
	if err := util.ZipDir(tempDir, destZip); err != nil {
		slog.Error("Error zipping dir", "error", err)
		return archivingResult{}, err
	}
	return result, nil
}

func hashFile(path string) ([32]byte, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return [32]byte{}, 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return [32]byte{}, 0, err
	}
	var sum [32]byte
	copy(sum[:], hash.Sum(nil))
	return sum, size, nil
}
