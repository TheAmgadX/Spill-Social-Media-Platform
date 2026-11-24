package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
	"github.com/google/uuid"
)

type callCountingAssetRepo struct {
	createCalls   int
	getCalls      int
	completeCalls int
}

func (c *callCountingAssetRepo) CreateAsset(ctx context.Context, id, ownerID, kind, bucket, storageKey, mimeType string, sizeBytes int64) (*models.Asset, error) {
	c.createCalls++
	return &models.Asset{ID: id, OwnerID: ownerID, Status: "pending"}, nil
}

func (c *callCountingAssetRepo) GetAssetByID(ctx context.Context, id string) (*models.Asset, error) {
	c.getCalls++
	return nil, models.ErrNotFound
}

func (c *callCountingAssetRepo) CompleteAsset(ctx context.Context, id string) (*models.Asset, error) {
	c.completeCalls++
	return &models.Asset{ID: id, Status: "ready"}, nil
}

func (c *callCountingAssetRepo) GetExpiredPendingAssets(ctx context.Context) ([]*models.Asset, error) {
	return nil, nil
}

func (c *callCountingAssetRepo) DeleteAssets(ctx context.Context, ids []string) error {
	return nil
}

func TestAssetService_ValidateCreateAsset(t *testing.T) {
	validOwner := uuid.NewString()

	tests := []struct {
		name      string
		ownerID   string
		kind      string
		mimeType  string
		sizeBytes int64
		wantErr   bool
		errType   error
	}{
		{"Valid JPEG post_image", validOwner, "post_image", "image/jpeg", 2048, false, nil},
		{"Valid PNG avatar", validOwner, "avatar", "image/png", 1024, false, nil},
		{"Valid WEBP default kind", validOwner, "", "image/webp", 5242880, false, nil},
		{"Size exact 1 byte boundary", validOwner, "post_image", "image/png", 1, false, nil},
		{"Size exact 5MB boundary", validOwner, "post_image", "image/png", 5242880, false, nil},
		{"Size 0 bytes (min - 1)", validOwner, "post_image", "image/png", 0, true, services.ErrValidation},
		{"Size negative bytes", validOwner, "post_image", "image/png", -10, true, services.ErrValidation},
		{"Size 5MB + 1 byte (max + 1)", validOwner, "post_image", "image/png", 5242881, true, services.ErrPayloadTooLarge},
		{"Disallowed MIME type PDF", validOwner, "post_image", "application/pdf", 1000, true, services.ErrValidation},
		{"Disallowed MIME type GIF", validOwner, "post_image", "image/gif", 1000, true, services.ErrValidation},
		{"Disallowed MIME type plain text", validOwner, "post_image", "text/plain", 1000, true, services.ErrValidation},
		{"Invalid kind document", validOwner, "document", "image/jpeg", 1000, true, services.ErrValidation},
		{"Malformed owner UUID", "not-a-uuid", "post_image", "image/jpeg", 1000, true, services.ErrValidation},
		{"Nil owner UUID", "00000000-0000-0000-0000-000000000000", "post_image", "image/jpeg", 1000, true, services.ErrValidation},
		{"Empty owner UUID", "", "post_image", "image/jpeg", 1000, true, services.ErrValidation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := services.ValidateCreateAsset(tt.ownerID, tt.kind, tt.mimeType, tt.sizeBytes)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCreateAsset() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errType != nil && !errors.Is(err, tt.errType) {
				t.Errorf("expected error %v, got %v", tt.errType, err)
			}
		})
	}
}

func TestAssetService_ValidateCompleteAsset(t *testing.T) {
	validAsset := uuid.NewString()
	validUser := uuid.NewString()

	tests := []struct {
		name    string
		assetID string
		userID  string
		role    string
		wantErr bool
	}{
		{"Valid owner", validAsset, validUser, "user", false},
		{"Valid admin", validAsset, validUser, "admin", false},
		{"Malformed asset ID", "not-a-uuid", validUser, "user", true},
		{"Nil asset ID", "00000000-0000-0000-0000-000000000000", validUser, "user", true},
		{"Empty asset ID", "", validUser, "user", true},
		{"Malformed user ID", validAsset, "invalid", "user", true},
		{"Invalid role", validAsset, validUser, "guest", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := services.ValidateCompleteAsset(tt.assetID, tt.userID, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCompleteAsset() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAssetService_FailFastZeroRepoCalls(t *testing.T) {
	repo := &callCountingAssetRepo{}
	store := newMockStorageClient()
	svc := services.NewAssetService(repo, store, "media", "https://storage.local")
	ctx := context.Background()

	// Invalid CreateAsset -> zero DB calls
	_, _, err := svc.CreateAsset(ctx, "invalid-owner", "post_image", "image/jpeg", 1000)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if repo.createCalls != 0 {
		t.Errorf("expected 0 repo create calls on validation failure, got %d", repo.createCalls)
	}

	// Invalid CompleteAsset -> zero DB calls
	_, err = svc.CompleteAsset(ctx, "invalid-asset", uuid.NewString(), "user")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if repo.getCalls != 0 || repo.completeCalls != 0 {
		t.Errorf("expected 0 repo calls on complete validation failure, got get=%d complete=%d", repo.getCalls, repo.completeCalls)
	}
}
