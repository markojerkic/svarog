package auth_test

import (
	"context"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestMongoAuthService_ResetPassword(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		userCollection    *mongo.Collection
		sessionCollection *mongo.Collection
		client            *mongo.Client
		sessionStore      sessions.Store
		// Named input parameters for target function.
		userId  string
		form    types.ResetPasswordForm
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			self := auth.NewMongoAuthService(tt.userCollection, tt.sessionCollection, tt.client, tt.sessionStore)
			gotErr := self.ResetPassword(context.Background(), tt.userId, tt.form)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("ResetPassword() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("ResetPassword() succeeded unexpectedly")
			}
		})
	}
}
