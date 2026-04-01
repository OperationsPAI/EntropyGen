package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/entropyGen/entropyGen/internal/common/pgclient"
)

// UserStore is the interface for user storage operations.
type UserStore interface {
	FindByUsername(ctx context.Context, username string) (*pgclient.User, error)
	ListUsers(ctx context.Context) ([]*pgclient.User, error)
	CreateUser(ctx context.Context, in pgclient.CreateUserInput) (*pgclient.User, error)
	UpdateUser(ctx context.Context, username string, in pgclient.UpdateUserInput) (*pgclient.User, error)
	DeleteUser(ctx context.Context, username string) error
}

// AuthHandler handles authentication.
type AuthHandler struct {
	fallbackUsername     string
	fallbackPasswordHash string
	secret               []byte
	store                UserStore // nil = DB-less fallback mode
}

func NewAuthHandler(username, passwordHash string, secret []byte, store UserStore) *AuthHandler {
	return &AuthHandler{
		fallbackUsername:     username,
		fallbackPasswordHash: passwordHash,
		secret:               secret,
		store:                store,
	}
}

// @Summary      Login
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest  true  "Credentials"
// @Success      200   {object}  SuccessResponse{data=LoginResponseData}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Router       /auth/login [post]
//
// Login validates credentials against DB (or fallback env vars) and returns a 24h JWT.
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}

	role := "admin"

	if h.store != nil {
		user, err := h.store.FindByUsername(c.Request.Context(), req.Username)
		if err != nil || user == nil {
			c.JSON(http.StatusUnauthorized, apiError("INVALID_CREDENTIALS", "invalid username or password", ""))
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
			c.JSON(http.StatusUnauthorized, apiError("INVALID_CREDENTIALS", "invalid username or password", ""))
			return
		}
		role = user.Role
	} else {
		if req.Username != h.fallbackUsername {
			c.JSON(http.StatusUnauthorized, apiError("INVALID_CREDENTIALS", "invalid username or password", ""))
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(h.fallbackPasswordHash), []byte(req.Password)) != nil {
			c.JSON(http.StatusUnauthorized, apiError("INVALID_CREDENTIALS", "invalid username or password", ""))
			return
		}
	}

	claims := jwt.MapClaims{
		"sub":  req.Username,
		"role": role,
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

// @Summary      Get current user
// @Tags         auth
// @Produce      json
// @Success      200  {object}  SuccessResponse{data=UserInfo}
// @Failure      401  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	username, _ := c.Get("username")
	role, _ := c.Get("role")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"username": username, "role": role},
	})
}

// @Summary      Logout
// @Tags         auth
// @Produce      json
// @Success      200  {object}  SuccessResponse
// @Security     BearerAuth
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// OptionalJWTMiddleware sets username/role from token if present, else role="guest".
// Never rejects the request — guest access is always allowed.
func OptionalJWTMiddleware(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Set("username", "")
			c.Set("role", "guest")
			c.Next()
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
			// Treat invalid/expired token as guest
			c.Set("username", "")
			c.Set("role", "guest")
			c.Next()
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		c.Set("username", claims["sub"])
		role, _ := claims["role"].(string)
		if role == "" {
			role = "member"
		}
		c.Set("role", role)
		c.Next()
	}
}

// RequireRole aborts with 401/403 if the current role is not among allowed.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		if _, ok := allowed[roleStr]; !ok {
			if roleStr == "guest" || roleStr == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized,
					apiError("UNAUTHORIZED", "authentication required", ""))
			} else {
				c.AbortWithStatusJSON(http.StatusForbidden,
					apiError("FORBIDDEN", "insufficient permissions", ""))
			}
			return
		}
		c.Next()
	}
}

// JWTMiddleware validates JWT tokens and rejects requests without a valid token.
// Kept for backward compatibility with agent tokens.
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
		role, _ := claims["role"].(string)
		if role == "" {
			role = "member"
		}
		c.Set("role", role)
		c.Next()
	}
}

// apiError returns the standard error response format.
func apiError(code, msg, detail string) gin.H {
	return gin.H{"success": false, "error": msg, "code": code, "detail": detail}
}
