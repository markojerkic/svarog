package mcpserver

import (
	"context"
	"errors"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	svarogauth "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
)

type tools struct {
	users    Users
	projects Projects
}

func registerTools(server *mcp.Server, t *tools) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_projects",
		Title:       "List projects",
		Description: "List the Svarog projects the signed-in user can access, with each project's active clients.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, t.listProjects)
}

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
	ID   string `json:"id"`
	Name string `json:"name"`
}

var errToolFailed = errors.New("the request could not be completed")

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
		slog.ErrorContext(ctx, "MCP list_projects failed", "error", err)
		return nil, listProjectsOutput{}, errToolFailed
	}

	output := listProjectsOutput{Projects: make([]projectSummary, 0, len(visible))}
	for _, project := range visible {
		clients := make([]clientSummary, 0, len(project.Clients))
		for _, client := range project.Clients {
			clients = append(clients, clientSummary{ID: client.ID.String(), Name: client.Name})
		}
		output.Projects = append(output.Projects, projectSummary{ID: project.ID.String(), Name: project.Name, Clients: clients})
	}
	return nil, output, nil
}

func (t *tools) currentUser(ctx context.Context, request *mcp.CallToolRequest) (svarogauth.User, error) {
	if request.Extra == nil || request.Extra.TokenInfo == nil {
		return svarogauth.User{}, errors.New("authentication required")
	}
	user, err := t.users.GetUserByID(ctx, request.Extra.TokenInfo.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "MCP user lookup failed", "error", err)
		return svarogauth.User{}, errToolFailed
	}
	return user, nil
}
