package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv                 string
	HTTPPort               string
	DatabaseURL            string
	MigrationDatabaseURL   string
	StorageBucket          string
	StoragePublicBaseURL   string
	SupabaseURL            string
	SupabaseServiceRoleKey string

	// Auth & Security
	JWTSecret         []byte
	JWTIssuer         string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	SignupTokenTTL    time.Duration
	OTPHMACKey        []byte
	OTPTTL            time.Duration
	OTPResendCooldown time.Duration
	OTPMaxAttempts    int

	// Mailer
	Mailer       string
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	migURL := os.Getenv("MIGRATION_DATABASE_URL")
	if migURL == "" {
		migURL = dbURL
	}

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}

	// JWT Secret: must be at least 32 bytes
	jwtSecretStr := os.Getenv("JWT_SECRET")
	if len(jwtSecretStr) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes (got %d)", len(jwtSecretStr))
	}
	jwtSecret := []byte(jwtSecretStr)

	// OTP HMAC Key: must be at least 32 bytes
	otpHmacStr := os.Getenv("OTP_HMAC_KEY")
	if len(otpHmacStr) < 32 {
		return nil, fmt.Errorf("OTP_HMAC_KEY must be at least 32 bytes (got %d)", len(otpHmacStr))
	}
	otpHmacKey := []byte(otpHmacStr)

	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = "social-api"
	}

	accessTokenTTL := parseDurationEnv("ACCESS_TOKEN_TTL", 15*time.Minute)
	refreshTokenTTL := parseDurationEnv("REFRESH_TOKEN_TTL", 720*time.Hour) // 30 days
	signupTokenTTL := parseDurationEnv("SIGNUP_TOKEN_TTL", 10*time.Minute)
	otpTTL := parseDurationEnv("OTP_TTL", 10*time.Minute)
	otpCooldown := parseDurationEnv("OTP_RESEND_COOLDOWN", 60*time.Second)

	otpMaxAttempts := parseIntEnv("OTP_MAX_ATTEMPTS", 5)

	mailerType := os.Getenv("MAILER")
	if mailerType == "" {
		mailerType = "mailpit"
	}

	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "587"
	}

	smtpFrom := os.Getenv("SMTP_FROM")
	if smtpFrom == "" {
		smtpFrom = "noreply@spill.local"
	}

	storageBucket := os.Getenv("STORAGE_BUCKET")
	if storageBucket == "" {
		storageBucket = "media"
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseServiceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	storagePublicBaseURL := os.Getenv("STORAGE_PUBLIC_BASE_URL")
	if storagePublicBaseURL == "" && supabaseURL != "" {
		storagePublicBaseURL = fmt.Sprintf("%s/storage/v1/object/public/%s", supabaseURL, storageBucket)
	}

	return &Config{
		AppEnv:                 appEnv,
		HTTPPort:               port,
		DatabaseURL:            dbURL,
		MigrationDatabaseURL:   migURL,
		StorageBucket:          storageBucket,
		StoragePublicBaseURL:   storagePublicBaseURL,
		SupabaseURL:            supabaseURL,
		SupabaseServiceRoleKey: supabaseServiceRoleKey,
		JWTSecret:              jwtSecret,
		JWTIssuer:              jwtIssuer,
		AccessTokenTTL:         accessTokenTTL,
		RefreshTokenTTL:        refreshTokenTTL,
		SignupTokenTTL:         signupTokenTTL,
		OTPHMACKey:             otpHmacKey,
		OTPTTL:                 otpTTL,
		OTPResendCooldown:      otpCooldown,
		OTPMaxAttempts:         otpMaxAttempts,
		Mailer:                 mailerType,
		SMTPHost:               os.Getenv("SMTP_HOST"),
		SMTPPort:               smtpPort,
		SMTPUsername:           os.Getenv("SMTP_USERNAME"),
		SMTPPassword:           os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:               smtpFrom,
	}, nil
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return d
}

func parseIntEnv(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return i
}
