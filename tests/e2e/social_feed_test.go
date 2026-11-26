//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
)

// TestSocialGraphAndFeedPropagation covers Journey 3:
// Create User A and User B -> User A follows User B -> verify /followers & /following ->
// User B creates post -> verify post appears in User A's GET /feed ->
// User A unfollows User B -> verify feed updates and post is removed.
func TestSocialGraphAndFeedPropagation(t *testing.T) {
	env := newE2EEnv(t)
	userA := env.registerUser(t, "j3a")
	userB := env.registerUser(t, "j3b")

	// 1. User A calls PUT /users/{B}/follow
	resp := env.doJSON(t, userA.Client, http.MethodPut, "/users/"+userB.UserID+"/follow", userA.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "PUT /users/{B}/follow")
	followRes := decodeJSON[views.FollowStateResponse](t, resp)
	if !followRes.Following {
		t.Fatalf("expected following=true, got false")
	}

	// 2. Verify GET /users/{B}/followers includes User A
	resp = env.doJSON(t, userA.Client, http.MethodGet, "/users/"+userB.UserID+"/followers", "", nil)
	requireStatus(t, resp, http.StatusOK, "GET /users/{B}/followers")
	followersRes := decodeJSON[views.UserListResponse](t, resp)
	if len(followersRes.Data) != 1 || followersRes.Data[0].ID != userA.UserID || followersRes.Data[0].Username != userA.Username {
		t.Fatalf("expected User A (%s) in User B's followers, got %+v", userA.UserID, followersRes.Data)
	}

	// Verify GET /users/{A}/following includes User B
	resp = env.doJSON(t, userA.Client, http.MethodGet, "/users/"+userA.UserID+"/following", "", nil)
	requireStatus(t, resp, http.StatusOK, "GET /users/{A}/following")
	followingRes := decodeJSON[views.UserListResponse](t, resp)
	if len(followingRes.Data) != 1 || followingRes.Data[0].ID != userB.UserID || followingRes.Data[0].Username != userB.Username {
		t.Fatalf("expected User B (%s) in User A's following, got %+v", userB.UserID, followingRes.Data)
	}
	if followingRes.Data[0].FollowerCount != 1 {
		t.Fatalf("expected User B followerCount=1, got %d", followingRes.Data[0].FollowerCount)
	}

	// 3. User B creates a post
	postBody := "Hello followers from User B"
	resp = env.doJSON(t, userB.Client, http.MethodPost, "/posts", userB.AccessToken, map[string]string{
		"body": postBody,
	})
	requireStatus(t, resp, http.StatusCreated, "User B POST /posts")
	createdPost := decodeJSON[views.PostResponse](t, resp)

	// 4. Verify the post appears in User A's GET /feed
	resp = env.doJSON(t, userA.Client, http.MethodGet, "/feed", userA.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "User A GET /feed while following B")
	feedRes := decodeJSON[views.PostListResponse](t, resp)
	if !containsPost(feedRes.Data, createdPost.ID) {
		t.Fatalf("expected post %s in User A's feed, got %+v", createdPost.ID, feedRes.Data)
	}

	// 5. User A calls DELETE /users/{B}/follow
	resp = env.doJSON(t, userA.Client, http.MethodDelete, "/users/"+userB.UserID+"/follow", userA.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "DELETE /users/{B}/follow")
	unfollowRes := decodeJSON[views.FollowStateResponse](t, resp)
	if unfollowRes.Following {
		t.Fatalf("expected following=false after unfollow, got true")
	}

	// 6. Verify User A's GET /feed updates and no longer contains User B's post
	resp = env.doJSON(t, userA.Client, http.MethodGet, "/feed", userA.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "User A GET /feed after unfollowing B")
	feedAfter := decodeJSON[views.PostListResponse](t, resp)
	if containsPost(feedAfter.Data, createdPost.ID) {
		t.Fatalf("expected post %s to be absent from User A's feed after unfollow", createdPost.ID)
	}

	// Also verify follower/following lists and counters reflect the unfollow
	resp = env.doJSON(t, userA.Client, http.MethodGet, "/users/"+userB.UserID+"/followers", "", nil)
	requireStatus(t, resp, http.StatusOK, "GET /users/{B}/followers after unfollow")
	followersAfter := decodeJSON[views.UserListResponse](t, resp)
	if len(followersAfter.Data) != 0 {
		t.Fatalf("expected 0 followers for User B after unfollow, got %d", len(followersAfter.Data))
	}

	resp = env.doJSON(t, userA.Client, http.MethodGet, "/users/"+userA.UserID+"/following", "", nil)
	requireStatus(t, resp, http.StatusOK, "GET /users/{A}/following after unfollow")
	followingAfter := decodeJSON[views.UserListResponse](t, resp)
	if len(followingAfter.Data) != 0 {
		t.Fatalf("expected 0 following for User A after unfollow, got %d", len(followingAfter.Data))
	}
}

func containsPost(posts []views.PostResponse, postID string) bool {
	for _, p := range posts {
		if p.ID == postID {
			return true
		}
	}
	return false
}
