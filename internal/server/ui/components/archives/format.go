package archives

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/server/i18n"
)

func ArchivePanelURL(projectID, clientID string) string {
	return fmt.Sprintf("/logs/%s/%s/archives", projectID, clientID)
}

func ArchiveSettingsURL(projectID, clientID string) string {
	return ArchivePanelURL(projectID, clientID) + "/settings"
}

func ArchiveDownloadURL(projectID, clientID, archiveID string) string {
	return fmt.Sprintf("/logs/%s/%s/archives/%s", projectID, clientID, archiveID)
}

func ArchiveListURL(projectID, clientID string, next *archive.Cursor) string {
	path := fmt.Sprintf("/logs/%s/%s/archives/list", projectID, clientID)
	if next == nil {
		return path
	}
	query := url.Values{}
	query.Set("before", next.CreatedAt.UTC().Format(time.RFC3339Nano))
	query.Set("beforeId", next.ID.String())
	return path + "?" + query.Encode()
}

func formatRuleAge(ctx context.Context, weeks int) string {
	if weeks < 1 {
		weeks = 1
	}
	return i18n.Format(ctx, i18n.Message{
		ID:    "archives.rule_age",
		Count: &weeks,
		Data:  map[string]any{"Count": weeks},
	})
}

func formatArchiveTime(ctx context.Context, value time.Time) string {
	value = value.In(time.Local)
	if i18n.LanguageFrom(ctx) == i18n.Croatian {
		return value.Format("02. 01. 2006. 15:04")
	}
	return value.Format("02 Jan 2006, 15:04")
}

func formatArchiveDate(ctx context.Context, value time.Time) string {
	value = value.In(time.Local)
	if i18n.LanguageFrom(ctx) == i18n.Croatian {
		return value.Format("02. 01. 2006.")
	}
	return value.Format("02 Jan 2006")
}

func formatArchiveSummary(ctx context.Context, count int64, size int64) string {
	shown := archiveCount(count)
	return i18n.Format(ctx, i18n.Message{
		ID:    "archives.summary",
		Count: &shown,
		Data: map[string]any{
			"Count": shown,
			"Size":  formatArchiveSize(size),
		},
	})
}

func formatArchiveRange(ctx context.Context, from, to time.Time) string {
	if from.IsZero() || to.IsZero() {
		return ""
	}
	return i18n.Format(ctx, i18n.Message{
		ID: "archives.range",
		Data: map[string]any{
			"From": formatArchiveDate(ctx, from),
			"To":   formatArchiveDate(ctx, to),
		},
	})
}

func formatArchiveSize(size int64) string {
	if size < 0 {
		size = 0
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", size, units[0])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func archiveCount(count int64) int {
	if count < 0 {
		return 0
	}
	if count > math.MaxInt {
		return math.MaxInt
	}
	return int(count)
}
