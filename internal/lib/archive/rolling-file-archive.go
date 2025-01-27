package archive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/db"
)

func (a *ArchiveServiceImpl) createRollingArchive(ctx context.Context, tempDir string, projectID string, clientID string, cuttoffDate time.Time) (string, error) {
	fileIndex := 0

	cursor := &db.LastCursor{
		Timestamp:  cuttoffDate,
		IsBackward: true,
	}

	for {
		tempFile := filepath.Join(tempDir, fmt.Sprintf("archive_%s_%s_%d.log", projectID, clientID, fileIndex))
		fileContent := make([]byte, 0, 1024*1024)
		logs, err := a.logsService.GetLogs(ctx, clientID, nil, 5000, nil, cursor)
		if err != nil {
			log.Error("Error getting logs", "error", err)
			return "", err
		}

		if len(logs) == 0 {
			log.Debug("No logs found")
			break
		}

		for _, log := range logs {
			line := fmt.Sprintf("[%s %s] %s\n", log.Timestamp.Format(time.RFC3339), log.Client.IpAddress, log.LogLine)
			fileContent = append(fileContent, []byte(line)...)
		}

		err = os.WriteFile(tempFile, fileContent, 0644)
		if err != nil {
			log.Error("Error writing file", "error", err)
			return "", err
		}
		fileIndex++

		cursor = &db.LastCursor{
			Timestamp:      logs[len(logs)-1].Timestamp,
			SequenceNumber: int(logs[len(logs)-1].SequenceNumber),
			IsBackward:     true,
		}
	}

	zipDir, err := os.MkdirTemp("", fmt.Sprintf("archive_%s_%s", projectID, clientID))
	if err != nil {
		log.Error("Error creating temp dir", "error", err)
		return "", err
	}

	zipFile := filepath.Join(zipDir, fmt.Sprintf("archive_%s_%s.zip", projectID, clientID))
	err = util.ZipDir(tempDir, zipFile)
	if err != nil {
		log.Error("Error zipping dir", "error", err)
	}

	return zipFile, err
}
