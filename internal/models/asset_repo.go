package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Asset struct {
	ID               string
	OwnerID          string
	Kind             string
	StorageBucket    string
	StorageKey       string
	OriginalFilename *string
	MimeType         string
	SizeBytes        int64
	WidthPx          *int
	HeightPx         *int
	Sha256           *string
	Status           string
	UploadExpiresAt  time.Time
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

// CreateAsset creates a new pending asset row.
func (r *Repository) CreateAsset(ctx context.Context, id, ownerID, kind, bucket, storageKey, mimeType string, sizeBytes int64) (*Asset, error) {
	query := `
INSERT INTO public.assets (id, owner_id, kind, storage_bucket, storage_key, mime_type, size_bytes, status, upload_expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', now() + interval '1 hour', now())
RETURNING id, owner_id, kind, storage_bucket, storage_key, original_filename, mime_type, size_bytes, width_px, height_px, sha256, status, upload_expires_at, created_at, completed_at;`

	var a Asset
	err := r.db.QueryRow(ctx, query, id, ownerID, kind, bucket, storageKey, mimeType, sizeBytes).Scan(
		&a.ID,
		&a.OwnerID,
		&a.Kind,
		&a.StorageBucket,
		&a.StorageKey,
		&a.OriginalFilename,
		&a.MimeType,
		&a.SizeBytes,
		&a.WidthPx,
		&a.HeightPx,
		&a.Sha256,
		&a.Status,
		&a.UploadExpiresAt,
		&a.CreatedAt,
		&a.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset: %w", err)
	}
	return &a, nil
}

// GetAssetByID retrieves an asset by UUID.
func (r *Repository) GetAssetByID(ctx context.Context, id string) (*Asset, error) {
	query := `
SELECT id, owner_id, kind, storage_bucket, storage_key, original_filename, mime_type, size_bytes, width_px, height_px, sha256, status, upload_expires_at, created_at, completed_at
FROM public.assets
WHERE id = $1;`

	var a Asset
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID,
		&a.OwnerID,
		&a.Kind,
		&a.StorageBucket,
		&a.StorageKey,
		&a.OriginalFilename,
		&a.MimeType,
		&a.SizeBytes,
		&a.WidthPx,
		&a.HeightPx,
		&a.Sha256,
		&a.Status,
		&a.UploadExpiresAt,
		&a.CreatedAt,
		&a.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get asset by id: %w", err)
	}
	return &a, nil
}

// CompleteAsset updates status to 'ready' and sets completed_at = now() atomically.
func (r *Repository) CompleteAsset(ctx context.Context, id string) (*Asset, error) {
	query := `
UPDATE public.assets
SET status = 'ready', completed_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING id, owner_id, kind, storage_bucket, storage_key, original_filename, mime_type, size_bytes, width_px, height_px, sha256, status, upload_expires_at, created_at, completed_at;`

	var a Asset
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID,
		&a.OwnerID,
		&a.Kind,
		&a.StorageBucket,
		&a.StorageKey,
		&a.OriginalFilename,
		&a.MimeType,
		&a.SizeBytes,
		&a.WidthPx,
		&a.HeightPx,
		&a.Sha256,
		&a.Status,
		&a.UploadExpiresAt,
		&a.CreatedAt,
		&a.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to complete asset: %w", err)
	}
	return &a, nil
}

// GetExpiredPendingAssets retrieves pending assets whose upload deadline has passed.
func (r *Repository) GetExpiredPendingAssets(ctx context.Context) ([]*Asset, error) {
	query := `
SELECT id, owner_id, kind, storage_bucket, storage_key, original_filename, mime_type, size_bytes, width_px, height_px, sha256, status, upload_expires_at, created_at, completed_at
FROM public.assets
WHERE status = 'pending' AND upload_expires_at < now()
LIMIT 100;`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query expired pending assets: %w", err)
	}
	defer rows.Close()

	var assets []*Asset
	for rows.Next() {
		var a Asset
		if err := rows.Scan(
			&a.ID,
			&a.OwnerID,
			&a.Kind,
			&a.StorageBucket,
			&a.StorageKey,
			&a.OriginalFilename,
			&a.MimeType,
			&a.SizeBytes,
			&a.WidthPx,
			&a.HeightPx,
			&a.Sha256,
			&a.Status,
			&a.UploadExpiresAt,
			&a.CreatedAt,
			&a.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan expired asset row: %w", err)
		}
		assets = append(assets, &a)
	}
	return assets, rows.Err()
}

// DeleteAssets permanently removes asset rows by IDs.
func (r *Repository) DeleteAssets(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query := `DELETE FROM public.assets WHERE id = ANY($1);`
	_, err := r.db.Exec(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("failed to delete assets: %w", err)
	}
	return nil
}
