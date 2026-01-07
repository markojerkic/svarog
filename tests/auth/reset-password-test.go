package auth

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (suite *AuthSuite) TestResetPassword() {
	t := suite.T()
	e := echo.New()

	// Setup initial user
	req := httptest.NewRequest(http.MethodPost, "/register", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	user, err := suite.authService.Register(ctx, types.RegisterForm{
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	})
	assert.NoError(t, err)
	assert.NotNil(t, user)

	// Test cases
	testCases := []struct {
		name          string
		resetForm     types.ResetPasswordForm
		expectedError bool
	}{
		{
			name: "successful password reset",
			resetForm: types.ResetPasswordForm{
				Password:         "newpassword123",
				RepeatedPassword: "newpassword123",
			},
			expectedError: false,
		},
		{
			name: "mismatched passwords",
			resetForm: types.ResetPasswordForm{
				Password:         "newpassword123",
				RepeatedPassword: "differentpassword",
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Login to get session
			// loginReq := httptest.NewRequest(http.MethodPost, "/login", nil)
			// loginRec := httptest.NewRecorder()
			// loginCtx := e.NewContext(loginReq, loginRec)
			// err = suite.authService.Login(loginCtx, "testuser", "oldpassword")
			// assert.NoError(t, err)

			// Perform password reset
			resetReq := httptest.NewRequest(http.MethodPost, "/reset-password", nil)
			resetRec := httptest.NewRecorder()
			resetCtx := e.NewContext(resetReq, resetRec)

			user, err := suite.authService.GetUserByUsername(context.Background(), "testuser")
			assert.NoError(t, err)
			userId := user.ID.Hex()

			err = suite.authService.ResetPassword(resetCtx.Request().Context(), userId, tc.resetForm)

			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// Verify old password no longer works
			oldPassReq := httptest.NewRequest(http.MethodPost, "/login", nil)
			oldPassRec := httptest.NewRecorder()
			oldPassCtx := e.NewContext(oldPassReq, oldPassRec)
			err = suite.authService.Login(oldPassCtx, "testuser", "oldpassword")
			assert.Error(t, err, "old password should no longer work")

			// Verify new password works
			if !tc.expectedError {
				newPassReq := httptest.NewRequest(http.MethodPost, "/login", nil)
				newPassRec := httptest.NewRecorder()
				newPassCtx := e.NewContext(newPassReq, newPassRec)
				err = suite.authService.Login(newPassCtx, "testuser", tc.resetForm.Password)
				assert.NoError(t, err, "new password should work")
			}

			// Verify user state
			updatedUser, err := suite.authService.GetUserByUsername(context.Background(), "testuser")
			assert.NoError(t, err)
			assert.False(t, updatedUser.NeedsPasswordReset)
		})
	}
}
