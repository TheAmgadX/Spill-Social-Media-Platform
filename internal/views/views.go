// Package views provides response DTOs and mapper functions that form
// the View layer in the MVC architecture.  Controllers call these mappers
// to convert internal model structs into JSON-safe response types.
//
// Key rule: PublicUserResponse never exposes email or birthdate.
// Only GET /me uses PrivateUserResponse.
package views

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
)

type contextKey string

const reqIDContextKey contextKey = "requestID"

// GetReqID retrieves the request ID from the context if present.
func GetReqID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(reqIDContextKey).(string); ok {
		return id
	}
	return ""
}

// WithReqID returns a new context containing the given request ID.
func WithReqID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, reqIDContextKey, reqID)
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// PaginationMeta is included in every paginated response.
type PaginationMeta struct {
	NextCursor *string `json:"nextCursor"` // nil when no more pages
	Limit      int     `json:"limit"`
	HasMore    bool    `json:"hasMore"`
}

// EncodeCursor produces an opaque base64 cursor from a (createdAt, id) pair.
func EncodeCursor(createdAt time.Time, id string) string {
	raw := fmt.Sprintf("%d:%s", createdAt.UnixMicro(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// ---------------------------------------------------------------------------
// Error (RFC 9457 problem+json)
// ---------------------------------------------------------------------------

// ProblemDetail represents an RFC 9457 error response.
type ProblemDetail struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail"`
	RequestID string `json:"requestId"`
}

// ---------------------------------------------------------------------------
// User DTOs
// ---------------------------------------------------------------------------

// PublicUserResponse is the public profile — it excludes email and birthdate.
type PublicUserResponse struct {
	ID             string  `json:"id"`
	Username       string  `json:"username"`
	DisplayName    string  `json:"displayName"`
	AvatarURL      *string `json:"avatarUrl"`
	Role           string  `json:"role"`
	FollowerCount  int     `json:"followerCount"`
	FollowingCount int     `json:"followingCount"`
	CreatedAt      string  `json:"createdAt"`
}

// PrivateUserResponse is returned only by GET /me.
// It includes every field in PublicUserResponse plus email and birthdate.
type PrivateUserResponse struct {
	ID             string  `json:"id"`
	Username       string  `json:"username"`
	Email          string  `json:"email"`
	DisplayName    string  `json:"displayName"`
	Birthdate      string  `json:"birthdate"`
	AvatarURL      *string `json:"avatarUrl"`
	Role           string  `json:"role"`
	FollowerCount  int     `json:"followerCount"`
	FollowingCount int     `json:"followingCount"`
	CreatedAt      string  `json:"createdAt"`
}

// ToPublicUser maps a models.User to a PublicUserResponse.
// avatarURLFn converts an asset ID to a full URL (nil-safe).
func ToPublicUser(u *models.User, avatarURLFn func(assetID string) string) PublicUserResponse {
	var avatarURL *string
	if u.AvatarAssetID != nil {
		url := avatarURLFn(*u.AvatarAssetID)
		avatarURL = &url
	}
	return PublicUserResponse{
		ID:             u.ID,
		Username:       u.Username,
		DisplayName:    u.DisplayName,
		AvatarURL:      avatarURL,
		Role:           u.Role,
		FollowerCount:  u.FollowerCount,
		FollowingCount: u.FollowingCount,
		CreatedAt:      u.CreatedAt.Format(time.RFC3339),
	}
}

// ToPrivateUser maps a models.User to a PrivateUserResponse (GET /me only).
func ToPrivateUser(u *models.User, avatarURLFn func(assetID string) string) PrivateUserResponse {
	var avatarURL *string
	if u.AvatarAssetID != nil {
		url := avatarURLFn(*u.AvatarAssetID)
		avatarURL = &url
	}
	return PrivateUserResponse{
		ID:             u.ID,
		Username:       u.Username,
		Email:          u.Email,
		DisplayName:    u.DisplayName,
		Birthdate:      u.Birthdate.Format("2006-01-02"),
		AvatarURL:      avatarURL,
		Role:           u.Role,
		FollowerCount:  u.FollowerCount,
		FollowingCount: u.FollowingCount,
		CreatedAt:      u.CreatedAt.Format(time.RFC3339),
	}
}

// ToPublicUsers maps a slice of users to public DTOs.
func ToPublicUsers(users []*models.User, avatarURLFn func(assetID string) string) []PublicUserResponse {
	out := make([]PublicUserResponse, len(users))
	for i, u := range users {
		out[i] = ToPublicUser(u, avatarURLFn)
	}
	return out
}

// ---------------------------------------------------------------------------
// Follow
// ---------------------------------------------------------------------------

// FollowStateResponse is returned by PUT/DELETE /users/{id}/follow.
type FollowStateResponse struct {
	Following bool `json:"following"`
}

// ---------------------------------------------------------------------------
// Post DTOs
// ---------------------------------------------------------------------------

// PostResponse is the public representation of a post.
type PostResponse struct {
	ID           string              `json:"id"`
	AuthorID     string              `json:"authorId"`
	Author       *PublicUserResponse `json:"author,omitempty"`
	Body         string              `json:"body"`
	Assets       []AssetResponse     `json:"assets,omitempty"`
	LikeCount    int                 `json:"likeCount"`
	CommentCount int                 `json:"commentCount"`
	ShareCount   int                 `json:"shareCount"`
	Liked        *bool               `json:"liked"` // nil for anonymous
	CreatedAt    string              `json:"createdAt"`
}

// ---------------------------------------------------------------------------
// Like
// ---------------------------------------------------------------------------

// LikeStateResponse is returned by PUT/DELETE /posts/{id}/like.
type LikeStateResponse struct {
	Liked bool `json:"liked"`
}

// ---------------------------------------------------------------------------
// Comment DTOs
// ---------------------------------------------------------------------------

// CommentResponse is the public representation of a comment.
type CommentResponse struct {
	ID        string              `json:"id"`
	PostID    string              `json:"postId"`
	AuthorID  string              `json:"authorId"`
	Author    *PublicUserResponse `json:"author,omitempty"`
	Body      string              `json:"body"`
	CreatedAt string              `json:"createdAt"`
}

// ---------------------------------------------------------------------------
// Share DTOs
// ---------------------------------------------------------------------------

// ShareResponse is the public representation of a share/repost.
type ShareResponse struct {
	ID        string  `json:"id"`
	UserID    string  `json:"userId"`
	PostID    string  `json:"postId"`
	Quote     *string `json:"quote"`
	CreatedAt string  `json:"createdAt"`
}

// ---------------------------------------------------------------------------
// Asset DTOs
// ---------------------------------------------------------------------------

// AssetResponse represents a ready (or pending) asset.
type AssetResponse struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	MimeType  string `json:"mimeType"`
	SizeBytes int64  `json:"sizeBytes"`
	Status    string `json:"status"`
}

// CreateAssetResponse is returned when a presigned upload URL is issued.
type CreateAssetResponse struct {
	AssetID         string `json:"assetId"`
	PresignedPutURL string `json:"presignedPutUrl"`
}

// CompleteAssetResponse is returned after an asset is marked ready.
type CompleteAssetResponse struct {
	AssetID string `json:"assetId"`
	URL     string `json:"url"`
}

// ---------------------------------------------------------------------------
// Auth DTOs
// ---------------------------------------------------------------------------

// VerifyCodeResponse is returned by POST /auth/verify.
type VerifyCodeResponse struct {
	Action            string `json:"action"` // "login" or "signup"
	NewUser           bool   `json:"new_user,omitempty"`
	AccessToken       string `json:"accessToken,omitempty"`
	RefreshToken      string `json:"refreshToken,omitempty"`
	SignupToken       string `json:"signupToken,omitempty"`
	AccessTokenSnake  string `json:"access_token,omitempty"`
	RefreshTokenSnake string `json:"refresh_token,omitempty"`
	SignupTokenSnake  string `json:"signup_token,omitempty"`
}

// SignupResponse is returned by POST /auth/signup.
type SignupResponse struct {
	AccessToken       string              `json:"accessToken"`
	RefreshToken      string              `json:"refreshToken"`
	User              PrivateUserResponse `json:"user"`
	AccessTokenSnake  string              `json:"access_token,omitempty"`
	RefreshTokenSnake string              `json:"refresh_token,omitempty"`
}

// TokenResponse is returned by POST /auth/refresh.
type TokenResponse struct {
	AccessToken       string `json:"accessToken"`
	RefreshToken      string `json:"refreshToken"`
	AccessTokenSnake  string `json:"access_token,omitempty"`
	RefreshTokenSnake string `json:"refresh_token,omitempty"`
}

// ---------------------------------------------------------------------------
// Paginated list wrappers
// ---------------------------------------------------------------------------

// UserListResponse wraps a paginated list of public user profiles.
type UserListResponse struct {
	Data       []PublicUserResponse `json:"data"`
	Pagination PaginationMeta       `json:"pagination"`
}

// PostListResponse wraps a paginated list of posts.
type PostListResponse struct {
	Data       []PostResponse `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

// CommentListResponse wraps a paginated list of comments.
type CommentListResponse struct {
	Data       []CommentResponse `json:"data"`
	Pagination PaginationMeta    `json:"pagination"`
}

// SearchResponse wraps paginated search results (mixed types).
type SearchResponse struct {
	Data       any            `json:"data"` // []PublicUserResponse | []PostResponse | []CommentResponse
	Pagination PaginationMeta `json:"pagination"`
}

// DecodeCursor decodes an opaque base64 keyset cursor into (createdAt, id).
func DecodeCursor(cursorStr string) (time.Time, string, error) {
	if cursorStr == "" {
		return time.Time{}, "", errors.New("empty cursor")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(cursorStr)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(cursorStr)
		if err != nil {
			decoded, err = base64.StdEncoding.DecodeString(cursorStr)
			if err != nil {
				return time.Time{}, "", fmt.Errorf("invalid cursor encoding: %w", err)
			}
		}
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("malformed cursor structure")
	}

	micros, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor timestamp: %w", err)
	}

	return time.UnixMicro(micros).UTC(), parts[1], nil
}

// WriteProblem writes an RFC 9457 application/problem+json response.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, detail string) {
	reqID := GetReqID(r.Context())
	if reqID == "" {
		var b [8]byte
		_, _ = rand.Read(b[:])
		reqID = "req_" + hex.EncodeToString(b[:])
	}

	var pType, title string
	switch status {
	case http.StatusBadRequest:
		pType = "https://api.spill.social/problems/bad-request"
		title = "Bad Request"
	case http.StatusUnauthorized:
		pType = "https://api.spill.social/problems/unauthorized"
		title = "Unauthorized"
	case http.StatusForbidden:
		pType = "https://api.spill.social/problems/forbidden"
		title = "Forbidden"
	case http.StatusNotFound:
		pType = "https://api.spill.social/problems/not-found"
		title = "Not Found"
	case http.StatusConflict:
		pType = "https://api.spill.social/problems/conflict"
		title = "Conflict"
	case http.StatusRequestEntityTooLarge:
		pType = "https://api.spill.social/problems/payload-too-large"
		title = "Payload Too Large"
	case http.StatusUnprocessableEntity:
		pType = "https://api.spill.social/problems/validation-error"
		title = "Unprocessable Entity"
	case http.StatusTooManyRequests:
		pType = "https://api.spill.social/problems/rate-limit-exceeded"
		title = "Too Many Requests"
	default:
		pType = "https://api.spill.social/problems/internal-error"
		title = "Internal Server Error"
		if detail == "" {
			detail = "An unexpected error occurred."
		}
	}

	problem := ProblemDetail{
		Type:      pType,
		Title:     title,
		Status:    status,
		Detail:    detail,
		RequestID: reqID,
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
