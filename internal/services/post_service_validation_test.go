package services_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
	"github.com/google/uuid"
)

type callCountingPostRepo struct {
	*FakePostRepo
	createCalls       int
	getByIDCalls      int
	updateCalls       int
	deleteCalls       int
	getFeedCalls      int
	getUserPostsCalls int
	likeCalls         int
	unlikeCalls       int
	shareCalls        int
}

func newCallCountingPostRepo() *callCountingPostRepo {
	return &callCountingPostRepo{
		FakePostRepo: newFakePostRepo(),
	}
}

func (c *callCountingPostRepo) CreatePost(ctx context.Context, authorID, body string, assetIDs []string) (*models.PostDetail, error) {
	c.createCalls++
	return c.FakePostRepo.CreatePost(ctx, authorID, body, assetIDs)
}

func (c *callCountingPostRepo) GetPostByID(ctx context.Context, id string, currentUserID *string) (*models.PostDetail, error) {
	c.getByIDCalls++
	return c.FakePostRepo.GetPostByID(ctx, id, currentUserID)
}

func (c *callCountingPostRepo) UpdatePost(ctx context.Context, id, body string) (*models.Post, error) {
	c.updateCalls++
	return c.FakePostRepo.UpdatePost(ctx, id, body)
}

func (c *callCountingPostRepo) SoftDeletePost(ctx context.Context, id string) error {
	c.deleteCalls++
	return c.FakePostRepo.SoftDeletePost(ctx, id)
}

func (c *callCountingPostRepo) GetFeed(ctx context.Context, userID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.PostDetail, error) {
	c.getFeedCalls++
	return c.FakePostRepo.GetFeed(ctx, userID, limit, cursorTime, cursorID)
}

func (c *callCountingPostRepo) GetUserPosts(ctx context.Context, targetUserID string, currentUserID *string, limit int, cursorTime *time.Time, cursorID string) ([]*models.PostDetail, error) {
	c.getUserPostsCalls++
	return c.FakePostRepo.GetUserPosts(ctx, targetUserID, currentUserID, limit, cursorTime, cursorID)
}

func (c *callCountingPostRepo) LikePost(ctx context.Context, userID, postID string) (bool, error) {
	c.likeCalls++
	return c.FakePostRepo.LikePost(ctx, userID, postID)
}

func (c *callCountingPostRepo) UnlikePost(ctx context.Context, userID, postID string) (bool, error) {
	c.unlikeCalls++
	return c.FakePostRepo.UnlikePost(ctx, userID, postID)
}

func (c *callCountingPostRepo) CreateShare(ctx context.Context, userID, postID string, quote *string) (*models.Share, error) {
	c.shareCalls++
	return c.FakePostRepo.CreateShare(ctx, userID, postID, quote)
}

func TestPostService_ValidateCreatePost(t *testing.T) {
	validAuthor := uuid.NewString()
	validAsset1 := uuid.NewString()
	validAsset2 := uuid.NewString()
	validAsset3 := uuid.NewString()
	validAsset4 := uuid.NewString()
	validAsset5 := uuid.NewString()

	tests := []struct {
		name     string
		authorID string
		body     string
		assets   []string
		wantErr  bool
	}{
		{"Valid text only post", validAuthor, "Hello world", nil, false},
		{"Valid image only post", validAuthor, "", []string{validAsset1}, false},
		{"Valid text with 1 image", validAuthor, "Check out this pic", []string{validAsset1}, false},
		{"Valid text with 4 images (boundary)", validAuthor, "Photo album", []string{validAsset1, validAsset2, validAsset3, validAsset4}, false},
		{"Valid Arabic text post", validAuthor, "منشور تجريبي باللغة العربية مع تفاصيل جميلة", nil, false},
		{"Valid emoji post", validAuthor, "Spill is amazing! 🎉🚀🔥💯", nil, false},
		{"Valid post body with formatting newlines and tabs", validAuthor, "Heading\n\n- item 1\n- item 2\tindented", nil, false},
		{"Boundary exact 2000 runes", validAuthor, strings.Repeat("A", 2000), nil, false},
		{"Boundary 2001 runes (exceeds max)", validAuthor, strings.Repeat("A", 2001), nil, true},
		{"Empty body and empty assets", validAuthor, "", nil, true},
		{"Whitespace body and empty assets", validAuthor, "   \t\n  ", nil, true},
		{"Body with null byte", validAuthor, "Invalid \x00 post", nil, true},
		{"Body with bell control character", validAuthor, "Invalid \x07 post", nil, true},
		{"Too many images (5 images)", validAuthor, "Too many", []string{validAsset1, validAsset2, validAsset3, validAsset4, validAsset5}, true},
		{"Duplicate asset IDs", validAuthor, "Duplicate image", []string{validAsset1, validAsset1}, true},
		{"Malformed author UUID", "not-a-uuid", "Valid body", nil, true},
		{"Nil author UUID", "00000000-0000-0000-0000-000000000000", "Valid body", nil, true},
		{"Malformed asset UUID", validAuthor, "Valid body", []string{"invalid-asset-uuid"}, true},
		{"Nil asset UUID", validAuthor, "Valid body", []string{"00000000-0000-0000-0000-000000000000"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := services.ValidateCreatePost(tt.authorID, tt.body, tt.assets)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreatePost() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPostService_ValidateGetPost(t *testing.T) {
	validPost := uuid.NewString()
	validUser := uuid.NewString()

	tests := []struct {
		name          string
		postID        string
		currentUserID *string
		wantErr       bool
	}{
		{"Valid post with nil user", validPost, nil, false},
		{"Valid post with valid user", validPost, &validUser, false},
		{"Malformed post ID", "invalid-id", nil, true},
		{"Nil post ID", "00000000-0000-0000-0000-000000000000", nil, true},
		{"Malformed user ID", validPost, stringPtr("invalid-user"), true},
		{"Nil user ID", validPost, stringPtr("00000000-0000-0000-0000-000000000000"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := services.ValidateGetPost(tt.postID, tt.currentUserID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGetPost() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPostService_ValidateUpdatePost(t *testing.T) {
	validPost := uuid.NewString()
	validUser := uuid.NewString()

	tests := []struct {
		name    string
		postID  string
		userID  string
		role    string
		body    string
		wantErr bool
	}{
		{"Valid update as user", validPost, validUser, "user", "Updated body content", false},
		{"Valid update as admin", validPost, validUser, "admin", "Updated body content", false},
		{"Valid update with Arabic", validPost, validUser, "user", "تحديث النص باللغة العربية", false},
		{"Valid update exact 2000 runes", validPost, validUser, "user", strings.Repeat("B", 2000), false},
		{"Empty updated body", validPost, validUser, "user", "", true},
		{"Whitespace updated body", validPost, validUser, "user", "    \n\t  ", true},
		{"Updated body > 2000 runes", validPost, validUser, "user", strings.Repeat("B", 2001), true},
		{"Updated body with null byte", validPost, validUser, "user", "null\x00byte", true},
		{"Malformed post ID", "not-a-uuid", validUser, "user", "Valid body", true},
		{"Malformed user ID", validPost, "not-a-uuid", "user", "Valid body", true},
		{"Invalid role", validPost, validUser, "guest", "Valid body", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := services.ValidateUpdatePost(tt.postID, tt.userID, tt.role, tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUpdatePost() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPostService_ValidateDeletePost(t *testing.T) {
	validPost := uuid.NewString()
	validUser := uuid.NewString()

	tests := []struct {
		name    string
		postID  string
		userID  string
		role    string
		wantErr bool
	}{
		{"Valid user role", validPost, validUser, "user", false},
		{"Valid admin role", validPost, validUser, "admin", false},
		{"Malformed post ID", "invalid-id", validUser, "user", true},
		{"Nil post ID", "00000000-0000-0000-0000-000000000000", validUser, "user", true},
		{"Malformed user ID", validPost, "invalid-user", "user", true},
		{"Nil user ID", validPost, "00000000-0000-0000-0000-000000000000", "user", true},
		{"Invalid role", validPost, validUser, "guest", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := services.ValidateDeletePost(tt.postID, tt.userID, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDeletePost() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPostService_ValidateFeedAndUserPosts(t *testing.T) {
	validUser := uuid.NewString()
	validTarget := uuid.NewString()

	t.Run("ValidateGetFeed", func(t *testing.T) {
		tests := []struct {
			name    string
			userID  string
			limit   int
			cursor  string
			wantErr bool
		}{
			{"Valid feed request", validUser, 20, "", false},
			{"Valid feed limit boundary 1", validUser, 1, "", false},
			{"Valid feed limit boundary 100", validUser, 100, "", false},
			{"Malformed user ID", "invalid-id", 20, "", true},
			{"Limit < 1", validUser, 0, "", true},
			{"Limit > 100", validUser, 101, "", true},
			{"Malformed cursor", validUser, 20, "invalid-cursor-val", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, _, _, err := services.ValidateGetFeed(tt.userID, tt.limit, tt.cursor)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateGetFeed() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("ValidateGetUserPosts", func(t *testing.T) {
		tests := []struct {
			name          string
			targetUserID  string
			currentUserID *string
			limit         int
			cursor        string
			wantErr       bool
		}{
			{"Valid target user with nil current user", validTarget, nil, 20, "", false},
			{"Valid target user with current user", validTarget, &validUser, 20, "", false},
			{"Malformed target user ID", "invalid-target", nil, 20, "", true},
			{"Malformed current user ID", validTarget, stringPtr("invalid-current"), 20, "", true},
			{"Invalid limit", validTarget, nil, 0, "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, _, _, _, err := services.ValidateGetUserPosts(tt.targetUserID, tt.currentUserID, tt.limit, tt.cursor)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateGetUserPosts() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestPostService_ValidateLikeUnlikeShare(t *testing.T) {
	validUser := uuid.NewString()
	validPost := uuid.NewString()

	t.Run("ValidateLikePost and ValidateUnlikePost", func(t *testing.T) {
		tests := []struct {
			name    string
			userID  string
			postID  string
			wantErr bool
		}{
			{"Valid IDs", validUser, validPost, false},
			{"Malformed user ID", "not-a-uuid", validPost, true},
			{"Nil user ID", "00000000-0000-0000-0000-000000000000", validPost, true},
			{"Malformed post ID", validUser, "not-a-uuid", true},
			{"Nil post ID", validUser, "00000000-0000-0000-0000-000000000000", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, err1 := services.ValidateLikePost(tt.userID, tt.postID)
				if (err1 != nil) != tt.wantErr {
					t.Errorf("ValidateLikePost() error = %v, wantErr %v", err1, tt.wantErr)
				}
				_, _, err2 := services.ValidateUnlikePost(tt.userID, tt.postID)
				if (err2 != nil) != tt.wantErr {
					t.Errorf("ValidateUnlikePost() error = %v, wantErr %v", err2, tt.wantErr)
				}
			})
		}
	})

	t.Run("ValidateSharePost", func(t *testing.T) {
		tests := []struct {
			name    string
			userID  string
			postID  string
			quote   *string
			wantErr bool
		}{
			{"Valid share without quote", validUser, validPost, nil, false},
			{"Valid share with normal quote", validUser, validPost, stringPtr("Must read!"), false},
			{"Valid share with Arabic quote", validUser, validPost, stringPtr("مقال ومنشور مميز جدا"), false},
			{"Valid share exact 500 runes", validUser, validPost, stringPtr(strings.Repeat("Q", 500)), false},
			{"Quote exceeds 500 runes", validUser, validPost, stringPtr(strings.Repeat("Q", 501)), true},
			{"Quote with null byte", validUser, validPost, stringPtr("quote\x00null"), true},
			{"Malformed user ID", "not-a-uuid", validPost, nil, true},
			{"Malformed post ID", validUser, "not-a-uuid", nil, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, _, err := services.ValidateSharePost(tt.userID, tt.postID, tt.quote)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateSharePost() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestPostService_FailFastZeroRepoCalls(t *testing.T) {
	repo := newCallCountingPostRepo()
	svc := services.NewPostService(repo, "https://storage.local")
	ctx := context.Background()

	// 1. CreatePost invalid -> 0 repo calls
	_, err := svc.CreatePost(ctx, "invalid-author-uuid", "Hello", nil)
	if err == nil {
		t.Fatal("expected error on invalid author UUID")
	}
	if repo.createCalls != 0 {
		t.Errorf("expected 0 repo create calls, got %d", repo.createCalls)
	}

	// 2. GetPost invalid -> 0 repo calls
	_, err = svc.GetPost(ctx, "invalid-post-uuid", nil)
	if err == nil {
		t.Fatal("expected error on invalid post UUID")
	}
	if repo.getByIDCalls != 0 {
		t.Errorf("expected 0 repo get calls, got %d", repo.getByIDCalls)
	}

	// 3. UpdatePost invalid -> 0 repo calls
	_, err = svc.UpdatePost(ctx, "invalid-post-uuid", uuid.NewString(), "user", "updated body")
	if err == nil {
		t.Fatal("expected error on invalid post UUID in update")
	}
	if repo.getByIDCalls != 0 || repo.updateCalls != 0 {
		t.Errorf("expected 0 repo calls on invalid update, got get=%d update=%d", repo.getByIDCalls, repo.updateCalls)
	}

	// 4. DeletePost invalid -> 0 repo calls
	err = svc.DeletePost(ctx, "invalid-post-uuid", uuid.NewString(), "user")
	if err == nil {
		t.Fatal("expected error on invalid post UUID in delete")
	}
	if repo.getByIDCalls != 0 || repo.deleteCalls != 0 {
		t.Errorf("expected 0 repo calls on invalid delete, got get=%d delete=%d", repo.getByIDCalls, repo.deleteCalls)
	}

	// 5. GetFeed invalid -> 0 repo calls
	_, err = svc.GetFeed(ctx, "invalid-user-uuid", 20, "")
	if err == nil {
		t.Fatal("expected error on invalid user UUID in feed")
	}
	if repo.getFeedCalls != 0 {
		t.Errorf("expected 0 repo getFeed calls, got %d", repo.getFeedCalls)
	}

	// 6. GetUserPosts invalid -> 0 repo calls
	_, err = svc.GetUserPosts(ctx, "invalid-target-uuid", nil, 20, "")
	if err == nil {
		t.Fatal("expected error on invalid target UUID in user posts")
	}
	if repo.getUserPostsCalls != 0 {
		t.Errorf("expected 0 repo getUserPosts calls, got %d", repo.getUserPostsCalls)
	}

	// 7. LikePost invalid -> 0 repo calls
	_, err = svc.LikePost(ctx, "invalid-user-uuid", uuid.NewString())
	if err == nil {
		t.Fatal("expected error on invalid user UUID in like")
	}
	if repo.likeCalls != 0 {
		t.Errorf("expected 0 repo like calls, got %d", repo.likeCalls)
	}

	// 8. UnlikePost invalid -> 0 repo calls
	_, err = svc.UnlikePost(ctx, "invalid-user-uuid", uuid.NewString())
	if err == nil {
		t.Fatal("expected error on invalid user UUID in unlike")
	}
	if repo.unlikeCalls != 0 {
		t.Errorf("expected 0 repo unlike calls, got %d", repo.unlikeCalls)
	}

	// 9. SharePost invalid -> 0 repo calls
	_, err = svc.SharePost(ctx, "invalid-user-uuid", uuid.NewString(), nil)
	if err == nil {
		t.Fatal("expected error on invalid user UUID in share")
	}
	if repo.getByIDCalls != 0 || repo.shareCalls != 0 {
		t.Errorf("expected 0 repo share calls, got %d", repo.shareCalls)
	}
}

func stringPtr(s string) *string {
	return &s
}
