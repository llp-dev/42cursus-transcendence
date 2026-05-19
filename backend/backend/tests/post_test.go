package tests

import (
	"net/http"
	"testing"
)

// helper to create a post and return its ID
func createPost(t *testing.T, router http.Handler, token, content string) string {
	t.Helper()

	req := authRequest(t, "POST", "/api/posts", token, map[string]string{"content": content})
	w := doRequest(router, req)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("createPost failed: %d body=%s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	id, ok := resp["id"].(string)
	if !ok || id == "" {
		t.Fatalf("createPost: response should contain id")
	}
	return id
}

// TestCreatePost_Success verifies that an authenticated user can create a post.
func TestCreatePost_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	id := createPost(t, router, token, "Hello world")
	if id == "" {
		t.Fatalf("expected non-empty post ID")
	}
}

// TestCreatePost_NoAuth verifies that anonymous users can't create posts.
func TestCreatePost_NoAuth(t *testing.T) {
	router, _ := SetupTestEnv()

	req := authRequest(t, "POST", "/api/posts", "", map[string]string{"content": "Hello"})
	w := doRequest(router, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestCreatePost_MissingContent rejects empty content.
func TestCreatePost_MissingContent(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	req := authRequest(t, "POST", "/api/posts", token, map[string]string{"content": ""})
	w := doRequest(router, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty content, got %d", w.Code)
	}
}

// TestGetPosts_Pagination retrieves the paginated list.
func TestGetPosts_Pagination(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	for i := 0; i < 3; i++ {
		createPost(t, router, token, "Post content")
	}

	req := authRequest(t, "GET", "/api/posts?page=1&limit=10", "", nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := parseJSON(t, w)
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array")
	}
	if len(data) < 3 {
		t.Fatalf("expected at least 3 posts, got %d", len(data))
	}
	if _, ok := resp["total"]; !ok {
		t.Fatalf("response should include total")
	}
}

// TestGetPostsByUser returns posts of a specific user.
func TestGetPostsByUser_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	_, bobToken := registerAndLogin(t, router, "bob")

	createPost(t, router, aliceToken, "Alice's post")
	createPost(t, router, bobToken, "Bob's post")

	req := authRequest(t, "GET", "/api/posts/user/"+aliceID, "", nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("expected 1 post for Alice, got %d", len(data))
	}
}

// TestUpdatePost_Success updates the user's own post.
func TestUpdatePost_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")
	postID := createPost(t, router, token, "Original")

	req := authRequest(t, "PUT", "/api/posts/"+postID, token, map[string]string{"content": "Updated content"})
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestUpdatePost_NotOwner rejects updates by non-owners.
func TestUpdatePost_NotOwner(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	_, bobToken := registerAndLogin(t, router, "bob")

	postID := createPost(t, router, aliceToken, "Alice's post")

	req := authRequest(t, "PUT", "/api/posts/"+postID, bobToken, map[string]string{"content": "hacked"})
	w := doRequest(router, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// TestDeletePost_Success deletes the user's own post.
func TestDeletePost_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")
	postID := createPost(t, router, token, "to delete")

	req := authRequest(t, "DELETE", "/api/posts/"+postID, token, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestDeletePost_NotOwner rejects deletion by non-owners.
func TestDeletePost_NotOwner(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	_, bobToken := registerAndLogin(t, router, "bob")

	postID := createPost(t, router, aliceToken, "Alice's post")

	req := authRequest(t, "DELETE", "/api/posts/"+postID, bobToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// TestToggleLike_AddRemove tests the like toggle (add then remove).
func TestToggleLike_AddRemove(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	_, bobToken := registerAndLogin(t, router, "bob")

	postID := createPost(t, router, aliceToken, "Like me!")

	// Bob likes
	req := authRequest(t, "POST", "/api/posts/"+postID+"/like", bobToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on first like, got %d", w.Code)
	}

	// Bob unlikes (toggle)
	req = authRequest(t, "POST", "/api/posts/"+postID+"/like", bobToken, nil)
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on second like (unlike), got %d", w.Code)
	}
}

// TestCreateComment_Success adds a comment to a post.
func TestCreateComment_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	_, bobToken := registerAndLogin(t, router, "bob")

	postID := createPost(t, router, aliceToken, "Discuss!")

	req := authRequest(t, "POST", "/api/posts/"+postID+"/comments", bobToken,
		map[string]string{"content": "Nice post!"})
	w := doRequest(router, req)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestGetComments_Success lists comments for a post.
func TestGetComments_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	_, bobToken := registerAndLogin(t, router, "bob")

	postID := createPost(t, router, aliceToken, "Discuss!")
	doRequest(router, authRequest(t, "POST", "/api/posts/"+postID+"/comments", bobToken,
		map[string]string{"content": "Nice!"}))

	req := authRequest(t, "GET", "/api/posts/"+postID+"/comments", "", nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
