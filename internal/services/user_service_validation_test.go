package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
	"github.com/google/uuid"
)

type callCountingUserRepo struct {
	*FakeUserRepo
	getByIDCalls   int
	getByNameCalls int
	updateCalls    int
	followCalls    int
	unfollowCalls  int
	isFollowCalls  int
	followersCalls int
	followingCalls int
}

func newCallCountingUserRepo() *callCountingUserRepo {
	return &callCountingUserRepo{
		FakeUserRepo: newFakeUserRepo(),
	}
}

func (c *callCountingUserRepo) GetUserByID(ctx context.Context, id string) (*models.UserDetail, error) {
	c.getByIDCalls++
	return c.FakeUserRepo.GetUserByID(ctx, id)
}

func (c *callCountingUserRepo) GetUserByUsername(ctx context.Context, username string) (*models.UserDetail, error) {
	c.getByNameCalls++
	return c.FakeUserRepo.GetUserByUsername(ctx, username)
}

func (c *callCountingUserRepo) UpdateUserProfile(ctx context.Context, id string, displayName *string, avatarAssetID *string, clearAvatar bool) (*models.UserDetail, error) {
	c.updateCalls++
	return c.FakeUserRepo.UpdateUserProfile(ctx, id, displayName, avatarAssetID, clearAvatar)
}

func (c *callCountingUserRepo) Follow(ctx context.Context, followerID, followeeID string) (bool, error) {
	c.followCalls++
	return c.FakeUserRepo.Follow(ctx, followerID, followeeID)
}

func (c *callCountingUserRepo) Unfollow(ctx context.Context, followerID, followeeID string) (bool, error) {
	c.unfollowCalls++
	return c.FakeUserRepo.Unfollow(ctx, followerID, followeeID)
}

func (c *callCountingUserRepo) GetFollowers(ctx context.Context, followeeID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.UserDetail, error) {
	c.followersCalls++
	return c.FakeUserRepo.GetFollowers(ctx, followeeID, limit, cursorTime, cursorID)
}

func (c *callCountingUserRepo) GetFollowing(ctx context.Context, followerID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.UserDetail, error) {
	c.followingCalls++
	return c.FakeUserRepo.GetFollowing(ctx, followerID, limit, cursorTime, cursorID)
}

func TestUserService_ValidateGetProfileAndGetMe(t *testing.T) {
	t.Run("ValidateGetProfile", func(t *testing.T) {
		tests := []struct {
			name     string
			username string
			wantErr  bool
		}{
			{"Valid standard username", "alice", false},
			{"Valid username with digits and underscores", "bob_123_test", false},
			{"Valid 3 char minimum boundary", "abc", false},
			{"Valid 20 char maximum boundary", "abcdefghijklmnopqrst", false},
			{"Too short (2 chars)", "ab", true},
			{"Too long (21 chars)", "abcdefghijklmnopqrstu", true},
			{"Contains uppercase letters", "Alice", true},
			{"Contains hyphens", "alice-bob", true},
			{"Contains special symbols", "alice@world", true},
			{"Contains null byte", "ali\x00ce", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := services.ValidateGetProfile(tt.username)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateGetProfile() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("ValidateGetMe", func(t *testing.T) {
		validID := uuid.NewString()

		tests := []struct {
			name    string
			userID  string
			wantErr bool
		}{
			{"Valid UUID", validID, false},
			{"Malformed UUID", "not-a-uuid", true},
			{"Nil UUID", "00000000-0000-0000-0000-000000000000", true},
			{"Empty UUID", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := services.ValidateGetMe(tt.userID)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateGetMe() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestUserService_ValidateUpdateMe(t *testing.T) {
	validUser := uuid.NewString()
	validAsset := uuid.NewString()

	tests := []struct {
		name          string
		userID        string
		displayName   *string
		avatarAssetID *string
		clearAvatar   bool
		wantErr       bool
	}{
		{"Valid display name only", validUser, stringPtr("New Name"), nil, false, false},
		{"Valid avatar asset ID only", validUser, nil, &validAsset, false, false},
		{"Valid clear avatar only", validUser, nil, nil, true, false},
		{"Valid display name and avatar ID", validUser, stringPtr("New Name"), &validAsset, false, false},
		{"Valid Arabic display name", validUser, stringPtr("اسم عربي جديد"), nil, false, false},
		{"Valid emoji display name", validUser, stringPtr("Cool Guy 😎"), nil, false, false},
		{"No fields provided (no-op update)", validUser, nil, nil, false, true},
		{"Both avatarAssetID and clearAvatar set", validUser, nil, &validAsset, true, true},
		{"Malformed userID", "not-a-uuid", stringPtr("Name"), nil, false, true},
		{"Nil userID", "00000000-0000-0000-0000-000000000000", stringPtr("Name"), nil, false, true},
		{"Invalid empty display name", validUser, stringPtr(""), nil, false, true},
		{"Invalid whitespace display name", validUser, stringPtr("   \t  "), nil, false, true},
		{"Display name > 50 runes", validUser, stringPtr(strings.Repeat("D", 51)), nil, false, true},
		{"Display name with newline", validUser, stringPtr("User\nName"), nil, false, true},
		{"Display name with null byte", validUser, stringPtr("User\x00Name"), nil, false, true},
		{"Malformed avatarAssetID", validUser, nil, stringPtr("not-a-uuid"), false, true},
		{"Nil avatarAssetID", validUser, nil, stringPtr("00000000-0000-0000-0000-000000000000"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := services.ValidateUpdateMe(tt.userID, tt.displayName, tt.avatarAssetID, tt.clearAvatar)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUpdateMe() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserService_ValidateFollowAndUnfollow(t *testing.T) {
	actorID := uuid.NewString()
	targetID := uuid.NewString()

	t.Run("ValidateFollowUser", func(t *testing.T) {
		tests := []struct {
			name      string
			currentID string
			targetID  string
			wantErr   bool
			errType   error
		}{
			{"Valid follow distinct users", actorID, targetID, false, nil},
			{"Self-follow rejected", actorID, actorID, true, services.ErrSelfFollow},
			{"Malformed currentID", "not-a-uuid", targetID, true, services.ErrValidation},
			{"Nil currentID", "00000000-0000-0000-0000-000000000000", targetID, true, services.ErrValidation},
			{"Malformed targetID", actorID, "not-a-uuid", true, services.ErrValidation},
			{"Nil targetID", actorID, "00000000-0000-0000-0000-000000000000", true, services.ErrValidation},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, err := services.ValidateFollowUser(tt.currentID, tt.targetID)
				if (err != nil) != tt.wantErr {
					t.Fatalf("ValidateFollowUser() error = %v, wantErr %v", err, tt.wantErr)
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error %v, got %v", tt.errType, err)
				}
			})
		}
	})

	t.Run("ValidateUnfollowUser", func(t *testing.T) {
		tests := []struct {
			name      string
			currentID string
			targetID  string
			wantErr   bool
			errType   error
		}{
			{"Valid unfollow distinct users", actorID, targetID, false, nil},
			{"Self-unfollow rejected", actorID, actorID, true, services.ErrSelfFollow},
			{"Malformed currentID", "not-a-uuid", targetID, true, services.ErrValidation},
			{"Malformed targetID", actorID, "not-a-uuid", true, services.ErrValidation},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, err := services.ValidateUnfollowUser(tt.currentID, tt.targetID)
				if (err != nil) != tt.wantErr {
					t.Fatalf("ValidateUnfollowUser() error = %v, wantErr %v", err, tt.wantErr)
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error %v, got %v", tt.errType, err)
				}
			})
		}
	})
}

func TestUserService_ValidateFollowersAndFollowing(t *testing.T) {
	validTarget := uuid.NewString()

	tests := []struct {
		name    string
		target  string
		limit   int
		cursor  string
		wantErr bool
	}{
		{"Valid pagination parameters", validTarget, 20, "", false},
		{"Valid limit boundary 1", validTarget, 1, "", false},
		{"Valid limit boundary 100", validTarget, 100, "", false},
		{"Malformed target user ID", "not-a-uuid", 20, "", true},
		{"Limit < 1", validTarget, 0, "", true},
		{"Limit > 100", validTarget, 101, "", true},
		{"Invalid cursor format", validTarget, 20, "invalid-cursor-data", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err1 := services.ValidateGetFollowers(tt.target, tt.limit, tt.cursor)
			if (err1 != nil) != tt.wantErr {
				t.Errorf("ValidateGetFollowers() error = %v, wantErr %v", err1, tt.wantErr)
			}
			_, _, _, _, err2 := services.ValidateGetFollowing(tt.target, tt.limit, tt.cursor)
			if (err2 != nil) != tt.wantErr {
				t.Errorf("ValidateGetFollowing() error = %v, wantErr %v", err2, tt.wantErr)
			}
		})
	}
}

func TestUserService_FailFastZeroRepoCalls(t *testing.T) {
	repo := newCallCountingUserRepo()
	svc := services.NewUserService(repo, "https://storage.local")
	ctx := context.Background()

	// 1. GetProfile invalid username -> 0 repo calls and returns ErrNotFound
	_, err := svc.GetProfile(ctx, "in")
	if !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if repo.getByNameCalls != 0 {
		t.Errorf("expected 0 repo getByName calls, got %d", repo.getByNameCalls)
	}

	// 2. GetMe invalid userID -> 0 repo calls
	_, err = svc.GetMe(ctx, "invalid-user-uuid")
	if !errors.Is(err, services.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if repo.getByIDCalls != 0 {
		t.Errorf("expected 0 repo getByID calls, got %d", repo.getByIDCalls)
	}

	// 3. UpdateMe empty update -> 0 repo calls
	_, err = svc.UpdateMe(ctx, uuid.NewString(), nil, nil, false)
	if !errors.Is(err, services.ErrValidation) {
		t.Fatalf("expected ErrValidation on empty update, got %v", err)
	}
	if repo.updateCalls != 0 {
		t.Errorf("expected 0 repo update calls, got %d", repo.updateCalls)
	}

	// 4. FollowUser self-follow -> 0 repo calls
	selfID := uuid.NewString()
	_, err = svc.FollowUser(ctx, selfID, selfID)
	if !errors.Is(err, services.ErrSelfFollow) {
		t.Fatalf("expected ErrSelfFollow, got %v", err)
	}
	if repo.followCalls != 0 || repo.isFollowCalls != 0 {
		t.Errorf("expected 0 repo calls on self-follow, got follow=%d isFollow=%d", repo.followCalls, repo.isFollowCalls)
	}

	// 5. UnfollowUser self-unfollow -> 0 repo calls
	_, err = svc.UnfollowUser(ctx, selfID, selfID)
	if !errors.Is(err, services.ErrSelfFollow) {
		t.Fatalf("expected ErrSelfFollow, got %v", err)
	}
	if repo.unfollowCalls != 0 || repo.isFollowCalls != 0 {
		t.Errorf("expected 0 repo calls on self-unfollow, got unfollow=%d isFollow=%d", repo.unfollowCalls, repo.isFollowCalls)
	}

	// 6. GetFollowers invalid target -> 0 repo calls
	_, err = svc.GetFollowers(ctx, "invalid-uuid", 20, "")
	if !errors.Is(err, services.ErrValidation) {
		t.Fatalf("expected ErrValidation on invalid target in GetFollowers, got %v", err)
	}
	if repo.followersCalls != 0 {
		t.Errorf("expected 0 repo followers calls, got %d", repo.followersCalls)
	}

	// 7. GetFollowing invalid target -> 0 repo calls
	_, err = svc.GetFollowing(ctx, "invalid-uuid", 20, "")
	if !errors.Is(err, services.ErrValidation) {
		t.Fatalf("expected ErrValidation on invalid target in GetFollowing, got %v", err)
	}
	if repo.followingCalls != 0 {
		t.Errorf("expected 0 repo following calls, got %d", repo.followingCalls)
	}
}
