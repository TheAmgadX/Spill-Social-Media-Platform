package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/storage"
	"github.com/google/uuid"
)

type AssetRepository interface {
	CreateAsset(ctx context.Context, id, ownerID, kind, bucket, storageKey, mimeType string, sizeBytes int64) (*models.Asset, error)
	GetAssetByID(ctx context.Context, id string) (*models.Asset, error)
	CompleteAsset(ctx context.Context, id string) (*models.Asset, error)
	GetExpiredPendingAssets(ctx context.Context) ([]*models.Asset, error)
	DeleteAssets(ctx context.Context, ids []string) error
}

type AssetService struct {
	repo                 AssetRepository
	storageClient        storage.StorageClient
	storageBucket        string
	storagePublicBaseURL string
}

func NewAssetService(repo AssetRepository, storageClient storage.StorageClient, storageBucket, storagePublicBaseURL string) *AssetService {
	return &AssetService{
		repo:                 repo,
		storageClient:        storageClient,
		storageBucket:        storageBucket,
		storagePublicBaseURL: storagePublicBaseURL,
	}
}

func (s *AssetService) CreateAsset(ctx context.Context, ownerID, kind, mimeType string, sizeBytes int64) (string, string, error) {
	cleanOwnerID, cleanKind, cleanMime, err := ValidateCreateAsset(ownerID, kind, mimeType, sizeBytes)
	if err != nil {
		return "", "", err
	}

	var ext string
	switch cleanMime {
	case "image/jpeg":
		ext = "jpg"
	case "image/png":
		ext = "png"
	case "image/webp":
		ext = "webp"
	}

	assetID := uuid.NewString()
	storageKey := fmt.Sprintf("u/%s/%s.%s", cleanOwnerID, assetID, ext)

	// Insert pending row into database
	_, err = s.repo.CreateAsset(ctx, assetID, cleanOwnerID, cleanKind, s.storageBucket, storageKey, cleanMime, sizeBytes)
	if err != nil {
		return "", "", fmt.Errorf("failed to save asset record: %w", err)
	}

	// Obtain signed PUT upload URL from Supabase Storage API
	signedURL, err := s.storageClient.CreateSignedUploadURL(ctx, s.storageBucket, storageKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to obtain presigned upload URL: %w", err)
	}

	return assetID, signedURL, nil
}

func (s *AssetService) CompleteAsset(ctx context.Context, assetID, currentUserID, userRole string) (string, error) {
	cleanAssetID, cleanUserID, cleanRole, err := ValidateCompleteAsset(assetID, currentUserID, userRole)
	if err != nil {
		return "", err
	}

	asset, err := s.repo.GetAssetByID(ctx, cleanAssetID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}

	// Ownership check
	if asset.OwnerID != cleanUserID && cleanRole != "admin" {
		return "", ErrForbidden
	}

	if asset.Status == "ready" {
		return "", ErrAlreadyReady
	}

	// Verify object exists in storage and size matches
	if err := s.storageClient.VerifyObject(ctx, asset.StorageBucket, asset.StorageKey, asset.SizeBytes); err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return "", fmt.Errorf("%w: storage object not found in bucket", ErrValidation)
		}
		if errors.Is(err, storage.ErrSizeMismatch) {
			return "", fmt.Errorf("%w: uploaded size does not match expected size", ErrValidation)
		}
		return "", fmt.Errorf("%w: storage verification failed: %s", ErrValidation, err.Error())
	}

	// Set status = 'ready' and completed_at = now()
	completed, err := s.repo.CompleteAsset(ctx, cleanAssetID)
	if err != nil {
		return "", fmt.Errorf("failed to mark asset ready: %w", err)
	}

	publicURL := fmt.Sprintf("%s/%s", s.storagePublicBaseURL, completed.StorageKey)
	return publicURL, nil
}

func (s *AssetService) CleanupExpiredAssets(ctx context.Context) (int, error) {
	expired, err := s.repo.GetExpiredPendingAssets(ctx)
	if err != nil {
		return 0, err
	}
	if len(expired) == 0 {
		return 0, nil
	}

	bucketKeys := make(map[string][]string)
	var ids []string
	for _, a := range expired {
		bucketKeys[a.StorageBucket] = append(bucketKeys[a.StorageBucket], a.StorageKey)
		ids = append(ids, a.ID)
	}

	// Delete from storage
	for bucket, keys := range bucketKeys {
		if err := s.storageClient.DeleteObjects(ctx, bucket, keys); err != nil {
			log.Printf("[AssetCleanup] Error deleting objects from bucket %s: %v", bucket, err)
		}
	}

	// Delete from DB
	if err := s.repo.DeleteAssets(ctx, ids); err != nil {
		return 0, fmt.Errorf("failed to delete expired asset rows: %w", err)
	}

	return len(ids), nil
}

func (s *AssetService) StartCleanupJob(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				count, err := s.CleanupExpiredAssets(cleanupCtx)
				cancel()
				if err != nil {
					log.Printf("[AssetCleanup] Job error: %v", err)
				} else if count > 0 {
					log.Printf("[AssetCleanup] Purged %d expired pending assets", count)
				}
			}
		}
	}()
}
