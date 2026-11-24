package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken        = errors.New("invalid or expired token")
	ErrUnexpectedAlg       = errors.New("unexpected signing algorithm")
	ErrInvalidTokenPurpose = errors.New("invalid token purpose")
)

// AccessClaims represents claims inside an access JWT.
type AccessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// SignupClaims represents claims inside a temporary signup token.
type SignupClaims struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
	jwt.RegisteredClaims
}

// GenerateAccessToken signs an access JWT with sub, role, iat, exp, and iss.
func GenerateAccessToken(userID string, role string, secret []byte, issuer string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ParseAndVerifyAccessToken parses an access token, pinning the signing algorithm to HS256.
func ParseAndVerifyAccessToken(tokenString string, secret []byte) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("%w: got %s, expected %s", ErrUnexpectedAlg, token.Method.Alg(), jwt.SigningMethodHS256.Alg())
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GenerateSignupToken creates a short-lived token (10m) encoding the verified email and purpose="signup".
func GenerateSignupToken(email string, secret []byte, issuer string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := SignupClaims{
		Email:   email,
		Purpose: "signup",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   email,
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ParseAndVerifySignupToken parses a signup token, pinning HS256 and enforcing purpose="signup".
func ParseAndVerifySignupToken(tokenString string, secret []byte) (*SignupClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &SignupClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("%w: got %s, expected %s", ErrUnexpectedAlg, token.Method.Alg(), jwt.SigningMethodHS256.Alg())
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*SignupClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.Purpose != "signup" {
		return nil, ErrInvalidTokenPurpose
	}

	return claims, nil
}

// GenerateRefreshToken creates a 30-day opaque random string (32 random bytes).
func GenerateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate random refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashRefreshToken calculates the SHA-256 hex digest of the raw refresh token.
// Only this hash is persisted to the database.
func HashRefreshToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// CalculateAge computes the integer age of a user given their birthdate and today's date.
func CalculateAge(birthdate, today time.Time) int {
	age := today.Year() - birthdate.Year()
	// If the current date is before this year's birthday, subtract 1
	if today.Month() < birthdate.Month() || (today.Month() == birthdate.Month() && today.Day() < birthdate.Day()) {
		age--
	}
	return age
}
