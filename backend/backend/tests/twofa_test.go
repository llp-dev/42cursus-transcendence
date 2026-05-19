package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// TestTwoFA_Setup_Success verifies that calling Setup returns secret + QR code URL.
func TestTwoFA_Setup_Success(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	req := authRequest(t, "POST", "/api/2fa/setup", token, nil)
	w := doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	secret, ok := resp["secret"].(string)
	if !ok || secret == "" {
		t.Fatalf("response should contain non-empty secret, got: %v", resp)
	}
	qrURL, ok := resp["qr_code_url"].(string)
	if !ok || qrURL == "" {
		t.Fatalf("response should contain non-empty qr_code_url")
	}
}

// TestTwoFA_Setup_NoAuth verifies 401 without authentication.
func TestTwoFA_Setup_NoAuth(t *testing.T) {
	router, _ := SetupTestEnv()

	req := authRequest(t, "POST", "/api/2fa/setup", "", nil)
	w := doRequest(router, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestTwoFA_Enable_Success activates 2FA on the account.
func TestTwoFA_Enable_Success(t *testing.T) {
	router, db := SetupTestEnv()

	userID, token := registerAndLogin(t, router, "alice")

	// Setup gets the secret
	w := doRequest(router, authRequest(t, "POST", "/api/2fa/setup", token, nil))
	resp := parseJSON(t, w)
	secret := resp["secret"].(string)

	// Generate a valid TOTP code for the current time
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code: %v", err)
	}

	// Enable
	req := authRequest(t, "POST", "/api/2fa/enable", token, map[string]string{"code": code})
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	// Verify DB
	var enabled bool
	db.Table("users").Select("two_fa_enabled").Where("id = ?", userID).Scan(&enabled)
	if !enabled {
		t.Fatalf("expected two_fa_enabled = true in DB")
	}
}

// TestTwoFA_Enable_BadCode rejects an invalid TOTP code.
func TestTwoFA_Enable_BadCode(t *testing.T) {
	router, db := SetupTestEnv()

	userID, token := registerAndLogin(t, router, "alice")

	doRequest(router, authRequest(t, "POST", "/api/2fa/setup", token, nil))

	req := authRequest(t, "POST", "/api/2fa/enable", token, map[string]string{"code": "000000"})
	w := doRequest(router, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Verify DB stayed unchanged
	var enabled bool
	db.Table("users").Select("two_fa_enabled").Where("id = ?", userID).Scan(&enabled)
	if enabled {
		t.Fatalf("expected two_fa_enabled to remain false after bad code")
	}
}

// TestTwoFA_Enable_InvalidFormat rejects codes that are not 6 digits.
func TestTwoFA_Enable_InvalidFormat(t *testing.T) {
	router, _ := SetupTestEnv()

	_, token := registerAndLogin(t, router, "alice")

	cases := []struct {
		name string
		code string
	}{
		{"too short", "12345"},
		{"too long", "1234567"},
		{"not numeric", "abcdef"},
		{"empty", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := authRequest(t, "POST", "/api/2fa/enable", token, map[string]string{"code": tc.code})
			w := doRequest(router, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400, got %d", tc.name, w.Code)
			}
		})
	}
}

// TestTwoFA_Login_TriggersFlow verifies that a 2FA-enabled user gets a pending_token
// instead of a JWT on login.
func TestTwoFA_Login_TriggersFlow(t *testing.T) {
	router, db := SetupTestEnv()

	userID, token := registerAndLogin(t, router, "alice")

	// Activate 2FA
	w := doRequest(router, authRequest(t, "POST", "/api/2fa/setup", token, nil))
	secret := parseJSON(t, w)["secret"].(string)
	code, _ := totp.GenerateCode(secret, time.Now())
	doRequest(router, authRequest(t, "POST", "/api/2fa/enable", token, map[string]string{"code": code}))

	// Fetch the email used by registerAndLogin (helpers append the unique suffix)
	var email string
	db.Table("users").Select("email").Where("id = ?", userID).Scan(&email)

	// Now login with email + password
	loginBody := map[string]string{"email": email, "password": "StrongPass123!"}
	req := authRequest(t, "POST", "/api/auth/login", "", loginBody)
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from login, got %d body=%s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	needs2FA, _ := resp["needs_2fa"].(bool)
	if !needs2FA {
		t.Fatalf("expected needs_2fa = true, got: %v", resp)
	}
	pendingToken, _ := resp["pending_token"].(string)
	if pendingToken == "" {
		t.Fatalf("expected non-empty pending_token")
	}
	if _, hasJWT := resp["token"]; hasJWT {
		t.Fatalf("login response should NOT contain JWT when 2FA is required")
	}
}

// TestTwoFA_Verify_Success completes a 2-step login and returns the real JWT.
func TestTwoFA_Verify_Success(t *testing.T) {
	router, db := SetupTestEnv()

	userID, token := registerAndLogin(t, router, "alice")

	// Activate 2FA
	w := doRequest(router, authRequest(t, "POST", "/api/2fa/setup", token, nil))
	secret := parseJSON(t, w)["secret"].(string)
	code, _ := totp.GenerateCode(secret, time.Now())
	doRequest(router, authRequest(t, "POST", "/api/2fa/enable", token, map[string]string{"code": code}))

	// Login → get pending_token
	var email string
	db.Table("users").Select("email").Where("id = ?", userID).Scan(&email)
	loginBody := map[string]string{"email": email, "password": "StrongPass123!"}
	w = doRequest(router, authRequest(t, "POST", "/api/auth/login", "", loginBody))
	pendingToken := parseJSON(t, w)["pending_token"].(string)

	// Generate a fresh code (the previous one was used for Enable)
	verifyCode, _ := totp.GenerateCode(secret, time.Now())

	verifyBody := map[string]string{
		"pending_token": pendingToken,
		"code":          verifyCode,
	}
	req := authRequest(t, "POST", "/api/auth/2fa/verify", "", verifyBody)
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	if _, ok := resp["token"].(string); !ok {
		t.Fatalf("verify response should contain JWT, got: %v", resp)
	}
}

// TestTwoFA_Verify_BadCode rejects an invalid TOTP code during verify.
func TestTwoFA_Verify_BadCode(t *testing.T) {
	router, db := SetupTestEnv()

	userID, token := registerAndLogin(t, router, "alice")

	// Activate 2FA
	w := doRequest(router, authRequest(t, "POST", "/api/2fa/setup", token, nil))
	secret := parseJSON(t, w)["secret"].(string)
	code, _ := totp.GenerateCode(secret, time.Now())
	doRequest(router, authRequest(t, "POST", "/api/2fa/enable", token, map[string]string{"code": code}))

	var email string
	db.Table("users").Select("email").Where("id = ?", userID).Scan(&email)
	loginBody := map[string]string{"email": email, "password": "StrongPass123!"}
	w = doRequest(router, authRequest(t, "POST", "/api/auth/login", "", loginBody))
	pendingToken := parseJSON(t, w)["pending_token"].(string)

	verifyBody := map[string]string{
		"pending_token": pendingToken,
		"code":          "000000",
	}
	req := authRequest(t, "POST", "/api/auth/2fa/verify", "", verifyBody)
	w = doRequest(router, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestTwoFA_Verify_PendingTokenReplay rejects a second use of the same pending_token.
func TestTwoFA_Verify_PendingTokenReplay(t *testing.T) {
	router, db := SetupTestEnv()

	userID, token := registerAndLogin(t, router, "alice")

	// Activate 2FA
	w := doRequest(router, authRequest(t, "POST", "/api/2fa/setup", token, nil))
	secret := parseJSON(t, w)["secret"].(string)
	code, _ := totp.GenerateCode(secret, time.Now())
	doRequest(router, authRequest(t, "POST", "/api/2fa/enable", token, map[string]string{"code": code}))

	var email string
	db.Table("users").Select("email").Where("id = ?", userID).Scan(&email)
	loginBody := map[string]string{"email": email, "password": "StrongPass123!"}
	w = doRequest(router, authRequest(t, "POST", "/api/auth/login", "", loginBody))
	pendingToken := parseJSON(t, w)["pending_token"].(string)

	// First verify with WRONG code (consumes the pending_token)
	doRequest(router, authRequest(t, "POST", "/api/auth/2fa/verify", "",
		map[string]string{"pending_token": pendingToken, "code": "000000"}))

	// Second verify even with a GOOD code should fail (token already consumed)
	goodCode, _ := totp.GenerateCode(secret, time.Now())
	req := authRequest(t, "POST", "/api/auth/2fa/verify", "",
		map[string]string{"pending_token": pendingToken, "code": goodCode})
	w = doRequest(router, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on replay, got %d", w.Code)
	}
}

// TestTwoFA_Disable_Success disables 2FA with a valid code.
func TestTwoFA_Disable_Success(t *testing.T) {
	router, db := SetupTestEnv()

	userID, token := registerAndLogin(t, router, "alice")

	// Activate 2FA
	w := doRequest(router, authRequest(t, "POST", "/api/2fa/setup", token, nil))
	secret := parseJSON(t, w)["secret"].(string)
	code, _ := totp.GenerateCode(secret, time.Now())
	doRequest(router, authRequest(t, "POST", "/api/2fa/enable", token, map[string]string{"code": code}))

	// Now disable
	disableCode, _ := totp.GenerateCode(secret, time.Now())
	req := authRequest(t, "POST", "/api/2fa/disable", token, map[string]string{"code": disableCode})
	w = doRequest(router, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	// Verify DB
	var enabled bool
	var secretStored *string
	db.Table("users").Select("two_fa_enabled, two_fa_secret").Where("id = ?", userID).Row().Scan(&enabled, &secretStored)
	if enabled {
		t.Fatalf("expected two_fa_enabled = false after disable")
	}
	if secretStored != nil {
		t.Fatalf("expected two_fa_secret = NULL after disable, got %v", *secretStored)
	}
}

// TestTwoFA_Disable_BadCode keeps 2FA enabled if the code is invalid.
func TestTwoFA_Disable_BadCode(t *testing.T) {
	router, db := SetupTestEnv()

	userID, token := registerAndLogin(t, router, "alice")

	// Activate
	w := doRequest(router, authRequest(t, "POST", "/api/2fa/setup", token, nil))
	secret := parseJSON(t, w)["secret"].(string)
	code, _ := totp.GenerateCode(secret, time.Now())
	doRequest(router, authRequest(t, "POST", "/api/2fa/enable", token, map[string]string{"code": code}))

	// Try to disable with bad code
	req := authRequest(t, "POST", "/api/2fa/disable", token, map[string]string{"code": "000000"})
	w = doRequest(router, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Verify 2FA still enabled
	var enabled bool
	db.Table("users").Select("two_fa_enabled").Where("id = ?", userID).Scan(&enabled)
	if !enabled {
		t.Fatalf("expected 2FA to remain enabled after bad disable code")
	}
}
