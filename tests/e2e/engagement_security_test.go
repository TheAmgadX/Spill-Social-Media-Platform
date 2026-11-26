//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
)

// TestEngagementAndOwnershipSecurity covers Journey 4:
// User A likes (PUT), shares (POST), and comments (POST) on User B's post ->
// verify counts and GET /search discovery -> assert 403 on cross-user
// PATCH /posts/{id}, DELETE /posts/{id}, and DELETE /comments/{id} ->
// User B deletes the post cleanly.
func TestEngagementAndOwnershipSecurity(t *testing.T) {
	env := newE2EEnv(t)
	userA := env.registerUser(t, "j4a")
	userB := env.registerUser(t, "j4b")

	uniqueToken := newUniqueSuffix(t)
	postKeyword := "e2epost" + uniqueToken
	commentKeyword := "e2ecomment" + uniqueToken

	// User B creates a post
	resp := env.doJSON(t, userB.Client, http.MethodPost, "/posts", userB.AccessToken, map[string]string{
		"body": fmt.Sprintf("User B discovery post %s", postKeyword),
	})
	requireStatus(t, resp, http.StatusCreated, "User B POST /posts")
	post := decodeJSON[views.PostResponse](t, resp)

	// 1. User A likes (PUT) User B's post
	resp = env.doJSON(t, userA.Client, http.MethodPut, "/posts/"+post.ID+"/like", userA.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "User A PUT /posts/{id}/like")
	likeRes := decodeJSON[views.LikeStateResponse](t, resp)
	if !likeRes.Liked {
		t.Fatalf("expected liked=true, got false")
	}

	// 2. User A shares (POST) User B's post
	resp = env.doJSON(t, userA.Client, http.MethodPost, "/posts/"+post.ID+"/share", userA.AccessToken, map[string]string{
		"quote": "Sharing this post",
	})
	requireStatus(t, resp, http.StatusCreated, "User A POST /posts/{id}/share")
	shareRes := decodeJSON[views.ShareResponse](t, resp)
	if shareRes.PostID != post.ID || shareRes.UserID != userA.UserID {
		t.Fatalf("unexpected share response: %+v", shareRes)
	}

	// 3. User A comments (POST) on User B's post
	resp = env.doJSON(t, userA.Client, http.MethodPost, "/posts/"+post.ID+"/comments", userA.AccessToken, map[string]string{
		"body": fmt.Sprintf("User A comment %s", commentKeyword),
	})
	requireStatus(t, resp, http.StatusCreated, "User A POST /posts/{id}/comments")
	userAComment := decodeJSON[views.CommentResponse](t, resp)
	if userAComment.PostID != post.ID || userAComment.AuthorID != userA.UserID {
		t.Fatalf("unexpected comment response: %+v", userAComment)
	}

	// 4. Verify engagement counts via GET /posts/{id} (authenticated and anonymous)
	resp = env.doJSON(t, userA.Client, http.MethodGet, "/posts/"+post.ID, userA.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "GET /posts/{id} as User A")
	fetchedAuthed := decodeJSON[views.PostResponse](t, resp)
	if fetchedAuthed.LikeCount != 1 || fetchedAuthed.ShareCount != 1 || fetchedAuthed.CommentCount != 1 {
		t.Fatalf("expected counts (1,1,1), got like=%d share=%d comment=%d",
			fetchedAuthed.LikeCount, fetchedAuthed.ShareCount, fetchedAuthed.CommentCount)
	}
	if fetchedAuthed.Liked == nil || !*fetchedAuthed.Liked {
		t.Fatalf("expected liked=true for User A, got %v", fetchedAuthed.Liked)
	}

	resp = env.doJSON(t, userA.Client, http.MethodGet, "/posts/"+post.ID, "", nil)
	requireStatus(t, resp, http.StatusOK, "GET /posts/{id} anonymously")
	fetchedAnon := decodeJSON[views.PostResponse](t, resp)
	if fetchedAnon.Liked != nil {
		t.Fatalf("expected liked=nil for anonymous GET /posts/{id}, got %v", *fetchedAnon.Liked)
	}

	// 5. Verify GET /search discovery across posts, comments, and users
	resp = env.doJSON(t, userA.Client, http.MethodGet, "/search?q="+postKeyword+"&type=posts", userA.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "GET /search?type=posts")
	postSearch := decodeJSON[struct {
		Data []views.PostResponse `json:"data"`
	}](t, resp)
	if len(postSearch.Data) != 1 || postSearch.Data[0].ID != post.ID {
		t.Fatalf("expected post %s in search results, got %+v", post.ID, postSearch.Data)
	}
	if postSearch.Data[0].LikeCount != 1 || postSearch.Data[0].ShareCount != 1 || postSearch.Data[0].CommentCount != 1 {
		t.Fatalf("expected search result counts (1,1,1), got (%d,%d,%d)",
			postSearch.Data[0].LikeCount, postSearch.Data[0].ShareCount, postSearch.Data[0].CommentCount)
	}

	resp = env.doJSON(t, userA.Client, http.MethodGet, "/search?q="+commentKeyword+"&type=comments", "", nil)
	requireStatus(t, resp, http.StatusOK, "GET /search?type=comments")
	commentSearch := decodeJSON[struct {
		Data []views.CommentResponse `json:"data"`
	}](t, resp)
	if len(commentSearch.Data) != 1 || commentSearch.Data[0].ID != userAComment.ID {
		t.Fatalf("expected comment %s in search results, got %+v", userAComment.ID, commentSearch.Data)
	}

	resp = env.doJSON(t, userA.Client, http.MethodGet, "/search?q="+userB.Username+"&type=users", "", nil)
	requireStatus(t, resp, http.StatusOK, "GET /search?type=users")
	userSearch := decodeJSON[struct {
		Data []views.PublicUserResponse `json:"data"`
	}](t, resp)
	if len(userSearch.Data) != 1 || userSearch.Data[0].ID != userB.UserID {
		t.Fatalf("expected user %s in search results, got %+v", userB.UserID, userSearch.Data)
	}

	// User B also creates a comment on their post so we can assert User A gets 403 deleting User B's comment
	resp = env.doJSON(t, userB.Client, http.MethodPost, "/posts/"+post.ID+"/comments", userB.AccessToken, map[string]string{
		"body": "User B author reply",
	})
	requireStatus(t, resp, http.StatusCreated, "User B POST /posts/{id}/comments")
	userBComment := decodeJSON[views.CommentResponse](t, resp)

	// 6. Ownership Security (403 Forbidden)
	// User A cannot PATCH User B's post
	resp = env.doJSON(t, userA.Client, http.MethodPatch, "/posts/"+post.ID, userA.AccessToken, map[string]string{
		"body": "Unauthorized edit attempt",
	})
	requireStatus(t, resp, http.StatusForbidden, "User A PATCH User B post")
	_ = readBodyString(t, resp)

	// User A cannot DELETE User B's post
	resp = env.doJSON(t, userA.Client, http.MethodDelete, "/posts/"+post.ID, userA.AccessToken, nil)
	requireStatus(t, resp, http.StatusForbidden, "User A DELETE User B post")
	_ = readBodyString(t, resp)

	// User A cannot DELETE User B's comment
	resp = env.doJSON(t, userA.Client, http.MethodDelete, "/comments/"+userBComment.ID, userA.AccessToken, nil)
	requireStatus(t, resp, http.StatusForbidden, "User A DELETE User B comment")
	_ = readBodyString(t, resp)

	// User B (even as post owner) cannot DELETE User A's comment
	resp = env.doJSON(t, userB.Client, http.MethodDelete, "/comments/"+userAComment.ID, userB.AccessToken, nil)
	requireStatus(t, resp, http.StatusForbidden, "User B DELETE User A comment")
	_ = readBodyString(t, resp)

	// 7. User B deletes the post cleanly
	resp = env.doJSON(t, userB.Client, http.MethodDelete, "/posts/"+post.ID, userB.AccessToken, nil)
	requireStatus(t, resp, http.StatusNoContent, "User B DELETE own post")
	_ = readBodyString(t, resp)

	// Post is gone (404) and excluded from search
	resp = env.doJSON(t, userB.Client, http.MethodGet, "/posts/"+post.ID, userB.AccessToken, nil)
	requireStatus(t, resp, http.StatusNotFound, "GET /posts/{id} after User B delete")
	_ = readBodyString(t, resp)

	resp = env.doJSON(t, userB.Client, http.MethodGet, "/search?q="+postKeyword+"&type=posts", userB.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "GET /search?type=posts after delete")
	postSearchAfter := decodeJSON[struct {
		Data []views.PostResponse `json:"data"`
	}](t, resp)
	if len(postSearchAfter.Data) != 0 {
		t.Fatalf("expected soft-deleted post to be excluded from search, got %+v", postSearchAfter.Data)
	}
}
