package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
	"github.com/jackc/pgx/v5/pgconn"
)

type PostRepository interface {
	GetUserByID(ctx context.Context, id string) (*models.UserDetail, error)
	CreatePost(ctx context.Context, authorID, body string, assetIDs []string) (*models.PostDetail, error)
	GetPostByID(ctx context.Context, id string, currentUserID *string) (*models.PostDetail, error)
	UpdatePost(ctx context.Context, id, body string) (*models.Post, error)
	SoftDeletePost(ctx context.Context, id string) error
	GetFeed(ctx context.Context, userID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.PostDetail, error)
	GetUserPosts(ctx context.Context, targetUserID string, currentUserID *string, limit int, cursorTime *time.Time, cursorID string) ([]*models.PostDetail, error)
	LikePost(ctx context.Context, userID, postID string) (bool, error)
	UnlikePost(ctx context.Context, userID, postID string) (bool, error)
	CreateShare(ctx context.Context, userID, postID string, quote *string) (*models.Share, error)
}

type PostService struct {
	repo                 PostRepository
	storagePublicBaseURL string
}

func NewPostService(repo PostRepository, storagePublicBaseURL string) *PostService {
	return &PostService{
		repo:                 repo,
		storagePublicBaseURL: storagePublicBaseURL,
	}
}

func (s *PostService) ToPostResponse(p *models.PostDetail) views.PostResponse {
	var avatarURL *string
	if p.AuthorAvatarStorageKey != nil && *p.AuthorAvatarStorageKey != "" {
		u := fmt.Sprintf("%s/%s", s.storagePublicBaseURL, *p.AuthorAvatarStorageKey)
		avatarURL = &u
	}

	author := views.PublicUserResponse{
		ID:             p.AuthorID,
		Username:       p.AuthorUsername,
		DisplayName:    p.AuthorDisplayName,
		AvatarURL:      avatarURL,
		Role:           p.AuthorRole,
		FollowerCount:  p.AuthorFollowerCount,
		FollowingCount: p.AuthorFollowingCount,
		CreatedAt:      p.AuthorCreatedAt.Format(time.RFC3339),
	}

	var assets []views.AssetResponse
	for _, a := range p.Assets {
		assets = append(assets, views.AssetResponse{
			ID:        a.ID,
			URL:       fmt.Sprintf("%s/%s", s.storagePublicBaseURL, a.StorageKey),
			MimeType:  a.MimeType,
			SizeBytes: a.SizeBytes,
			Status:    a.Status,
		})
	}
	if assets == nil {
		assets = []views.AssetResponse{}
	}

	return views.PostResponse{
		ID:           p.ID,
		AuthorID:     p.AuthorID,
		Author:       &author,
		Body:         p.Body,
		Assets:       assets,
		LikeCount:    p.LikeCount,
		CommentCount: p.CommentCount,
		ShareCount:   p.ShareCount,
		Liked:        p.Liked,
		CreatedAt:    p.CreatedAt.Format(time.RFC3339),
	}
}

func (s *PostService) CreatePost(ctx context.Context, authorID, body string, assetIDs []string) (*views.PostResponse, error) {
	cleanAuthorID, cleanBody, cleanAssetIDs, err := ValidateCreatePost(authorID, body, assetIDs)
	if err != nil {
		return nil, err
	}

	post, err := s.repo.CreatePost(ctx, cleanAuthorID, cleanBody, cleanAssetIDs)
	if err != nil {
		var trgErr *models.TriggerError
		if errors.As(err, &trgErr) {
			return nil, fmt.Errorf("%w: %s", ErrValidation, trgErr.Message)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "P0001", "23503", "23505", "23514":
				return nil, fmt.Errorf("%w: %s", ErrValidation, pgErr.Message)
			}
		}
		return nil, err
	}

	res := s.ToPostResponse(post)
	return &res, nil
}

func (s *PostService) GetPost(ctx context.Context, id string, currentUserID *string) (*views.PostResponse, error) {
	cleanID, cleanUserID, err := ValidateGetPost(id, currentUserID)
	if err != nil {
		return nil, err
	}

	post, err := s.repo.GetPostByID(ctx, cleanID, cleanUserID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	res := s.ToPostResponse(post)
	return &res, nil
}

func (s *PostService) UpdatePost(ctx context.Context, id, currentUserID, userRole, body string) (*views.PostResponse, error) {
	cleanID, cleanUserID, cleanRole, cleanBody, err := ValidateUpdatePost(id, currentUserID, userRole, body)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetPostByID(ctx, cleanID, nil)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if existing.AuthorID != cleanUserID && cleanRole != "admin" {
		return nil, ErrForbidden
	}

	if _, err := s.repo.UpdatePost(ctx, cleanID, cleanBody); err != nil {
		return nil, err
	}

	return s.GetPost(ctx, cleanID, &cleanUserID)
}

func (s *PostService) DeletePost(ctx context.Context, id, currentUserID, userRole string) error {
	cleanID, cleanUserID, cleanRole, err := ValidateDeletePost(id, currentUserID, userRole)
	if err != nil {
		return err
	}

	existing, err := s.repo.GetPostByID(ctx, cleanID, nil)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if existing.AuthorID != cleanUserID && cleanRole != "admin" {
		return ErrForbidden
	}

	return s.repo.SoftDeletePost(ctx, cleanID)
}

func (s *PostService) GetFeed(ctx context.Context, currentUserID string, limit int, cursorStr string) (*views.PostListResponse, error) {
	cleanUserID, cleanLimit, cursorTime, cursorID, err := ValidateGetFeed(currentUserID, limit, cursorStr)
	if err != nil {
		return nil, err
	}

	posts, err := s.repo.GetFeed(ctx, cleanUserID, cleanLimit+1, cursorTime, cursorID)
	if err != nil {
		return nil, err
	}

	hasMore := len(posts) > cleanLimit
	if hasMore {
		posts = posts[:cleanLimit]
	}

	var data []views.PostResponse
	for _, p := range posts {
		data = append(data, s.ToPostResponse(p))
	}

	var nextCursor *string
	if hasMore && len(posts) > 0 {
		last := posts[len(posts)-1]
		c := views.EncodeCursor(last.CreatedAt, last.ID)
		nextCursor = &c
	}

	return &views.PostListResponse{
		Data: data,
		Pagination: views.PaginationMeta{
			NextCursor: nextCursor,
			Limit:      cleanLimit,
			HasMore:    hasMore,
		},
	}, nil
}

func (s *PostService) GetUserPosts(ctx context.Context, targetUserID string, currentUserID *string, limit int, cursorStr string) (*views.PostListResponse, error) {
	cleanTargetID, cleanUserID, cleanLimit, cursorTime, cursorID, err := ValidateGetUserPosts(targetUserID, currentUserID, limit, cursorStr)
	if err != nil {
		return nil, err
	}

	// Verify user exists
	if _, err := s.repo.GetUserByID(ctx, cleanTargetID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	posts, err := s.repo.GetUserPosts(ctx, cleanTargetID, cleanUserID, cleanLimit+1, cursorTime, cursorID)
	if err != nil {
		return nil, err
	}

	hasMore := len(posts) > cleanLimit
	if hasMore {
		posts = posts[:cleanLimit]
	}

	var data []views.PostResponse
	for _, p := range posts {
		data = append(data, s.ToPostResponse(p))
	}

	var nextCursor *string
	if hasMore && len(posts) > 0 {
		last := posts[len(posts)-1]
		c := views.EncodeCursor(last.CreatedAt, last.ID)
		nextCursor = &c
	}

	return &views.PostListResponse{
		Data: data,
		Pagination: views.PaginationMeta{
			NextCursor: nextCursor,
			Limit:      cleanLimit,
			HasMore:    hasMore,
		},
	}, nil
}

func (s *PostService) LikePost(ctx context.Context, currentUserID, postID string) (bool, error) {
	cleanUserID, cleanPostID, err := ValidateLikePost(currentUserID, postID)
	if err != nil {
		return false, err
	}

	// Verify post exists and not soft-deleted
	if _, err := s.repo.GetPostByID(ctx, cleanPostID, nil); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return false, ErrNotFound
		}
		return false, err
	}

	_, err = s.repo.LikePost(ctx, cleanUserID, cleanPostID)
	if err != nil {
		return false, err
	}
	// PUT /posts/{id}/like is idempotent and returns current state (liked: true)
	return true, nil
}

func (s *PostService) UnlikePost(ctx context.Context, currentUserID, postID string) (bool, error) {
	cleanUserID, cleanPostID, err := ValidateUnlikePost(currentUserID, postID)
	if err != nil {
		return false, err
	}

	// Verify post exists and not soft-deleted
	if _, err := s.repo.GetPostByID(ctx, cleanPostID, nil); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return false, ErrNotFound
		}
		return false, err
	}

	_, err = s.repo.UnlikePost(ctx, cleanUserID, cleanPostID)
	if err != nil {
		return false, err
	}
	// DELETE /posts/{id}/like returns current state (liked: false)
	return false, nil
}

func (s *PostService) SharePost(ctx context.Context, currentUserID, postID string, quote *string) (*views.ShareResponse, error) {
	cleanUserID, cleanPostID, cleanQuote, err := ValidateSharePost(currentUserID, postID, quote)
	if err != nil {
		return nil, err
	}

	// Verify post exists and not soft-deleted
	if _, err := s.repo.GetPostByID(ctx, cleanPostID, nil); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	sh, err := s.repo.CreateShare(ctx, cleanUserID, cleanPostID, cleanQuote)
	if err != nil {
		return nil, err
	}

	return &views.ShareResponse{
		ID:        sh.ID,
		UserID:    sh.UserID,
		PostID:    sh.PostID,
		Quote:     sh.Quote,
		CreatedAt: sh.CreatedAt.Format(time.RFC3339),
	}, nil
}
