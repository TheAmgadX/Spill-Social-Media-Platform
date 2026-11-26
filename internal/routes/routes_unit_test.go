package routes

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/config"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/controllers"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
)

func TestSetupRouter_MethodNotAllowedAndAllowHeader(t *testing.T) {
	authCtrl := controllers.NewAuthController(&services.AuthService{}, []byte("01234567890123456789012345678901"))
	userCtrl := controllers.NewUserController(&services.UserService{})
	postCtrl := controllers.NewPostController(&services.PostService{})

	r := SetupRouter(RouterConfig{
		AuthController: authCtrl,
		UserController: userCtrl,
		PostController: postCtrl,
		JWTSecret:      []byte("01234567890123456789012345678901"),
	})

	tests := []struct {
		path      string
		wantAllow string
	}{
		{"/auth/request-code", "POST"},
		{"/auth/verify", "POST"},
		{"/auth/signup", "POST"},
		{"/auth/refresh", "POST"},
		{"/auth/logout", "POST"},
		{"/api/v1/auth/request-code", "POST"},
		{"/me", "GET, PATCH"},
		{"/api/v1/posts/550e8400-e29b-41d4-a716-446655440010", "GET, PATCH, DELETE"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodTrace, tt.path, nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("TRACE %s: expected 405, got %d", tt.path, rr.Code)
		}
		gotAllow := rr.Header().Get("Allow")
		if gotAllow != tt.wantAllow {
			t.Errorf("TRACE %s: expected Allow=%q, got %q", tt.path, tt.wantAllow, gotAllow)
		}
	}
}

func TestRequestCode_RejectsOversizedEmail(t *testing.T) {
	authCtrl := controllers.NewAuthController(&services.AuthService{}, []byte("01234567890123456789012345678901"))
	r := SetupRouter(RouterConfig{
		AuthController: authCtrl,
		JWTSecret:      []byte("01234567890123456789012345678901"),
	})

	oversized := `{"email":"0@0.com` + strings.Repeat("*", 248) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/request-code", bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for >254 char email, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSignup_RejectsNullOrEmptyUsernameWith422(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: []byte("01234567890123456789012345678901"),
	}
	authSvc := services.NewAuthService(nil, nil, cfg)
	authCtrl := controllers.NewAuthController(authSvc, cfg.JWTSecret)
	r := SetupRouter(RouterConfig{
		AuthController: authCtrl,
		JWTSecret:      cfg.JWTSecret,
	})

	for _, payload := range []string{
		`{"username": null, "displayName": "Alice", "birthdate": "2000-05-15"}`,
		`{"username": "", "displayName": "Alice", "birthdate": "2000-05-15"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer dummy-signup-token")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("payload %s: expected 422, got %d: %s", payload, rr.Code, rr.Body.String())
		}
	}
}
