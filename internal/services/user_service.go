package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
)

type UserRepository interface {
	GetUserByUsername(ctx context.Context, username string) (*models.UserDetail, error)
	GetUserByID(ctx context.Context, id string) (*models.UserDetail, error)
	UpdateUserProfile(ctx context.Context, id string, displayName *string, avatarAssetID *string, clearAvatar bool) (*models.UserDetail, error)
	Follow(ctx context.Context, followerID, followeeID string) (bool, error)
	Unfollow(ctx context.Context, followerID, followeeID string) (bool, error)
	GetFollowers(ctx context.Context, followeeID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.UserDetail, error)
	GetFollowing(ctx context.Context, followerID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.UserDetail, error)
}

type UserService struct {
	repo                 UserRepository
	storagePublicBaseURL string
}

func NewUserService(repo UserRepository, storagePublicBaseURL string) *UserService {
	return &UserService{
		repo:                 repo,
		storagePublicBaseURL: storagePublicBaseURL,
	}
}

func (s *UserService) AssetURL(storageKey *string) *string {
	if storageKey == nil || *storageKey == "" {
		return nil
	}
	url := fmt.Sprintf("%s/%s", s.storagePublicBaseURL, *storageKey)
	return &url
}

func (s *UserService) GetProfile(ctx context.Context, username string) (*views.PublicUserResponse, error) {
	cleanUsername, err := ValidateGetProfile(username)
	if err != nil {
		return nil, err
	}

	u, err := s.repo.GetUserByUsername(ctx, cleanUsername)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	res := views.PublicUserResponse{
		ID:             u.ID,
		Username:       u.Username,
		DisplayName:    u.DisplayName,
		AvatarURL:      s.AssetURL(u.AvatarStorageKey),
		Role:           u.Role,
		FollowerCount:  u.FollowerCount,
		FollowingCount: u.FollowingCount,
		CreatedAt:      u.CreatedAt.Format(time.RFC3339),
	}
	return &res, nil
}

func (s *UserService) GetMe(ctx context.Context, userID string) (*views.PrivateUserResponse, error) {
	cleanUserID, err := ValidateGetMe(userID)
	if err != nil {
		return nil, err
	}

	u, err := s.repo.GetUserByID(ctx, cleanUserID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	res := views.PrivateUserResponse{
		ID:             u.ID,
		Username:       u.Username,
		Email:          u.Email,
		DisplayName:    u.DisplayName,
		Birthdate:      u.Birthdate.Format("2006-01-02"),
		AvatarURL:      s.AssetURL(u.AvatarStorageKey),
		Role:           u.Role,
		FollowerCount:  u.FollowerCount,
		FollowingCount: u.FollowingCount,
		CreatedAt:      u.CreatedAt.Format(time.RFC3339),
	}
	return &res, nil
}

func (s *UserService) UpdateMe(ctx context.Context, userID string, displayName *string, avatarAssetID *string, clearAvatar bool) (*views.PrivateUserResponse, error) {
	cleanUserID, cleanDisplayName, cleanAvatarAssetID, cleanClearAvatar, err := ValidateUpdateMe(userID, displayName, avatarAssetID, clearAvatar)
	if err != nil {
		return nil, err
	}

	u, err := s.repo.UpdateUserProfile(ctx, cleanUserID, cleanDisplayName, cleanAvatarAssetID, cleanClearAvatar)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: %s", ErrValidation, err.Error())
	}

	res := views.PrivateUserResponse{
		ID:             u.ID,
		Username:       u.Username,
		Email:          u.Email,
		DisplayName:    u.DisplayName,
		Birthdate:      u.Birthdate.Format("2006-01-02"),
		AvatarURL:      s.AssetURL(u.AvatarStorageKey),
		Role:           u.Role,
		FollowerCount:  u.FollowerCount,
		FollowingCount: u.FollowingCount,
		CreatedAt:      u.CreatedAt.Format(time.RFC3339),
	}
	return &res, nil
}

func (s *UserService) FollowUser(ctx context.Context, currentUserID, targetUserID string) (bool, error) {
	cleanUserID, cleanTargetID, err := ValidateFollowUser(currentUserID, targetUserID)
	if err != nil {
		return false, err
	}

	// Verify target user exists
	if _, err := s.repo.GetUserByID(ctx, cleanTargetID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return false, ErrNotFound
		}
		return false, err
	}

	_, err = s.repo.Follow(ctx, cleanUserID, cleanTargetID)
	if err != nil {
		return false, err
	}
	// PUT is idempotent and returns current state (following: true)
	return true, nil
}

func (s *UserService) UnfollowUser(ctx context.Context, currentUserID, targetUserID string) (bool, error) {
	cleanUserID, cleanTargetID, err := ValidateUnfollowUser(currentUserID, targetUserID)
	if err != nil {
		return false, err
	}

	// Verify target user exists
	if _, err := s.repo.GetUserByID(ctx, cleanTargetID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return false, ErrNotFound
		}
		return false, err
	}

	_, err = s.repo.Unfollow(ctx, cleanUserID, cleanTargetID)
	if err != nil {
		return false, err
	}
	// DELETE returns current state (following: false)
	return false, nil
}

func (s *UserService) GetFollowers(ctx context.Context, targetUserID string, limit int, cursorStr string) (*views.UserListResponse, error) {
	cleanTargetID, cleanLimit, cursorTime, cursorID, err := ValidateGetFollowers(targetUserID, limit, cursorStr)
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

	// Fetch limit + 1 to detect hasMore
	users, err := s.repo.GetFollowers(ctx, cleanTargetID, cleanLimit+1, cursorTime, cursorID)
	if err != nil {
		return nil, err
	}

	hasMore := len(users) > cleanLimit
	if hasMore {
		users = users[:cleanLimit]
	}

	var data []views.PublicUserResponse
	for _, u := range users {
		data = append(data, views.PublicUserResponse{
			ID:             u.ID,
			Username:       u.Username,
			DisplayName:    u.DisplayName,
			AvatarURL:      s.AssetURL(u.AvatarStorageKey),
			Role:           u.Role,
			FollowerCount:  u.FollowerCount,
			FollowingCount: u.FollowingCount,
			CreatedAt:      u.CreatedAt.Format(time.RFC3339),
		})
	}

	var nextCursor *string
	if hasMore && len(users) > 0 {
		last := users[len(users)-1]
		if last.EdgeCreatedAt != nil {
			c := views.EncodeCursor(*last.EdgeCreatedAt, last.ID)
			nextCursor = &c
		}
	}

	return &views.UserListResponse{
		Data: data,
		Pagination: views.PaginationMeta{
			NextCursor: nextCursor,
			Limit:      cleanLimit,
			HasMore:    hasMore,
		},
	}, nil
}

func (s *UserService) GetFollowing(ctx context.Context, targetUserID string, limit int, cursorStr string) (*views.UserListResponse, error) {
	cleanTargetID, cleanLimit, cursorTime, cursorID, err := ValidateGetFollowing(targetUserID, limit, cursorStr)
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

	users, err := s.repo.GetFollowing(ctx, cleanTargetID, cleanLimit+1, cursorTime, cursorID)
	if err != nil {
		return nil, err
	}

	hasMore := len(users) > cleanLimit
	if hasMore {
		users = users[:cleanLimit]
	}

	var data []views.PublicUserResponse
	for _, u := range users {
		data = append(data, views.PublicUserResponse{
			ID:             u.ID,
			Username:       u.Username,
			DisplayName:    u.DisplayName,
			AvatarURL:      s.AssetURL(u.AvatarStorageKey),
			Role:           u.Role,
			FollowerCount:  u.FollowerCount,
			FollowingCount: u.FollowingCount,
			CreatedAt:      u.CreatedAt.Format(time.RFC3339),
		})
	}

	var nextCursor *string
	if hasMore && len(users) > 0 {
		last := users[len(users)-1]
		if last.EdgeCreatedAt != nil {
			c := views.EncodeCursor(*last.EdgeCreatedAt, last.ID)
			nextCursor = &c
		}
	}

	return &views.UserListResponse{
		Data: data,
		Pagination: views.PaginationMeta{
			NextCursor: nextCursor,
			Limit:      cleanLimit,
			HasMore:    hasMore,
		},
	}, nil
}
