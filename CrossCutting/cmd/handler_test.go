package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"crosscutting/middleware/auth"
	"crosscutting/middleware/logging"
	"crosscutting/middleware/ratelimit"
	"crosscutting/middleware/recovery"
	"crosscutting/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

// setupTestRouter builds the same Gin engine as main() but without Redis.
// The in‑memory token bucket is used for rate limiting.
func setupTestRouter(secret string) *gin.Engine {
	// Use a valid writer – discard logs during testing (we use t.Log instead)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Suppress Gin’s own debug output
	gin.SetMode(gin.TestMode)

	utils.RegisterCustomValidations()

	router := gin.New()

	router.Use(recovery.Recovery(logger))
	router.Use(logging.Logging(logger))

	// Use token bucket rate limiter only (no Redis)
	router.Use(ratelimit.TokenBucketRateLimit(rate.Limit(10), 20))

	authMiddleware := auth.Auth(auth.AuthConfig{
		JWTSecret:      secret,
		TokenLookup:    "header:Authorization",
		AuthScheme:     "Bearer",
		ExcludePaths:   []string{"/health"},
		RoleRequired:   false,
		AdminRoleValue: "admin",
	})

	router.GET("/health", healthHandler)

	api := router.Group("/api/v1")
	api.Use(authMiddleware)
	{
		api.GET("/profile", getProfile)
		api.POST("/orders", createOrder)

		admin := api.Group("/admin")
		admin.Use(auth.RequireRole("admin"))
		{
			admin.GET("/users", listUsers)
		}
	}

	return router
}

// generateTestJWT creates a signed JWT with the given sub and role.
func generateTestJWT(secret, sub, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  sub,
		"role": role,
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	// Log token details (partial) for debugging – never do this in production!
	// t.Logf(...) can’t be used here because we don’t have *testing.T.
	return signed, nil
}

// helper to log the request and perform it
func doRequest(t *testing.T, router *gin.Engine, method, url, token string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, url, strings.NewReader(body))
	} else {
		req, _ = http.NewRequest(method, url, nil)
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		t.Logf("➡️  %s %s (Authorization: Bearer ...%s)", method, url, token[len(token)-8:])
	} else {
		t.Logf("➡️  %s %s (no auth)", method, url)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
		t.Logf("   Body: %s", body)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	t.Logf("⬅️  Status: %d", w.Code)
	if w.Code >= 400 {
		var resp utils.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Error != nil {
			t.Logf("   Error: %s (%s)", resp.Error.Message, resp.Error.Code)
		}
	} else if w.Code == 200 || w.Code == 201 {
		var resp utils.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Data != nil {
			dataStr, _ := json.Marshal(resp.Data)
			t.Logf("   Data: %s", string(dataStr))
		}
	}
	return w
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter("test-secret")

	w := doRequest(t, router, "GET", "/health", "", "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	t.Log("✅ Health check passed")
}

func TestProfileWithoutToken(t *testing.T) {
	router := setupTestRouter("test-secret")

	w := doRequest(t, router, "GET", "/api/v1/profile", "", "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	t.Log("✅ Unauthorized when no token provided")
}

func TestProfileWithValidCustomerToken(t *testing.T) {
	secret := "test-secret"
	router := setupTestRouter(secret)

	token, err := generateTestJWT(secret, "customer-1", "customer")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("🔑 Generated JWT (sub=customer-1, role=customer)")

	w := doRequest(t, router, "GET", "/api/v1/profile", token, "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp utils.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatal("response should be successful")
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data missing")
	}
	if data["user_id"] != "customer-1" {
		t.Errorf("expected user_id 'customer-1', got %v", data["user_id"])
	}
	t.Log("✅ Customer can access profile, user_id = customer-1")
}

func TestCreateOrderInvalidPayload(t *testing.T) {
	secret := "test-secret"
	router := setupTestRouter(secret)

	token, err := generateTestJWT(secret, "customer-1", "customer")
	if err != nil {
		t.Fatal(err)
	}

	body := `{"product_sku":"bad","quantity":0,"price":-1}`
	w := doRequest(t, router, "POST", "/api/v1/orders", token, body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	t.Log("✅ Invalid order payload correctly rejected with 400")
}

func TestCreateOrderValidPayload(t *testing.T) {
	secret := "test-secret"
	router := setupTestRouter(secret)

	token, err := generateTestJWT(secret, "customer-1", "customer")
	if err != nil {
		t.Fatal(err)
	}

	body := `{"product_sku":"SKU12345","quantity":2,"price":29.99}`
	w := doRequest(t, router, "POST", "/api/v1/orders", token, body)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp utils.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatal("response should be successful")
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data missing")
	}
	if data["user_id"] != "customer-1" {
		t.Errorf("expected user_id 'customer-1', got %v", data["user_id"])
	}
	if data["order_id"] != "ord_123" {
		t.Errorf("expected order_id 'ord_123', got %v", data["order_id"])
	}
	t.Log("✅ Order created successfully, user_id = customer-1, order_id = ord_123")
}

func TestAdminEndpointWithCustomer(t *testing.T) {
	secret := "test-secret"
	router := setupTestRouter(secret)

	token, err := generateTestJWT(secret, "customer-1", "customer")
	if err != nil {
		t.Fatal(err)
	}

	w := doRequest(t, router, "GET", "/api/v1/admin/users", token, "")

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	t.Log("✅ Customer forbidden from admin endpoint")
}

func TestAdminEndpointWithAdmin(t *testing.T) {
	secret := "test-secret"
	router := setupTestRouter(secret)

	token, err := generateTestJWT(secret, "admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("🔑 Generated JWT (sub=admin-1, role=admin)")

	w := doRequest(t, router, "GET", "/api/v1/admin/users", token, "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	t.Log("✅ Admin can access admin endpoint")
}

func TestRateLimiting(t *testing.T) {
	secret := "test-secret"
	router := setupTestRouter(secret)

	token, err := generateTestJWT(secret, "customer-1", "customer")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("🎯 Sending 21 requests (burst = 20) to test rate limiting...")

	for i := 0; i < 21; i++ {
		w := doRequest(t, router, "GET", "/api/v1/profile", token, "")
		if i < 20 {
			if w.Code != http.StatusOK {
				t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
			}
			t.Logf("   request %d: 200 OK", i+1)
		} else {
			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d: expected 429, got %d", i+1, w.Code)
			}
			t.Logf("   request %d: 429 Rate limit exceeded", i+1)
		}
	}
	t.Log("✅ Rate limiting correctly triggered after burst limit")
}
