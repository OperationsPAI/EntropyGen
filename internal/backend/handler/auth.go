package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles admin authentication.
type AuthHandler struct {
	username     string
	passwordHash string
	secret       []byte
}

func NewAuthHandler(username, passwordHash string, secret []byte) *AuthHandler {
	return &AuthHandler{username: username, passwordHash: passwordHash, secret: secret}
}

// Login validates credentials and returns a 24h JWT.
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	if req.Username != h.username {
		c.JSON(http.StatusUnauthorized, apiError("INVALID_CREDENTIALS", "invalid username or password", ""))
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(h.passwordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, apiError("INVALID_CREDENTIALS", "invalid username or password", ""))
		return
	}
	claims := jwt.MapClaims{
		"sub":  req.Username,
		"role": "admin",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(h.secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("TOKEN_ERROR", "failed to sign token", ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "token": signed})
}

func (h *AuthHandler) Me(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"username": username, "role": "admin"},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// JWTMiddleware validates admin JWT tokens (with expiry check, unlike agent tokens).
func JWTMiddleware(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				apiError("UNAUTHORIZED", "missing bearer token", ""))
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return secret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				apiError("UNAUTHORIZED", "invalid or expired token", ""))
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		c.Set("username", claims["sub"])
		c.Next()
	}
}

// apiError returns the standard error response format.
func apiError(code, msg, detail string) gin.H {
	return gin.H{"success": false, "error": msg, "code": code, "detail": detail}
}
