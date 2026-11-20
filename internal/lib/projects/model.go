package projects

import "github.com/google/uuid"

type Project struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Clients            []Client  `json:"clients"`
	HasRetainedClients bool      `json:"hasRetainedClients"`
	TotalStorageSize   float64   `json:"totalStorageSize"`
}

type Client struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ProjectClient is a client of any status. Inactive clients keep their
// retained logs readable.
type ProjectClient struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Name      string
	Active    bool
}

func (p Project) ClientNames() []string {
	names := make([]string, len(p.Clients))
	for i, client := range p.Clients {
		names[i] = client.Name
	}
	return names
}

type ProjectInput struct {
	Name    string
	Clients []string
}

// PageQuery is constructed by NewPageQuery. Its zero value is the default first page.
type PageQuery struct {
	search     string
	page, size int64
}

func (q PageQuery) Search() string { return q.search }
func (q PageQuery) Page() int64    { return q.page }

func (q PageQuery) Size() int64 {
	if q.size == 0 {
		return 10
	}
	return q.size
}

// ClampPage moves to an earlier available page without changing the search or size.
func (q PageQuery) ClampPage(maximum int64) PageQuery {
	q.page = min(q.page, max(0, maximum))
	return q
}
