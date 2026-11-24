package services_test

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
	"github.com/google/uuid"
)

type callCountingSearchRepo struct {
	*FakeSearchRepo
	searchUsersCalls    int
	searchPostsCalls    int
	searchCommentsCalls int
}

func newCallCountingSearchRepo() *callCountingSearchRepo {
	return &callCountingSearchRepo{
		FakeSearchRepo: &FakeSearchRepo{},
	}
}

func (c *callCountingSearchRepo) SearchUsers(ctx context.Context, q string, limit, offset int) ([]*models.UserDetail, error) {
	c.searchUsersCalls++
	return c.FakeSearchRepo.SearchUsers(ctx, q, limit, offset)
}

func (c *callCountingSearchRepo) SearchPosts(ctx context.Context, q string, currentUserID *string, limit, offset int) ([]*models.PostDetail, error) {
	c.searchPostsCalls++
	return c.FakeSearchRepo.SearchPosts(ctx, q, currentUserID, limit, offset)
}

func (c *callCountingSearchRepo) SearchComments(ctx context.Context, q string, limit, offset int) ([]*models.CommentDetail, error) {
	c.searchCommentsCalls++
	return c.FakeSearchRepo.SearchComments(ctx, q, limit, offset)
}

func TestSearchService_ValidateSearch(t *testing.T) {
	validUser := uuid.NewString()
	validCursor := base64.StdEncoding.EncodeToString([]byte("10"))

	tests := []struct {
		name       string
		q          string
		searchType string
		userID     *string
		limit      int
		cursor     string
		wantErr    bool
		wantOffset int
	}{
		{"Valid users search", "alice", "users", nil, 20, "", false, 0},
		{"Valid posts search with current user", "tech news", "posts", &validUser, 50, "", false, 0},
		{"Valid comments search", "interesting comment", "comments", nil, 10, "", false, 0},
		{"Valid query with cursor offset 10", "golang", "posts", nil, 20, validCursor, false, 10},
		{"Valid Arabic query", "برمجة غولانغ", "posts", nil, 20, "", false, 0},
		{"Valid case insensitive type USERS", "alice", "USERS", nil, 20, "", false, 0},
		{"Valid boundary exact 200 runes", strings.Repeat("A", 200), "posts", nil, 20, "", false, 0},
		{"Empty search query", "", "posts", nil, 20, "", true, 0},
		{"Whitespace only search query", "   \t\n  ", "posts", nil, 20, "", true, 0},
		{"Query exceeds 200 runes (boundary 201)", strings.Repeat("A", 201), "posts", nil, 20, "", true, 0},
		{"Query contains null byte", "test\x00query", "posts", nil, 20, "", true, 0},
		{"Query contains bell control char", "test\x07query", "posts", nil, 20, "", true, 0},
		{"Invalid search type", "test", "hashtags", nil, 20, "", true, 0},
		{"Empty search type", "test", "", nil, 20, "", true, 0},
		{"Malformed current user ID", "test", "posts", stringPtr("not-a-uuid"), 20, "", true, 0},
		{"Limit < 1", "test", "posts", nil, 0, "", true, 0},
		{"Limit negative", "test", "posts", nil, -5, "", true, 0},
		{"Limit > 100", "test", "posts", nil, 101, "", true, 0},
		{"Invalid base64 cursor", "test", "posts", nil, 20, "not-base-64!!", true, 0},
		{"Negative cursor offset", "test", "posts", nil, 20, base64.StdEncoding.EncodeToString([]byte("-5")), true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanQ, cleanType, _, cleanLimit, offset, err := services.ValidateSearch(tt.q, tt.searchType, tt.userID, tt.limit, tt.cursor)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateSearch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if cleanQ != strings.TrimSpace(tt.q) {
					t.Errorf("cleanQ = %s, want %s", cleanQ, strings.TrimSpace(tt.q))
				}
				if cleanType != strings.ToLower(strings.TrimSpace(tt.searchType)) {
					t.Errorf("cleanType = %s, want %s", cleanType, strings.ToLower(strings.TrimSpace(tt.searchType)))
				}
				if cleanLimit != tt.limit {
					t.Errorf("cleanLimit = %d, want %d", cleanLimit, tt.limit)
				}
				if offset != tt.wantOffset {
					t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
				}
			}
		})
	}
}

func TestSearchService_FailFastZeroRepoCalls(t *testing.T) {
	repo := newCallCountingSearchRepo()
	svc := services.NewSearchService(repo, "https://storage.local")
	ctx := context.Background()

	// 1. Empty query -> 0 repo calls
	_, err := svc.Search(ctx, "", "posts", nil, 20, "")
	if !errors.Is(err, services.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if repo.searchPostsCalls != 0 || repo.searchUsersCalls != 0 || repo.searchCommentsCalls != 0 {
		t.Errorf("expected 0 repo calls on empty query, got posts=%d users=%d comments=%d",
			repo.searchPostsCalls, repo.searchUsersCalls, repo.searchCommentsCalls)
	}

	// 2. Invalid search type -> 0 repo calls
	_, err = svc.Search(ctx, "valid query", "disallowed_type", nil, 20, "")
	if !errors.Is(err, services.ErrValidation) {
		t.Fatalf("expected ErrValidation on invalid type, got %v", err)
	}
	if repo.searchPostsCalls != 0 || repo.searchUsersCalls != 0 || repo.searchCommentsCalls != 0 {
		t.Errorf("expected 0 repo calls on invalid search type, got posts=%d users=%d comments=%d",
			repo.searchPostsCalls, repo.searchUsersCalls, repo.searchCommentsCalls)
	}

	// 3. Query with control characters -> 0 repo calls
	_, err = svc.Search(ctx, "null\x00query", "users", nil, 20, "")
	if !errors.Is(err, services.ErrValidation) {
		t.Fatalf("expected ErrValidation on control chars, got %v", err)
	}
	if repo.searchUsersCalls != 0 {
		t.Errorf("expected 0 repo calls on control char query, got %d", repo.searchUsersCalls)
	}

	// 4. Invalid limit -> 0 repo calls
	_, err = svc.Search(ctx, "query", "posts", nil, 999, "")
	if !errors.Is(err, services.ErrValidation) {
		t.Fatalf("expected ErrValidation on limit > 100, got %v", err)
	}
	if repo.searchPostsCalls != 0 {
		t.Errorf("expected 0 repo calls on invalid limit, got %d", repo.searchPostsCalls)
	}
}
