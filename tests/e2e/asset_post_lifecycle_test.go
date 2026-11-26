//go:build e2e

package e2e

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
)

// TestAssetUploadAndPostLifecycle covers Journey 2:
// POST /assets -> PUT to presignedPutUrl -> POST /assets/{id}/complete ->
// POST /posts with assetIds -> GET /posts/{id} (verify asset attached) ->
// PATCH /posts/{id} -> DELETE /posts/{id} -> GET /posts/{id} returns 404.
func TestAssetUploadAndPostLifecycle(t *testing.T) {
	env := newE2EEnv(t)
	user := env.registerUser(t, "j2")

	payload := bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47}, 32) // 128 bytes

	// 1. POST /assets
	resp := env.doJSON(t, user.Client, http.MethodPost, "/assets", user.AccessToken, map[string]any{
		"mimeType":  "image/png",
		"sizeBytes": len(payload),
		"kind":      "post_image",
	})
	requireStatus(t, resp, http.StatusCreated, "POST /assets")
	createAssetRes := decodeJSON[views.CreateAssetResponse](t, resp)
	if createAssetRes.AssetID == "" || createAssetRes.PresignedPutURL == "" {
		t.Fatalf("expected non-empty assetId and presignedPutUrl, got %+v", createAssetRes)
	}

	// 2. Upload object bytes to the issued presignedPutUrl (stubbed Supabase Storage server)
	putReq, err := http.NewRequest(http.MethodPut, createAssetRes.PresignedPutURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("failed to construct PUT request to presigned URL: %v", err)
	}
	putReq.Header.Set("Content-Type", "image/png")
	putResp, err := user.Client.Do(putReq)
	if err != nil {
		t.Fatalf("PUT to presigned URL failed: %v", err)
	}
	requireStatus(t, putResp, http.StatusOK, "PUT presignedPutUrl")
	_ = readBodyString(t, putResp)

	// 3. POST /assets/{id}/complete
	resp = env.doJSON(t, user.Client, http.MethodPost, "/assets/"+createAssetRes.AssetID+"/complete", user.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "POST /assets/{id}/complete")
	completeRes := decodeJSON[views.CompleteAssetResponse](t, resp)
	if completeRes.AssetID != createAssetRes.AssetID || completeRes.URL == "" {
		t.Fatalf("unexpected complete asset response: %+v", completeRes)
	}

	// 4. POST /posts with assetIds
	initialBody := "E2E Journey 2 post with PNG attachment"
	resp = env.doJSON(t, user.Client, http.MethodPost, "/posts", user.AccessToken, map[string]any{
		"body":     initialBody,
		"assetIds": []string{createAssetRes.AssetID},
	})
	requireStatus(t, resp, http.StatusCreated, "POST /posts")
	createdPost := decodeJSON[views.PostResponse](t, resp)
	if createdPost.ID == "" || createdPost.Body != initialBody {
		t.Fatalf("unexpected created post: %+v", createdPost)
	}
	if len(createdPost.Assets) != 1 || createdPost.Assets[0].ID != createAssetRes.AssetID {
		t.Fatalf("expected 1 attached asset %s on created post, got %+v", createAssetRes.AssetID, createdPost.Assets)
	}

	// 5. GET /posts/{id} and verify asset is attached
	resp = env.doJSON(t, user.Client, http.MethodGet, "/posts/"+createdPost.ID, user.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK, "GET /posts/{id}")
	fetchedPost := decodeJSON[views.PostResponse](t, resp)
	if fetchedPost.ID != createdPost.ID || fetchedPost.Body != initialBody {
		t.Fatalf("unexpected fetched post: %+v", fetchedPost)
	}
	if len(fetchedPost.Assets) != 1 {
		t.Fatalf("expected 1 asset on GET /posts/{id}, got %d", len(fetchedPost.Assets))
	}
	attached := fetchedPost.Assets[0]
	if attached.ID != createAssetRes.AssetID ||
		attached.URL != completeRes.URL ||
		attached.MimeType != "image/png" ||
		attached.SizeBytes != int64(len(payload)) ||
		attached.Status != "ready" {
		t.Fatalf("unexpected attached asset details: %+v (expected url=%q)", attached, completeRes.URL)
	}

	// 6. PATCH /posts/{id}
	updatedBody := "E2E Journey 2 updated post body"
	resp = env.doJSON(t, user.Client, http.MethodPatch, "/posts/"+createdPost.ID, user.AccessToken, map[string]string{
		"body": updatedBody,
	})
	requireStatus(t, resp, http.StatusOK, "PATCH /posts/{id}")
	updatedPost := decodeJSON[views.PostResponse](t, resp)
	if updatedPost.Body != updatedBody {
		t.Fatalf("expected updated body %q, got %q", updatedBody, updatedPost.Body)
	}
	if len(updatedPost.Assets) != 1 || updatedPost.Assets[0].ID != createAssetRes.AssetID {
		t.Fatalf("expected asset to remain attached after PATCH, got %+v", updatedPost.Assets)
	}

	// 7. DELETE /posts/{id}
	resp = env.doJSON(t, user.Client, http.MethodDelete, "/posts/"+createdPost.ID, user.AccessToken, nil)
	requireStatus(t, resp, http.StatusNoContent, "DELETE /posts/{id}")
	_ = readBodyString(t, resp)

	// 8. Verify GET /posts/{id} returns 404 afterward
	resp = env.doJSON(t, user.Client, http.MethodGet, "/posts/"+createdPost.ID, user.AccessToken, nil)
	requireStatus(t, resp, http.StatusNotFound, "GET /posts/{id} after delete")
	_ = readBodyString(t, resp)
}
