package projects

import (
	"context"

	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (p *ProjectsSuite) TestCreateProject() {
	t := p.Suite.T()

	testCases := []struct {
		name    string
		clients []string
		wantErr bool
	}{
		{
			name:    "test1",
			clients: []string{"test client"},
			wantErr: false,
		},
		{
			// Project name already exists
			name:    "test1",
			clients: []string{"test client"},
			wantErr: true,
		},
		{
			name: "test2",
			// Client name already exists
			clients: []string{"test client", "test client"},
			wantErr: false,
		},
		{
			name:    "test3",
			clients: []string{"test client", "test client2"},
			wantErr: false,
		},
	}

	for i, tc := range testCases {
		_, err := p.ProjectsService.CreateProject(context.Background(), tc.name, tc.clients)
		if tc.wantErr {
			assert.Error(t, err, "Test case %d failed", i)
		} else {
			assert.NoError(t, err, "Test case %d failed", i)
		}
	}

}

func (p *ProjectsSuite) TestGetProject() {
	t := p.Suite.T()

	project, err := p.ProjectsService.CreateProject(context.Background(), "test1", []string{"test client"})
	if err != nil {
		t.Fatalf("Could not create project: %s", err)
	}

	testCases := []struct {
		name    string
		id      string
		clients []string
		wantErr bool
	}{
		{
			name:    "test1",
			id:      project.ID.Hex(),
			clients: []string{"test client"},
			wantErr: false,
		},
		{
			name:    "test2",
			id:      primitive.NewObjectID().Hex(),
			clients: []string{"test client", "test client2"},
			wantErr: true,
		},
	}

	for i, tc := range testCases {
		_, err = p.ProjectsService.GetProject(context.Background(), tc.id)
		if tc.wantErr {
			assert.Error(t, err, "Test case %d failed", i)
			if err != nil {
				assert.Equal(t, projects.ErrProjectNotFound, err.Error(), "Test case %d failed", i)
			}
		} else {
			assert.NoError(t, err, "Test case %d failed", i)
		}
	}
}

func (p *ProjectsSuite) TestGetProjectByClient() {
	t := p.Suite.T()

	targetClient := "test client"
	_, err := p.ProjectsService.CreateProject(context.Background(), "test1", []string{targetClient})
	if err != nil {
		t.Fatalf("Could not create project: %s", err)
	}

	testCases := []struct {
		client  string
		wantErr bool
	}{
		{
			client:  targetClient,
			wantErr: false,
		},
		{
			client:  "test client2",
			wantErr: true,
		},
	}

	for i, tc := range testCases {
		_, err = p.ProjectsService.GetProjectByClient(context.Background(), tc.client)
		if tc.wantErr {
			assert.Error(t, err, "Test case %d failed", i)
			if err != nil {
				assert.Equal(t, projects.ErrProjectNotFound, err.Error(), "Test case %d failed", i)
			}
		} else {
			assert.NoError(t, err, "Test case %d failed", i)
		}
	}
}

func (p *ProjectsSuite) TestDeleteProject() {
	t := p.Suite.T()

	project, err := p.ProjectsService.CreateProject(context.Background(), "test1", []string{"test client"})
	if err != nil {
		t.Fatalf("Could not create project: %s", err)
	}

	err = p.ProjectsService.DeleteProject(context.Background(), project.ID.Hex())
	assert.NoError(t, err)

	_, err = p.ProjectsService.GetProject(context.Background(), project.ID.Hex())
	assert.Error(t, err)
}
