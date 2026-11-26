package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/auth"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
)

type AuthController struct {
	authService *services.AuthService
	jwtSecret   []byte
}

func NewAuthController(authService *services.AuthService, jwtSecret []byte) *AuthController {
	return &AuthController{
		authService: authService,
		jwtSecret:   jwtSecret,
	}
}

type RequestCodeRequest struct {
	Email string `json:"email"`
}

type VerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type SignupRequest struct {
	SignupToken      string `json:"signup_token"`
	SignupTokenCamel string `json:"signupToken"`
	Username         string `json:"username"`
	DisplayName      string `json:"display_name"`
	DisplayNameCamel string `json:"displayName"`
	Birthdate        string `json:"birthdate"`
}

type RefreshRequest struct {
	RefreshToken      string `json:"refresh_token"`
	RefreshTokenCamel string `json:"refreshToken"`
}

type LogoutRequest struct {
	RefreshToken      string `json:"refresh_token"`
	RefreshTokenCamel string `json:"refreshToken"`
}

// RequestCode handles POST /auth/request-code.
// Always returns 202 Accepted for syntactically valid emails to prevent email enumeration.
func (c *AuthController) RequestCode(w http.ResponseWriter, r *http.Request) {
	var req RequestCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		views.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request body")
		return
	}

	normalizedEmail, err := services.ValidateRequestCode(req.Email)
	if err != nil {
		views.WriteProblem(w, r, http.StatusUnprocessableEntity, err.Error())
		return
	}

	_ = c.authService.RequestCode(r.Context(), normalizedEmail)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "If the email is valid, a login code has been sent.",
	})
}

// Verify handles POST /auth/verify.
func (c *AuthController) Verify(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		views.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request body")
		return
	}

	trimmedEmail := strings.TrimSpace(req.Email)
	trimmedCode := strings.TrimSpace(req.Code)
	if trimmedEmail == "" || len(trimmedCode) != 6 {
		views.WriteProblem(w, r, http.StatusUnprocessableEntity, "email and 6-digit code are required")
		return
	}

	result, err := c.authService.VerifyCode(r.Context(), trimmedEmail, trimmedCode, r.UserAgent())
	if err != nil {
		if errors.Is(err, services.ErrTooManyAttempts) {
			views.WriteProblem(w, r, http.StatusUnauthorized, err.Error())
			return
		}
		if errors.Is(err, services.ErrInvalidCredentials) {
			views.WriteProblem(w, r, http.StatusUnauthorized, "invalid or expired verification code")
			return
		}
		views.WriteProblem(w, r, http.StatusInternalServerError, "verification failed")
		return
	}

	if result.IsNewUser {
		writeJSON(w, http.StatusOK, views.VerifyCodeResponse{
			Action:           "signup",
			NewUser:          true,
			SignupToken:      result.SignupToken,
			SignupTokenSnake: result.SignupToken,
		})
		return
	}

	// Set refresh token cookie
	if result.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    result.RefreshToken,
			Path:     "/auth",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
		})
	}

	writeJSON(w, http.StatusOK, views.VerifyCodeResponse{
		Action:            "login",
		NewUser:           false,
		AccessToken:       result.AccessToken,
		RefreshToken:      result.RefreshToken,
		AccessTokenSnake:  result.AccessToken,
		RefreshTokenSnake: result.RefreshToken,
	})
}

// Signup handles POST /auth/signup.
func (c *AuthController) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		views.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid request body")
		return
	}

	signupToken := req.SignupToken
	if signupToken == "" {
		signupToken = req.SignupTokenCamel
	}
	if signupToken == "" {
		authHeader := r.Header.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			signupToken = parts[1]
		}
	}

	if signupToken == "" {
		views.WriteProblem(w, r, http.StatusUnauthorized, "signup token is required")
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.DisplayNameCamel
	}

	tokens, user, err := c.authService.Signup(r.Context(), signupToken, req.Username, displayName, req.Birthdate, r.UserAgent())
	if err != nil {
		if strings.Contains(err.Error(), "invalid signup token") {
			views.WriteProblem(w, r, http.StatusUnauthorized, err.Error())
			return
		}
		if errors.Is(err, services.ErrInvalidUsername) ||
			errors.Is(err, services.ErrInvalidDisplayName) ||
			errors.Is(err, services.ErrInvalidBirthdate) ||
			errors.Is(err, services.ErrUnderage) ||
			errors.Is(err, services.ErrValidation) {
			views.WriteProblem(w, r, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, models.ErrUsernameTaken) {
			views.WriteProblem(w, r, http.StatusConflict, "username is already taken")
			return
		}
		if errors.Is(err, models.ErrEmailRegistered) {
			views.WriteProblem(w, r, http.StatusConflict, "email is already registered")
			return
		}
		views.WriteProblem(w, r, http.StatusInternalServerError, "signup failed")
		return
	}

	if tokens.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    tokens.RefreshToken,
			Path:     "/auth",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
		})
	}

	writeJSON(w, http.StatusCreated, views.SignupResponse{
		AccessToken:       tokens.AccessToken,
		RefreshToken:      tokens.RefreshToken,
		AccessTokenSnake:  tokens.AccessToken,
		RefreshTokenSnake: tokens.RefreshToken,
		User: views.PrivateUserResponse{
			ID:             user.ID,
			Username:       user.Username,
			Email:          user.Email,
			DisplayName:    user.DisplayName,
			Birthdate:      user.Birthdate.Format("2006-01-02"),
			Role:           user.Role,
			FollowerCount:  user.FollowerCount,
			FollowingCount: user.FollowingCount,
			CreatedAt:      user.CreatedAt.Format(time.RFC3339),
		},
	})
}

// Refresh handles POST /auth/refresh.
func (c *AuthController) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	refreshToken := req.RefreshToken
	if refreshToken == "" {
		refreshToken = req.RefreshTokenCamel
	}
	if refreshToken == "" {
		if cookie, err := r.Cookie("refresh_token"); err == nil {
			refreshToken = cookie.Value
		}
	}

	if refreshToken == "" {
		views.WriteProblem(w, r, http.StatusUnprocessableEntity, "refresh token is required")
		return
	}

	tokens, err := c.authService.RefreshToken(r.Context(), refreshToken, r.UserAgent())
	if err != nil {
		views.WriteProblem(w, r, http.StatusUnauthorized, "invalid, expired, or revoked refresh token")
		return
	}

	if tokens.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    tokens.RefreshToken,
			Path:     "/auth",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
		})
	}

	writeJSON(w, http.StatusOK, views.TokenResponse{
		AccessToken:       tokens.AccessToken,
		RefreshToken:      tokens.RefreshToken,
		AccessTokenSnake:  tokens.AccessToken,
		RefreshTokenSnake: tokens.RefreshToken,
	})
}

// Logout handles POST /auth/logout.
// Requires Bearer access token authorization and revokes the provided refresh token.
func (c *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		views.WriteProblem(w, r, http.StatusUnauthorized, "authorization header required")
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		views.WriteProblem(w, r, http.StatusUnauthorized, "invalid authorization header format, expected Bearer <token>")
		return
	}

	tokenStr := strings.TrimSpace(parts[1])
	if len(c.jwtSecret) > 0 {
		if _, err := auth.ParseAndVerifyAccessToken(tokenStr, c.jwtSecret); err != nil {
			views.WriteProblem(w, r, http.StatusUnauthorized, "invalid or expired access token")
			return
		}
	}

	var req LogoutRequest
	if r.Body != nil {
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil && err != io.EOF {
			views.WriteProblem(w, r, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	refreshToken := req.RefreshToken
	if refreshToken == "" {
		refreshToken = req.RefreshTokenCamel
	}
	if refreshToken == "" {
		if cookie, err := r.Cookie("refresh_token"); err == nil {
			refreshToken = cookie.Value
		}
	}

	_ = c.authService.Logout(r.Context(), refreshToken)

	// Clear refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/auth",
		MaxAge:   -1,
		HttpOnly: true,
	})

	w.WriteHeader(http.StatusNoContent)
}
