package services

import (
	"fmt"
	"time"
)

// ValidateGetProfile validates username format for profile lookup.
// If the username format is invalid, ErrNotFound is returned so that non-existent/malformed
// user profiles fail fast with HTTP 404 per OpenAPI specification without touching the repository.
func ValidateGetProfile(username string) (string, error) {
	cleanUsername, err := ValidateUsernameFormat(username)
	if err != nil {
		return "", ErrNotFound
	}
	return cleanUsername, nil
}

// ValidateGetMe validates the current user ID.
func ValidateGetMe(userID string) (string, error) {
	if err := ValidateUUID(userID, "userID"); err != nil {
		return "", err
	}
	return userID, nil
}

// ValidateUpdateMe validates user profile updates ensuring optional fields are properly bounded
// and at least one mutable field is supplied.
func ValidateUpdateMe(userID string, displayName *string, avatarAssetID *string, clearAvatar bool) (string, *string, *string, bool, error) {
	if err := ValidateUUID(userID, "userID"); err != nil {
		return "", nil, nil, false, err
	}

	if displayName == nil && avatarAssetID == nil && !clearAvatar {
		return "", nil, nil, false, fmt.Errorf("%w: at least one field must be provided for update", ErrValidation)
	}

	if avatarAssetID != nil && clearAvatar {
		return "", nil, nil, false, fmt.Errorf("%w: cannot set avatarAssetId and clearAvatar simultaneously", ErrValidation)
	}

	var cleanDisplayName *string
	if displayName != nil {
		dn, err := ValidateDisplayName(*displayName)
		if err != nil {
			return "", nil, nil, false, err
		}
		cleanDisplayName = &dn
	}

	if avatarAssetID != nil {
		if err := ValidateUUID(*avatarAssetID, "avatarAssetId"); err != nil {
			return "", nil, nil, false, err
		}
	}

	return userID, cleanDisplayName, avatarAssetID, clearAvatar, nil
}

// ValidateFollowUser validates actor and target IDs, explicitly rejecting self-follow.
func ValidateFollowUser(currentUserID, targetUserID string) (string, string, error) {
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", err
	}
	if err := ValidateUUID(targetUserID, "targetUserID"); err != nil {
		return "", "", err
	}
	if currentUserID == targetUserID {
		return "", "", ErrSelfFollow
	}
	return currentUserID, targetUserID, nil
}

// ValidateUnfollowUser validates actor and target IDs, explicitly rejecting self-unfollow.
func ValidateUnfollowUser(currentUserID, targetUserID string) (string, string, error) {
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", err
	}
	if err := ValidateUUID(targetUserID, "targetUserID"); err != nil {
		return "", "", err
	}
	if currentUserID == targetUserID {
		return "", "", ErrSelfFollow
	}
	return currentUserID, targetUserID, nil
}

// ValidateGetFollowers validates target user ID and pagination parameters.
func ValidateGetFollowers(targetUserID string, limit int, cursorStr string) (string, int, *time.Time, string, error) {
	if err := ValidateUUID(targetUserID, "targetUserID"); err != nil {
		return "", 0, nil, "", err
	}
	limit, cursorTime, cursorID, err := ValidatePagination(limit, cursorStr, 100)
	if err != nil {
		return "", 0, nil, "", err
	}
	return targetUserID, limit, cursorTime, cursorID, nil
}

// ValidateGetFollowing validates target user ID and pagination parameters.
func ValidateGetFollowing(targetUserID string, limit int, cursorStr string) (string, int, *time.Time, string, error) {
	if err := ValidateUUID(targetUserID, "targetUserID"); err != nil {
		return "", 0, nil, "", err
	}
	limit, cursorTime, cursorID, err := ValidatePagination(limit, cursorStr, 100)
	if err != nil {
		return "", 0, nil, "", err
	}
	return targetUserID, limit, cursorTime, cursorID, nil
}
