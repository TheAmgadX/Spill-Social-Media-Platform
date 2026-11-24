package services

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ValidateCreatePost validates post author, body content, and media attachments.
func ValidateCreatePost(authorID, body string, assetIDs []string) (string, string, []string, error) {
	if err := ValidateUUID(authorID, "authorID"); err != nil {
		return "", "", nil, err
	}

	trimmed := strings.TrimSpace(body)
	if trimmed == "" && len(assetIDs) == 0 {
		return "", "", nil, fmt.Errorf("%w: post must have body text or at least one image", ErrValidation)
	}

	if err := ValidateCleanString(body, "body", true); err != nil {
		return "", "", nil, err
	}

	if utf8.RuneCountInString(body) > 2000 {
		return "", "", nil, fmt.Errorf("%w: post body must not exceed 2000 characters", ErrValidation)
	}

	if len(assetIDs) > 4 {
		return "", "", nil, fmt.Errorf("%w: posts cannot have more than 4 images", ErrValidation)
	}

	seen := make(map[string]struct{}, len(assetIDs))
	for _, aid := range assetIDs {
		if err := ValidateUUID(aid, "assetID"); err != nil {
			return "", "", nil, err
		}
		if _, exists := seen[aid]; exists {
			return "", "", nil, fmt.Errorf("%w: duplicate asset ID in post attachments: %s", ErrValidation, aid)
		}
		seen[aid] = struct{}{}
	}

	return authorID, body, assetIDs, nil
}

// ValidateGetPost validates post ID and optional current user ID.
func ValidateGetPost(id string, currentUserID *string) (string, *string, error) {
	if err := ValidateUUID(id, "postID"); err != nil {
		return "", nil, err
	}
	if err := ValidateOptionalUUID(currentUserID, "currentUserID"); err != nil {
		return "", nil, err
	}
	return id, currentUserID, nil
}

// ValidateUpdatePost validates post ID, ownership actor, role, and non-empty updated body.
func ValidateUpdatePost(id, currentUserID, userRole, body string) (string, string, string, string, error) {
	if err := ValidateUUID(id, "postID"); err != nil {
		return "", "", "", "", err
	}
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", "", "", err
	}
	cleanRole, err := ValidateRole(userRole)
	if err != nil {
		return "", "", "", "", err
	}

	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", "", "", "", fmt.Errorf("%w: post body cannot be empty", ErrValidation)
	}

	if err := ValidateCleanString(body, "body", true); err != nil {
		return "", "", "", "", err
	}

	l := utf8.RuneCountInString(body)
	if l < 1 || l > 2000 {
		return "", "", "", "", fmt.Errorf("%w: post body must be between 1 and 2000 characters", ErrValidation)
	}

	return id, currentUserID, cleanRole, body, nil
}

// ValidateDeletePost validates post ID, actor ID, and role.
func ValidateDeletePost(id, currentUserID, userRole string) (string, string, string, error) {
	if err := ValidateUUID(id, "postID"); err != nil {
		return "", "", "", err
	}
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", "", err
	}
	cleanRole, err := ValidateRole(userRole)
	if err != nil {
		return "", "", "", err
	}
	return id, currentUserID, cleanRole, nil
}

// ValidateGetFeed validates user ID and pagination parameters for the timeline.
func ValidateGetFeed(currentUserID string, limit int, cursorStr string) (string, int, *time.Time, string, error) {
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", 0, nil, "", err
	}
	limit, cursorTime, cursorID, err := ValidatePagination(limit, cursorStr, 100)
	if err != nil {
		return "", 0, nil, "", err
	}
	return currentUserID, limit, cursorTime, cursorID, nil
}

// ValidateGetUserPosts validates target user ID, optional requesting user ID, and pagination.
func ValidateGetUserPosts(targetUserID string, currentUserID *string, limit int, cursorStr string) (string, *string, int, *time.Time, string, error) {
	if err := ValidateUUID(targetUserID, "targetUserID"); err != nil {
		return "", nil, 0, nil, "", err
	}
	if err := ValidateOptionalUUID(currentUserID, "currentUserID"); err != nil {
		return "", nil, 0, nil, "", err
	}
	limit, cursorTime, cursorID, err := ValidatePagination(limit, cursorStr, 100)
	if err != nil {
		return "", nil, 0, nil, "", err
	}
	return targetUserID, currentUserID, limit, cursorTime, cursorID, nil
}

// ValidateLikePost validates user ID and post ID for like operation.
func ValidateLikePost(currentUserID, postID string) (string, string, error) {
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", err
	}
	if err := ValidateUUID(postID, "postID"); err != nil {
		return "", "", err
	}
	return currentUserID, postID, nil
}

// ValidateUnlikePost validates user ID and post ID for unlike operation.
func ValidateUnlikePost(currentUserID, postID string) (string, string, error) {
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", err
	}
	if err := ValidateUUID(postID, "postID"); err != nil {
		return "", "", err
	}
	return currentUserID, postID, nil
}

// ValidateSharePost validates user ID, post ID, and optional commentary quote.
func ValidateSharePost(currentUserID, postID string, quote *string) (string, string, *string, error) {
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", nil, err
	}
	if err := ValidateUUID(postID, "postID"); err != nil {
		return "", "", nil, err
	}
	if quote != nil {
		if err := ValidateCleanString(*quote, "quote", true); err != nil {
			return "", "", nil, err
		}
		if utf8.RuneCountInString(*quote) > 500 {
			return "", "", nil, fmt.Errorf("%w: quote must not exceed 500 characters", ErrValidation)
		}
	}
	return currentUserID, postID, quote, nil
}
