package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/auth"
)

// ValidateRequestCode validates the email for code request.
func ValidateRequestCode(rawEmail string) (string, error) {
	return ValidateEmail(rawEmail)
}

// ValidateVerifyCode validates the email, 6-digit verification code, and optional user-agent.
func ValidateVerifyCode(rawEmail, rawCode, userAgent string) (string, string, string, error) {
	cleanEmail, err := ValidateEmail(rawEmail)
	if err != nil {
		return "", "", "", ErrInvalidCredentials
	}

	code := strings.TrimSpace(rawCode)
	if len(code) != 6 {
		return "", "", "", ErrInvalidCredentials
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return "", "", "", ErrInvalidCredentials
		}
	}

	cleanUA := strings.TrimSpace(userAgent)
	if ContainsNullByte(cleanUA) {
		return "", "", "", ErrInvalidCredentials
	}
	if len(cleanUA) > 512 {
		cleanUA = cleanUA[:512]
	}

	return cleanEmail, code, cleanUA, nil
}

// ValidateSignup validates all input fields for completing new user registration.
func ValidateSignup(signupToken, username, displayName, birthdateStr, userAgent string) (string, string, string, time.Time, string, error) {
	token := strings.TrimSpace(signupToken)
	if token == "" {
		return "", "", "", time.Time{}, "", fmt.Errorf("%w: signup token cannot be empty", ErrInvalidInput)
	}
	if ContainsNullByte(token) || ContainsControlChars(token, false) {
		return "", "", "", time.Time{}, "", fmt.Errorf("%w: signup token contains invalid characters", ErrInvalidInput)
	}

	cleanUsername, err := ValidateSignupUsername(username)
	if err != nil {
		return "", "", "", time.Time{}, "", err
	}

	cleanDisplayName, err := ValidateDisplayName(displayName)
	if err != nil {
		return "", "", "", time.Time{}, "", err
	}

	cleanBD := strings.TrimSpace(birthdateStr)
	birthdate, err := time.Parse("2006-01-02", cleanBD)
	if err != nil {
		return "", "", "", time.Time{}, "", ErrInvalidBirthdate
	}
	if !birthdate.After(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return "", "", "", time.Time{}, "", fmt.Errorf("%w: birthdate must be after 1900-01-01", ErrInvalidBirthdate)
	}

	age := auth.CalculateAge(birthdate, time.Now())
	if age < 13 {
		return "", "", "", time.Time{}, "", ErrUnderage
	}

	cleanUA := strings.TrimSpace(userAgent)
	if ContainsNullByte(cleanUA) {
		return "", "", "", time.Time{}, "", fmt.Errorf("%w: user-agent contains null bytes", ErrInvalidInput)
	}
	if len(cleanUA) > 512 {
		cleanUA = cleanUA[:512]
	}

	return token, cleanUsername, cleanDisplayName, birthdate, cleanUA, nil
}

// ValidateRefreshToken validates a raw refresh token before hash lookup and rotation.
func ValidateRefreshToken(rawRefreshToken, userAgent string) (string, string, error) {
	token := strings.TrimSpace(rawRefreshToken)
	if token == "" {
		return "", "", auth.ErrInvalidToken
	}
	if ContainsNullByte(token) || ContainsControlChars(token, false) || strings.Contains(token, " ") {
		return "", "", auth.ErrInvalidToken
	}
	cleanUA := strings.TrimSpace(userAgent)
	if len(cleanUA) > 512 {
		cleanUA = cleanUA[:512]
	}
	return token, cleanUA, nil
}

// ValidateLogout validates a raw refresh token before revocation.
func ValidateLogout(rawRefreshToken string) (string, error) {
	token := strings.TrimSpace(rawRefreshToken)
	if token == "" {
		return "", fmt.Errorf("%w: refresh token is required for logout", ErrInvalidInput)
	}
	if ContainsNullByte(token) || ContainsControlChars(token, false) || strings.Contains(token, " ") {
		return "", fmt.Errorf("%w: refresh token contains invalid characters", ErrInvalidInput)
	}
	return token, nil
}
