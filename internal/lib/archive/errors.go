package archive

import "errors"

var (
	ErrInvalidInput       = errors.New("invalid archive input")
	ErrClientNotFound     = errors.New("client not found in project")
	ErrArchiveNotFound    = errors.New("archive not found")
	ErrNotConfigured      = errors.New("archiving is not configured")
	ErrArchiveUnavailable = errors.New("archive content is unavailable")
)
