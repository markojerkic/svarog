package grpcserver

import (
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"slices"
)

// projectId to clientIds mapping
var projectsCache = make(map[string][]string)
var projectsCacheMutex = &sync.Mutex{}
var cacheCreated time.Time = time.Unix(0, 0)

func (g *GrpcServer) isAuthorized(ctx context.Context, projectId string, clientName string) bool {
	g.checkIfCacheValid(ctx)

	clientIds, ok := projectsCache[projectId]
	if !ok {
		return false
	}

	return slices.Contains(clientIds, clientName)

}

func (g *GrpcServer) checkIfCacheValid(ctx context.Context) {
	if time.Since(cacheCreated) > 10*time.Minute {
		projectsCacheMutex.Lock()
		defer projectsCacheMutex.Unlock()
		projectsCache = make(map[string][]string)

		projects, err := g.projectsService.GetProjects(ctx)
		if err != nil {
			log.Error("Failed to get projects", "error", err)
			return
		}
		for _, project := range projects {
			projectsCache[project.ID.Hex()] = project.Clients
		}

		cacheCreated = time.Now()
	}
}
