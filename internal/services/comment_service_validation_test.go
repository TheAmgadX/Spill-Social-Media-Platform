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

type callCountingCommentRepo struct {
	*FakeCommentRepo
	createCalls     int
	getByIDCalls    int
	listCalls       int
	softDeleteCalls int
}

func newCallCountingCommentRepo() *callCountingCommentRepo {
	return &callCountingCommentRepo{
		FakeCommentRepo: newFakeCommentRepo(),
	}
}

func (c *callCountingCommentRepo) CreateComment(ctx context.Context, postID, authorID, body string) (*models.CommentDetail, error) {
	c.createCalls++
	return c.FakeCommentRepo.CreateComment(ctx, postID, authorID, body)
}

func (c *callCountingCommentRepo) GetCommentByID(ctx context.Context, id string) (*models.Comment, error) {
	c.getByIDCalls++
	return c.FakeCommentRepo.GetCommentByID(ctx, id)
}

func (c *callCountingCommentRepo) GetCommentsByPostID(ctx context.Context, postID string, limit int, cursorTime *time.Time, cursorID string) ([]*models.CommentDetail, error) {
	c.listCalls++
	return c.FakeCommentRepo.GetCommentsByPostID(ctx, postID, limit, cursorTime, cursorID)
}

func (c *callCountingCommentRepo) SoftDeleteComment(ctx context.Context, id string) error {
	c.softDeleteCalls++
	return c.FakeCommentRepo.SoftDeleteComment(ctx, id)
}

func TestCommentService_ValidateCreateComment(t *testing.T) {
	validAuthor := uuid.NewString()
	validPost := uuid.NewString()

	tests := []struct {
		name     string
		authorID string
		postID   string
		body     string
		wantErr  bool
	}{
		{"Valid standard comment", validAuthor, validPost, "This is a great comment!", false},
		{"Valid comment with Arabic", validAuthor, validPost, "هذا تعليق رائع جدا ومفيد", false},
		{"Valid comment with emojis", validAuthor, validPost, "Awesome work! 🔥💯👏", false},
		{"Valid comment with newlines and tabs", validAuthor, validPost, "Line 1\n\tLine 2\r\nLine 3", false},
		{"Boundary exact 1 character", validAuthor, validPost, "A", false},
		{"Boundary exact 1000 characters", validAuthor, validPost, strings.Repeat("A", 1000), false},
		{"Boundary 1001 characters (exceeds max)", validAuthor, validPost, strings.Repeat("A", 1001), true},
		{"Empty body", validAuthor, validPost, "", true},
		{"Whitespace only body", validAuthor, validPost, "   \t\r\n  ", true},
		{"Body with null byte", validAuthor, validPost, "Comment with \x00 null byte", true},
		{"Body with invalid control character", validAuthor, validPost, "Comment with \x07 bell", true},
		{"Malformed author UUID", "not-a-uuid", validPost, "Valid body", true},
		{"Nil author UUID", "00000000-0000-0000-0000-000000000000", validPost, "Valid body", true},
		{"Empty author UUID", "", validPost, "Valid body", true},
		{"Malformed post UUID", validAuthor, "not-a-uuid", "Valid body", true},
		{"Nil post UUID", validAuthor, "00000000-0000-0000-0000-000000000000", "Valid body", true},
		{"Empty post UUID", validAuthor, "", "Valid body", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := services.ValidateCreateComment(tt.authorID, tt.postID, tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreateComment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommentService_ValidateGetComments(t *testing.T) {
	validPost := uuid.NewString()

	tests := []struct {
		name    string
		postID  string
		limit   int
		cursor  string
		wantErr bool
	}{
		{"Valid limit and empty cursor", validPost, 20, "", false},
		{"Valid limit boundary 1", validPost, 1, "", false},
		{"Valid limit boundary 100", validPost, 100, "", false},
		{"Malformed post ID", "invalid-uuid", 20, "", true},
		{"Nil post ID", "00000000-0000-0000-0000-000000000000", 20, "", true},
		{"Negative limit", validPost, -1, "", true},
		{"Limit exceeds max 100", validPost, 101, "", true},
		{"Invalid cursor format", validPost, 20, "invalid-cursor-value", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := services.ValidateGetComments(tt.postID, tt.limit, tt.cursor)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGetComments() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommentService_ValidateDeleteComment(t *testing.T) {
	validComment := uuid.NewString()
	validUser := uuid.NewString()

	tests := []struct {
		name      string
		commentID string
		userID    string
		role      string
		wantErr   bool
	}{
		{"Valid user role", validComment, validUser, "user", false},
		{"Valid admin role", validComment, validUser, "admin", false},
		{"Malformed comment ID", "not-a-uuid", validUser, "user", true},
		{"Nil comment ID", "00000000-0000-0000-0000-000000000000", validUser, "user", true},
		{"Malformed user ID", validComment, "not-a-uuid", "user", true},
		{"Nil user ID", validComment, "00000000-0000-0000-0000-000000000000", "user", true},
		{"Invalid role guest", validComment, validUser, "guest", true},
		{"Empty role", validComment, validUser, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := services.ValidateDeleteComment(tt.commentID, tt.userID, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDeleteComment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommentService_FailFastZeroRepoCalls(t *testing.T) {
	repo := newCallCountingCommentRepo()
	svc := services.NewCommentService(repo, "https://storage.local")
	ctx := context.Background()

	// 1. CreateComment with invalid post UUID -> 0 repo calls
	_, err := svc.CreateComment(ctx, "invalid-post-uuid", uuid.NewString(), "Some body")
	if err == nil {
		t.Fatal("expected error for invalid post UUID")
	}
	if repo.createCalls != 0 {
		t.Errorf("expected 0 repo create calls on validation failure, got %d", repo.createCalls)
	}

	// 2. CreateComment with empty body -> 0 repo calls
	_, err = svc.CreateComment(ctx, uuid.NewString(), uuid.NewString(), "   ")
	if err == nil {
		t.Fatal("expected error for empty body")
	}
	if repo.createCalls != 0 {
		t.Errorf("expected 0 repo create calls on empty body, got %d", repo.createCalls)
	}

	// 3. GetComments with invalid post UUID -> 0 repo calls
	_, err = svc.GetComments(ctx, "invalid-post-uuid", 20, "")
	if err == nil {
		t.Fatal("expected error for invalid post UUID in GetComments")
	}
	if repo.listCalls != 0 {
		t.Errorf("expected 0 repo list calls on invalid post UUID, got %d", repo.listCalls)
	}

	// 4. DeleteComment with invalid comment ID -> 0 repo calls
	err = svc.DeleteComment(ctx, "invalid-comment-uuid", uuid.NewString(), "user")
	if err == nil {
		t.Fatal("expected error for invalid comment UUID in DeleteComment")
	}
	if repo.getByIDCalls != 0 || repo.softDeleteCalls != 0 {
		t.Errorf("expected 0 repo calls on invalid comment UUID, got getByID=%d delete=%d", repo.getByIDCalls, repo.softDeleteCalls)
	}
}
