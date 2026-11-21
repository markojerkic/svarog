package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/archivelink"
	svarogauth "github.com/markojerkic/svarog/internal/lib/auth"
)

const maxArchiveLimit = 100

var errInvalidArchiveCursor = errors.New("cursor is invalid; use next_cursor from a previous list_archives result")

type archiveSummary struct {
	ID        string `json:"id"`
	FileName  string `json:"file_name"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
	LogCount  int64  `json:"log_count"`
	FirstAt   string `json:"first_line_at" jsonschema:"time of the oldest archived line"`
	LastAt    string `json:"last_line_at" jsonschema:"time of the newest archived line"`
	CreatedAt string `json:"created_at"`
}

type listArchivesInput struct {
	ProjectID string `json:"project_id" jsonschema:"project UUID from list_projects"`
	ClientID  string `json:"client_id" jsonschema:"client UUID from list_projects"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"next_cursor from a previous result"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum number of archives, 1 to 100; default 20"`
}

type listArchivesOutput struct {
	Archives   []archiveSummary `json:"archives"`
	NextCursor string           `json:"next_cursor,omitempty" jsonschema:"pass to list_archives for older archives"`
}

// archiveCursor is opaque to clients; List scopes it to the requested client.
type archiveCursor struct {
	CreatedAt time.Time `json:"t"`
	ID        uuid.UUID `json:"i"`
}

func (t *tools) listArchives(ctx context.Context, request *mcp.CallToolRequest, input listArchivesInput) (*mcp.CallToolResult, listArchivesOutput, error) {
	if input.Limit < 0 || input.Limit > maxArchiveLimit {
		return nil, listArchivesOutput{}, errors.New("limit must be between 1 and 100")
	}
	query := archive.ListQuery{Limit: input.Limit}
	if input.Cursor != "" {
		cursor, err := decodeArchiveCursor(input.Cursor)
		if err != nil {
			return nil, listArchivesOutput{}, err
		}
		query.Before = &archive.Cursor{CreatedAt: cursor.CreatedAt, ID: cursor.ID}
	}
	user, err := t.currentUser(ctx, request)
	if err != nil {
		return nil, listArchivesOutput{}, err
	}
	scope, err := t.authorizeArchives(ctx, user, input.ProjectID, input.ClientID)
	if err != nil {
		return nil, listArchivesOutput{}, err
	}
	page, err := t.archives.List(ctx, scope, query)
	if errors.Is(err, archive.ErrInvalidInput) {
		return nil, listArchivesOutput{}, errInvalidArchiveCursor
	}
	if err != nil {
		return nil, listArchivesOutput{}, archiveFailure(ctx, "list archives", err)
	}
	output := listArchivesOutput{Archives: make([]archiveSummary, len(page.Items))}
	for i, item := range page.Items {
		output.Archives[i] = summarizeArchive(item)
	}
	if page.Next != nil {
		output.NextCursor = encodeArchiveCursor(archiveCursor{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID})
	}
	return nil, output, nil
}

type archiveLinkInput struct {
	ProjectID string `json:"project_id" jsonschema:"project UUID from list_projects"`
	ClientID  string `json:"client_id" jsonschema:"client UUID from list_projects"`
	ArchiveID string `json:"archive_id" jsonschema:"archive ID from list_archives or search_archived_logs"`
}

type archiveLinkOutput struct {
	URL       string         `json:"url"`
	ExpiresAt string         `json:"expires_at"`
	Archive   archiveSummary `json:"archive"`
}

func (t *tools) getArchiveDownloadLink(ctx context.Context, request *mcp.CallToolRequest, input archiveLinkInput) (*mcp.CallToolResult, archiveLinkOutput, error) {
	archiveID, err := uuid.Parse(input.ArchiveID)
	if err != nil || archiveID == uuid.Nil {
		return nil, archiveLinkOutput{}, errors.New("archive_id must be an ID from list_archives or search_archived_logs")
	}
	user, err := t.currentUser(ctx, request)
	if err != nil {
		return nil, archiveLinkOutput{}, err
	}
	scope, err := t.authorizeArchives(ctx, user, input.ProjectID, input.ClientID)
	if err != nil {
		return nil, archiveLinkOutput{}, err
	}
	found, err := t.archives.Get(ctx, scope, archiveID)
	if err != nil {
		return nil, archiveLinkOutput{}, archiveFailure(ctx, "get archive", err)
	}
	expiresAt := time.Now().Add(archivelink.TTL)
	token := t.archiveLinks.Sign(archivelink.Claims{
		UserID: user.ID, ProjectID: scope.ProjectID, ClientID: scope.ClientID, ArchiveID: found.ID, ExpiresAt: expiresAt,
	})
	return nil, archiveLinkOutput{
		URL:       t.baseURL + archivelink.Path + token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		Archive:   summarizeArchive(found),
	}, nil
}

// authorizeArchives applies the archive UI's rule: administrators only.
func (t *tools) authorizeArchives(ctx context.Context, user svarogauth.User, projectID, clientID string) (archive.Scope, error) {
	if user.Role != svarogauth.ADMIN {
		return archive.Scope{}, errAdminOnly
	}
	scope, err := t.authorizeClient(ctx, user, projectID, clientID)
	if err != nil {
		return archive.Scope{}, err
	}
	return archive.Scope{ProjectID: scope.ProjectID, ClientID: scope.ClientID}, nil
}

func archiveFailure(ctx context.Context, action string, err error) error {
	switch {
	case errors.Is(err, archive.ErrArchiveNotFound):
		return errors.New("archive not found for this client")
	case errors.Is(err, archive.ErrClientNotFound):
		return errNotAccessible
	case errors.Is(err, archive.ErrNotConfigured):
		return errors.New("archive storage is not configured on this server")
	default:
		return toolFailure(ctx, action, err)
	}
}

func summarizeArchive(item archive.Archive) archiveSummary {
	return archiveSummary{
		ID:        item.ID.String(),
		FileName:  item.FileName,
		SizeBytes: item.SizeBytes,
		SHA256:    hex.EncodeToString(item.SHA256[:]),
		LogCount:  item.LogCount,
		FirstAt:   item.FromAt.UTC().Format(time.RFC3339Nano),
		LastAt:    item.ToAt.UTC().Format(time.RFC3339Nano),
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func encodeArchiveCursor(cursor archiveCursor) string {
	data, err := json.Marshal(cursor)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeArchiveCursor(encoded string) (archiveCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return archiveCursor{}, errInvalidArchiveCursor
	}
	var cursor archiveCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return archiveCursor{}, errInvalidArchiveCursor
	}
	return cursor, nil
}
