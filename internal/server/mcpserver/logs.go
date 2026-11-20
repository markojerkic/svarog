package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/markojerkic/svarog/internal/server/logstore"
)

const (
	defaultContextLines = 20
	maxContextLines     = 100
	maxMessageRunes     = 2000
)

type logRow struct {
	ID         int64  `json:"id"`
	OccurredAt string `json:"occurred_at"`
	Instance   string `json:"instance"`
	Sequence   int64  `json:"sequence"`
	Message    string `json:"message"`
	Truncated  bool   `json:"truncated,omitempty"`
	Anchor     bool   `json:"anchor,omitempty"`
	Link       string `json:"link"`
}

type logPage struct {
	Rows        []logRow `json:"rows" jsonschema:"log lines from oldest to newest"`
	OlderCursor string   `json:"older_cursor,omitempty" jsonschema:"pass to get_log_page for earlier lines"`
	NewerCursor string   `json:"newer_cursor,omitempty" jsonschema:"pass to get_log_page for later lines"`
}

type listInstancesInput struct {
	ProjectID string `json:"project_id" jsonschema:"project UUID from list_projects"`
	ClientID  string `json:"client_id" jsonschema:"client UUID from list_projects"`
}

type listInstancesOutput struct {
	Instances []string `json:"instances"`
}

func (t *tools) listInstances(ctx context.Context, request *mcp.CallToolRequest, input listInstancesInput) (*mcp.CallToolResult, listInstancesOutput, error) {
	user, err := t.currentUser(ctx, request)
	if err != nil {
		return nil, listInstancesOutput{}, err
	}
	scope, err := t.authorizeClient(ctx, user, input.ProjectID, input.ClientID)
	if err != nil {
		return nil, listInstancesOutput{}, err
	}
	instances, err := t.logs.Instances(ctx, scope)
	if err != nil {
		return nil, listInstancesOutput{}, toolFailure(ctx, "list instances", err)
	}
	return nil, listInstancesOutput{Instances: instances}, nil
}

type searchLogsInput struct {
	ProjectID string   `json:"project_id" jsonschema:"project UUID from list_projects"`
	ClientID  string   `json:"client_id" jsonschema:"client UUID from list_projects"`
	Query     string   `json:"query,omitempty" jsonschema:"space-separated literal terms that must all appear in a line, ignoring case; at least one term needs three consecutive letters or digits"`
	Instances []string `json:"instances,omitempty" jsonschema:"exact instance names from list_instances; omit for all instances"`
	From      string   `json:"from,omitempty" jsonschema:"RFC 3339 timestamp; only lines at or after it"`
	To        string   `json:"to,omitempty" jsonschema:"RFC 3339 timestamp; only lines before it"`
	Limit     int      `json:"limit,omitempty" jsonschema:"maximum number of lines, 1 to 200; default 50"`
}

func (t *tools) searchLogs(ctx context.Context, request *mcp.CallToolRequest, input searchLogsInput) (*mcp.CallToolResult, logPage, error) {
	user, err := t.currentUser(ctx, request)
	if err != nil {
		return nil, logPage{}, err
	}
	scope, err := t.authorizeClient(ctx, user, input.ProjectID, input.ClientID)
	if err != nil {
		return nil, logPage{}, err
	}
	query, err := newLogQuery(input.Query, input.Instances, input.From, input.To, input.Limit)
	if err != nil {
		return nil, logPage{}, err
	}
	page, err := t.readPage(ctx, scope, query, nil)
	return nil, page, err
}

type getLogContextInput struct {
	Link      string `json:"link,omitempty" jsonschema:"link of a log line from another tool or copied from Svarog"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"project UUID, used with client_id and log_id instead of link"`
	ClientID  string `json:"client_id,omitempty" jsonschema:"client UUID, used with project_id and log_id instead of link"`
	LogID     int64  `json:"log_id,omitempty" jsonschema:"log line ID, used with project_id and client_id instead of link"`
	Lines     int    `json:"lines,omitempty" jsonschema:"lines on each side of the anchor, 1 to 100; default 20"`
}

func (t *tools) getLogContext(ctx context.Context, request *mcp.CallToolRequest, input getLogContextInput) (*mcp.CallToolResult, logPage, error) {
	projectID, clientID, logID, err := contextTarget(input)
	if err != nil {
		return nil, logPage{}, err
	}
	lines := input.Lines
	if lines == 0 {
		lines = defaultContextLines
	}
	if lines < 1 || lines > maxContextLines {
		return nil, logPage{}, errors.New("lines must be between 1 and 100")
	}
	user, err := t.currentUser(ctx, request)
	if err != nil {
		return nil, logPage{}, err
	}
	scope, err := t.authorizeClient(ctx, user, projectID, clientID)
	if err != nil {
		return nil, logPage{}, err
	}
	page, found, err := t.logs.Around(ctx, scope, logID, lines, nil)
	if err != nil {
		return nil, logPage{}, toolFailure(ctx, "read log context", err)
	}
	if !found {
		return nil, logPage{}, fmt.Errorf("log line %d was not found for this client; it may have been archived or deleted", logID)
	}

	// Context continues within the anchor's instance, without a search.
	query := logQuery{Instances: []string{*page.AnchorInstance}, Limit: lines}
	output := logPage{Rows: t.logRows(page.Rows, logID)}
	if page.Older != nil {
		output.OlderCursor = encodeCursor(scope, query, false, *page.Older)
	}
	if page.Newer != nil {
		output.NewerCursor = encodeCursor(scope, query, true, *page.Newer)
	}
	return nil, output, nil
}

// contextTarget accepts either a link or the three IDs.
func contextTarget(input getLogContextInput) (projectID, clientID string, logID int64, err error) {
	hasIDs := input.ProjectID != "" || input.ClientID != "" || input.LogID != 0
	switch {
	case input.Link != "" && hasIDs:
		return "", "", 0, errors.New("pass either link or project_id, client_id and log_id, not both")
	case input.Link != "":
		return parseLogLink(input.Link)
	case input.ProjectID == "" || input.ClientID == "" || input.LogID <= 0:
		return "", "", 0, errors.New("pass a link, or project_id, client_id and a positive log_id")
	default:
		return input.ProjectID, input.ClientID, input.LogID, nil
	}
}

// parseLogLink reads /logs/{project}/{client}?logLine={id} from an absolute
// URL or a path. The host is ignored; access is checked separately.
func parseLogLink(raw string) (projectID, clientID string, logID int64, err error) {
	invalid := errors.New("link must look like /logs/{project_id}/{client_id}?logLine={id}")
	link, parseErr := url.Parse(strings.TrimSpace(raw))
	if parseErr != nil {
		return "", "", 0, invalid
	}
	parts := strings.Split(strings.Trim(link.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "logs" {
		return "", "", 0, invalid
	}
	logID, parseErr = strconv.ParseInt(link.Query().Get("logLine"), 10, 64)
	if parseErr != nil || logID <= 0 {
		return "", "", 0, invalid
	}
	return parts[1], parts[2], logID, nil
}

type getLogPageInput struct {
	Cursor string `json:"cursor" jsonschema:"older_cursor or newer_cursor from a previous result"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of lines, 1 to 200; default: the limit of the request that returned the cursor"`
}

func (t *tools) getLogPage(ctx context.Context, request *mcp.CallToolRequest, input getLogPageInput) (*mcp.CallToolResult, logPage, error) {
	cursor, err := decodeCursor(input.Cursor)
	if err != nil {
		return nil, logPage{}, err
	}
	if input.Limit != 0 {
		cursor.Limit = input.Limit
	}
	user, err := t.currentUser(ctx, request)
	if err != nil {
		return nil, logPage{}, err
	}
	scope, err := t.authorizeClient(ctx, user, cursor.ProjectID.String(), cursor.ClientID.String())
	if err != nil {
		return nil, logPage{}, err
	}
	page, err := t.readPage(ctx, scope, cursor.logQuery, &cursor)
	return nil, page, err
}

// readPage reads one page in the cursor's direction, or the newest page
// without a cursor, and returns cursors for both directions.
func (t *tools) readPage(ctx context.Context, scope logstore.Scope, query logQuery, from *logCursor) (logPage, error) {
	pageQuery, err := query.pageQuery(scope)
	if err != nil {
		return logPage{}, err
	}
	if from != nil && from.Newer {
		pageQuery.Newer = from.position()
	} else if from != nil {
		pageQuery.Older = from.position()
	}
	page, err := t.logs.Page(ctx, pageQuery)
	if err != nil {
		return logPage{}, toolFailure(ctx, "read logs", err)
	}

	output := logPage{Rows: t.logRows(page.Rows, 0)}
	// The store returns a cursor in the reading direction only for a full page.
	if page.Older != nil {
		output.OlderCursor = encodeCursor(scope, query, false, *page.Older)
	}
	if page.Newer != nil {
		output.NewerCursor = encodeCursor(scope, query, true, *page.Newer)
	}
	// Lines exist on the side the cursor came from, so allow stepping back.
	if from != nil && len(page.Rows) > 0 {
		if from.Newer {
			output.OlderCursor = encodeCursor(scope, query, false, rowPosition(page.Rows[len(page.Rows)-1]))
		} else {
			output.NewerCursor = encodeCursor(scope, query, true, rowPosition(page.Rows[0]))
		}
	}
	return output, nil
}

// logRows converts the store's newest-first rows to oldest-first output.
func (t *tools) logRows(rows []logstore.Row, anchorID int64) []logRow {
	output := make([]logRow, len(rows))
	for i, row := range rows {
		output[len(rows)-1-i] = t.logRow(row, row.ID == anchorID)
	}
	return output
}

func (t *tools) logRow(row logstore.Row, anchor bool) logRow {
	message, truncated := row.Message, false
	if !anchor {
		message, truncated = truncateRunes(row.Message, maxMessageRunes)
	}
	return logRow{
		ID:         row.ID,
		OccurredAt: row.OccurredAt.UTC().Format(time.RFC3339Nano),
		Instance:   row.Instance,
		Sequence:   row.Sequence,
		Message:    message,
		Truncated:  truncated,
		Anchor:     anchor,
		Link:       t.logLink(row),
	}
}

// logLink matches the web UI's "Copy link" action.
func (t *tools) logLink(row logstore.Row) string {
	query := url.Values{"logLine": {strconv.FormatInt(row.ID, 10)}, "instance": {row.Instance}}
	return t.baseURL + "/logs/" + row.Scope.ProjectID.String() + "/" + row.Scope.ClientID.String() + "?" + query.Encode()
}

func truncateRunes(text string, limit int) (string, bool) {
	if utf8.RuneCountInString(text) <= limit {
		return text, false
	}
	count := 0
	for i := range text {
		if count == limit {
			return text[:i], true
		}
		count++
	}
	return text, false
}
