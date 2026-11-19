package pages

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/markojerkic/svarog/internal/server/logstore"
)

type LogRange struct {
	logstore.TimeRange
	FromDate string
	ToDate   string
	Offset   int
}

func (p LogsPageProps) ToPath(instances *[]string, cursor *logstore.Cursor, direction, search string) string {
	u := url.URL{
		Path: fmt.Sprintf("/logs/%s/%s", p.ProjectId, p.ClientId),
	}
	query := u.Query()
	if cursor != nil {
		query.Set("cursorTimeUs", strconv.FormatInt(cursor.OccurredAt.UnixMicro(), 10))
		query.Set("cursorSequenceNumber", strconv.FormatInt(cursor.Sequence, 10))
		query.Set("cursorId", strconv.FormatInt(cursor.ID, 10))
	}
	if direction != "" {
		query.Set("direction", direction)
	}
	if instances != nil {
		for _, instance := range *instances {
			query.Add("instance", instance)
		}
	}
	if search != "" {
		query.Set("search", search)
	}
	if p.Range != nil {
		for key, values := range p.Range.Query() {
			query[key] = values
		}
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func (r *LogRange) Query() url.Values {
	query := url.Values{}
	if r == nil {
		return query
	}
	if r.FromDate != "" {
		query.Set("from", r.FromDate)
		query.Set("to", r.ToDate)
		query.Set("offset", strconv.Itoa(r.Offset))
	} else {
		query.Set("fromAt", r.From.Format(time.RFC3339Nano))
		query.Set("toAt", r.To.Format(time.RFC3339Nano))
	}
	return query
}
