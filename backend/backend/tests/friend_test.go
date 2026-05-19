package tests

import (
	"net/http"
	"testing"
)

// TestFriendRequest_Success verifies that sending a friend request creates
// a pending row in the friends table.
func TestFriendRequest_Success(t *testing.T) {
	router, db := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, _ := registerAndLogin(t, router, "bob")

	req := authRequest(t, "POST", "/api/friends/request/"+bobID, aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	// Verify in DB
	var count int64
	db.Table("friends").
		Where("user_id = ? AND friend_id = ? AND status = ?", aliceID, bobID, "pending").
		Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 pending row, got %d", count)
	}
}

// TestFriendRequest_NoAuth verifies that an unauthenticated request returns 401.
func TestFriendRequest_NoAuth(t *testing.T) {
	router, _ := SetupTestEnv()
	_, _ = registerAndLogin(t, router, "alice") // Just to have someone in DB
	bobID, _ := registerAndLogin(t, router, "bob")

	req := authRequest(t, "POST", "/api/friends/request/"+bobID, "", nil)
	w := doRequest(router, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestFriendAccept_Success verifies that accepting a pending request
// changes the status to "accepted".
func TestFriendAccept_Success(t *testing.T) {
	router, db := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, bobToken := registerAndLogin(t, router, "bob")

	// Alice sends request to Bob
	doRequest(router, authRequest(t, "POST", "/api/friends/request/"+bobID, aliceToken, nil))

	// Bob accepts
	req := authRequest(t, "POST", "/api/friends/accept/"+aliceID, bobToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var count int64
	db.Table("friends").
		Where("user_id = ? AND friend_id = ? AND status = ?", aliceID, bobID, "accepted").
		Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 accepted row, got %d", count)
	}
}

// TestFriendReject_Success verifies that rejecting a request deletes the row.
func TestFriendReject_Success(t *testing.T) {
	router, db := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, bobToken := registerAndLogin(t, router, "bob")

	doRequest(router, authRequest(t, "POST", "/api/friends/request/"+bobID, aliceToken, nil))

	req := authRequest(t, "POST", "/api/friends/reject/"+aliceID, bobToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var count int64
	db.Table("friends").
		Where("user_id = ? AND friend_id = ?", aliceID, bobID).
		Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 rows after reject, got %d", count)
	}
}

// TestFollow_Success verifies that following a user creates a follow row.
func TestFollow_Success(t *testing.T) {
	router, db := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, _ := registerAndLogin(t, router, "bob")

	req := authRequest(t, "POST", "/api/friends/follow/"+bobID, aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var count int64
	db.Table("friends").
		Where("user_id = ? AND friend_id = ? AND status = ?", aliceID, bobID, "follow").
		Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 follow row, got %d", count)
	}
}

// TestFollow_AlreadyFollowing rejects a duplicate follow.
func TestFollow_AlreadyFollowing(t *testing.T) {
	router, _ := SetupTestEnv()

	_, aliceToken := registerAndLogin(t, router, "alice")
	bobID, _ := registerAndLogin(t, router, "bob")

	// First follow OK
	doRequest(router, authRequest(t, "POST", "/api/friends/follow/"+bobID, aliceToken, nil))

	// Second one should fail
	req := authRequest(t, "POST", "/api/friends/follow/"+bobID, aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestFollow_CompatibleWithFriendship verifies that an existing friendship
// does NOT block a follow (the bug we fixed earlier).
func TestFollow_CompatibleWithFriendship(t *testing.T) {
	router, _ := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, bobToken := registerAndLogin(t, router, "bob")

	// Make them friends first
	doRequest(router, authRequest(t, "POST", "/api/friends/request/"+bobID, aliceToken, nil))
	doRequest(router, authRequest(t, "POST", "/api/friends/accept/"+aliceID, bobToken, nil))

	// Now Alice should still be able to follow Bob
	req := authRequest(t, "POST", "/api/friends/follow/"+bobID, aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (follow + accepted should coexist), got %d body=%s",
			w.Code, w.Body.String())
	}
}

// TestUnfollow_Success removes the follow row.
func TestUnfollow_Success(t *testing.T) {
	router, db := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, _ := registerAndLogin(t, router, "bob")

	doRequest(router, authRequest(t, "POST", "/api/friends/follow/"+bobID, aliceToken, nil))

	req := authRequest(t, "DELETE", "/api/friends/follow/"+bobID, aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var count int64
	db.Table("friends").
		Where("user_id = ? AND friend_id = ? AND status = ?", aliceID, bobID, "follow").
		Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 follow rows after unfollow, got %d", count)
	}
}

// TestRemoveFriend deletes accepted friendship in either direction.
func TestRemoveFriend_Success(t *testing.T) {
	router, db := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, bobToken := registerAndLogin(t, router, "bob")

	// Make them friends
	doRequest(router, authRequest(t, "POST", "/api/friends/request/"+bobID, aliceToken, nil))
	doRequest(router, authRequest(t, "POST", "/api/friends/accept/"+aliceID, bobToken, nil))

	// Bob removes Alice (the row was created with user_id=Alice, friend_id=Bob)
	req := authRequest(t, "DELETE", "/api/friends/"+aliceID, bobToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var count int64
	db.Table("friends").
		Where("status = ? AND ((user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?))",
			"accepted", aliceID, bobID, bobID, aliceID).
		Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 accepted rows after remove, got %d", count)
	}
}

// TestGetFollowers_Success lists who follows a user.
func TestGetFollowers_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, bobToken := registerAndLogin(t, router, "bob")

	// Bob follows Alice
	doRequest(router, authRequest(t, "POST", "/api/friends/follow/"+aliceID, bobToken, nil))

	// Get Alice's followers
	req := authRequest(t, "GET", "/api/users/"+aliceID+"/followers", aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("response should contain 'data' array, got: %v", resp)
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 follower, got %d", len(data))
	}
	follower := data[0].(map[string]interface{})
	if follower["id"] != bobID {
		t.Fatalf("expected follower to be bob (%s), got %v", bobID, follower["id"])
	}
}

// TestGetFollowing_Success lists who a user follows.
func TestGetFollowing_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, _ := registerAndLogin(t, router, "bob")

	doRequest(router, authRequest(t, "POST", "/api/friends/follow/"+bobID, aliceToken, nil))

	req := authRequest(t, "GET", "/api/users/"+aliceID+"/following", aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("expected 1 following, got %d", len(data))
	}
	if data[0].(map[string]interface{})["id"] != bobID {
		t.Fatalf("expected following to be bob")
	}
}

// TestGetFriends_Success lists accepted friends in both directions.
func TestGetFriends_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	bobID, bobToken := registerAndLogin(t, router, "bob")

	// Make them friends
	doRequest(router, authRequest(t, "POST", "/api/friends/request/"+bobID, aliceToken, nil))
	doRequest(router, authRequest(t, "POST", "/api/friends/accept/"+aliceID, bobToken, nil))

	// Alice gets her friends
	req := authRequest(t, "GET", "/api/users/"+aliceID+"/friends", aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("expected 1 friend, got %d", len(data))
	}
}

// TestUserResponse_FollowersCount verifies that GetUser returns counts.
func TestUserResponse_FollowersCount(t *testing.T) {
	router, _ := SetupTestEnv()

	aliceID, aliceToken := registerAndLogin(t, router, "alice")
	_, bobToken := registerAndLogin(t, router, "bob")
	_, charlieToken := registerAndLogin(t, router, "charlie")

	// Bob and Charlie follow Alice
	doRequest(router, authRequest(t, "POST", "/api/friends/follow/"+aliceID, bobToken, nil))
	doRequest(router, authRequest(t, "POST", "/api/friends/follow/"+aliceID, charlieToken, nil))

	// Get Alice profile
	req := authRequest(t, "GET", "/api/users/"+aliceID, aliceToken, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := parseJSON(t, w)
	followersCount, ok := resp["followers_count"].(float64) // JSON numbers are float64
	if !ok {
		t.Fatalf("response should contain followers_count, got: %v", resp)
	}
	if int(followersCount) != 2 {
		t.Fatalf("expected 2 followers, got %d", int(followersCount))
	}
}
