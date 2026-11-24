package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/auth"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/mailer"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
)

type callCountingAuthRepo struct {
	*FakeAuthRepo
	checkCooldownCalls int
	createCodeCalls    int
	getActiveCodeCalls int
	incrementCalls     int
	createUserCalls    int
	findTokenCalls     int
	revokeTokenCalls   int
}

func newCallCountingAuthRepo() *callCountingAuthRepo {
	return &callCountingAuthRepo{
		FakeAuthRepo: newFakeAuthRepo(),
	}
}

func (c *callCountingAuthRepo) CheckLoginCodeCooldown(ctx context.Context, email string, cooldown time.Duration) (bool, error) {
	c.checkCooldownCalls++
	return c.FakeAuthRepo.CheckLoginCodeCooldown(ctx, email, cooldown)
}

func (c *callCountingAuthRepo) CreateLoginCode(ctx context.Context, email, codeHash string, expiresAt time.Time) error {
	c.createCodeCalls++
	return c.FakeAuthRepo.CreateLoginCode(ctx, email, codeHash, expiresAt)
}

func (c *callCountingAuthRepo) GetLatestActiveCode(ctx context.Context, email string) (*models.LoginCode, error) {
	c.getActiveCodeCalls++
	return c.FakeAuthRepo.GetLatestActiveCode(ctx, email)
}

func (c *callCountingAuthRepo) IncrementAttempts(ctx context.Context, id string) error {
	c.incrementCalls++
	return c.FakeAuthRepo.IncrementAttempts(ctx, id)
}

func (c *callCountingAuthRepo) CreateUser(ctx context.Context, username, email, displayName string, birthdate time.Time, role string) (*models.User, error) {
	c.createUserCalls++
	return c.FakeAuthRepo.CreateUser(ctx, username, email, displayName, birthdate, role)
}

func (c *callCountingAuthRepo) FindRefreshTokenByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	c.findTokenCalls++
	return c.FakeAuthRepo.FindRefreshTokenByHash(ctx, tokenHash)
}

func (c *callCountingAuthRepo) RevokeRefreshTokenByHash(ctx context.Context, tokenHash string) error {
	c.revokeTokenCalls++
	return c.FakeAuthRepo.RevokeRefreshTokenByHash(ctx, tokenHash)
}

func TestAuthService_ValidateRequestCode(t *testing.T) {
	tests := []struct {
		name      string
		rawEmail  string
		wantEmail string
		wantErr   bool
	}{
		{"Valid standard email", "user@example.com", "user@example.com", false},
		{"Uppercase email normalized to lowercase", "USER@EXAMPLE.COM", "user@example.com", false},
		{"Email with whitespace trimmed", "   test@domain.org  \t ", "test@domain.org", false},
		{"Email with tags and subdomains", "user+label@sub.domain.co", "user+label@sub.domain.co", false},
		{"Empty email", "", "", true},
		{"Whitespace only email", "   \t\n  ", "", true},
		{"Missing domain", "user@", "", true},
		{"Missing username", "@example.com", "", true},
		{"Missing @ symbol", "userexample.com", "", true},
		{"Double @ symbol", "user@@example.com", "", true},
		{"Domain without dot", "user@localhost", "", true},
		{"Email with null byte", "user\x00@example.com", "", true},
		{"Email with control character", "user\r@example.com", "", true},
		{"Email exceeding 255 chars", strings.Repeat("a", 250) + "@example.com", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEmail, err := services.ValidateRequestCode(tt.rawEmail)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateRequestCode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && gotEmail != tt.wantEmail {
				t.Errorf("ValidateRequestCode() gotEmail = %s, wantEmail %s", gotEmail, tt.wantEmail)
			}
		})
	}
}

func TestAuthService_ValidateVerifyCode(t *testing.T) {
	validEmail := "user@example.com"
	validCode := "123456"
	validUA := "Mozilla/5.0 (X11; Linux x86_64)"

	tests := []struct {
		name    string
		email   string
		code    string
		ua      string
		wantErr bool
		errType error
	}{
		{"Valid parameters", validEmail, validCode, validUA, false, nil},
		{"Valid with whitespace trimmed code", validEmail, "  123456  ", validUA, false, nil},
		{"Code with leading zeroes", validEmail, "000123", validUA, false, nil},
		{"Short code (5 digits)", validEmail, "12345", validUA, true, services.ErrInvalidCredentials},
		{"Long code (7 digits)", validEmail, "1234567", validUA, true, services.ErrInvalidCredentials},
		{"Code with letters", validEmail, "12a456", validUA, true, services.ErrInvalidCredentials},
		{"Empty code", validEmail, "", validUA, true, services.ErrInvalidCredentials},
		{"Whitespace only code", validEmail, "      ", validUA, true, services.ErrInvalidCredentials},
		{"Malformed email", "not-an-email", validCode, validUA, true, services.ErrInvalidCredentials},
		{"UA with null byte", validEmail, validCode, "agent\x00malicious", true, services.ErrInvalidCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEmail, cleanCode, _, err := services.ValidateVerifyCode(tt.email, tt.code, tt.ua)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateVerifyCode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errType != nil && !errors.Is(err, tt.errType) {
				t.Errorf("expected error %v, got %v", tt.errType, err)
			}
			if !tt.wantErr {
				if cleanEmail != strings.ToLower(strings.TrimSpace(tt.email)) {
					t.Errorf("email not normalized: got %s", cleanEmail)
				}
				if cleanCode != strings.TrimSpace(tt.code) {
					t.Errorf("code not trimmed: got %s", cleanCode)
				}
			}
		})
	}
}

func TestAuthService_ValidateSignup(t *testing.T) {
	validToken := "valid-token-string"
	validUsername := "valid_user"
	validDisplayName := "Valid Name"
	validBirthdate := "2000-01-01"
	validUA := "TestAgent/1.0"

	tests := []struct {
		name        string
		token       string
		username    string
		displayName string
		birthdate   string
		ua          string
		wantErr     bool
		errType     error
	}{
		{"Valid all inputs", validToken, validUsername, validDisplayName, validBirthdate, validUA, false, nil},
		{"Valid with Arabic display name", validToken, "arabic_user", "أحمد محمد", validBirthdate, validUA, false, nil},
		{"Valid with Emoji in display name", validToken, "cool_user", "Cool User 🚀", validBirthdate, validUA, false, nil},
		{"Empty signup token", "", validUsername, validDisplayName, validBirthdate, validUA, true, services.ErrInvalidInput},
		{"Whitespace signup token", "   ", validUsername, validDisplayName, validBirthdate, validUA, true, services.ErrInvalidInput},
		{"Signup token with null byte", "tok\x00en", validUsername, validDisplayName, validBirthdate, validUA, true, services.ErrInvalidInput},
		{"Username too short (2 chars)", validToken, "ab", validDisplayName, validBirthdate, validUA, true, services.ErrInvalidUsername},
		{"Username too long (21 chars)", validToken, "abcdefghijklmnopqrstu", validDisplayName, validBirthdate, validUA, true, services.ErrInvalidUsername},
		{"Username with uppercase letters", validToken, "ValidUser", validDisplayName, validBirthdate, validUA, true, services.ErrInvalidUsername},
		{"Username with hyphen", validToken, "valid-user", validDisplayName, validBirthdate, validUA, true, services.ErrInvalidUsername},
		{"Username with leading underscore", validToken, "_validuser", validDisplayName, validBirthdate, validUA, true, services.ErrInvalidUsername},
		{"Username with trailing underscore", validToken, "validuser_", validDisplayName, validBirthdate, validUA, true, services.ErrInvalidUsername},
		{"Reserved username admin", validToken, "admin", validDisplayName, validBirthdate, validUA, true, services.ErrInvalidUsername},
		{"Reserved username system", validToken, "system", validDisplayName, validBirthdate, validUA, true, services.ErrInvalidUsername},
		{"Display name empty", validToken, validUsername, "", validBirthdate, validUA, true, services.ErrValidation},
		{"Display name whitespace only", validToken, validUsername, "    ", validBirthdate, validUA, true, services.ErrValidation},
		{"Display name too long (51 runes)", validToken, validUsername, strings.Repeat("A", 51), validBirthdate, validUA, true, services.ErrValidation},
		{"Display name with newline", validToken, validUsername, "User\nName", validBirthdate, validUA, true, services.ErrValidation},
		{"Display name with null byte", validToken, validUsername, "User\x00Name", validBirthdate, validUA, true, services.ErrValidation},
		{"Invalid birthdate format", validToken, validUsername, validDisplayName, "01-01-2000", validUA, true, services.ErrInvalidBirthdate},
		{"Birthdate before 1900", validToken, validUsername, validDisplayName, "1899-12-31", validUA, true, services.ErrInvalidBirthdate},
		{"Underage user (<13 years)", validToken, validUsername, validDisplayName, time.Now().Format("2006-01-02"), validUA, true, services.ErrUnderage},
		{"UA with null byte", validToken, validUsername, validDisplayName, validBirthdate, "ua\x00null", true, services.ErrInvalidInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, _, err := services.ValidateSignup(tt.token, tt.username, tt.displayName, tt.birthdate, tt.ua)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateSignup() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errType != nil && !errors.Is(err, tt.errType) {
				t.Errorf("expected error %v, got %v", tt.errType, err)
			}
		})
	}
}

func TestAuthService_ValidateRefreshTokenAndLogout(t *testing.T) {
	t.Run("ValidateRefreshToken", func(t *testing.T) {
		tests := []struct {
			name    string
			token   string
			wantErr bool
		}{
			{"Valid token", "valid-refresh-token-12345", false},
			{"Empty token", "", true},
			{"Whitespace token", "   \t ", true},
			{"Token with spaces", "token with space", true},
			{"Token with null byte", "token\x00bad", true},
			{"Token with control char", "token\nbad", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, err := services.ValidateRefreshToken(tt.token, "ua")
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateRefreshToken() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("ValidateLogout", func(t *testing.T) {
		tests := []struct {
			name    string
			token   string
			wantErr bool
		}{
			{"Valid token", "valid-refresh-token-12345", false},
			{"Empty token", "", true},
			{"Whitespace token", "   \t ", true},
			{"Token with spaces", "token with space", true},
			{"Token with null byte", "token\x00bad", true},
			{"Token with control char", "token\nbad", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := services.ValidateLogout(tt.token)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateLogout() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestAuthService_FailFastZeroRepoCalls(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()
	repo := newCallCountingAuthRepo()
	mockMail := mailer.NewMockMailer()
	svc := services.NewAuthService(repo, mockMail, cfg)

	// 1. RequestCode with invalid email format returns nil (anti-enumeration) but makes 0 repo calls
	err := svc.RequestCode(ctx, "invalid-email-format")
	if err != nil {
		t.Fatalf("expected nil error for invalid email to prevent enumeration, got %v", err)
	}
	if repo.checkCooldownCalls != 0 || repo.createCodeCalls != 0 {
		t.Errorf("expected 0 repo calls for invalid email, got cooldown=%d create=%d", repo.checkCooldownCalls, repo.createCodeCalls)
	}

	// 2. VerifyCode with invalid code format -> 0 repo calls
	_, err = svc.VerifyCode(ctx, "user@example.com", "notnum", "ua")
	if !errors.Is(err, services.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if repo.getActiveCodeCalls != 0 || repo.incrementCalls != 0 {
		t.Errorf("expected 0 repo calls on invalid verify code, got active=%d inc=%d", repo.getActiveCodeCalls, repo.incrementCalls)
	}

	// 3. Signup with invalid username -> 0 repo calls
	signupToken, _ := auth.GenerateSignupToken("newuser@example.com", cfg.JWTSecret, cfg.JWTIssuer, 10*time.Minute)
	_, _, err = svc.Signup(ctx, signupToken, "bad-name!", "Valid Display", "2000-01-01", "ua")
	if !errors.Is(err, services.ErrInvalidUsername) {
		t.Fatalf("expected ErrInvalidUsername, got %v", err)
	}
	if repo.createUserCalls != 0 {
		t.Errorf("expected 0 repo calls on invalid signup username, got %d", repo.createUserCalls)
	}

	// 4. RefreshToken with invalid token -> 0 repo calls
	_, err = svc.RefreshToken(ctx, "token with space", "ua")
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
	if repo.findTokenCalls != 0 {
		t.Errorf("expected 0 repo calls on invalid refresh token, got %d", repo.findTokenCalls)
	}

	// 5. Logout with invalid token -> 0 repo calls
	err = svc.Logout(ctx, "token with space")
	if !errors.Is(err, services.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if repo.revokeTokenCalls != 0 {
		t.Errorf("expected 0 repo calls on invalid logout token, got %d", repo.revokeTokenCalls)
	}
}
