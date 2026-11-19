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
	require.NoError(t, err, "create a client for the server's own log records")
	clientID := project.Clients[0].ID
	config, err := s.NatsCredsService.GenerateConnString(ctx, serverauth.CredentialGenerationRequest{
		ProjectID: project.ID.String(), ClientID: clientID.String(),
	})
	require.NoError(t, err, "issue the server's publishing credentials")
	config.ServerAddr = strings.TrimPrefix(s.NatsAddr, "nats://")

	binary := filepath.Join(t.TempDir(), "server")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/server")
	build.Dir = "../.."
	output, err := build.CombinedOutput()
	require.NoError(t, err, "build the external server binary: %s", output)

	cmd := exec.CommandContext(ctx, binary)
	cmd.Dir = build.Dir
	cmd.Env = append(os.Environ(), "DOCKER=true", "HTTP_SERVER_PORT=0", "ARCHIVE_DIR=",
		"POSTGRES_URL="+s.PostgresPool.Config().ConnString(), "NATS_ADDR="+s.NatsAddr,
		"SVAROG_CONN_STRING="+config.GetConnString(), "SVAROG_INSTANCE_ID=test-server", "LOG_LEVEL=info",
		"SVAROG_LOG_QUEUE_CAPACITY=invalid")
	var console bytes.Buffer
	cmd.Stderr = &console
	require.NoError(t, cmd.Start(), "start the server with self-logging enabled")
	defer func() {
		if cmd.ProcessState == nil {
			cancel()
			_ = cmd.Wait()
		}
	}()
	hasLogMatching := func(pattern string) (bool, error) {
		var stored bool
		err := s.PostgresPool.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM logs WHERE client_id = $1 AND instance_name = 'test-server'
			AND message LIKE $2)`, clientID, pattern).Scan(&stored)
		return stored, err
	}
	// Only the real server is running a consumer. A post-broker startup record
	// in the client's history exercises both halves of self-logging.
	require.Eventually(t, func() bool {
		stored, err := hasLogMatching(`level=INFO msg="Connected to NATS"%`)
		return err == nil && stored
	}, 15*time.Second, 100*time.Millisecond, "a startup record written after the broker starts is forwarded and committed")
	require.NoError(t, cmd.Process.Signal(os.Interrupt), "request graceful server shutdown")
	err = cmd.Wait()
	require.NoError(t, err, "the server exits cleanly after SIGINT: %s", console.String())
	require.Contains(t, console.String(), "Invalid SVAROG_LOG_QUEUE_CAPACITY; using default", "the console reports that the queue uses its default capacity")
	require.Contains(t, console.String(), "PostgreSQL migrations applied", "the console reports that migrations completed")
	stored, err := hasLogMatching(`level=INFO msg="PostgreSQL migrations applied"`)
	require.NoError(t, err, "check the server client's logs for the migration record")
	require.False(t, stored, "migration logs written before broker startup are console-only")
}
