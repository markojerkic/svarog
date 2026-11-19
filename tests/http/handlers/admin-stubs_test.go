package handlers_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
)

// Each test supplies callbacks for the routes it exercises. An unexpected
// service call fails the test instead of accepting a zero-value result.
type userRoutesStub struct {
	get    func(context.Context, string) (auth.User, error)
	page   func(context.Context, auth.PageQuery) ([]auth.User, int64, error)
	create func(context.Context, auth.UserInput) (auth.User, string, error)
	update func(context.Context, uuid.UUID, auth.UserInput) (auth.User, error)
}

func (s userRoutesStub) GetUserByID(ctx context.Context, id string) (auth.User, error) {
	return s.get(ctx, id)
}

func (s userRoutesStub) GetUserPage(ctx context.Context, query auth.PageQuery) ([]auth.User, int64, error) {
	return s.page(ctx, query)
}

func (s userRoutesStub) CreateUser(ctx context.Context, input auth.UserInput) (auth.User, string, error) {
	return s.create(ctx, input)
}

func (s userRoutesStub) UpdateUser(ctx context.Context, id uuid.UUID, input auth.UserInput) (auth.User, error) {
	return s.update(ctx, id, input)
}

func (userRoutesStub) DeleteUser(context.Context, string) error {
	panic("unexpected user deletion")
}

func (userRoutesStub) GenerateLoginToken(context.Context, string) (string, error) {
	panic("unexpected login token generation")
}

type projectRoutesStub struct {
	get           func(context.Context, string) (projects.Project, error)
	page          func(context.Context, projects.PageQuery) ([]projects.Project, int64, error)
	create        func(context.Context, projects.ProjectInput) (projects.Project, error)
	update        func(context.Context, uuid.UUID, projects.ProjectInput) (projects.Project, error)
	byIDs         func(context.Context, []string) ([]projects.Project, error)
	deleteProject func(context.Context, string, string) error
}

func (s projectRoutesStub) Get(ctx context.Context, id string) (projects.Project, error) {
	return s.get(ctx, id)
}

func (s projectRoutesStub) Page(ctx context.Context, query projects.PageQuery) ([]projects.Project, int64, error) {
	return s.page(ctx, query)
}

func (s projectRoutesStub) Create(ctx context.Context, input projects.ProjectInput) (projects.Project, error) {
	return s.create(ctx, input)
}

func (s projectRoutesStub) Update(ctx context.Context, id uuid.UUID, input projects.ProjectInput) (projects.Project, error) {
	return s.update(ctx, id, input)
}

func (s projectRoutesStub) ByIDs(ctx context.Context, ids []string) ([]projects.Project, error) {
	return s.byIDs(ctx, ids)
}

func (s projectRoutesStub) Delete(ctx context.Context, id, confirmationName string) error {
	return s.deleteProject(ctx, id, confirmationName)
}
