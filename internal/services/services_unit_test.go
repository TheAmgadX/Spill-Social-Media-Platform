package services_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/auth"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/config"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/mailer"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/storage"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Fakes & Mocks
// ---------------------------------------------------------------------------

type FakeAuthRepo struct {
	mu           sync.Mutex
	codes        map[string]*models.LoginCode
	users        map[string]*models.User // by ID
	usersByEmail map[string]*models.User
	usersByName  map[string]*models.User
	tokens       map[string]*models.RefreshToken // by token_hash
	revokedUsers map[string]bool
	lastCreated  time.Time
}

func newFakeAuthRepo() *FakeAuthRepo {
	return &FakeAuthRepo{
		codes:        make(map[string]*models.LoginCode),
		users:        make(map[string]*models.User),
		usersByEmail: make(map[string]*models.User),
		usersByName:  make(map[string]*models.User),
		tokens:       make(map[string]*models.RefreshToken),
		revokedUsers: make(map[string]bool),
	}
}

func (f *FakeAuthRepo) CheckLoginCodeCooldown(ctx context.Context, email string, cooldown time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.codes {
		if strings.EqualFold(c.Email, email) && c.ConsumedAt == nil {
			if time.Since(c.CreatedAt) < cooldown {
				return true, nil
			}
		}
	}
	return false, nil
}

func (f *FakeAuthRepo) CreateLoginCode(ctx context.Context, email, codeHash string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := uuid.NewString()
	f.codes[id] = &models.LoginCode{
		ID:        id,
		Email:     email,
		CodeHash:  codeHash,
		Attempts:  0,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	f.lastCreated = time.Now()
	return nil
}

func (f *FakeAuthRepo) GetLatestActiveCode(ctx context.Context, email string) (*models.LoginCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest *models.LoginCode
	for _, c := range f.codes {
		if strings.EqualFold(c.Email, email) && c.ConsumedAt == nil && c.ExpiresAt.After(time.Now()) {
			if latest == nil || c.CreatedAt.After(latest.CreatedAt) {
				latest = c
			}
		}
	}
	if latest == nil {
		return nil, models.ErrNotFound
	}
	return latest, nil
}

func (f *FakeAuthRepo) IncrementAttempts(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.codes[id]; ok {
		c.Attempts++
		return nil
	}
	return models.ErrNotFound
}

func (f *FakeAuthRepo) MarkCodeConsumed(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.codes[id]; ok {
		now := time.Now()
		c.ConsumedAt = &now
		return nil
	}
	return models.ErrNotFound
}

func (f *FakeAuthRepo) FindUserByEmail(ctx context.Context, email string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.usersByEmail[strings.ToLower(email)]; ok {
		return u, nil
	}
	return nil, models.ErrNotFound
}

func (f *FakeAuthRepo) FindUserByID(ctx context.Context, id string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, models.ErrNotFound
}

func (f *FakeAuthRepo) IsUsernameTaken(ctx context.Context, username string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.usersByName[strings.ToLower(username)]
	return ok, nil
}

func (f *FakeAuthRepo) CreateUser(ctx context.Context, username, email, displayName string, birthdate time.Time, role string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.usersByName[strings.ToLower(username)]; ok {
		return nil, models.ErrUsernameTaken
	}
	if _, ok := f.usersByEmail[strings.ToLower(email)]; ok {
		return nil, models.ErrEmailRegistered
	}
	u := &models.User{
		ID:          uuid.NewString(),
		Username:    username,
		Email:       email,
		DisplayName: displayName,
		Birthdate:   birthdate,
		Role:        role,
		CreatedAt:   time.Now(),
	}
	f.users[u.ID] = u
	f.usersByEmail[strings.ToLower(email)] = u
	f.usersByName[strings.ToLower(username)] = u
	return u, nil
}

func (f *FakeAuthRepo) CreateRefreshToken(ctx context.Context, userID, tokenHash string, userAgent *string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[tokenHash] = &models.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: tokenHash,
		UserAgent: userAgent,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	return nil
}

func (f *FakeAuthRepo) FindRefreshTokenByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tokens[tokenHash]; ok {
		return t, nil
	}
	return nil, models.ErrNotFound
}

func (f *FakeAuthRepo) RotateRefreshToken(ctx context.Context, oldTokenID, userID, newTokenHash string, userAgent *string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tokens {
		if t.ID == oldTokenID {
			now := time.Now()
			t.RevokedAt = &now
			break
		}
	}
	f.tokens[newTokenHash] = &models.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: newTokenHash,
		UserAgent: userAgent,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	return nil
}

func (f *FakeAuthRepo) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revokedUsers[userID] = true
	now := time.Now()
	for _, t := range f.tokens {
		if t.UserID == userID {
			t.RevokedAt = &now
		}
	}
	return nil
}

func (f *FakeAuthRepo) RevokeRefreshTokenByHash(ctx context.Context, tokenHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tokens[tokenHash]; ok {
		now := time.Now()
		t.RevokedAt = &now
		return nil
	}
	return nil
}

type FakeUserRepo struct {
	mu        sync.Mutex
	users     map[string]*models.UserDetail
	followers map[string]map[string]time.Time // followeeID -> followerID -> createdAt
}

func newFakeUserRepo() *FakeUserRepo {
	return &FakeUserRepo{
		users:     make(map[string]*models.UserDetail),
		followers: make(map[string]map[string]time.Time),
	}
}

func (f *FakeUserRepo) GetUserByUsername(ctx context.Context, username string) (*models.UserDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if strings.EqualFold(u.Username, username) {
			return u, nil
		}
	}
	return nil, models.ErrNotFound
}

func (f *FakeUserRepo) GetUserByID(ctx context.Context, id string) (*models.UserDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, models.ErrNotFound
}

func (f *FakeUserRepo) UpdateUserProfile(ctx context.Context, id string, displayName *string, avatarAssetID *string, clearAvatar bool) (*models.UserDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	if displayName != nil {
		u.DisplayName = *displayName
	}
	if clearAvatar {
		u.AvatarAssetID = nil
		u.AvatarStorageKey = nil
	} else if avatarAssetID != nil {
		u.AvatarAssetID = avatarAssetID
		k := "u/" + id + "/" + *avatarAssetID + ".png"
		u.AvatarStorageKey = &k
	}
	return u, nil
}

func (f *FakeUserRepo) Follow(ctx context.Context, followerID, followeeID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.followers[followeeID] == nil {
		f.followers[followeeID] = make(map[string]time.Time)
	}
	if _, ok := f.followers[followeeID][followerID]; ok {
		return false, nil
	}
	f.followers[followeeID][followerID] = time.Now()
	if u, ok := f.users[followeeID]; ok {
		u.FollowerCount++
	}
	if u, ok := f.users[followerID]; ok {
		u.FollowingCount++
	}
	return true, nil
}

func (f *FakeUserRepo) Unfollow(ctx context.Context, followerID, followeeID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.followers[followeeID] != nil {
		if _, ok := f.followers[followeeID][followerID]; ok {
			delete(f.followers[followeeID], followerID)
			if u, ok := f.users[followeeID]; ok {
				u.FollowerCount = max(0, u.FollowerCount-1)
			}
			if u, ok := f.users[followerID]; ok {
				u.FollowingCount = max(0, u.FollowingCount-1)
			}
			return true, nil
		}
	}
	return false, nil
}

func (f *FakeUserRepo) GetFollowers(ctx context.Context, followeeID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.UserDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var res []*models.UserDetail
	if m, ok := f.followers[followeeID]; ok {
		for fID, t := range m {
			if u, ok := f.users[fID]; ok {
				uCopy := *u
				uCopy.EdgeCreatedAt = &t
				res = append(res, &uCopy)
				if len(res) >= limit {
					break
				}
			}
		}
	}
	return res, nil
}

func (f *FakeUserRepo) GetFollowing(ctx context.Context, followerID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.UserDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var res []*models.UserDetail
	for fID, m := range f.followers {
		if t, ok := m[followerID]; ok {
			if u, ok := f.users[fID]; ok {
				uCopy := *u
				uCopy.EdgeCreatedAt = &t
				res = append(res, &uCopy)
				if len(res) >= limit {
					break
				}
			}
		}
	}
	return res, nil
}

type FakePostRepo struct {
	mu     sync.Mutex
	posts  map[string]*models.PostDetail
	users  map[string]*models.UserDetail
	likes  map[string]map[string]bool // postID -> userID -> true
	shares map[string]*models.Share
}

func newFakePostRepo() *FakePostRepo {
	return &FakePostRepo{
		posts:  make(map[string]*models.PostDetail),
		users:  make(map[string]*models.UserDetail),
		likes:  make(map[string]map[string]bool),
		shares: make(map[string]*models.Share),
	}
}

func (f *FakePostRepo) GetUserByID(ctx context.Context, id string) (*models.UserDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, models.ErrNotFound
}

func (f *FakePostRepo) CreatePost(ctx context.Context, authorID, body string, assetIDs []string) (*models.PostDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := uuid.NewString()
	p := &models.PostDetail{
		Post: models.Post{
			ID:        id,
			AuthorID:  authorID,
			Body:      body,
			CreatedAt: time.Now(),
		},
		Assets: []models.PostAssetInfo{},
	}
	for i, aid := range assetIDs {
		p.Assets = append(p.Assets, models.PostAssetInfo{
			ID:         aid,
			StorageKey: "u/" + authorID + "/" + aid + ".png",
			MimeType:   "image/png",
			SizeBytes:  1024,
			Status:     "ready",
			Position:   i,
		})
	}
	f.posts[id] = p
	return p, nil
}

func (f *FakePostRepo) GetPostByID(ctx context.Context, id string, currentUserID *string) (*models.PostDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.posts[id]
	if !ok || p.DeletedAt != nil {
		return nil, models.ErrNotFound
	}
	pCopy := *p
	if currentUserID != nil && f.likes[id] != nil && f.likes[id][*currentUserID] {
		liked := true
		pCopy.Liked = &liked
	}
	return &pCopy, nil
}

func (f *FakePostRepo) UpdatePost(ctx context.Context, id, body string) (*models.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.posts[id]
	if !ok || p.DeletedAt != nil {
		return nil, models.ErrNotFound
	}
	p.Body = body
	now := time.Now()
	p.EditedAt = &now
	return &p.Post, nil
}

func (f *FakePostRepo) SoftDeletePost(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.posts[id]
	if !ok || p.DeletedAt != nil {
		return models.ErrNotFound
	}
	now := time.Now()
	p.DeletedAt = &now
	return nil
}

func (f *FakePostRepo) GetFeed(ctx context.Context, userID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.PostDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var res []*models.PostDetail
	for _, p := range f.posts {
		if p.DeletedAt == nil {
			res = append(res, p)
			if len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func (f *FakePostRepo) GetUserPosts(ctx context.Context, targetUserID string, currentUserID *string, limit int, cursorTime *time.Time, cursorID string) ([]*models.PostDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var res []*models.PostDetail
	for _, p := range f.posts {
		if p.AuthorID == targetUserID && p.DeletedAt == nil {
			res = append(res, p)
			if len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func (f *FakePostRepo) LikePost(ctx context.Context, userID, postID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.likes[postID] == nil {
		f.likes[postID] = make(map[string]bool)
	}
	if f.likes[postID][userID] {
		return false, nil
	}
	f.likes[postID][userID] = true
	if p, ok := f.posts[postID]; ok {
		p.LikeCount++
	}
	return true, nil
}

func (f *FakePostRepo) UnlikePost(ctx context.Context, userID, postID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.likes[postID] != nil && f.likes[postID][userID] {
		delete(f.likes[postID], userID)
		if p, ok := f.posts[postID]; ok {
			p.LikeCount = max(0, p.LikeCount-1)
		}
		return true, nil
	}
	return false, nil
}

func (f *FakePostRepo) CreateShare(ctx context.Context, userID, postID string, quote *string) (*models.Share, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := &models.Share{
		ID:        uuid.NewString(),
		UserID:    userID,
		PostID:    postID,
		Quote:     quote,
		CreatedAt: time.Now(),
	}
	f.shares[s.ID] = s
	if p, ok := f.posts[postID]; ok {
		p.ShareCount++
	}
	return s, nil
}

type FakeCommentRepo struct {
	mu       sync.Mutex
	comments map[string]*models.CommentDetail
	posts    map[string]*models.PostDetail
}

func newFakeCommentRepo() *FakeCommentRepo {
	return &FakeCommentRepo{
		comments: make(map[string]*models.CommentDetail),
		posts:    make(map[string]*models.PostDetail),
	}
}

func (f *FakeCommentRepo) GetPostByID(ctx context.Context, id string, currentUserID *string) (*models.PostDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.posts[id]; ok && p.DeletedAt == nil {
		return p, nil
	}
	return nil, models.ErrNotFound
}

func (f *FakeCommentRepo) CreateComment(ctx context.Context, postID, authorID, body string) (*models.CommentDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := &models.CommentDetail{
		Comment: models.Comment{
			ID:        uuid.NewString(),
			PostID:    postID,
			AuthorID:  authorID,
			Body:      body,
			CreatedAt: time.Now(),
		},
	}
	f.comments[c.ID] = c
	if p, ok := f.posts[postID]; ok {
		p.CommentCount++
	}
	return c, nil
}

func (f *FakeCommentRepo) GetCommentByID(ctx context.Context, id string) (*models.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.comments[id]; ok {
		return &c.Comment, nil
	}
	return nil, models.ErrNotFound
}

func (f *FakeCommentRepo) GetCommentsByPostID(ctx context.Context, postID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.CommentDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var res []*models.CommentDetail
	for _, c := range f.comments {
		if c.PostID == postID && c.DeletedAt == nil {
			res = append(res, c)
			if len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func (f *FakeCommentRepo) SoftDeleteComment(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.comments[id]; ok && c.DeletedAt == nil {
		now := time.Now()
		c.DeletedAt = &now
		if p, ok := f.posts[c.PostID]; ok {
			p.CommentCount = max(0, p.CommentCount-1)
		}
		return nil
	}
	return models.ErrNotFound
}

type FakeAssetRepo struct {
	mu     sync.Mutex
	assets map[string]*models.Asset
}

func newFakeAssetRepo() *FakeAssetRepo {
	return &FakeAssetRepo{assets: make(map[string]*models.Asset)}
}

func (f *FakeAssetRepo) CreateAsset(ctx context.Context, id, ownerID, kind, bucket, storageKey, mimeType string, sizeBytes int64) (*models.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := &models.Asset{
		ID:              id,
		OwnerID:         ownerID,
		Kind:            kind,
		StorageBucket:   bucket,
		StorageKey:      storageKey,
		MimeType:        mimeType,
		SizeBytes:       sizeBytes,
		Status:          "pending",
		UploadExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt:       time.Now(),
	}
	f.assets[id] = a
	return a, nil
}

func (f *FakeAssetRepo) GetAssetByID(ctx context.Context, id string) (*models.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.assets[id]; ok {
		return a, nil
	}
	return nil, models.ErrNotFound
}

func (f *FakeAssetRepo) CompleteAsset(ctx context.Context, id string) (*models.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.assets[id]; ok {
		a.Status = "ready"
		now := time.Now()
		a.CompletedAt = &now
		return a, nil
	}
	return nil, models.ErrNotFound
}

func (f *FakeAssetRepo) GetExpiredPendingAssets(ctx context.Context) ([]*models.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var res []*models.Asset
	for _, a := range f.assets {
		if a.Status == "pending" && a.UploadExpiresAt.Before(time.Now()) {
			res = append(res, a)
		}
	}
	return res, nil
}

func (f *FakeAssetRepo) DeleteAssets(ctx context.Context, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		delete(f.assets, id)
	}
	return nil
}

type FakeSearchRepo struct {
	users    []*models.UserDetail
	posts    []*models.PostDetail
	comments []*models.CommentDetail
}

func (f *FakeSearchRepo) SearchUsers(ctx context.Context, q string, limit, offset int) ([]*models.UserDetail, error) {
	return f.users, nil
}

func (f *FakeSearchRepo) SearchPosts(ctx context.Context, q string, currentUserID *string, limit, offset int) ([]*models.PostDetail, error) {
	return f.posts, nil
}

func (f *FakeSearchRepo) SearchComments(ctx context.Context, q string, limit, offset int) ([]*models.CommentDetail, error) {
	return f.comments, nil
}

type MockStorageClient struct {
	mu      sync.Mutex
	objects map[string]int64
}

func newMockStorageClient() *MockStorageClient {
	return &MockStorageClient{objects: make(map[string]int64)}
}

func (m *MockStorageClient) CreateSignedUploadURL(ctx context.Context, bucket, objectKey string) (string, error) {
	return "https://storage.local/upload/" + bucket + "/" + objectKey, nil
}

func (m *MockStorageClient) VerifyObject(ctx context.Context, bucket, objectKey string, expectedSize int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	size, ok := m.objects[objectKey]
	if !ok {
		return storage.ErrObjectNotFound
	}
	if size != expectedSize {
		return storage.ErrSizeMismatch
	}
	return nil
}

func (m *MockStorageClient) DeleteObjects(ctx context.Context, bucket string, objectKeys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range objectKeys {
		delete(m.objects, k)
	}
	return nil
}

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:         []byte("12345678901234567890123456789012"),
		JWTIssuer:         "test-api",
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   720 * time.Hour,
		SignupTokenTTL:    10 * time.Minute,
		OTPHMACKey:        []byte("12345678901234567890123456789012"),
		OTPTTL:            10 * time.Minute,
		OTPResendCooldown: 60 * time.Second,
		OTPMaxAttempts:    5,
	}
}

// ---------------------------------------------------------------------------
// 1. Ownership Rules Tests
// ---------------------------------------------------------------------------

func TestService_OwnershipRules(t *testing.T) {
	ctx := context.Background()

	t.Run("PostService_UpdatePost_Ownership", func(t *testing.T) {
		repo := newFakePostRepo()
		svc := services.NewPostService(repo, "https://storage.local")

		authorID := uuid.NewString()
		adminID := uuid.NewString()
		intruderID := uuid.NewString()

		post, _ := repo.CreatePost(ctx, authorID, "Initial body", nil)

		tests := []struct {
			name        string
			userID      string
			role        string
			expectError error
		}{
			{"Author can edit", authorID, "user", nil},
			{"Admin can edit", adminID, "admin", nil},
			{"Non-author non-admin cannot edit", intruderID, "user", services.ErrForbidden},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := svc.UpdatePost(ctx, post.ID, tt.userID, tt.role, "Updated body")
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			})
		}
	})

	t.Run("PostService_DeletePost_Ownership", func(t *testing.T) {
		authorID := uuid.NewString()
		adminID := uuid.NewString()
		intruderID := uuid.NewString()

		tests := []struct {
			name        string
			userID      string
			role        string
			expectError error
		}{
			{"Author can delete", authorID, "user", nil},
			{"Admin can delete", adminID, "admin", nil},
			{"Non-author cannot delete", intruderID, "user", services.ErrForbidden},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repo := newFakePostRepo()
				svc := services.NewPostService(repo, "https://storage.local")
				post, _ := repo.CreatePost(ctx, authorID, "Post to delete", nil)

				err := svc.DeletePost(ctx, post.ID, tt.userID, tt.role)
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			})
		}
	})

	t.Run("CommentService_DeleteComment_Ownership", func(t *testing.T) {
		postID := uuid.NewString()
		commenterID := uuid.NewString()
		adminID := uuid.NewString()
		otherUserID := uuid.NewString()

		tests := []struct {
			name        string
			userID      string
			role        string
			expectError error
		}{
			{"Author can delete comment", commenterID, "user", nil},
			{"Admin can delete comment", adminID, "admin", nil},
			{"Non-author cannot delete comment", otherUserID, "user", services.ErrForbidden},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repo := newFakeCommentRepo()
				svc := services.NewCommentService(repo, "https://storage.local")
				comm, _ := repo.CreateComment(ctx, postID, commenterID, "Nice post")

				err := svc.DeleteComment(ctx, comm.ID, tt.userID, tt.role)
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			})
		}
	})

	t.Run("AssetService_CompleteAsset_Ownership", func(t *testing.T) {
		assetID := uuid.NewString()
		ownerID := uuid.NewString()
		adminID := uuid.NewString()
		intruderID := uuid.NewString()

		tests := []struct {
			name        string
			userID      string
			role        string
			expectError error
		}{
			{"Owner can complete asset", ownerID, "user", nil},
			{"Admin can complete asset", adminID, "admin", nil},
			{"Non-owner non-admin cannot complete asset", intruderID, "user", services.ErrForbidden},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repo := newFakeAssetRepo()
				store := newMockStorageClient()
				svc := services.NewAssetService(repo, store, "media", "https://storage.local")

				asset, _ := repo.CreateAsset(ctx, assetID, ownerID, "post_image", "media", fmt.Sprintf("u/%s/%s.png", ownerID, assetID), "image/png", 1024)
				store.objects[asset.StorageKey] = 1024

				_, err := svc.CompleteAsset(ctx, asset.ID, tt.userID, tt.role)
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			})
		}
	})
}

// ---------------------------------------------------------------------------
// 2. OTP Attempt Limits Tests
// ---------------------------------------------------------------------------

func TestAuthService_OTPAttemptLimits(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	tests := []struct {
		name          string
		attemptsPrior int
		expectError   error
	}{
		{"Attempt 1 with wrong code fails with ErrInvalidCredentials", 0, services.ErrInvalidCredentials},
		{"Attempt 3 with wrong code fails with ErrInvalidCredentials", 2, services.ErrInvalidCredentials},
		{"Attempt 4 with wrong code fails with ErrInvalidCredentials", 3, services.ErrInvalidCredentials},
		{"Attempt 5 (reaching limit) fails with ErrInvalidCredentials", 4, services.ErrInvalidCredentials},
		{"Attempt 6 (exceeding limit) fails with ErrTooManyAttempts", 5, services.ErrTooManyAttempts},
		{"Attempt 7 (already blocked) fails with ErrTooManyAttempts", 6, services.ErrTooManyAttempts},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeAuthRepo()
			mockMail := mailer.NewMockMailer()
			svc := services.NewAuthService(repo, mockMail, cfg)

			email := "user@example.com"
			codeHash := auth.HashOTP("123456", cfg.OTPHMACKey)
			_ = repo.CreateLoginCode(ctx, email, codeHash, time.Now().Add(10*time.Minute))
			activeCode, _ := repo.GetLatestActiveCode(ctx, email)
			activeCode.Attempts = int16(tt.attemptsPrior)

			_, err := svc.VerifyCode(ctx, email, "999999", "agent")
			if !errors.Is(err, tt.expectError) {
				t.Errorf("for %d prior attempts: expected %v, got %v", tt.attemptsPrior, tt.expectError, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Request Cooldown Tests
// ---------------------------------------------------------------------------

func TestAuthService_RequestCooldown(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	tests := []struct {
		name               string
		hasRecentCode      bool
		expectedEmailsSent int
	}{
		{
			name:               "First request sends email",
			hasRecentCode:      false,
			expectedEmailsSent: 1,
		},
		{
			name:               "Second request within cooldown is dropped silently",
			hasRecentCode:      true,
			expectedEmailsSent: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeAuthRepo()
			mockMail := mailer.NewMockMailer()
			svc := services.NewAuthService(repo, mockMail, cfg)

			email := "cool@example.com"
			if tt.hasRecentCode {
				_ = repo.CreateLoginCode(ctx, email, "somehash", time.Now().Add(10*time.Minute))
			}

			err := svc.RequestCode(ctx, email)
			if err != nil {
				t.Fatalf("RequestCode returned error: %v", err)
			}

			if len(mockMail.SentMessages) != tt.expectedEmailsSent {
				t.Errorf("expected %d sent messages, got %d", tt.expectedEmailsSent, len(mockMail.SentMessages))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. Age Validation Tests (>= 13 years old)
// ---------------------------------------------------------------------------

func TestAuthService_AgeValidation(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	now := time.Now()
	thirteenYearsAgo := now.AddDate(-13, 0, 0).Format("2006-01-02")
	twentyYearsAgo := now.AddDate(-20, 0, 0).Format("2006-01-02")
	twelveYearsAgo := now.AddDate(-12, 0, 0).Format("2006-01-02")
	fiveYearsAgo := now.AddDate(-5, 0, 0).Format("2006-01-02")

	tests := []struct {
		name        string
		birthdate   string
		expectError error
	}{
		{"Exactly 13 years old succeeds", thirteenYearsAgo, nil},
		{"20 years old succeeds", twentyYearsAgo, nil},
		{"12 years old fails with ErrUnderage", twelveYearsAgo, services.ErrUnderage},
		{"5 years old fails with ErrUnderage", fiveYearsAgo, services.ErrUnderage},
		{"Invalid date format fails", "not-a-date", services.ErrInvalidBirthdate},
		{"Bad format month fails", "2010/05/05", services.ErrInvalidBirthdate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeAuthRepo()
			mockMail := mailer.NewMockMailer()
			svc := services.NewAuthService(repo, mockMail, cfg)

			signupToken, _ := auth.GenerateSignupToken("newuser@example.com", cfg.JWTSecret, cfg.JWTIssuer, 10*time.Minute)

			_, _, err := svc.Signup(ctx, signupToken, "valid_user", "Valid Name", tt.birthdate, "agent")
			if !errors.Is(err, tt.expectError) {
				t.Errorf("for birthdate %s: expected %v, got %v", tt.birthdate, tt.expectError, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 5. Username Rules Tests (3-20 chars, ^[a-z0-9_]{3,20}$)
// ---------------------------------------------------------------------------

func TestAuthService_UsernameRules(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	tests := []struct {
		name        string
		username    string
		expectError error
	}{
		{"Valid standard username", "johndoe", nil},
		{"Valid with numbers and underscores", "john_123", nil},
		{"Valid minimum 3 chars", "abc", nil},
		{"Valid maximum 20 chars", "abcdefghijklmnopqrst", nil},
		{"Too short (2 chars)", "ab", services.ErrInvalidUsername},
		{"Too long (21 chars)", "abcdefghijklmnopqrstu", services.ErrInvalidUsername},
		{"Contains uppercase letters", "JohnDoe", services.ErrInvalidUsername},
		{"Contains hyphen", "john-doe", services.ErrInvalidUsername},
		{"Contains space", "john doe", services.ErrInvalidUsername},
		{"Contains special characters", "john@doe", services.ErrInvalidUsername},
		{"Contains dots", "john.doe", services.ErrInvalidUsername},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeAuthRepo()
			mockMail := mailer.NewMockMailer()
			svc := services.NewAuthService(repo, mockMail, cfg)

			signupToken, _ := auth.GenerateSignupToken("newuser@example.com", cfg.JWTSecret, cfg.JWTIssuer, 10*time.Minute)

			_, _, err := svc.Signup(ctx, signupToken, tt.username, "Valid Name", "2000-01-01", "agent")
			if !errors.Is(err, tt.expectError) {
				t.Errorf("for username %s: expected error %v, got %v", tt.username, tt.expectError, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 6. Refresh Token Reuse Detection Tests
// ---------------------------------------------------------------------------

func TestAuthService_RefreshTokenReuseDetection(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	t.Run("Reusing rotated token revokes all user sessions", func(t *testing.T) {
		repo := newFakeAuthRepo()
		mockMail := mailer.NewMockMailer()
		svc := services.NewAuthService(repo, mockMail, cfg)

		u, _ := repo.CreateUser(ctx, "refreshtest", "refresh@example.com", "Refresh Test", time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC), "user")

		// Issue initial refresh token
		rawToken, _ := auth.GenerateRefreshToken()
		tokenHash := auth.HashRefreshToken(rawToken)
		_ = repo.CreateRefreshToken(ctx, u.ID, tokenHash, nil, time.Now().Add(30*24*time.Hour))

		// First rotation: valid token rotates successfully
		tokens1, err := svc.RefreshToken(ctx, rawToken, "agent-1")
		if err != nil {
			t.Fatalf("first refresh failed: %v", err)
		}

		// Second rotation using the OLD rawToken: must trigger token reuse detection!
		_, err = svc.RefreshToken(ctx, rawToken, "agent-hacker")
		if !errors.Is(err, services.ErrTokenRevoked) {
			t.Fatalf("expected ErrTokenRevoked on reuse, got %v", err)
		}

		// Verify that all refresh tokens for user were revoked
		if !repo.revokedUsers[u.ID] {
			t.Errorf("expected RevokeAllUserRefreshTokens to be called on token reuse")
		}

		// Even the new token issued in rotation 1 should now be revoked!
		_, err = svc.RefreshToken(ctx, tokens1.RefreshToken, "agent-1")
		if !errors.Is(err, services.ErrTokenRevoked) {
			t.Errorf("expected new token to also be revoked after reuse incident, got %v", err)
		}
	})

	t.Run("Expired refresh token fails", func(t *testing.T) {
		repo := newFakeAuthRepo()
		mockMail := mailer.NewMockMailer()
		svc := services.NewAuthService(repo, mockMail, cfg)

		u, _ := repo.CreateUser(ctx, "expireduser", "exp@example.com", "Exp User", time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC), "user")

		rawToken, _ := auth.GenerateRefreshToken()
		tokenHash := auth.HashRefreshToken(rawToken)
		// Token expired 1 hour ago
		_ = repo.CreateRefreshToken(ctx, u.ID, tokenHash, nil, time.Now().Add(-1*time.Hour))

		_, err := svc.RefreshToken(ctx, rawToken, "agent")
		if !errors.Is(err, services.ErrTokenExpired) {
			t.Fatalf("expected ErrTokenExpired, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 7. Additional Service Rules & Edge Cases (High Coverage)
// ---------------------------------------------------------------------------

func TestServices_AdditionalRules(t *testing.T) {
	ctx := context.Background()

	t.Run("PostService_Creation_Validation", func(t *testing.T) {
		repo := newFakePostRepo()
		svc := services.NewPostService(repo, "https://storage.local")
		authorID := uuid.NewString()
		assetID := uuid.NewString()

		// Both empty
		_, err := svc.CreatePost(ctx, authorID, "   ", []string{})
		if !errors.Is(err, services.ErrValidation) {
			t.Errorf("expected ErrValidation on empty body and no images, got %v", err)
		}

		// Body too long (> 2000 chars)
		longBody := strings.Repeat("a", 2001)
		_, err = svc.CreatePost(ctx, authorID, longBody, nil)
		if !errors.Is(err, services.ErrValidation) {
			t.Errorf("expected ErrValidation on body > 2000 chars, got %v", err)
		}

		// Valid image only
		p, err := svc.CreatePost(ctx, authorID, "", []string{assetID})
		if err != nil || p == nil {
			t.Fatalf("expected success with asset only, got %v", err)
		}
	})

	t.Run("PostService_SharePost_Validation", func(t *testing.T) {
		repo := newFakePostRepo()
		svc := services.NewPostService(repo, "https://storage.local")
		authorID := uuid.NewString()
		userID := uuid.NewString()
		nonExistentID := uuid.NewString()
		p, _ := repo.CreatePost(ctx, authorID, "Post to share", nil)

		// Quote > 500
		longQuote := strings.Repeat("q", 501)
		_, err := svc.SharePost(ctx, userID, p.ID, &longQuote)
		if !errors.Is(err, services.ErrValidation) {
			t.Errorf("expected ErrValidation on quote > 500, got %v", err)
		}

		// Non-existent post
		quote := "Nice"
		_, err = svc.SharePost(ctx, userID, nonExistentID, &quote)
		if !errors.Is(err, services.ErrNotFound) {
			t.Errorf("expected ErrNotFound on nonexistent post share, got %v", err)
		}

		// Valid share
		sh, err := svc.SharePost(ctx, userID, p.ID, &quote)
		if err != nil || sh == nil {
			t.Fatalf("expected valid share, got %v", err)
		}
	})

	t.Run("UserService_Follow_SelfFollow", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := services.NewUserService(repo, "https://storage.local")
		userID := uuid.NewString()

		_, err := svc.FollowUser(ctx, userID, userID)
		if !errors.Is(err, services.ErrSelfFollow) {
			t.Errorf("expected ErrSelfFollow, got %v", err)
		}
	})

	t.Run("AssetService_Validation", func(t *testing.T) {
		repo := newFakeAssetRepo()
		store := newMockStorageClient()
		svc := services.NewAssetService(repo, store, "media", "https://storage.local")
		ownerID := uuid.NewString()

		// Size > 5MB
		_, _, err := svc.CreateAsset(ctx, ownerID, "post_image", "image/png", 5242881)
		if !errors.Is(err, services.ErrPayloadTooLarge) {
			t.Errorf("expected ErrPayloadTooLarge, got %v", err)
		}

		// Size < 1
		_, _, err = svc.CreateAsset(ctx, ownerID, "post_image", "image/png", 0)
		if !errors.Is(err, services.ErrValidation) {
			t.Errorf("expected ErrValidation on size < 1, got %v", err)
		}

		// Invalid mime type
		_, _, err = svc.CreateAsset(ctx, ownerID, "post_image", "image/gif", 1024)
		if !errors.Is(err, services.ErrValidation) {
			t.Errorf("expected ErrValidation on invalid mime, got %v", err)
		}

		// Invalid kind
		_, _, err = svc.CreateAsset(ctx, ownerID, "invalid_kind", "image/png", 1024)
		if !errors.Is(err, services.ErrValidation) {
			t.Errorf("expected ErrValidation on invalid kind, got %v", err)
		}
	})

	t.Run("SearchService_Validation", func(t *testing.T) {
		repo := &FakeSearchRepo{}
		svc := services.NewSearchService(repo, "https://storage.local")

		// Empty q
		_, err := svc.Search(ctx, "", "posts", nil, 20, "")
		if !errors.Is(err, services.ErrValidation) {
			t.Errorf("expected ErrValidation on empty q, got %v", err)
		}

		// q > 200
		_, err = svc.Search(ctx, strings.Repeat("x", 201), "posts", nil, 20, "")
		if !errors.Is(err, services.ErrValidation) {
			t.Errorf("expected ErrValidation on q > 200, got %v", err)
		}

		// Invalid type
		_, err = svc.Search(ctx, "query", "unknown", nil, 20, "")
		if !errors.Is(err, services.ErrValidation) {
			t.Errorf("expected ErrValidation on invalid type, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 8. Full Coverage Suite for UserService, PostService, CommentService, etc.
// ---------------------------------------------------------------------------

func TestUserService_FullSuite(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := services.NewUserService(repo, "https://storage.local")

	u1ID := uuid.NewString()
	u2ID := uuid.NewString()
	aid := uuid.NewString()
	nonExistentID := uuid.NewString()

	u1 := &models.UserDetail{
		ID:             u1ID,
		Username:       "alice",
		Email:          "alice@example.com",
		DisplayName:    "Alice",
		Birthdate:      time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC),
		Role:           "user",
		FollowerCount:  10,
		FollowingCount: 5,
		CreatedAt:      time.Now(),
	}
	u2 := &models.UserDetail{
		ID:          u2ID,
		Username:    "bob",
		Email:       "bob@example.com",
		DisplayName: "Bob",
		Birthdate:   time.Date(1998, 2, 2, 0, 0, 0, 0, time.UTC),
		Role:        "user",
		CreatedAt:   time.Now(),
	}
	repo.users[u1ID] = u1
	repo.users[u2ID] = u2

	// GetProfile
	p, err := svc.GetProfile(ctx, "alice")
	if err != nil || p.Username != "alice" {
		t.Fatalf("failed GetProfile: %v", err)
	}
	_, err = svc.GetProfile(ctx, "nonexistent")
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent user, got %v", err)
	}

	// GetMe
	me, err := svc.GetMe(ctx, u1ID)
	if err != nil || me.Email != "alice@example.com" {
		t.Fatalf("failed GetMe: %v", err)
	}
	_, err = svc.GetMe(ctx, nonExistentID)
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent me, got %v", err)
	}

	// UpdateMe
	longName := strings.Repeat("n", 51)
	_, err = svc.UpdateMe(ctx, u1ID, &longName, nil, false)
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on long displayName, got %v", err)
	}
	emptyName := ""
	_, err = svc.UpdateMe(ctx, u1ID, &emptyName, nil, false)
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on empty displayName, got %v", err)
	}
	newName := "Alice New"
	updated, err := svc.UpdateMe(ctx, u1ID, &newName, &aid, false)
	if err != nil || updated.DisplayName != "Alice New" {
		t.Fatalf("failed UpdateMe: %v", err)
	}
	cleared, err := svc.UpdateMe(ctx, u1ID, nil, nil, true)
	if err != nil || cleared.AvatarURL != nil {
		t.Fatalf("failed clearing avatar: %v", err)
	}

	// Follow & Unfollow
	_, err = svc.FollowUser(ctx, u1ID, nonExistentID)
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound following nonexistent user, got %v", err)
	}
	ok, err := svc.FollowUser(ctx, u1ID, u2ID)
	if err != nil || !ok {
		t.Fatalf("failed follow user: %v", err)
	}

	_, err = svc.UnfollowUser(ctx, u1ID, nonExistentID)
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound unfollowing nonexistent user, got %v", err)
	}
	ok, err = svc.UnfollowUser(ctx, u1ID, u2ID)
	if err != nil || ok {
		t.Fatalf("failed unfollow user: %v", err)
	}

	// Re-follow for followers list test
	_, _ = svc.FollowUser(ctx, u1ID, u2ID)

	// GetFollowers & GetFollowing
	fList, err := svc.GetFollowers(ctx, u2ID, 10, "")
	if err != nil || len(fList.Data) == 0 {
		t.Fatalf("failed GetFollowers: %v", err)
	}
	_, err = svc.GetFollowers(ctx, nonExistentID, 10, "")
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on GetFollowers for nonexistent user, got %v", err)
	}
	_, err = svc.GetFollowers(ctx, u2ID, 10, "invalid-cursor-encoding")
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on invalid cursor, got %v", err)
	}

	followingList, err := svc.GetFollowing(ctx, u1ID, 10, "")
	if err != nil || len(followingList.Data) == 0 {
		t.Fatalf("failed GetFollowing: %v", err)
	}
	_, err = svc.GetFollowing(ctx, nonExistentID, 10, "")
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on GetFollowing for nonexistent user, got %v", err)
	}
	_, err = svc.GetFollowing(ctx, u1ID, 10, "invalid-cursor-encoding")
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on invalid cursor, got %v", err)
	}
}

func TestPostService_FullSuite(t *testing.T) {
	ctx := context.Background()
	repo := newFakePostRepo()
	svc := services.NewPostService(repo, "https://storage.local")

	authorID := uuid.NewString()
	asset1 := uuid.NewString()
	asset2 := uuid.NewString()
	nonExistentID := uuid.NewString()
	likerID := uuid.NewString()

	u := &models.UserDetail{ID: authorID, Username: "auth", DisplayName: "Auth User"}
	repo.users[authorID] = u

	// CreatePost
	p, err := svc.CreatePost(ctx, authorID, "Valid post body", []string{asset1, asset2})
	if err != nil || p == nil {
		t.Fatalf("failed CreatePost: %v", err)
	}

	// GetPost
	gp, err := svc.GetPost(ctx, p.ID, nil)
	if err != nil || gp.ID != p.ID {
		t.Fatalf("failed GetPost: %v", err)
	}
	_, err = svc.GetPost(ctx, nonExistentID, nil)
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on nonexistent GetPost, got %v", err)
	}

	// UpdatePost
	_, err = svc.UpdatePost(ctx, p.ID, authorID, "user", "")
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on empty post update, got %v", err)
	}
	_, err = svc.UpdatePost(ctx, p.ID, authorID, "user", strings.Repeat("x", 2001))
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on post update > 2000 chars, got %v", err)
	}
	up, err := svc.UpdatePost(ctx, p.ID, authorID, "user", "Valid update")
	if err != nil || up.Body != "Valid update" {
		t.Fatalf("failed valid UpdatePost: %v", err)
	}

	// LikePost & UnlikePost
	liked, err := svc.LikePost(ctx, likerID, p.ID)
	if err != nil || !liked {
		t.Fatalf("failed LikePost: %v", err)
	}
	_, err = svc.LikePost(ctx, likerID, nonExistentID)
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on liking nonexistent post, got %v", err)
	}

	unliked, err := svc.UnlikePost(ctx, likerID, p.ID)
	if err != nil || unliked {
		t.Fatalf("failed UnlikePost: %v", err)
	}
	_, err = svc.UnlikePost(ctx, likerID, nonExistentID)
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on unliking nonexistent post, got %v", err)
	}

	// GetFeed
	feed, err := svc.GetFeed(ctx, authorID, 10, "")
	if err != nil || len(feed.Data) == 0 {
		t.Fatalf("failed GetFeed: %v", err)
	}
	_, err = svc.GetFeed(ctx, authorID, 10, "bad-cursor")
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on bad feed cursor, got %v", err)
	}

	// GetUserPosts
	uPosts, err := svc.GetUserPosts(ctx, authorID, nil, 10, "")
	if err != nil || len(uPosts.Data) == 0 {
		t.Fatalf("failed GetUserPosts: %v", err)
	}
	_, err = svc.GetUserPosts(ctx, nonExistentID, nil, 10, "")
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on GetUserPosts for nonexistent user, got %v", err)
	}
	_, err = svc.GetUserPosts(ctx, authorID, nil, 10, "bad-cursor")
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on bad user posts cursor, got %v", err)
	}
}

func TestCommentService_FullSuite(t *testing.T) {
	ctx := context.Background()
	repo := newFakeCommentRepo()
	svc := services.NewCommentService(repo, "https://storage.local")

	postID := uuid.NewString()
	authorID := uuid.NewString()
	userID := uuid.NewString()
	nonExistentPostID := uuid.NewString()
	nonExistentCommentID := uuid.NewString()

	post := &models.PostDetail{
		Post: models.Post{
			ID:        postID,
			AuthorID:  authorID,
			Body:      "Post for comments",
			CreatedAt: time.Now(),
		},
	}
	repo.posts[postID] = post

	// CreateComment
	_, err := svc.CreateComment(ctx, userID, postID, "")
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on empty comment body, got %v", err)
	}
	_, err = svc.CreateComment(ctx, userID, postID, strings.Repeat("c", 1001))
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on comment body > 1000, got %v", err)
	}
	_, err = svc.CreateComment(ctx, userID, nonExistentPostID, "Valid comment")
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on nonexistent post comment, got %v", err)
	}

	c, err := svc.CreateComment(ctx, userID, postID, "Valid comment")
	if err != nil || c.Body != "Valid comment" {
		t.Fatalf("failed CreateComment: %v", err)
	}

	// GetComments
	cList, err := svc.GetComments(ctx, postID, 10, "")
	if err != nil || len(cList.Data) == 0 {
		t.Fatalf("failed GetComments: %v", err)
	}
	_, err = svc.GetComments(ctx, nonExistentPostID, 10, "")
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on GetComments nonexistent post, got %v", err)
	}
	_, err = svc.GetComments(ctx, postID, 10, "bad-cursor")
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on bad comment cursor, got %v", err)
	}

	// DeleteComment
	err = svc.DeleteComment(ctx, nonExistentCommentID, userID, "user")
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on deleting nonexistent comment, got %v", err)
	}
	err = svc.DeleteComment(ctx, c.ID, userID, "user")
	if err != nil {
		t.Fatalf("failed DeleteComment: %v", err)
	}
	// Soft deleted comment should now return ErrNotFound
	err = svc.DeleteComment(ctx, c.ID, userID, "user")
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound on already deleted comment, got %v", err)
	}
}

func TestAssetService_FullSuite(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAssetRepo()
	store := newMockStorageClient()
	svc := services.NewAssetService(repo, store, "media", "https://storage.local")

	userID := uuid.NewString()
	nonExistentAssetID := uuid.NewString()
	expiredAssetID := uuid.NewString()

	// CreateAsset (valid cases)
	id1, url1, err := svc.CreateAsset(ctx, userID, "post_image", "image/jpeg", 2048)
	if err != nil || id1 == "" || url1 == "" {
		t.Fatalf("failed CreateAsset jpeg: %v", err)
	}
	id2, url2, err := svc.CreateAsset(ctx, userID, "avatar", "image/webp", 4096)
	if err != nil || id2 == "" || url2 == "" {
		t.Fatalf("failed CreateAsset webp: %v", err)
	}

	// CompleteAsset
	// Nonexistent asset
	_, err = svc.CompleteAsset(ctx, nonExistentAssetID, userID, "user")
	if !errors.Is(err, services.ErrNotFound) {
		t.Errorf("expected ErrNotFound completing nonexistent asset, got %v", err)
	}

	// Storage object missing
	_, err = svc.CompleteAsset(ctx, id1, userID, "user")
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation when storage object missing, got %v", err)
	}

	// Size mismatch in storage
	asset1, _ := repo.GetAssetByID(ctx, id1)
	store.objects[asset1.StorageKey] = 1000 // expected 2048
	_, err = svc.CompleteAsset(ctx, id1, userID, "user")
	if !errors.Is(err, services.ErrValidation) {
		t.Errorf("expected ErrValidation on size mismatch, got %v", err)
	}

	// Valid completion
	store.objects[asset1.StorageKey] = 2048
	publicURL, err := svc.CompleteAsset(ctx, id1, userID, "user")
	if err != nil || publicURL == "" {
		t.Fatalf("failed valid CompleteAsset: %v", err)
	}

	// Already ready -> ErrAlreadyReady
	_, err = svc.CompleteAsset(ctx, id1, userID, "user")
	if !errors.Is(err, services.ErrAlreadyReady) {
		t.Errorf("expected ErrAlreadyReady on already completed asset, got %v", err)
	}

	// CleanupExpiredAssets
	expiredAsset, _ := repo.CreateAsset(ctx, expiredAssetID, userID, "post_image", "media", fmt.Sprintf("u/%s/expired.png", userID), "image/png", 500)
	expiredAsset.UploadExpiresAt = time.Now().Add(-2 * time.Hour)
	store.objects[expiredAsset.StorageKey] = 500

	count, err := svc.CleanupExpiredAssets(ctx)
	if err != nil || count == 0 {
		t.Fatalf("failed CleanupExpiredAssets: %v (count=%d)", err, count)
	}
}

func TestSearchService_FullSuite(t *testing.T) {
	ctx := context.Background()
	repo := &FakeSearchRepo{
		users: []*models.UserDetail{
			{ID: "u1", Username: "searchalice", DisplayName: "Search Alice"},
		},
		posts: []*models.PostDetail{
			{Post: models.Post{ID: "p1", AuthorID: "u1", Body: "Search post content"}},
		},
		comments: []*models.CommentDetail{
			{Comment: models.Comment{ID: "c1", PostID: "p1", AuthorID: "u1", Body: "Search comment body"}},
		},
	}
	svc := services.NewSearchService(repo, "https://storage.local")

	// Search users
	resU, err := svc.Search(ctx, "alice", "users", nil, 10, "")
	if err != nil || resU == nil {
		t.Fatalf("failed search users: %v", err)
	}

	// Search posts
	resP, err := svc.Search(ctx, "content", "posts", nil, 10, "0")
	if err != nil || resP == nil {
		t.Fatalf("failed search posts: %v", err)
	}

	// Search comments
	resC, err := svc.Search(ctx, "body", "comments", nil, 10, "MA==") // base64 "0"
	if err != nil || resC == nil {
		t.Fatalf("failed search comments: %v", err)
	}
}

func TestAuthService_Logout(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()
	repo := newFakeAuthRepo()
	mockMail := mailer.NewMockMailer()
	svc := services.NewAuthService(repo, mockMail, cfg)

	rawToken, _ := auth.GenerateRefreshToken()
	tokenHash := auth.HashRefreshToken(rawToken)
	_ = repo.CreateRefreshToken(ctx, "user-1", tokenHash, nil, time.Now().Add(10*time.Hour))

	err := svc.Logout(ctx, rawToken)
	if err != nil {
		t.Fatalf("failed Logout: %v", err)
	}
	tRecord, _ := repo.FindRefreshTokenByHash(ctx, tokenHash)
	if tRecord.RevokedAt == nil {
		t.Errorf("expected token to be revoked on logout")
	}
}

func TestServices_TargetedBranchCoverage(t *testing.T) {
	ctx := context.Background()

	// 1. AssetService StartCleanupJob
	t.Run("AssetService_StartCleanupJob", func(t *testing.T) {
		repo := newFakeAssetRepo()
		store := newMockStorageClient()
		svc := services.NewAssetService(repo, store, "media", "https://storage.local")

		jobCtx, cancel := context.WithCancel(context.Background())
		svc.StartCleanupJob(jobCtx, 10*time.Millisecond)
		time.Sleep(25 * time.Millisecond)
		cancel()
	})

	// 2. Auth VerifyCode for existing user vs new user
	t.Run("AuthService_VerifyCode_ExistingAndNew", func(t *testing.T) {
		cfg := testConfig()
		repo := newFakeAuthRepo()
		mockMail := mailer.NewMockMailer()
		svc := services.NewAuthService(repo, mockMail, cfg)

		// Create active code
		email := "exist@example.com"
		code := "123456"
		codeHash := auth.HashOTP(code, cfg.OTPHMACKey)
		_ = repo.CreateLoginCode(ctx, email, codeHash, time.Now().Add(10*time.Minute))

		// Case A: New user (not in DB)
		resNew, err := svc.VerifyCode(ctx, email, code, "test-agent")
		if err != nil || !resNew.IsNewUser || resNew.SignupToken == "" {
			t.Fatalf("failed new user verify code: %v", err)
		}

		// Re-create code and user
		_ = repo.CreateLoginCode(ctx, email, codeHash, time.Now().Add(10*time.Minute))
		_, _ = repo.CreateUser(ctx, "existuser", email, "Exist User", time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC), "user")

		// Case B: Existing user
		resExist, err := svc.VerifyCode(ctx, email, code, "test-agent")
		if err != nil || resExist.IsNewUser || resExist.AccessToken == "" || resExist.RefreshToken == "" {
			t.Fatalf("failed existing user verify code: %v", err)
		}
	})

	// 3. PostService CreatePost Trigger Error & Pagination NextCursor
	t.Run("PostService_TriggerError_And_Pagination", func(t *testing.T) {
		repo := newFakePostRepo()
		svc := services.NewPostService(repo, "https://storage.local")
		authorID := uuid.NewString()

		// Add 5 posts to test hasMore = true and nextCursor calculation
		for i := 0; i < 5; i++ {
			p, _ := repo.CreatePost(ctx, authorID, "Body", nil)
			p.AuthorAvatarStorageKey = new(string)
			*p.AuthorAvatarStorageKey = fmt.Sprintf("u/%s/avatar.png", authorID)
		}

		// Feed with limit 2 (hasMore = true)
		feed, err := svc.GetFeed(ctx, authorID, 2, "")
		if err != nil || !feed.Pagination.HasMore || feed.Pagination.NextCursor == nil {
			t.Fatalf("expected hasMore and nextCursor on feed, got %+v", feed)
		}

		// UserPosts with limit 2 (hasMore = true)
		repo.users[authorID] = &models.UserDetail{ID: authorID}
		uPosts, err := svc.GetUserPosts(ctx, authorID, nil, 2, "")
		if err != nil || !uPosts.Pagination.HasMore || uPosts.Pagination.NextCursor == nil {
			t.Fatalf("expected hasMore and nextCursor on user posts, got %+v", uPosts)
		}
	})

	// 4. CommentService Pagination NextCursor & Avatar
	t.Run("CommentService_Pagination_And_Avatar", func(t *testing.T) {
		repo := newFakeCommentRepo()
		svc := services.NewCommentService(repo, "https://storage.local")
		postID := uuid.NewString()
		authorID := uuid.NewString()
		repo.posts[postID] = &models.PostDetail{Post: models.Post{ID: postID}}

		for i := 0; i < 4; i++ {
			c, _ := repo.CreateComment(ctx, postID, authorID, "Body")
			c.AuthorAvatarStorageKey = new(string)
			*c.AuthorAvatarStorageKey = fmt.Sprintf("u/%s/avatar.png", authorID)
		}

		// Limit 2 (hasMore = true)
		cList, err := svc.GetComments(ctx, postID, 2, "")
		if err != nil || !cList.Pagination.HasMore || cList.Pagination.NextCursor == nil {
			t.Fatalf("expected hasMore and nextCursor on comments, got %+v", cList)
		}
	})

	// 5. UserService Pagination NextCursor
	t.Run("UserService_Pagination_NextCursor", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := services.NewUserService(repo, "https://storage.local")

		targetID := uuid.NewString()
		repo.users[targetID] = &models.UserDetail{ID: targetID}
		var firstFollower string
		for i := 0; i < 4; i++ {
			uid := uuid.NewString()
			if i == 0 {
				firstFollower = uid
			}
			repo.users[uid] = &models.UserDetail{ID: uid, Username: fmt.Sprintf("follower%d", i)}
			_, _ = repo.Follow(ctx, uid, targetID)
		}

		// Limit 2 followers (hasMore = true)
		fList, err := svc.GetFollowers(ctx, targetID, 2, "")
		if err != nil || !fList.Pagination.HasMore || fList.Pagination.NextCursor == nil {
			t.Fatalf("expected hasMore and nextCursor on followers, got %+v", fList)
		}

		// Limit 2 following (hasMore = true)
		followingList, err := svc.GetFollowing(ctx, firstFollower, 1, "")
		if err != nil || followingList == nil {
			t.Fatalf("failed GetFollowing: %v", err)
		}
	})

	// 6. SearchService Pagination NextCursor
	t.Run("SearchService_Pagination_NextCursor", func(t *testing.T) {
		repo := &FakeSearchRepo{
			users: []*models.UserDetail{
				{ID: "u1", Username: "u1", AvatarStorageKey: new(string)},
				{ID: "u2", Username: "u2"},
				{ID: "u3", Username: "u3"},
			},
			posts: []*models.PostDetail{
				{Post: models.Post{ID: "p1", AuthorID: "u1"}, AuthorAvatarStorageKey: new(string), Assets: []models.PostAssetInfo{{ID: "a1", StorageKey: "k1"}}},
				{Post: models.Post{ID: "p2", AuthorID: "u1"}},
				{Post: models.Post{ID: "p3", AuthorID: "u1"}},
			},
			comments: []*models.CommentDetail{
				{Comment: models.Comment{ID: "c1", PostID: "p1", AuthorID: "u1"}, AuthorAvatarStorageKey: new(string)},
				{Comment: models.Comment{ID: "c2", PostID: "p1", AuthorID: "u1"}},
				{Comment: models.Comment{ID: "c3", PostID: "p1", AuthorID: "u1"}},
			},
		}
		svc := services.NewSearchService(repo, "https://storage.local")

		// Users limit 2 (hasMore = true)
		sU, err := svc.Search(ctx, "u", "users", nil, 2, "0")
		if err != nil || !sU.Pagination.HasMore || sU.Pagination.NextCursor == nil {
			t.Fatalf("expected nextCursor on users search, got %+v", sU)
		}

		// Posts limit 2 (hasMore = true)
		sP, err := svc.Search(ctx, "p", "posts", nil, 2, "0")
		if err != nil || !sP.Pagination.HasMore || sP.Pagination.NextCursor == nil {
			t.Fatalf("expected nextCursor on posts search, got %+v", sP)
		}

		// Comments limit 2 (hasMore = true)
		sC, err := svc.Search(ctx, "c", "comments", nil, 2, "0")
		if err != nil || !sC.Pagination.HasMore || sC.Pagination.NextCursor == nil {
			t.Fatalf("expected nextCursor on comments search, got %+v", sC)
		}
	})
}
