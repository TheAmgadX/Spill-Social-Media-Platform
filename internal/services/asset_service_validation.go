package services

import (
	"fmt"
	"strings"
)

// ValidateCreateAsset validates arguments before creating a presigned asset upload.
func ValidateCreateAsset(ownerID, kind, mimeType string, sizeBytes int64) (string, string, string, error) {
	if err := ValidateUUID(ownerID, "ownerID"); err != nil {
		return "", "", "", err
	}

	if sizeBytes > 5242880 {
		return "", "", "", fmt.Errorf("%w: asset size must not exceed 5 242 880 bytes (5 MB)", ErrPayloadTooLarge)
	}
	if sizeBytes < 1 {
		return "", "", "", fmt.Errorf("%w: asset size must be at least 1 byte", ErrValidation)
	}

	cleanMime := strings.ToLower(strings.TrimSpace(mimeType))
	switch cleanMime {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return "", "", "", fmt.Errorf("%w: unsupported mime type %s (allowed: image/jpeg, image/png, image/webp)", ErrValidation, mimeType)
	}

	cleanKind := strings.ToLower(strings.TrimSpace(kind))
	if cleanKind == "" {
		cleanKind = "post_image"
	}
	if cleanKind != "post_image" && cleanKind != "avatar" {
		return "", "", "", fmt.Errorf("%w: invalid kind %s", ErrValidation, kind)
	}

	return ownerID, cleanKind, cleanMime, nil
}

// ValidateCompleteAsset validates arguments before marking an asset upload ready.
func ValidateCompleteAsset(assetID, currentUserID, userRole string) (string, string, string, error) {
	if err := ValidateUUID(assetID, "assetID"); err != nil {
		return "", "", "", err
	}
	if err := ValidateUUID(currentUserID, "currentUserID"); err != nil {
		return "", "", "", err
	}
	cleanRole, err := ValidateRole(userRole)
	if err != nil {
		return "", "", "", err
	}
	return assetID, currentUserID, cleanRole, nil
}
