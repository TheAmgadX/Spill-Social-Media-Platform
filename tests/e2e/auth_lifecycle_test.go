//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
)

// TestAuthAndSessionLifecycle covers Journey 1:
// POST /auth/request-code -> POST /auth/verify -> POST /auth/signup ->
// GET /me -> PATCH /me -> POST /auth/refresh (both HttpOnly cookie and JSON body) ->
// POST /auth/logout -> subsequent refresh fails with 401.
func TestAuthAndSessionLifecycle(t *testing.T) {
	env := newE2EEnv(t)

	suffix := newUniqueSuffix(t)
	email := fmt.Sprintf("e2e_j1_%s@spill-test.example", suffix)
	username := fmt.Sprintf("e2e_j1_%s", suffix)
	initialDisplayName := "Journey One User"
	updatedDisplayName := "Journey One Updated"
	birthdate := "1998-06-20"

	env.trackEmail(email)
	cookieClient := newHTTPClient(t, true)
	cookielessClient := newHTTPClient(t, false)

	// 1. POST /auth/request-code
	resp := env.doJSON(t, cookieClient, http.MethodPost, "/auth/request-code", "", map[string]string{
		"email": email,
	})
	requireStatus(t, resp, http.StatusAccepted, "POST /auth/request-code")
	reqCodeRes := decodeJSON[map[string]string](t, resp)
	if reqCodeRes["message"] == "" {
		t.Fatalf("expected non-empty message from /auth/request-code")
	}

	// 2. POST /auth/verify (new user receives signupToken)
	otp := env.mailer.latestOTP(t, email)
	resp = env.doJSON(t, cookieClient, http.MethodPost, "/auth/verify", "", map[string]string{
		"email": email,
		"code":  otp,
	})
	requireStatus(t, resp, http.StatusOK, "POST /auth/verify")
	verifyRes := decodeJSON[views.VerifyCodeResponse](t, resp)
	if verifyRes.Action != "signup" || !verifyRes.NewUser || verifyRes.SignupToken == "" {
		t.Fatalf("expected signup action with signupToken, got %+v", verifyRes)
	}

	// 3. POST /auth/signup (sets HttpOnly refresh_token cookie and returns tokens + user)
	resp = env.doJSON(t, cookieClient, http.MethodPost, "/auth/signup", "", map[string]string{
		"signupToken": verifyRes.SignupToken,
		"username":    username,
		"displayName": initialDisplayName,
		"birthdate":   birthdate,
	})
	requireStatus(t, resp, http.StatusCreated, "POST /auth/signup")

	var refreshCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatalf("expected refresh_token Set-Cookie on /auth/signup")
	}
	if !refreshCookie.HttpOnly || refreshCookie.Path != "/auth" || refreshCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected refresh_token cookie attributes: HttpOnly=%v Path=%q SameSite=%v",
			refreshCookie.HttpOnly, refreshCookie.Path, refreshCookie.SameSite)
	}

	signupRes := decodeJSON[views.SignupResponse](t, resp)
	env.trackUser(signupRes.User.ID, email)

	if signupRes.AccessToken == "" || signupRes.RefreshToken == "" {
		t.Fatalf("expected non-empty accessToken and refreshToken on signup")
	}
	if signupRes.User.Username != username || signupRes.User.Email != email ||
		signupRes.User.DisplayName != initialDisplayName || signupRes.User.Birthdate != birthdate {
		t.Fatalf("unexpected signed-up user profile: %+v", signupRes.User)
	}

	// 4. GET /me
	resp = env.doJSON(t, cookieClient, http.MethodGet, "/me", signupRes.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "GET /me")
	meRes := decodeJSON[views.PrivateUserResponse](t, resp)
	if meRes.ID != signupRes.User.ID || meRes.Email != email || meRes.DisplayName != initialDisplayName {
		t.Fatalf("GET /me mismatch: %+v", meRes)
	}

	// 5. PATCH /me
	resp = env.doJSON(t, cookieClient, http.MethodPatch, "/me", signupRes.AccessToken, map[string]string{
		"displayName": updatedDisplayName,
	})
	requireStatus(t, resp, http.StatusOK, "PATCH /me")
	patchedMe := decodeJSON[views.PrivateUserResponse](t, resp)
	if patchedMe.DisplayName != updatedDisplayName {
		t.Fatalf("expected displayName %q after PATCH /me, got %q", updatedDisplayName, patchedMe.DisplayName)
	}

	// Confirm PATCH /me persisted in DB
	resp = env.doJSON(t, cookieClient, http.MethodGet, "/me", signupRes.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "GET /me after PATCH")
	persistedMe := decodeJSON[views.PrivateUserResponse](t, resp)
	if persistedMe.DisplayName != updatedDisplayName {
		t.Fatalf("expected persisted displayName %q, got %q", updatedDisplayName, persistedMe.DisplayName)
	}

	// 6a. POST /auth/refresh via refresh_token HttpOnly cookie (empty JSON body)
	resp = env.doJSON(t, cookieClient, http.MethodPost, "/auth/refresh", "", map[string]any{})
	requireStatus(t, resp, http.StatusOK, "POST /auth/refresh via HttpOnly cookie")
	cookieRefreshRes := decodeJSON[views.TokenResponse](t, resp)
	if cookieRefreshRes.AccessToken == "" || cookieRefreshRes.RefreshToken == "" {
		t.Fatalf("expected non-empty tokens from cookie refresh")
	}
	if cookieRefreshRes.RefreshToken == signupRes.RefreshToken {
		t.Fatalf("expected refresh token rotation on cookie refresh")
	}

	// Verify rotated access token works on GET /me
	resp = env.doJSON(t, cookieClient, http.MethodGet, "/me", cookieRefreshRes.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "GET /me with cookie-refreshed access token")
	_ = readBodyString(t, resp)

	// 6b. POST /auth/refresh via JSON body (using cookieless client)
	resp = env.doJSON(t, cookielessClient, http.MethodPost, "/auth/refresh", "", map[string]string{
		"refreshToken": cookieRefreshRes.RefreshToken,
	})
	requireStatus(t, resp, http.StatusOK, "POST /auth/refresh via JSON body")
	bodyRefreshRes := decodeJSON[views.TokenResponse](t, resp)
	if bodyRefreshRes.AccessToken == "" || bodyRefreshRes.RefreshToken == "" {
		t.Fatalf("expected non-empty tokens from JSON body refresh")
	}
	if bodyRefreshRes.RefreshToken == cookieRefreshRes.RefreshToken {
		t.Fatalf("expected refresh token rotation on JSON body refresh")
	}

	// Verify second rotated access token works on GET /me
	resp = env.doJSON(t, cookielessClient, http.MethodGet, "/me", bodyRefreshRes.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "GET /me with body-refreshed access token")
	_ = readBodyString(t, resp)

	// 7. POST /auth/logout (revokes active refresh token and clears cookie)
	resp = env.doJSON(t, cookieClient, http.MethodPost, "/auth/logout", bodyRefreshRes.AccessToken, map[string]string{
		"refreshToken": bodyRefreshRes.RefreshToken,
	})
	requireStatus(t, resp, http.StatusNoContent, "POST /auth/logout")

	var clearedCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			clearedCookie = c
			break
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge >= 0 {
		t.Fatalf("expected refresh_token cookie to be cleared (MaxAge < 0) on logout, got %+v", clearedCookie)
	}
	_ = readBodyString(t, resp)

	// 8. Subsequent refresh with revoked token must fail with 401 (both JSON body and cookie)
	resp = env.doJSON(t, cookielessClient, http.MethodPost, "/auth/refresh", "", map[string]string{
		"refreshToken": bodyRefreshRes.RefreshToken,
	})
	requireStatus(t, resp, http.StatusUnauthorized, "POST /auth/refresh after logout (body)")
	_ = readBodyString(t, resp)

	// Also verify explicit cookie replay of the revoked token fails with 401
	req, err := http.NewRequest(http.MethodPost, env.server.URL+"/auth/refresh", nil)
	if err != nil {
		t.Fatalf("failed to create cookie replay request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: bodyRefreshRes.RefreshToken})
	replayResp, err := cookielessClient.Do(req)
	if err != nil {
		t.Fatalf("cookie replay request failed: %v", err)
	}
	requireStatus(t, replayResp, http.StatusUnauthorized, "POST /auth/refresh after logout (cookie)")
	_ = readBodyString(t, replayResp)
}
