package mcpserver

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	svarogauth "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/logstore"
)

type tools struct {
	baseURL      string
	users        Users
	projects     Projects
	logs         LogReader
	archives     Archives
	archiveLinks ArchiveLinks
}

func registerTools(server *mcp.Server, t *tools) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  "list_projects",
		Title: "List projects",
		Description: "List the projects the signed-in user can access, with all of their clients. " +
			"Inactive clients no longer send logs, but their retained logs remain searchable.",
		Annotations: readOnly(),
	}, t.listProjects)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_instances",
		Title:       "List client instances",
		Description: "List the instance names that have retained logs for a client. An empty name is the unnamed instance.",
		Annotations: readOnly(),
	}, t.listInstances)
	mcp.AddTool(server, &mcp.Tool{
		Name:  "search_logs",
		Title: "Search logs",
		Description: "Return the newest unarchived log lines of a client that match the filters, listed from oldest to newest. " +
			"The query is a list of space-separated literal terms; a line matches when it contains all of them, ignoring case. " +
			"Without a query, the newest lines are returned. Messages longer than 2000 characters are truncated; " +
			"get_log_context returns the full anchor line. Use older_cursor with get_log_page for earlier matches.",
		Annotations: readOnly(),
	}, t.searchLogs)
	mcp.AddTool(server, &mcp.Tool{
		Name:  "get_log_context",
		Title: "Get log context",
		Description: "Return a log line and the lines around it from the same instance, regardless of any search. " +
			"Identify the line by its link, or by project_id, client_id and log_id. The anchor line is marked and never truncated. " +
			"Use older_cursor or newer_cursor with get_log_page to read further in either direction.",
		Annotations: readOnly(),
	}, t.getLogContext)
	mcp.AddTool(server, &mcp.Tool{
		Name:  "get_log_page",
		Title: "Get next log page",
		Description: "Continue a previous search_logs, get_log_context or get_log_page result with its older_cursor or newer_cursor. " +
			"The cursor keeps the original filters. A missing cursor in the result means there are no more lines in that direction. " +
			"A page can be empty when the previous page ended exactly at the last line.",
		Annotations: readOnly(),
	}, t.getLogPage)
	mcp.AddTool(server, &mcp.Tool{
		Name:  "list_archives",
		Title: "List log archives",
		Description: "List a client's completed log archives, newest first. Use search_archived_logs to search their contents. " +
			"Requires the administrator role.",
		Annotations: readOnly(),
	}, t.listArchives)
	mcp.AddTool(server, &mcp.Tool{
		Name:  "search_archived_logs",
		Title: "Search archived logs",
		Description: "Search a client's completed archives for messages in [from, to). Requires the administrator role. " +
			"Returns total matches and up to the newest 200 matching records, listed from oldest to newest. " +
			"Messages longer than 2000 characters are truncated. Narrow the time range if the scan limit is exceeded.",
		Annotations: readOnly(),
	}, t.searchArchivedLogs)
	mcp.AddTool(server, &mcp.Tool{
		Name:  "get_archive_download_link",
		Title: "Get archive download link",
		Description: "Return a link that downloads an archive ZIP without signing in. It expires after 15 minutes " +
			"and works only while the user keeps access; treat it as a secret. Requires the administrator role.",
		Annotations: readOnly(),
	}, t.getArchiveDownloadLink)
}

func readOnly() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
}

var (
	errToolFailed    = errors.New("the request could not be completed")
	errNotAccessible = errors.New("the project or client was not found, or you do not have access to it")
	errAdminOnly     = errors.New("archives are available only to administrators")
)

type listProjectsInput struct{}

type listProjectsOutput struct {
	Projects []projectSummary `json:"projects"`
}

type projectSummary struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Clients []clientSummary `json:"clients"`
}

type clientSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func (t *tools) listProjects(ctx context.Context, request *mcp.CallToolRequest, _ listProjectsInput) (*mcp.CallToolResult, listProjectsOutput, error) {
	user, err := t.currentUser(ctx, request)
	if err != nil {
		return nil, listProjectsOutput{}, err
	}
	// Access is resolved on every call, so membership changes apply immediately.
	var visible []projects.Project
	if user.Role == svarogauth.ADMIN {
		visible, err = t.projects.All(ctx)
	} else if len(user.ProjectIDs) > 0 {
		visible, err = t.projects.ByIDs(ctx, user.ProjectIDs)
	}
	if err != nil {
		return nil, listProjectsOutput{}, toolFailure(ctx, "list projects", err)
	}

	output := listProjectsOutput{Projects: make([]projectSummary, len(visible))}
	index := make(map[uuid.UUID]int, len(visible))
	ids := make([]uuid.UUID, len(visible))
	for i, project := range visible {
		output.Projects[i] = projectSummary{ID: project.ID.String(), Name: project.Name, Clients: []clientSummary{}}
		index[project.ID] = i
		ids[i] = project.ID
	}
	if len(ids) == 0 {
		return nil, output, nil
	}
	clients, err := t.projects.ClientsForProjects(ctx, ids)
	if err != nil {
		return nil, listProjectsOutput{}, toolFailure(ctx, "list clients", err)
	}
	for _, client := range clients {
		project := &output.Projects[index[client.ProjectID]]
		project.Clients = append(project.Clients, clientSummary{ID: client.ID.String(), Name: client.Name, Active: client.Active})
	}
	return nil, output, nil
}

func (t *tools) currentUser(ctx context.Context, request *mcp.CallToolRequest) (svarogauth.User, error) {
	if request.Extra == nil || request.Extra.TokenInfo == nil {
		return svarogauth.User{}, errors.New("authentication required")
	}
	user, err := t.users.GetUserByID(ctx, request.Extra.TokenInfo.UserID)
	if err != nil {
		return svarogauth.User{}, toolFailure(ctx, "look up user", err)
	}
	return user, nil
}

// authorizeClient applies the web UI's rules: the user must have access to
// the project, and the client must belong to it. Inactive clients stay
// readable for their retained logs.
func (t *tools) authorizeClient(ctx context.Context, user svarogauth.User, projectID, clientID string) (logstore.Scope, error) {
	project, err := uuid.Parse(projectID)
	client, clientErr := uuid.Parse(clientID)
	if err != nil || clientErr != nil || project == uuid.Nil || client == uuid.Nil {
		return logstore.Scope{}, errors.New("project_id and client_id must be UUIDs from list_projects")
	}
	allowed, err := t.users.UserHasAccessToProject(ctx, user.ID.String(), project.String())
	if err != nil {
		return logstore.Scope{}, toolFailure(ctx, "check project access", err)
	}
	if !allowed {
		return logstore.Scope{}, errNotAccessible
	}
	belongs, err := t.projects.ClientBelongsToProject(ctx, project.String(), client.String())
	if err != nil {
		return logstore.Scope{}, toolFailure(ctx, "check client ownership", err)
	}
	if !belongs {
		return logstore.Scope{}, errNotAccessible
	}
	return logstore.Scope{ProjectID: project, ClientID: client}, nil
}

// toolFailure logs an internal error and hides its details from the client.
func toolFailure(ctx context.Context, action string, err error) error {
	slog.ErrorContext(ctx, "MCP tool failed", "action", action, "error", err)
	return errToolFailed
}
