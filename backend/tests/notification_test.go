package tests

import (
	"net/http"
	"testing"
)

// TestGetUnread_Empty verifies that a fresh user has 0 notifications.
func TestGetUnread_Empty(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")

	req := authRequest(t, "GET", "/api/notification", aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	total, ok := resp["total"].(float64)
	if !ok {
		t.Fatalf("response should contain total, got: %v", resp)
	}
	if int(total) != 0 {
		t.Fatalf("expected 0 notifications, got %d", int(total))
	}
}

// TestGetUnread_NoAuth verifies that an unauthenticated request returns 401.
func TestGetUnread_NoAuth(t *testing.T) {
	router, _ := SetupTestEnv()

	req := authRequest(t, "GET", "/api/notification", "", nil)
	w := doRequest(router, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestNotification_OnFriendRequest verifies that a friend request creates
// a notification for the receiver.
func TestNotification_OnFriendRequest(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	bobID, bobToken := registerAndLogin(t, router, "bob")

	// Alice sends a friend request to Bob
	w := doRequest(router, authRequest(t, "POST", "/api/friends/request/"+bobID, aliceToken, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("friend request failed: %s", w.Body.String())
	}

	// Bob should now have a notification
	req := authRequest(t, "GET", "/api/notification", bobToken, nil)
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := parseJSON(t, w)
	total := int(resp["total"].(float64))
	if total < 1 {
		t.Fatalf("expected at least 1 notification for Bob after Alice's friend request, got %d", total)
	}

	data, ok := resp["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Fatalf("expected notifications data array")
	}

	notif := data[0].(map[string]interface{})
	if notif["type"] != "friend_request" {
		t.Fatalf("expected notification type 'friend_request', got %v", notif["type"])
	}
}

// TestMarkAllRead_Success marks all notifications as read.
func TestMarkAllRead_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	bobID, bobToken := registerAndLogin(t, router, "bob")

	// Trigger a notification for Bob
	doRequest(router, authRequest(t, "POST", "/api/friends/request/"+bobID, aliceToken, nil))

	// Bob marks all as read (route is PATCH)
	req := authRequest(t, "PATCH", "/api/notification/read", bobToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	// Bob should now have 0 unread
	w = doRequest(router, authRequest(t, "GET", "/api/notification", bobToken, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseJSON(t, w)
	total := int(resp["total"].(float64))
	if total != 0 {
		t.Fatalf("expected 0 unread after MarkAllRead, got %d", total)
	}
}

// TestMarkAllRead_NoAuth returns 401 without authentication.
func TestMarkAllRead_NoAuth(t *testing.T) {
	router, _ := SetupTestEnv()

	req := authRequest(t, "PATCH", "/api/notification/read", "", nil)
	w := doRequest(router, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
