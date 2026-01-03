package archive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/db"
)

type archivingResult struct {
	zipFilePath string
	fromDate    time.Time
	toDate      time.Time
}

func (a *ArchiveServiceImpl) createRollingArchive(ctx context.Context, tempDir string, projectID string, clientID string, cuttoffDate time.Time) (archivingResult, error) {
	fileIndex := 0

	cursor := &db.LastCursor{
		Timestamp:  cuttoffDate,
		IsBackward: true,
	}

	result := &archivingResult{
		zipFilePath: "",
		fromDate:    cuttoffDate,
	}

	for {
		tempFile := filepath.Join(tempDir, fmt.Sprintf("archive_%s_%s_%d.log", projectID, clientID, fileIndex))
		fileContent := make([]byte, 0, 1024*1024)
		logPage, err := a.logsService.GetLogs(ctx, db.LogPageRequest{
			ProjectId: projectID,
			ClientId:  clientID,
			Instances: nil,
			PageSize:  5000,
			LogLineId: nil,
			Cursor:    cursor,
		})
		if err != nil {
			slog.Error("Error getting logs", "error", err)
			return archivingResult{}, err
		}

		if len(logPage.Logs) == 0 {
			slog.Debug("No logs found")
			break
		}

		for _, log := range logPage.Logs {
			line := fmt.Sprintf("[%s %s] %s\n", log.Timestamp.Format(time.RFC3339), log.Client.InstanceId, log.LogLine)
			fileContent = append(fileContent, []byte(line)...)
			if result.toDate.Before(log.Timestamp) {
				result.toDate = log.Timestamp
			}
		}

		err = os.WriteFile(tempFile, fileContent, 0644)
		if err != nil {
			slog.Error("Error writing file", "error", err)
			return archivingResult{}, err
		}
		fileIndex++

		cursor = &db.LastCursor{
			Timestamp:      logPage.Logs[len(logPage.Logs)-1].Timestamp,
			SequenceNumber: int(logPage.Logs[len(logPage.Logs)-1].SequenceNumber),
			IsBackward:     true,
		}

	}
	err := a.logsService.DeleteLogBeforeTimestamp(ctx, cuttoffDate)
	if err != nil {
		return archivingResult{}, errors.Join(errors.New("error deleting log lines while archiving"), err)
	}

	zipDir, err := os.MkdirTemp("", fmt.Sprintf("archive_%s_%s", projectID, clientID))
	if err != nil {
		slog.Error("Error creating temp dir", "error", err)
		return archivingResult{}, err
	}

	zipFile := filepath.Join(zipDir, fmt.Sprintf("archive_%s_%s.zip", projectID, clientID))
	err = util.ZipDir(tempDir, zipFile)
	if err != nil {
		slog.Error("Error zipping dir", "error", err)
	}

	result.zipFilePath = zipFile

	return *result, err
}
