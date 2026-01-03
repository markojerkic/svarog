package auth

import (
	"context"
	"log/slog"
)

type UserContext int

const (
	UserKey UserContext = iota
	IsAdminKey
)

var userContextKey = map[UserContext]string{
	UserKey:    "user",
	IsAdminKey: "isAdmin",
}

func (c UserContext) String() string {
	return userContextKey[c]
}

func IsAdmin(ctx context.Context) bool {
	isAdmin, ok := ctx.Value(IsAdminKey).(bool)
	return ok && isAdmin
}
