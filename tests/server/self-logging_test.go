package server_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/tests/testutils"
)

type serverSuite struct{ testutils.BaseSuite }

func TestServer(t *testing.T) { suite.Run(t, new(serverSuite)) }

func (s *serverSuite) TestSelfLogging() {
	t := s.T()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	project, err := s.ProjectsService.Create(ctx, projects.ProjectInput{Name: "Svarog", Clients: []string{"server"}})
	require.NoError(t, err)
	clientID := project.Clients[0].ID
	config, err := s.NatsCredsService.GenerateConnString(ctx, serverauth.CredentialGenerationRequest{
		ProjectID: project.ID.String(), ClientID: clientID.String(),
	})
	require.NoError(t, err)
	config.ServerAddr = strings.TrimPrefix(s.NatsAddr, "nats://")

	binary := filepath.Join(t.TempDir(), "server")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/server")
	build.Dir = "../.."
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)

	cmd := exec.CommandContext(ctx, binary)
	cmd.Dir = build.Dir
	cmd.Env = append(os.Environ(), "DOCKER=true", "HTTP_SERVER_PORT=0", "ARCHIVE_DIR=",
		"POSTGRES_URL="+s.PostgresPool.Config().ConnString(), "NATS_ADDR="+s.NatsAddr,
		"SVAROG_CONN_STRING="+config.GetConnString(), "SVAROG_INSTANCE_ID=test-server", "LOG_LEVEL=info",
		"SVAROG_LOG_QUEUE_CAPACITY=invalid")
	var console bytes.Buffer
	cmd.Stderr = &console
	require.NoError(t, cmd.Start())
	defer func() {
		if cmd.ProcessState == nil {
			cancel()
			_ = cmd.Wait()
		}
	}()
	// Only the real server is running a consumer. Seeing its startup record in
	// the client's history exercises both halves of self-logging.
	require.Eventually(t, func() bool {
		var stored bool
		err := s.PostgresPool.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM logs WHERE client_id = $1 AND instance_name = 'test-server'
			AND message = 'level=INFO msg="PostgreSQL migrations applied"')`, clientID).Scan(&stored)
		return err == nil && stored
	}, 15*time.Second, 100*time.Millisecond)
	require.NoError(t, cmd.Process.Signal(os.Interrupt))
	err = cmd.Wait()
	require.NoError(t, err, "%s", console.String())
	require.Contains(t, console.String(), "Invalid SVAROG_LOG_QUEUE_CAPACITY; using default")
	require.Contains(t, console.String(), "PostgreSQL migrations applied")
}
