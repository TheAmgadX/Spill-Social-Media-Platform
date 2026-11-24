package services

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ValidateSearch validates search query string, entity type, current user ID, limit, and cursor offset.
func ValidateSearch(q, searchType string, currentUserID *string, limit int, cursorStr string) (string, string, *string, int, int, error) {
	cleanQ := strings.TrimSpace(q)
	if cleanQ == "" {
		return "", "", nil, 0, 0, fmt.Errorf("%w: query parameter 'q' must not be empty", ErrValidation)
	}
	if ContainsNullByte(q) || ContainsControlChars(q, false) {
		return "", "", nil, 0, 0, fmt.Errorf("%w: query contains invalid control characters", ErrValidation)
	}
	if !utf8.ValidString(cleanQ) {
		return "", "", nil, 0, 0, fmt.Errorf("%w: query contains invalid UTF-8 byte sequences", ErrValidation)
	}
	if utf8.RuneCountInString(cleanQ) > 200 {
		return "", "", nil, 0, 0, fmt.Errorf("%w: query parameter 'q' must not exceed 200 characters", ErrValidation)
	}

	cleanType := strings.ToLower(strings.TrimSpace(searchType))
	switch cleanType {
	case "users", "posts", "comments":
	default:
		return "", "", nil, 0, 0, fmt.Errorf("%w: invalid search type '%s' (allowed: users, posts, comments)", ErrValidation, searchType)
	}

	if err := ValidateOptionalUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", nil, 0, 0, err
	}

	if limit < 1 || limit > 100 {
		return "", "", nil, 0, 0, fmt.Errorf("%w: limit must be between 1 and 100 (got %d)", ErrValidation, limit)
	}

	offset, err := ParseSearchCursor(cursorStr)
	if err != nil {
		return "", "", nil, 0, 0, fmt.Errorf("%w: invalid search cursor", ErrValidation)
	}

	return cleanQ, cleanType, currentUserID, limit, offset, nil
}
