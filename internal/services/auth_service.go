package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/auth"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/config"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/mailer"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid or expired verification code")
	ErrTooManyAttempts    = errors.New("maximum verification attempts exceeded")
	ErrInvalidUsername    = fmt.Errorf("%w: username must be 3-20 characters long and contain only lowercase letters, numbers, and underscores", ErrValidation)
	ErrInvalidDisplayName = fmt.Errorf("%w: display name must be between 1 and 50 characters", ErrValidation)
	ErrInvalidBirthdate   = fmt.Errorf("%w: invalid birthdate format, expected YYYY-MM-DD", ErrValidation)
	ErrUnderage           = fmt.Errorf("%w: users must be at least 13 years old to sign up", ErrValidation)
	ErrTokenRevoked       = errors.New("refresh token has been revoked")
	ErrTokenExpired       = errors.New("refresh token has expired")
)

type AuthRepository interface {
	CheckLoginCodeCooldown(ctx context.Context, email string, cooldown time.Duration) (bool, error)
	CreateLoginCode(ctx context.Context, email, codeHash string, expiresAt time.Time) error
	GetLatestActiveCode(ctx context.Context, email string) (*models.LoginCode, error)
	IncrementAttempts(ctx context.Context, id string) error
	MarkCodeConsumed(ctx context.Context, id string) error
	FindUserByEmail(ctx context.Context, email string) (*models.User, error)
	FindUserByID(ctx context.Context, id string) (*models.User, error)
	IsUsernameTaken(ctx context.Context, username string) (bool, error)
	CreateUser(ctx context.Context, username, email, displayName string, birthdate time.Time, role string) (*models.User, error)
	CreateRefreshToken(ctx context.Context, userID, tokenHash string, userAgent *string, expiresAt time.Time) error
	FindRefreshTokenByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	RotateRefreshToken(ctx context.Context, oldTokenID, userID, newTokenHash string, userAgent *string, expiresAt time.Time) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
	RevokeRefreshTokenByHash(ctx context.Context, tokenHash string) error
}

type AuthService struct {
	repo   AuthRepository
	mailer mailer.Mailer
	cfg    *config.Config
}

func NewAuthService(repo AuthRepository, m mailer.Mailer, cfg *config.Config) *AuthService {
	return &AuthService{
		repo:   repo,
		mailer: m,
		cfg:    cfg,
	}
}

type VerifyResult struct {
	IsNewUser    bool
	User         *models.User
	AccessToken  string
	RefreshToken string
	SignupToken  string
}

type AuthTokens struct {
	AccessToken  string
	RefreshToken string
}

// RequestCode validates email, generates a 6-digit code, hashes it with HMAC-SHA256,
// enforces a 60-second cooldown in the database, and sends it via mailer.
// Always returns nil (HTTP 202) regardless of whether the email was valid or on cooldown to prevent email enumeration.
func (s *AuthService) RequestCode(ctx context.Context, rawEmail string) error {
	normalizedEmail, err := ValidateRequestCode(rawEmail)
	if err != nil {
		// Return without error to maintain constant response and prevent email enumeration
		return nil
	}

	// Database check: Enforce one active code per email per 60 seconds
	onCooldown, err := s.repo.CheckLoginCodeCooldown(ctx, normalizedEmail, s.cfg.OTPResendCooldown)
	if err != nil {
		return fmt.Errorf("failed to check cooldown: %w", err)
	}
	if onCooldown {
		// Silently drop duplicate request within cooldown period
		return nil
	}

	code, err := auth.GenerateOTP()
	if err != nil {
		return fmt.Errorf("failed to generate code: %w", err)
	}

	codeHash := auth.HashOTP(code, s.cfg.OTPHMACKey)
	expiresAt := time.Now().Add(s.cfg.OTPTTL)

	if err := s.repo.CreateLoginCode(ctx, normalizedEmail, codeHash, expiresAt); err != nil {
		return fmt.Errorf("failed to persist login code: %w", err)
	}

	// Send code via configured mailer. Rule: NEVER log the code itself!
	subject := "Your Spill Verification Code"
	body := fmt.Sprintf("Your login code is: %s\n\nThis code will expire in %d minutes.\nIf you did not request this code, you can safely ignore this email.", code, int(s.cfg.OTPTTL.Minutes()))

	if err := s.mailer.Send(normalizedEmail, subject, body); err != nil {
		// Log delivery error (without sensitive code)
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}

// VerifyCode verifies the latest unconsumed, unexpired code for an email.
func (s *AuthService) VerifyCode(ctx context.Context, rawEmail string, rawCode string, userAgent string) (*VerifyResult, error) {
	normalizedEmail, code, cleanUA, err := ValidateVerifyCode(rawEmail, rawCode, userAgent)
	if err != nil {
		return nil, err
	}

	activeCode, err := s.repo.GetLatestActiveCode(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Brute-force protection: reject when attempts >= 5
	if activeCode.Attempts >= int16(s.cfg.OTPMaxAttempts) {
		return nil, ErrTooManyAttempts
	}

	// Constant-time HMAC comparison
	if !auth.VerifyOTP(code, s.cfg.OTPHMACKey, activeCode.CodeHash) {
		_ = s.repo.IncrementAttempts(ctx, activeCode.ID)
		return nil, ErrInvalidCredentials
	}

	// Mark code as consumed atomically
	if err := s.repo.MarkCodeConsumed(ctx, activeCode.ID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to consume code: %w", err)
	}

	// Check if user already exists
	user, err := s.repo.FindUserByEmail(ctx, normalizedEmail)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, err
	}

	if user != nil {
		// Existing user: generate access and refresh tokens
		accessToken, err := auth.GenerateAccessToken(user.ID, user.Role, s.cfg.JWTSecret, s.cfg.JWTIssuer, s.cfg.AccessTokenTTL)
		if err != nil {
			return nil, fmt.Errorf("failed to generate access token: %w", err)
		}

		rawRefreshToken, err := auth.GenerateRefreshToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate refresh token: %w", err)
		}

		tokenHash := auth.HashRefreshToken(rawRefreshToken)
		var ua *string
		if cleanUA != "" {
			ua = &cleanUA
		}
		expiresAt := time.Now().Add(s.cfg.RefreshTokenTTL)

		if err := s.repo.CreateRefreshToken(ctx, user.ID, tokenHash, ua, expiresAt); err != nil {
			return nil, fmt.Errorf("failed to persist refresh token: %w", err)
		}

		return &VerifyResult{
			IsNewUser:    false,
			User:         user,
			AccessToken:  accessToken,
			RefreshToken: rawRefreshToken,
		}, nil
	}

	// New user: generate temporary signup token with email claim
	signupToken, err := auth.GenerateSignupToken(normalizedEmail, s.cfg.JWTSecret, s.cfg.JWTIssuer, s.cfg.SignupTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate signup token: %w", err)
	}

	return &VerifyResult{
		IsNewUser:   true,
		SignupToken: signupToken,
	}, nil
}

// Signup creates a new user from a valid signup token.
func (s *AuthService) Signup(ctx context.Context, signupToken, username, displayName, birthdateStr, userAgent string) (*AuthTokens, *models.User, error) {
	token, cleanUsername, cleanDisplayName, birthdate, cleanUA, err := ValidateSignup(signupToken, username, displayName, birthdateStr, userAgent)
	if err != nil {
		return nil, nil, err
	}

	claims, err := auth.ParseAndVerifySignupToken(token, s.cfg.JWTSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid signup token: %w", err)
	}

	email := claims.Email

	// Create user
	user, err := s.repo.CreateUser(ctx, cleanUsername, email, cleanDisplayName, birthdate, "user")
	if err != nil {
		return nil, nil, err
	}

	// Issue access JWT
	accessToken, err := auth.GenerateAccessToken(user.ID, user.Role, s.cfg.JWTSecret, s.cfg.JWTIssuer, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Issue refresh token
	rawRefreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	tokenHash := auth.HashRefreshToken(rawRefreshToken)
	var ua *string
	if cleanUA != "" {
		ua = &cleanUA
	}
	expiresAt := time.Now().Add(s.cfg.RefreshTokenTTL)

	if err := s.repo.CreateRefreshToken(ctx, user.ID, tokenHash, ua, expiresAt); err != nil {
		return nil, nil, fmt.Errorf("failed to persist refresh token: %w", err)
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
	}, user, nil
}

// RefreshToken rotates the presented refresh token and issues a new access token.
// If a revoked token is presented, all refresh tokens for that user are immediately revoked (reuse detection).
func (s *AuthService) RefreshToken(ctx context.Context, rawRefreshToken, userAgent string) (*AuthTokens, error) {
	cleanRefreshToken, cleanUA, err := ValidateRefreshToken(rawRefreshToken, userAgent)
	if err != nil {
		return nil, err
	}

	tokenHash := auth.HashRefreshToken(cleanRefreshToken)
	tokenRecord, err := s.repo.FindRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, auth.ErrInvalidToken
		}
		return nil, err
	}

	// Token reuse detection: if a revoked token is presented, revoke ALL active tokens for this user
	if tokenRecord.RevokedAt != nil {
		_ = s.repo.RevokeAllUserRefreshTokens(ctx, tokenRecord.UserID)
		return nil, ErrTokenRevoked
	}

	// Expiry check
	if time.Now().After(tokenRecord.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Look up user to get current role and verify existence
	user, err := s.repo.FindUserByID(ctx, tokenRecord.UserID)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}

	// Issue new access JWT
	newAccessToken, err := auth.GenerateAccessToken(user.ID, user.Role, s.cfg.JWTSecret, s.cfg.JWTIssuer, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Issue new refresh token
	newRawRefreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	newTokenHash := auth.HashRefreshToken(newRawRefreshToken)
	var ua *string
	if cleanUA != "" {
		ua = &cleanUA
	}
	newExpiresAt := time.Now().Add(s.cfg.RefreshTokenTTL)

	// Rotate atomically: revoke old token and insert new token
	if err := s.repo.RotateRefreshToken(ctx, tokenRecord.ID, user.ID, newTokenHash, ua, newExpiresAt); err != nil {
		if errors.Is(err, models.ErrConflict) {
			_ = s.repo.RevokeAllUserRefreshTokens(ctx, user.ID)
			return nil, ErrTokenRevoked
		}
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	return &AuthTokens{
		AccessToken:  newAccessToken,
		RefreshToken: newRawRefreshToken,
	}, nil
}

// Logout revokes the presented refresh token.
func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	cleanRefreshToken, err := ValidateLogout(rawRefreshToken)
	if err != nil {
		return err
	}
	tokenHash := auth.HashRefreshToken(cleanRefreshToken)
	return s.repo.RevokeRefreshTokenByHash(ctx, tokenHash)
}
