package serverauth

import (
	"context"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
)

func TestCredentialExpiry(t *testing.T) {
	service := newCredentialService(t)
	ctx := context.Background()

	t.Run("selected UTC day expires at the next midnight", func(t *testing.T) {
		now := time.Now().UTC()
		var expiry types.NullableDate
		require.NoError(t, expiry.UnmarshalParam(now.Format("2006-01-02")), "today's UTC date must bind")
		want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
		require.True(t, expiry.Time.Equal(want), "a selected day must stay valid through the end of that UTC day")

		creds, err := service.GenerateUserCreds(ctx, credentialRequest(expiry))
		require.NoError(t, err, "today must be accepted while the end of that UTC day is still in the future")
		claims := userClaims(t, creds)
		require.Equal(t, want.Unix(), claims.Expires, "JWT exp must be the next UTC midnight after the selected day")
		require.True(t, time.Unix(claims.Expires, 0).After(time.Now()), "the stored expiry must still be in the future")
	})

	t.Run("instant that is not strictly in the future is rejected", func(t *testing.T) {
		_, err := service.GenerateUserCreds(ctx, credentialRequest(types.NullableDate{Time: time.Now(), Valid: true}))
		require.ErrorIs(t, err, serverauth.ErrExpiryNotInFuture, "an expiry that is not strictly after now must not produce credentials")
	})

	t.Run("past day is rejected", func(t *testing.T) {
		var expiry types.NullableDate
		require.NoError(t, expiry.UnmarshalParam("2000-01-01"), "a past calendar day is still a valid date format")
		require.True(t, expiry.Time.Equal(time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)), "a past day is still stored as the start of the next UTC day")

		_, err := service.GenerateUserCreds(ctx, credentialRequest(expiry))
		require.ErrorIs(t, err, serverauth.ErrExpiryNotInFuture, "a day whose exclusive end is already past must not produce credentials")
	})

	t.Run("empty expiry does not set exp", func(t *testing.T) {
		var expiry types.NullableDate
		require.NoError(t, expiry.UnmarshalParam(""), "an empty expiry must bind as unset")
		require.False(t, expiry.Valid, "an empty expiry must mean the credential never expires")

		creds, err := service.GenerateUserCreds(ctx, credentialRequest(expiry))
		require.NoError(t, err, "credentials without an expiry must still be issued")
		require.Zero(t, userClaims(t, creds).Expires, "an empty expiry must not set a JWT exp claim")
	})

	t.Run("RFC3339 stays an absolute instant", func(t *testing.T) {
		instant := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
		var expiry types.NullableDate
		require.NoError(t, expiry.UnmarshalParam(instant.Format(time.RFC3339)), "an RFC3339 expiry must bind")
		require.True(t, expiry.Time.Equal(instant), "an RFC3339 expiry must not be shifted to the next UTC midnight")

		creds, err := service.GenerateUserCreds(ctx, credentialRequest(expiry))
		require.NoError(t, err, "a future absolute instant must be accepted")
		require.Equal(t, instant.Unix(), userClaims(t, creds).Expires, "JWT exp must be the posted RFC3339 instant")
	})

	t.Run("directly constructed future timestamp is unchanged", func(t *testing.T) {
		expiryTime := time.Now().Add(time.Hour).Truncate(time.Second)
		creds, err := service.GenerateUserCreds(ctx, credentialRequest(types.NullableDate{Time: expiryTime, Valid: true}))
		require.NoError(t, err, "callers that pass a future timestamp directly must still receive credentials")
		require.Equal(t, expiryTime.Unix(), userClaims(t, creds).Expires, "a directly constructed future timestamp must be stored as that instant")
	})
}

func newCredentialService(t *testing.T) *serverauth.NatsCredentialService {
	t.Helper()
	account, err := nkeys.CreateAccount()
	require.NoError(t, err, "a test account key must be created without a NATS server")
	seed, err := account.Seed()
	require.NoError(t, err, "a test account seed must be readable")
	service, err := serverauth.NewNatsCredentialService(string(seed), "nats.example:4222", &testutils.NoopProjectService{})
	require.NoError(t, err, "credential generation must be constructible without a running NATS server")
	return service
}

func credentialRequest(expiry types.NullableDate) serverauth.CredentialGenerationRequest {
	return serverauth.CredentialGenerationRequest{
		ProjectID: "project",
		ClientID:  "client",
		Expiry:    expiry,
	}
}

func userClaims(t *testing.T, creds string) *jwt.UserClaims {
	t.Helper()
	userJWT, _, err := serverauth.ParseCredsFile(creds)
	require.NoError(t, err, "generated credentials must contain a user JWT")
	claims, err := jwt.DecodeUserClaims(userJWT)
	require.NoError(t, err, "the generated user JWT must be decodable")
	return claims
}
