package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/entropyGen/entropyGen/internal/backend/handler"
)

func init() { gin.SetMode(gin.TestMode) }

func hashPassword(t *testing.T, pass string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return string(hash)
}

func TestLogin_Success(t *testing.T) {
	h := handler.NewAuthHandler("admin", hashPassword(t, "testpass"),
		[]byte("testsecret-must-be-32-chars-long!"), nil)
	r := gin.New()
	r.POST("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "testpass"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["token"] == nil {
		t.Error("expected token in response")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	h := handler.NewAuthHandler("admin", hashPassword(t, "correct"),
		[]byte("testsecret-must-be-32-chars-long!"), nil)
	r := gin.New()
	r.POST("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestJWTMiddleware_Valid(t *testing.T) {
	secret := []byte("testsecret-must-be-32-chars-long!")
	h := handler.NewAuthHandler("admin", hashPassword(t, "pass"), secret, nil)
	mw := handler.JWTMiddleware(secret)

	r := gin.New()
	r.POST("/login", h.Login)
	r.GET("/protected", mw, func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	// Login first
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "pass"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var loginResp map[string]any
	json.NewDecoder(rec.Body).Decode(&loginResp)
	token, _ := loginResp["token"].(string)

	// Use token on protected route
	req2 := httptest.NewRequest("GET", "/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("protected route with valid token: got %d, want 200", rec2.Code)
	}
}

func TestJWTMiddleware_Missing(t *testing.T) {
	mw := handler.JWTMiddleware([]byte("secret"))
	r := gin.New()
	r.GET("/protected", mw, func(c *gin.Context) { c.JSON(200, nil) })

	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d, want 401", rec.Code)
	}
}
