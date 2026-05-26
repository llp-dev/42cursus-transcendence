package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// registerAndLogin creates a new user with a unique username/email and returns
// (userID, jwtToken). Uses the same default strong password for all test users.
//
// If the test framework's parallelism could create duplicates, the nano timestamp
// suffix in unique makes collisions extremely unlikely.
func registerAndLogin(t *testing.T, router http.Handler, prefix string) (userID, token string) {
	t.Helper()

	unique := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	password := "StrongPass123!"

	// Register
	registerBody := fmt.Sprintf(`{
		"username": "%s",
		"email": "%s@test.com",
		"password": "%s",
		"dateOfBirth": "2000-01-01"
	}`, unique, unique, password)

	req, _ := http.NewRequest("POST", "/api/auth/register", bytes.NewBufferString(registerBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("register failed: status=%d body=%s", w.Code, w.Body.String())
	}

	var registerResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &registerResp); err != nil {
		t.Fatalf("invalid register response: %v", err)
	}
	id, ok := registerResp["id"].(string)
	if !ok || id == "" {
		t.Fatalf("register response should contain id")
	}

	// Login to get a JWT
	loginBody := fmt.Sprintf(`{"email": "%s@test.com", "password": "%s"}`, unique, password)
	req, _ = http.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login failed: status=%d body=%s", w.Code, w.Body.String())
	}

	var loginResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("invalid login response: %v", err)
	}
	tok, ok := loginResp["token"].(string)
	if !ok || tok == "" {
		t.Fatalf("login response should contain token")
	}

	return id, tok
}

// authRequest builds an authenticated HTTP request with optional JSON body.
// Pass nil body for GET/DELETE without payload.
func authRequest(t *testing.T, method, path, token string, body interface{}) *http.Request {
	t.Helper()

	var buf *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		buf = bytes.NewBuffer(data)
	} else {
		buf = bytes.NewBuffer(nil)
	}

	req, _ := http.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// doRequest sends the request and returns the recorder.
func doRequest(router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// parseJSON parses the recorder body as a map.
func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON response: body=%s err=%v", w.Body.String(), err)
	}
	return out
}
