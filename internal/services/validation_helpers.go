package services

import (
	"encoding/base64"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
	"github.com/google/uuid"
)

var (
	emailFormatRegex       = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	usernameFormatRegex    = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)
	disposableEmailDomains = map[string]struct{}{
		"mailinator.com":    {},
		"10minutemail.com":  {},
		"guerrillamail.com": {},
		"tempmail.com":      {},
		"throwawaymail.com": {},
		"yopmail.com":       {},
		"sharklasers.com":   {},
		"dispostable.com":   {},
	}
	reservedUsernames = map[string]struct{}{
		"admin":     {},
		"root":      {},
		"system":    {},
		"api":       {},
		"support":   {},
		"help":      {},
		"official":  {},
		"moderator": {},
	}
)

// ValidateUUID ensures an identifier is a well-formed, non-nil, trimmed UUID string.
func ValidateUUID(id, fieldName string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("%w: %s cannot be empty", ErrInvalidInput, fieldName)
	}
	if len(id) != len(trimmed) {
		return fmt.Errorf("%w: %s must not contain leading or trailing whitespace", ErrInvalidInput, fieldName)
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil || parsed == uuid.Nil {
		return fmt.Errorf("%w: %s must be a valid non-nil UUID", ErrInvalidInput, fieldName)
	}
	return nil
}

// ValidateOptionalUUID ensures an optional identifier is either nil/empty or a valid UUID.
func ValidateOptionalUUID(id *string, fieldName string) error {
	if id == nil || *id == "" {
		return nil
	}
	return ValidateUUID(*id, fieldName)
}

// ContainsNullByte returns true if the string contains a null byte (\x00).
func ContainsNullByte(s string) bool {
	return strings.ContainsRune(s, 0)
}

// ContainsControlChars checks for non-printable control characters.
// If allowNewlines is true, tab (\t), newline (\n), and carriage return (\r) are permitted.
func ContainsControlChars(s string, allowNewlines bool) bool {
	for _, r := range s {
		if r == 0 {
			return true
		}
		if allowNewlines && (r == '\t' || r == '\n' || r == '\r') {
			continue
		}
		if r < 32 || r == 127 {
			return true
		}
	}
	return false
}

// ValidateCleanString checks for valid UTF-8, absence of null bytes, and absence of prohibited control characters.
func ValidateCleanString(s, fieldName string, allowNewlines bool) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%w: %s contains invalid UTF-8 sequence", ErrInvalidInput, fieldName)
	}
	if ContainsNullByte(s) {
		return fmt.Errorf("%w: %s must not contain null bytes", ErrInvalidInput, fieldName)
	}
	if ContainsControlChars(s, allowNewlines) {
		return fmt.Errorf("%w: %s contains disallowed control characters", ErrInvalidInput, fieldName)
	}
	return nil
}

// ValidateRuneLength validates the Unicode rune length against minimum and maximum bounds.
func ValidateRuneLength(s string, min, max int, fieldName string) error {
	count := utf8.RuneCountInString(s)
	if min > 0 && count < min {
		return fmt.Errorf("%w: %s must be at least %d characters", ErrInvalidInput, fieldName, min)
	}
	if max > 0 && count > max {
		return fmt.Errorf("%w: %s must not exceed %d characters", ErrInvalidInput, fieldName, max)
	}
	return nil
}

// ValidateEmail checks that an email is non-empty, trimmed, valid RFC 5322 format,
// within schema length bounds, without control characters, and not using a disposable domain.
func ValidateEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: email cannot be empty", ErrInvalidInput)
	}
	if ContainsNullByte(raw) || ContainsControlChars(trimmed, false) {
		return "", fmt.Errorf("%w: email contains invalid control characters", ErrInvalidInput)
	}
	if utf8.RuneCountInString(trimmed) > 254 {
		return "", fmt.Errorf("%w: email exceeds maximum length of 254 characters", ErrInvalidInput)
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil || addr.Address != trimmed {
		return "", fmt.Errorf("%w: invalid email address format", ErrInvalidInput)
	}
	normalized := strings.ToLower(addr.Address)
	if !emailFormatRegex.MatchString(normalized) {
		return "", fmt.Errorf("%w: email does not match required pattern", ErrInvalidInput)
	}
	parts := strings.Split(normalized, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("%w: malformed email address", ErrInvalidInput)
	}
	domain := parts[1]
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return "", fmt.Errorf("%w: malformed domain in email", ErrInvalidInput)
	}
	if _, disposable := disposableEmailDomains[domain]; disposable {
		return "", fmt.Errorf("%w: disposable email domains are not accepted", ErrInvalidInput)
	}
	return normalized, nil
}

// ValidateUsernameFormat checks username format for profile lookup (3-20 characters, alphanumeric and underscore).
func ValidateUsernameFormat(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: username cannot be empty", ErrInvalidInput)
	}
	if ContainsNullByte(raw) || ContainsControlChars(trimmed, false) {
		return "", fmt.Errorf("%w: username contains invalid control characters", ErrInvalidInput)
	}
	if !usernameFormatRegex.MatchString(trimmed) {
		return "", ErrInvalidUsername
	}
	return trimmed, nil
}

// ValidateSignupUsername validates a username during user registration, additionally checking
// no leading/trailing underscores and rejecting reserved system words.
func ValidateSignupUsername(raw string) (string, error) {
	clean, err := ValidateUsernameFormat(raw)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(clean, "_") || strings.HasSuffix(clean, "_") {
		return "", fmt.Errorf("%w: username cannot start or end with an underscore", ErrInvalidUsername)
	}
	if _, reserved := reservedUsernames[clean]; reserved {
		return "", fmt.Errorf("%w: username is reserved", ErrInvalidUsername)
	}
	return clean, nil
}

// ValidateDisplayName checks length (1-50 runes) and absence of control characters.
func ValidateDisplayName(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrInvalidDisplayName
	}
	if err := ValidateCleanString(raw, "displayName", false); err != nil {
		return "", ErrInvalidDisplayName
	}
	count := utf8.RuneCountInString(trimmed)
	if count < 1 || count > 50 {
		return "", ErrInvalidDisplayName
	}
	return trimmed, nil
}

// ValidatePagination enforces strict safe limits (1 <= limit <= maxLimit) and decodes/validates keyset cursors.
func ValidatePagination(limit int, cursorStr string, maxLimit int) (int, *time.Time, string, error) {
	if maxLimit <= 0 {
		maxLimit = 100
	}
	if limit < 1 || limit > maxLimit {
		return 0, nil, "", fmt.Errorf("%w: pagination limit must be between 1 and %d (got %d)", ErrInvalidInput, maxLimit, limit)
	}
	var cursorTime *time.Time
	var cursorID string
	if cursorStr != "" {
		t, id, err := views.DecodeCursor(cursorStr)
		if err != nil {
			return 0, nil, "", fmt.Errorf("%w: invalid pagination cursor", ErrInvalidInput)
		}
		if err := ValidateUUID(id, "cursorID"); err != nil {
			return 0, nil, "", fmt.Errorf("%w: cursor contains malformed record ID", ErrInvalidInput)
		}
		cursorTime = &t
		cursorID = id
	}
	return limit, cursorTime, cursorID, nil
}

// ParseSearchCursor parses an offset cursor (plain integer or base64 integer).
func ParseSearchCursor(cursorStr string) (int, error) {
	if cursorStr == "" {
		return 0, nil
	}
	if val, err := strconv.Atoi(cursorStr); err == nil {
		if val < 0 {
			return 0, fmt.Errorf("%w: search cursor offset cannot be negative", ErrInvalidInput)
		}
		return val, nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(cursorStr); err == nil {
		if val, err := strconv.Atoi(string(b)); err == nil {
			if val < 0 {
				return 0, fmt.Errorf("%w: search cursor offset cannot be negative", ErrInvalidInput)
			}
			return val, nil
		}
	}
	if b, err := base64.StdEncoding.DecodeString(cursorStr); err == nil {
		if val, err := strconv.Atoi(string(b)); err == nil {
			if val < 0 {
				return 0, fmt.Errorf("%w: search cursor offset cannot be negative", ErrInvalidInput)
			}
			return val, nil
		}
	}
	return 0, fmt.Errorf("%w: invalid search cursor format", ErrInvalidInput)
}

// ValidateRole ensures role is either 'user' or 'admin'.
func ValidateRole(role string) (string, error) {
	clean := strings.TrimSpace(role)
	if clean != "user" && clean != "admin" {
		return "", fmt.Errorf("%w: invalid user role '%s' (allowed: user, admin)", ErrInvalidInput, role)
	}
	return clean, nil
}

// ValidateFilename checks for path traversal sequences and null bytes.
func ValidateFilename(filename string) (string, error) {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" {
		return "", fmt.Errorf("%w: filename cannot be empty", ErrInvalidInput)
	}
	if ContainsNullByte(trimmed) {
		return "", fmt.Errorf("%w: filename must not contain null bytes", ErrInvalidInput)
	}
	if strings.Contains(trimmed, "..") || strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return "", fmt.Errorf("%w: filename contains prohibited path traversal sequences", ErrInvalidInput)
	}
	return trimmed, nil
}

// ValidateURL validates that a URL is non-empty, well-formed, and restricted to http/https.
func ValidateURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return fmt.Errorf("%w: URL cannot be empty", ErrInvalidInput)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("%w: invalid URL: %v", ErrInvalidInput, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: URL scheme must be http or https (got %s)", ErrInvalidInput, parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%w: URL host cannot be empty", ErrInvalidInput)
	}
	return nil
}
