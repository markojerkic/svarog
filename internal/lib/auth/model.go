package auth

import "github.com/google/uuid"

type Role string

const (
	ADMIN Role = "admin"
	USER  Role = "user"
)

type User struct {
	ID                 uuid.UUID `json:"id"`
	FirstName          string    `json:"firstName"`
	LastName           string    `json:"lastName"`
	Username           string    `json:"username"`
	Password           string    `json:"-"`
	Role               Role      `json:"role"`
	NeedsPasswordReset bool      `json:"needsPasswordReset"`
	ProjectIDs         []string  `json:"projectIds,omitempty"`
}

type LoggedInUser struct {
	ID                 string   `json:"id"`
	Username           string   `json:"username"`
	Role               Role     `json:"role"`
	NeedsPasswordReset bool     `json:"needsPasswordReset"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
}

type UserInput struct {
	Username, FirstName, LastName string
	Role                          Role
	ProjectIDs                    []uuid.UUID
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
