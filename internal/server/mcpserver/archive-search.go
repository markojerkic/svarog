package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/logsearch"
)

type searchArchivedLogsInput struct {
	ProjectID   string    `json:"project_id" jsonschema:"project UUID from list_projects"`
	ClientID    string    `json:"client_id" jsonschema:"client UUID from list_projects"`
	From        time.Time `json:"from" jsonschema:"inclusive RFC 3339 start timestamp"`
	To          time.Time `json:"to" jsonschema:"exclusive RFC 3339 end timestamp"`
	Query       string    `json:"query,omitempty" jsonschema:"space-separated literal terms that must all appear in the message, ignoring case; at most 256 characters, with at least one term containing three consecutive letters or digits; omit for all messages in the range"`
	OlderCursor string    `json:"older_cursor,omitempty" jsonschema:"older_cursor from the last search_archived_logs page; repeat the original project, client, range and query"`
}

type archivedLogRow struct {
	ArchiveID  string `json:"archive_id"`
	Entry      string `json:"entry" jsonschema:"file name inside the archive ZIP"`
	Line       int    `json:"line" jsonschema:"first physical line of the record in the entry, starting at 1"`
	OccurredAt string `json:"occurred_at"`
	Instance   string `json:"instance"`
	Message    string `json:"message"`
	Truncated  bool   `json:"truncated,omitempty" jsonschema:"true if the message was shortened"`
}

type searchArchivedLogsOutput struct {
	ArchivesScanned int              `json:"archives_scanned"`
	TotalMatches    int              `json:"total_matches"`
	Rows            []archivedLogRow `json:"rows" jsonschema:"matching archived records from oldest to newest"`
	Truncated       bool             `json:"truncated,omitempty" jsonschema:"true if older matching records were omitted"`
	OlderCursor     string           `json:"older_cursor,omitempty" jsonschema:"pass to search_archived_logs for the next older page; absent on the last page"`
}

func (t *tools) searchArchivedLogs(ctx context.Context, request *mcp.CallToolRequest, input searchArchivedLogsInput) (*mcp.CallToolResult, searchArchivedLogsOutput, error) {
	user, err := t.currentUser(ctx, request)
	if err != nil {
		return nil, searchArchivedLogsOutput{}, err
	}
	scope, err := t.authorizeArchives(ctx, user, input.ProjectID, input.ClientID)
	if err != nil {
		return nil, searchArchivedLogsOutput{}, err
	}
	if input.From.IsZero() || input.To.IsZero() || !input.From.Before(input.To) {
		return nil, searchArchivedLogsOutput{}, errors.New("from and to must be nonzero RFC 3339 timestamps with from earlier than to")
	}
	if utf8.RuneCountInString(input.Query) > archive.MaxSearchText {
		return nil, searchArchivedLogsOutput{}, fmt.Errorf("query must be at most %d characters", archive.MaxSearchText)
	}
	result, err := t.archives.Search(ctx, scope, archive.SearchQuery{From: input.From, To: input.To, Text: input.Query, OlderCursor: input.OlderCursor})
	if err != nil {
		switch {
		case errors.Is(err, logsearch.ErrTooShort):
			return nil, searchArchivedLogsOutput{}, logsearch.ErrTooShort
		case errors.Is(err, archive.ErrSearchCursor):
			return nil, searchArchivedLogsOutput{}, archive.ErrSearchCursor
		case errors.Is(err, archive.ErrSearchLimit), errors.Is(err, context.DeadlineExceeded):
			return nil, searchArchivedLogsOutput{}, errors.New("archive scan limit exceeded; use a narrower time range")
		default:
			return nil, searchArchivedLogsOutput{}, archiveFailure(ctx, "search archived logs", err)
		}
	}

	// Search retains the newest hits first; MCP lists those hits in chronological order.
	n := len(result.Hits)
	output := searchArchivedLogsOutput{
		ArchivesScanned: result.ArchivesScanned,
		TotalMatches:    result.TotalMatches,
		Rows:            make([]archivedLogRow, n),
		Truncated:       result.OlderCursor != "",
		OlderCursor:     result.OlderCursor,
	}
	for i, hit := range result.Hits {
		message, truncated := truncateRunes(hit.Message, maxMessageRunes)
		output.Rows[n-1-i] = archivedLogRow{
			ArchiveID: hit.ArchiveID.String(), Entry: hit.Entry, Line: hit.Line,
			OccurredAt: hit.OccurredAt.UTC().Format(time.RFC3339Nano), Instance: hit.Instance,
			Message: message, Truncated: truncated,
		}
	}
	return nil, output, nil
}
