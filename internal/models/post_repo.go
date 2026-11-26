package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Post struct {
	ID           string
	AuthorID     string
	Body         string
	LikeCount    int
	CommentCount int
	ShareCount   int
	CreatedAt    time.Time
	EditedAt     *time.Time
	DeletedAt    *time.Time
}

type PostAssetInfo struct {
	ID         string
	StorageKey string
	MimeType   string
	SizeBytes  int64
	Status     string
	Position   int
}

type PostDetail struct {
	Post
	AuthorUsername         string
	AuthorDisplayName      string
	AuthorAvatarStorageKey *string
	AuthorRole             string
	AuthorFollowerCount    int
	AuthorFollowingCount   int
	AuthorCreatedAt        time.Time
	Assets                 []PostAssetInfo
	Liked                  *bool
}

type TriggerError struct {
	Message string
}

func (e *TriggerError) Error() string {
	return e.Message
}

// CreatePost inserts a post and its post_assets rows in a single transaction.
func (r *Repository) CreatePost(ctx context.Context, authorID, body string, assetIDs []string) (*PostDetail, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin post transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var p Post
	insertPostQuery := `
INSERT INTO public.posts (id, author_id, body, created_at)
VALUES (gen_random_uuid(), $1, $2, now())
RETURNING id, author_id, body, like_count, comment_count, share_count, created_at;`

	err = tx.QueryRow(ctx, insertPostQuery, authorID, body).Scan(
		&p.ID,
		&p.AuthorID,
		&p.Body,
		&p.LikeCount,
		&p.CommentCount,
		&p.ShareCount,
		&p.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "P0001" {
				return nil, &TriggerError{Message: pgErr.Message}
			}
			if pgErr.Code == "23503" { // foreign_key_violation (missing author)
				return nil, &TriggerError{Message: "author does not exist"}
			}
		}
		return nil, fmt.Errorf("failed to insert post: %w", err)
	}

	for pos, assetID := range assetIDs {
		insertAssetQuery := `
INSERT INTO public.post_assets (post_id, asset_id, position, created_at)
VALUES ($1, $2, $3, now());`
		_, err := tx.Exec(ctx, insertAssetQuery, p.ID, assetID, pos)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				if pgErr.Code == "P0001" {
					return nil, &TriggerError{Message: pgErr.Message}
				}
				if pgErr.Code == "23514" { // check_violation (e.g. position > 3)
					return nil, &TriggerError{Message: "posts cannot have more than 4 images"}
				}
				if pgErr.Code == "23503" { // foreign_key_violation (missing/non-existent asset)
					return nil, &TriggerError{Message: fmt.Sprintf("asset %s does not exist or is not available", assetID)}
				}
				if pgErr.Code == "23505" { // unique_violation (asset already attached to a post)
					return nil, &TriggerError{Message: fmt.Sprintf("asset %s is already attached to another post", assetID)}
				}
			}
			return nil, fmt.Errorf("failed to attach asset %s: %w", assetID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit post transaction: %w", err)
	}

	return r.GetPostByID(ctx, p.ID, &authorID)
}

// GetPostByID fetches a single non-deleted post by ID.
func (r *Repository) GetPostByID(ctx context.Context, id string, currentUserID *string) (*PostDetail, error) {
	query := `
SELECT p.id, p.author_id, p.body, p.like_count, p.comment_count, p.share_count, p.created_at, p.edited_at,
       u.username, u.display_name, a_u.storage_key, u.role, u.follower_count, u.following_count, u.created_at,
       CASE WHEN $2::uuid IS NOT NULL THEN
         EXISTS (SELECT 1 FROM public.likes l WHERE l.user_id = $2::uuid AND l.post_id = p.id)
       ELSE NULL END AS liked
FROM public.posts p
JOIN public.users u ON u.id = p.author_id
LEFT JOIN public.assets a_u ON a_u.id = u.avatar_asset_id
WHERE p.id = $1 AND p.deleted_at IS NULL;`

	var p PostDetail
	err := r.db.QueryRow(ctx, query, id, currentUserID).Scan(
		&p.ID,
		&p.AuthorID,
		&p.Body,
		&p.LikeCount,
		&p.CommentCount,
		&p.ShareCount,
		&p.CreatedAt,
		&p.EditedAt,
		&p.AuthorUsername,
		&p.AuthorDisplayName,
		&p.AuthorAvatarStorageKey,
		&p.AuthorRole,
		&p.AuthorFollowerCount,
		&p.AuthorFollowingCount,
		&p.AuthorCreatedAt,
		&p.Liked,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get post by id: %w", err)
	}

	assets, err := r.getAssetsForPosts(ctx, []string{p.ID})
	if err != nil {
		return nil, err
	}
	p.Assets = assets[p.ID]
	if p.Assets == nil {
		p.Assets = []PostAssetInfo{}
	}

	return &p, nil
}

// UpdatePost updates the body of a non-deleted post.
func (r *Repository) UpdatePost(ctx context.Context, id, body string) (*Post, error) {
	query := `
UPDATE public.posts
SET body = $2, edited_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, author_id, body, like_count, comment_count, share_count, created_at, edited_at;`

	var p Post
	err := r.db.QueryRow(ctx, query, id, body).Scan(
		&p.ID,
		&p.AuthorID,
		&p.Body,
		&p.LikeCount,
		&p.CommentCount,
		&p.ShareCount,
		&p.CreatedAt,
		&p.EditedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to update post: %w", err)
	}
	return &p, nil
}

// SoftDeletePost marks a post as soft-deleted.
func (r *Repository) SoftDeletePost(ctx context.Context, id string) error {
	query := `UPDATE public.posts SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to soft delete post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetFeed returns posts from followed authors using keyset pagination.
func (r *Repository) GetFeed(ctx context.Context, userID string, limit int, cursorTime *time.Time, cursorID string) ([]*PostDetail, error) {
	var query string
	var rows pgx.Rows
	var err error

	if cursorTime != nil && cursorID != "" {
		query = `
SELECT p.id, p.author_id, p.body, p.like_count, p.comment_count, p.share_count, p.created_at, p.edited_at,
       u.username, u.display_name, a_u.storage_key, u.role, u.follower_count, u.following_count, u.created_at,
       EXISTS (SELECT 1 FROM public.likes l WHERE l.user_id = $1::uuid AND l.post_id = p.id) AS liked
FROM public.posts p
JOIN public.follows f ON f.followee_id = p.author_id
JOIN public.users u ON u.id = p.author_id
LEFT JOIN public.assets a_u ON a_u.id = u.avatar_asset_id
WHERE f.follower_id = $1::uuid
  AND p.deleted_at IS NULL
  AND (p.created_at, p.id) < ($2, $3::uuid)
ORDER BY p.created_at DESC, p.id DESC
LIMIT $4;`
		rows, err = r.db.Query(ctx, query, userID, *cursorTime, cursorID, limit)
	} else {
		query = `
SELECT p.id, p.author_id, p.body, p.like_count, p.comment_count, p.share_count, p.created_at, p.edited_at,
       u.username, u.display_name, a_u.storage_key, u.role, u.follower_count, u.following_count, u.created_at,
       EXISTS (SELECT 1 FROM public.likes l WHERE l.user_id = $1::uuid AND l.post_id = p.id) AS liked
FROM public.posts p
JOIN public.follows f ON f.followee_id = p.author_id
JOIN public.users u ON u.id = p.author_id
LEFT JOIN public.assets a_u ON a_u.id = u.avatar_asset_id
WHERE f.follower_id = $1::uuid
  AND p.deleted_at IS NULL
ORDER BY p.created_at DESC, p.id DESC
LIMIT $2;`
		rows, err = r.db.Query(ctx, query, userID, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query feed: %w", err)
	}
	defer rows.Close()

	return r.scanPostDetails(ctx, rows)
}

// GetUserPosts returns posts written by a specific user using keyset pagination.
func (r *Repository) GetUserPosts(ctx context.Context, targetUserID string, currentUserID *string, limit int, cursorTime *time.Time, cursorID string) ([]*PostDetail, error) {
	var query string
	var rows pgx.Rows
	var err error

	if cursorTime != nil && cursorID != "" {
		query = `
SELECT p.id, p.author_id, p.body, p.like_count, p.comment_count, p.share_count, p.created_at, p.edited_at,
       u.username, u.display_name, a_u.storage_key, u.role, u.follower_count, u.following_count, u.created_at,
       CASE WHEN $2::uuid IS NOT NULL THEN
         EXISTS (SELECT 1 FROM public.likes l WHERE l.user_id = $2::uuid AND l.post_id = p.id)
       ELSE NULL END AS liked
FROM public.posts p
JOIN public.users u ON u.id = p.author_id
LEFT JOIN public.assets a_u ON a_u.id = u.avatar_asset_id
WHERE p.author_id = $1::uuid
  AND p.deleted_at IS NULL
  AND (p.created_at, p.id) < ($3, $4::uuid)
ORDER BY p.created_at DESC, p.id DESC
LIMIT $5;`
		rows, err = r.db.Query(ctx, query, targetUserID, currentUserID, *cursorTime, cursorID, limit)
	} else {
		query = `
SELECT p.id, p.author_id, p.body, p.like_count, p.comment_count, p.share_count, p.created_at, p.edited_at,
       u.username, u.display_name, a_u.storage_key, u.role, u.follower_count, u.following_count, u.created_at,
       CASE WHEN $2::uuid IS NOT NULL THEN
         EXISTS (SELECT 1 FROM public.likes l WHERE l.user_id = $2::uuid AND l.post_id = p.id)
       ELSE NULL END AS liked
FROM public.posts p
JOIN public.users u ON u.id = p.author_id
LEFT JOIN public.assets a_u ON a_u.id = u.avatar_asset_id
WHERE p.author_id = $1::uuid
  AND p.deleted_at IS NULL
ORDER BY p.created_at DESC, p.id DESC
LIMIT $3;`
		rows, err = r.db.Query(ctx, query, targetUserID, currentUserID, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query user posts: %w", err)
	}
	defer rows.Close()

	return r.scanPostDetails(ctx, rows)
}

func (r *Repository) scanPostDetails(ctx context.Context, rows pgx.Rows) ([]*PostDetail, error) {
	var posts []*PostDetail
	var postIDs []string

	for rows.Next() {
		var p PostDetail
		if err := rows.Scan(
			&p.ID,
			&p.AuthorID,
			&p.Body,
			&p.LikeCount,
			&p.CommentCount,
			&p.ShareCount,
			&p.CreatedAt,
			&p.EditedAt,
			&p.AuthorUsername,
			&p.AuthorDisplayName,
			&p.AuthorAvatarStorageKey,
			&p.AuthorRole,
			&p.AuthorFollowerCount,
			&p.AuthorFollowingCount,
			&p.AuthorCreatedAt,
			&p.Liked,
		); err != nil {
			return nil, fmt.Errorf("failed to scan post detail: %w", err)
		}
		p.Assets = []PostAssetInfo{}
		posts = append(posts, &p)
		postIDs = append(postIDs, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(postIDs) > 0 {
		assetsMap, err := r.getAssetsForPosts(ctx, postIDs)
		if err != nil {
			return nil, err
		}
		for _, p := range posts {
			if a, ok := assetsMap[p.ID]; ok {
				p.Assets = a
			}
		}
	}

	return posts, nil
}

func (r *Repository) getAssetsForPosts(ctx context.Context, postIDs []string) (map[string][]PostAssetInfo, error) {
	query := `
SELECT pa.post_id, a.id, a.storage_key, a.mime_type, a.size_bytes, a.status, pa.position
FROM public.post_assets pa
JOIN public.assets a ON a.id = pa.asset_id
WHERE pa.post_id = ANY($1)
ORDER BY pa.post_id, pa.position ASC;`

	rows, err := r.db.Query(ctx, query, postIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query assets for posts: %w", err)
	}
	defer rows.Close()

	res := make(map[string][]PostAssetInfo)
	for rows.Next() {
		var postID string
		var asset PostAssetInfo
		if err := rows.Scan(
			&postID,
			&asset.ID,
			&asset.StorageKey,
			&asset.MimeType,
			&asset.SizeBytes,
			&asset.Status,
			&asset.Position,
		); err != nil {
			return nil, fmt.Errorf("failed to scan post asset: %w", err)
		}
		res[postID] = append(res[postID], asset)
	}
	return res, rows.Err()
}
