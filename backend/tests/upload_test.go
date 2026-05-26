package tests

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// generateTestPNG creates a small valid PNG image in memory.
// Used to upload a real image (not just random bytes) so that MIME detection passes.
func generateTestPNG() *bytes.Buffer {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	buf := new(bytes.Buffer)
	png.Encode(buf, img)
	return buf
}

// uploadFile is a helper that sends a multipart upload request.
// Returns the recorder so the test can inspect the response.
func uploadFile(t *testing.T, router http.Handler, token, visibility string, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	part.Write(content)
	writer.Close()

	path := "/api/upload"
	if visibility != "" {
		path += "?visibility=" + visibility
	}

	req, _ := http.NewRequest("POST", path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestUpload_Success uploads a valid PNG image.
func TestUpload_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	pngData := generateTestPNG().Bytes()
	w := uploadFile(t, router, token, "", "photo.png", pngData)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	id, ok := resp["id"].(string)
	if !ok || id == "" {
		t.Fatalf("response should contain id, got: %v", resp)
	}
	url, _ := resp["url"].(string)
	if url == "" {
		t.Fatalf("response should contain url")
	}
}

// TestUpload_NoAuth rejects unauthenticated uploads.
func TestUpload_NoAuth(t *testing.T) {
	router, _ := SetupTestEnv()

	pngData := generateTestPNG().Bytes()
	w := uploadFile(t, router, "", "", "photo.png", pngData)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestUpload_DisallowedMIME rejects a text file disguised as an image.
func TestUpload_DisallowedMIME(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	// Text content with .jpg extension — extension whitelist passes but MIME check should reject
	w := uploadFile(t, router, token, "", "fake.jpg", []byte("This is not an image"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-image content, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestUpload_BadExtension rejects unsupported extensions.
func TestUpload_BadExtension(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	w := uploadFile(t, router, token, "", "doc.txt", []byte("Hello"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for .txt extension, got %d", w.Code)
	}
}

// TestServeFile_PublicByOwner serves a public file to its owner.
func TestServeFile_PublicByOwner(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	pngData := generateTestPNG().Bytes()
	w := uploadFile(t, router, token, "public", "photo.png", pngData)
	if w.Code != http.StatusOK {
		t.Fatalf("upload failed: %s", w.Body.String())
	}
	fileID := parseJSON(t, w)["id"].(string)

	req := authRequest(t, "GET", "/api/files/"+fileID, token, nil)
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestServeFile_PublicByOther serves a public file to other authenticated users.
func TestServeFile_PublicByOther(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	_, bobToken := registerAndLogin(t, router, "bob")

	w := uploadFile(t, router, aliceToken, "public", "photo.png", generateTestPNG().Bytes())
	fileID := parseJSON(t, w)["id"].(string)

	// Bob accesses Alice's public file
	req := authRequest(t, "GET", "/api/files/"+fileID, bobToken, nil)
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (public access for authenticated user), got %d", w.Code)
	}
}

// TestServeFile_NoAuth rejects unauthenticated access.
func TestServeFile_NoAuth(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")
	w := uploadFile(t, router, token, "public", "photo.png", generateTestPNG().Bytes())
	fileID := parseJSON(t, w)["id"].(string)

	req := authRequest(t, "GET", "/api/files/"+fileID, "", nil)
	w = doRequest(router, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestServeFile_NotFound returns 404 for non-existent IDs.
func TestServeFile_NotFound(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	req := authRequest(t, "GET", "/api/files/00000000-0000-0000-0000-000000000000", token, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// TestServeFile_PrivateByOwner serves a private file to its owner.
func TestServeFile_PrivateByOwner(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")
	w := uploadFile(t, router, token, "private", "secret.png", generateTestPNG().Bytes())
	fileID := parseJSON(t, w)["id"].(string)

	req := authRequest(t, "GET", "/api/files/"+fileID, token, nil)
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for owner, got %d", w.Code)
	}
}

// TestServeFile_PrivateByOther rejects access from a non-granted user.
func TestServeFile_PrivateByOther(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	_, bobToken := registerAndLogin(t, router, "bob")

	w := uploadFile(t, router, aliceToken, "private", "secret.png", generateTestPNG().Bytes())
	fileID := parseJSON(t, w)["id"].(string)

	req := authRequest(t, "GET", "/api/files/"+fileID, bobToken, nil)
	w = doRequest(router, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-granted user, got %d", w.Code)
	}
}

// TestServeFile_FriendsByFriend serves a "friends" file to an accepted friend.
func TestServeFile_FriendsByFriend(t *testing.T) {
	router, _ := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, bobToken := registerAndLogin(t, router, "bob")

	// Make them friends
	doRequest(router, authRequest(t, "POST", "/api/friends/request/"+bobID, aliceToken, nil))
	doRequest(router, authRequest(t, "POST", "/api/friends/accept/"+aliceID, bobToken, nil))

	w := uploadFile(t, router, aliceToken, "friends", "shared.png", generateTestPNG().Bytes())
	fileID := parseJSON(t, w)["id"].(string)

	req := authRequest(t, "GET", "/api/files/"+fileID, bobToken, nil)
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for accepted friend, got %d", w.Code)
	}
}

// TestServeFile_FriendsByStranger rejects access from a non-friend.
func TestServeFile_FriendsByStranger(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	_, charlieToken := registerAndLogin(t, router, "charlie")

	w := uploadFile(t, router, aliceToken, "friends", "shared.png", generateTestPNG().Bytes())
	fileID := parseJSON(t, w)["id"].(string)

	req := authRequest(t, "GET", "/api/files/"+fileID, charlieToken, nil)
	w = doRequest(router, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-friend, got %d", w.Code)
	}
}

// TestUpload_InvalidVisibility rejects unknown visibility values.
func TestUpload_InvalidVisibility(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")
	w := uploadFile(t, router, token, "anyone", "photo.png", generateTestPNG().Bytes())

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}
