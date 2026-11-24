package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
)

type CommentRepository interface {
	GetPostByID(ctx context.Context, id string, currentUserID *string) (*models.PostDetail, error)
	CreateComment(ctx context.Context, postID, authorID, body string) (*models.CommentDetail, error)
	GetCommentByID(ctx context.Context, id string) (*models.Comment, error)
	GetCommentsByPostID(ctx context.Context, postID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.CommentDetail, error)
	SoftDeleteComment(ctx context.Context, id string) error
}

type CommentService struct {
	repo                 CommentRepository
	storagePublicBaseURL string
}

func NewCommentService(repo CommentRepository, storagePublicBaseURL string) *CommentService {
	return &CommentService{
		repo:                 repo,
		storagePublicBaseURL: storagePublicBaseURL,
	}
}

func (s *CommentService) ToCommentResponse(c *models.CommentDetail) views.CommentResponse {
	var avatarURL *string
	if c.AuthorAvatarStorageKey != nil && *c.AuthorAvatarStorageKey != "" {
		u := fmt.Sprintf("%s/%s", s.storagePublicBaseURL, *c.AuthorAvatarStorageKey)
		avatarURL = &u
	}

	author := views.PublicUserResponse{
		ID:             c.AuthorID,
		Username:       c.AuthorUsername,
		DisplayName:    c.AuthorDisplayName,
		AvatarURL:      avatarURL,
		Role:           c.AuthorRole,
		FollowerCount:  c.AuthorFollowerCount,
		FollowingCount: c.AuthorFollowingCount,
		CreatedAt:      c.AuthorCreatedAt.Format(time.RFC3339),
	}

	return views.CommentResponse{
		ID:        c.ID,
		PostID:    c.PostID,
		AuthorID:  c.AuthorID,
		Author:    &author,
		Body:      c.Body,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
}

func (s *CommentService) CreateComment(ctx context.Context, currentUserID, postID, body string) (*views.CommentResponse, error) {
	cleanUserID, cleanPostID, cleanBody, err := ValidateCreateComment(currentUserID, postID, body)
	if err != nil {
		return nil, err
	}

	// Verify post exists and is not soft-deleted
	if _, err := s.repo.GetPostByID(ctx, cleanPostID, nil); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	comment, err := s.repo.CreateComment(ctx, cleanPostID, cleanUserID, cleanBody)
	if err != nil {
		return nil, err
	}

	res := s.ToCommentResponse(comment)
	return &res, nil
}

func (s *CommentService) GetComments(ctx context.Context, postID string, limit int, cursorStr string) (*views.CommentListResponse, error) {
	cleanPostID, cleanLimit, cursorTime, cursorID, err := ValidateGetComments(postID, limit, cursorStr)
	if err != nil {
		return nil, err
	}

	// Verify post exists and is not soft-deleted
	if _, err := s.repo.GetPostByID(ctx, cleanPostID, nil); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	comments, err := s.repo.GetCommentsByPostID(ctx, cleanPostID, cleanLimit+1, cursorTime, cursorID)
	if err != nil {
		return nil, err
	}

	hasMore := len(comments) > cleanLimit
	if hasMore {
		comments = comments[:cleanLimit]
	}

	var data []views.CommentResponse
	for _, c := range comments {
		data = append(data, s.ToCommentResponse(c))
	}

	var nextCursor *string
	if hasMore && len(comments) > 0 {
		last := comments[len(comments)-1]
		c := views.EncodeCursor(last.CreatedAt, last.ID)
		nextCursor = &c
	}

	return &views.CommentListResponse{
		Data: data,
		Pagination: views.PaginationMeta{
			NextCursor: nextCursor,
			Limit:      cleanLimit,
			HasMore:    hasMore,
		},
	}, nil
}

func (s *CommentService) DeleteComment(ctx context.Context, commentID, currentUserID, userRole string) error {
	cleanCommentID, cleanUserID, cleanRole, err := ValidateDeleteComment(commentID, currentUserID, userRole)
	if err != nil {
		return err
	}

	comment, err := s.repo.GetCommentByID(ctx, cleanCommentID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if comment.DeletedAt != nil {
		return ErrNotFound
	}

	if comment.AuthorID != cleanUserID && cleanRole != "admin" {
		return ErrForbidden
	}

	return s.repo.SoftDeleteComment(ctx, cleanCommentID)
}
