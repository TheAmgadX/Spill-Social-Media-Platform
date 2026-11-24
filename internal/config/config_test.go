package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/config"
)

func TestConfig_Load_MissingDatabaseURL(t *testing.T) {
	// Temporarily clear environment
	origDB := os.Getenv("DATABASE_URL")
	defer os.Setenv("DATABASE_URL", origDB)
	os.Unsetenv("DATABASE_URL")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing, got nil")
	}
}

func TestConfig_Load_ShortJWTSecret(t *testing.T) {
	origDB := os.Getenv("DATABASE_URL")
	origJWT := os.Getenv("JWT_SECRET")
	defer func() {
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("JWT_SECRET", origJWT)
	}()

	os.Setenv("DATABASE_URL", "postgres://localhost:5432/test")
	os.Setenv("JWT_SECRET", "too-short") // < 32 bytes

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is < 32 bytes, got nil")
	}
}

func TestConfig_Load_ShortOTPHMACKey(t *testing.T) {
	origDB := os.Getenv("DATABASE_URL")
	origJWT := os.Getenv("JWT_SECRET")
	origHMAC := os.Getenv("OTP_HMAC_KEY")
	defer func() {
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("JWT_SECRET", origJWT)
		os.Setenv("OTP_HMAC_KEY", origHMAC)
	}()

	os.Setenv("DATABASE_URL", "postgres://localhost:5432/test")
	os.Setenv("JWT_SECRET", "12345678901234567890123456789012") // 32 bytes
	os.Setenv("OTP_HMAC_KEY", "short")                          // < 32 bytes

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when OTP_HMAC_KEY is < 32 bytes, got nil")
	}
}

func TestConfig_Load_SuccessWithDefaults(t *testing.T) {
	origDB := os.Getenv("DATABASE_URL")
	origMigDB := os.Getenv("MIGRATION_DATABASE_URL")
	origJWT := os.Getenv("JWT_SECRET")
	origHMAC := os.Getenv("OTP_HMAC_KEY")
	origPort := os.Getenv("HTTP_PORT")
	origEnv := os.Getenv("APP_ENV")
	origIssuer := os.Getenv("JWT_ISSUER")
	origMailer := os.Getenv("MAILER")
	origSMTPPort := os.Getenv("SMTP_PORT")
	defer func() {
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("MIGRATION_DATABASE_URL", origMigDB)
		os.Setenv("JWT_SECRET", origJWT)
		os.Setenv("OTP_HMAC_KEY", origHMAC)
		os.Setenv("HTTP_PORT", origPort)
		os.Setenv("APP_ENV", origEnv)
		os.Setenv("JWT_ISSUER", origIssuer)
		os.Setenv("MAILER", origMailer)
		os.Setenv("SMTP_PORT", origSMTPPort)
	}()

	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/spill_db")
	os.Unsetenv("MIGRATION_DATABASE_URL")
	os.Setenv("JWT_SECRET", "abcdefghijklmnopqrstuvwxyz123456")   // 32 bytes
	os.Setenv("OTP_HMAC_KEY", "1234567890abcdefghijklmnopqrstuv") // 32 bytes
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("APP_ENV")
	os.Unsetenv("JWT_ISSUER")
	os.Unsetenv("MAILER")
	os.Unsetenv("SMTP_PORT")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error loading valid config: %v", err)
	}

	if cfg.HTTPPort != "8080" {
		t.Errorf("expected default HTTPPort '8080', got '%s'", cfg.HTTPPort)
	}
	if cfg.AppEnv != "development" {
		t.Errorf("expected default AppEnv 'development', got '%s'", cfg.AppEnv)
	}
	if cfg.MigrationDatabaseURL != cfg.DatabaseURL {
		t.Errorf("expected MigrationDatabaseURL to default to DatabaseURL, got '%s'", cfg.MigrationDatabaseURL)
	}
	if cfg.JWTIssuer != "social-api" {
		t.Errorf("expected default JWTIssuer 'social-api', got '%s'", cfg.JWTIssuer)
	}
	if cfg.Mailer != "mailpit" {
		t.Errorf("expected default Mailer 'mailpit', got '%s'", cfg.Mailer)
	}
	if cfg.SMTPPort != "587" {
		t.Errorf("expected default SMTPPort '587', got '%s'", cfg.SMTPPort)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("expected default AccessTokenTTL 15m, got %v", cfg.AccessTokenTTL)
	}
	if cfg.OTPMaxAttempts != 5 {
		t.Errorf("expected default OTPMaxAttempts 5, got %d", cfg.OTPMaxAttempts)
	}
}

func TestConfig_Load_CustomValuesAndInvalidParsing(t *testing.T) {
	origDB := os.Getenv("DATABASE_URL")
	origJWT := os.Getenv("JWT_SECRET")
	origHMAC := os.Getenv("OTP_HMAC_KEY")
	origAccessTTL := os.Getenv("ACCESS_TOKEN_TTL")
	origOTPAttempts := os.Getenv("OTP_MAX_ATTEMPTS")
	defer func() {
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("JWT_SECRET", origJWT)
		os.Setenv("OTP_HMAC_KEY", origHMAC)
		os.Setenv("ACCESS_TOKEN_TTL", origAccessTTL)
		os.Setenv("OTP_MAX_ATTEMPTS", origOTPAttempts)
	}()

	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/spill_db")
	os.Setenv("JWT_SECRET", "abcdefghijklmnopqrstuvwxyz123456")
	os.Setenv("OTP_HMAC_KEY", "1234567890abcdefghijklmnopqrstuv")

	// Set invalid duration and invalid int to test graceful fallback
	os.Setenv("ACCESS_TOKEN_TTL", "invalid-duration-value")
	os.Setenv("OTP_MAX_ATTEMPTS", "invalid-int-value")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	// Should fall back to default duration
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("expected fallback AccessTokenTTL 15m, got %v", cfg.AccessTokenTTL)
	}
	// Should fall back to default int
	if cfg.OTPMaxAttempts != 5 {
		t.Errorf("expected fallback OTPMaxAttempts 5, got %d", cfg.OTPMaxAttempts)
	}

	// Now set valid custom values
	os.Setenv("ACCESS_TOKEN_TTL", "30m")
	os.Setenv("OTP_MAX_ATTEMPTS", "3")

	cfgCustom, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error loading custom config: %v", err)
	}

	if cfgCustom.AccessTokenTTL != 30*time.Minute {
		t.Errorf("expected custom AccessTokenTTL 30m, got %v", cfgCustom.AccessTokenTTL)
	}
	if cfgCustom.OTPMaxAttempts != 3 {
		t.Errorf("expected custom OTPMaxAttempts 3, got %d", cfgCustom.OTPMaxAttempts)
	}
}
