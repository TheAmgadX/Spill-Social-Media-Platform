package services

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ValidateCreateComment validates user, post, and comment body inputs.
func ValidateCreateComment(currentUserID, postID, body string) (string, string, string, error) {
	if err := ValidateUUID(currentUserID, "authorID"); err != nil {
		return "", "", "", err
	}
	if err := ValidateUUID(postID, "postID"); err != nil {
		return "", "", "", err
	}
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", "", "", fmt.Errorf("%w: comment body must not be empty", ErrValidation)
	}
	if err := ValidateCleanString(body, "body", true); err != nil {
		return "", "", "", err
	}
	l := utf8.RuneCountInString(body)
	if l < 1 || l > 1000 {
		return "", "", "", fmt.Errorf("%w: comment body must be between 1 and 1000 characters", ErrValidation)
	}
	return currentUserID, postID, body, nil
}

// ValidateGetComments validates the post ID and pagination parameters for comment listing.
func ValidateGetComments(postID string, limit int, cursorStr string) (string, int, *time.Time, string, error) {
	if err := ValidateUUID(postID, "postID"); err != nil {
		return "", 0, nil, "", err
	}
	limit, cursorTime, cursorID, err := ValidatePagination(limit, cursorStr, 100)
	if err != nil {
		return "", 0, nil, "", err
	}
	return postID, limit, cursorTime, cursorID, nil
}

// ValidateDeleteComment validates identifiers and user role for deleting a comment.
func ValidateDeleteComment(commentID, currentUserID, userRole string) (string, string, string, error) {
	if err := ValidateUUID(commentID, "commentID"); err != nil {
		return "", "", "", err
	}
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", "", err
	}
	cleanRole, err := ValidateRole(userRole)
	if err != nil {
		return "", "", "", err
	}
	return commentID, currentUserID, cleanRole, nil
}
