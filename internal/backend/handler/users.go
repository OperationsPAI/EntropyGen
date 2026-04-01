package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/entropyGen/entropyGen/internal/common/pgclient"
)

// UserHandler serves admin user management endpoints.
type UserHandler struct {
	store UserStore
}

func NewUserHandler(store UserStore) *UserHandler {
	return &UserHandler{store: store}
}

// @Summary      List users
// @Tags         users
// @Produce      json
// @Success      200  {object}  SuccessResponse{data=[]object}
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /users [get]
func (h *UserHandler) List(c *gin.Context) {
	users, err := h.store.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("DB_ERROR", "failed to list users", err.Error()))
		return
	}
	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, userToJSON(u))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// @Summary      Create user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      CreateUserRequest  true  "User to create"
// @Success      201   {object}  SuccessResponse{data=object}
// @Failure      400   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	if !validRole(req.Role) {
		c.JSON(http.StatusBadRequest, apiError("INVALID_ROLE", "role must be member or admin", ""))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("HASH_ERROR", "failed to hash password", ""))
		return
	}
	user, err := h.store.CreateUser(c.Request.Context(), pgclient.CreateUserInput{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         req.Role,
	})
	if err != nil {
		c.JSON(http.StatusConflict, apiError("CREATE_ERROR", err.Error(), ""))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": userToJSON(user)})
}

// @Summary      Update user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        username  path      string              true  "Username"
// @Param        body      body      UpdateUserRequest   true  "Fields to update"
// @Success      200       {object}  SuccessResponse{data=object}
// @Failure      400       {object}  ErrorResponse
// @Failure      404       {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /users/{username} [put]
func (h *UserHandler) Update(c *gin.Context) {
	username := c.Param("username")
	var req struct {
		Role     *string `json:"role"`
		Password *string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	if req.Role != nil && !validRole(*req.Role) {
		c.JSON(http.StatusBadRequest, apiError("INVALID_ROLE", "role must be member or admin", ""))
		return
	}
	in := pgclient.UpdateUserInput{Role: req.Role}
	if req.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, apiError("HASH_ERROR", "failed to hash password", ""))
			return
		}
		hashStr := string(hash)
		in.PasswordHash = &hashStr
	}
	user, err := h.store.UpdateUser(c.Request.Context(), username, in)
	if err != nil {
		c.JSON(http.StatusNotFound, apiError("UPDATE_ERROR", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": userToJSON(user)})
}

// @Summary      Delete user
// @Tags         users
// @Produce      json
// @Param        username  path      string  true  "Username"
// @Success      200       {object}  SuccessResponse
// @Failure      400       {object}  ErrorResponse
// @Failure      404       {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /users/{username} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	username := c.Param("username")
	// Prevent self-deletion
	if caller, _ := c.Get("username"); caller == username {
		c.JSON(http.StatusBadRequest, apiError("SELF_DELETE", "cannot delete yourself", ""))
		return
	}
	if err := h.store.DeleteUser(c.Request.Context(), username); err != nil {
		c.JSON(http.StatusNotFound, apiError("DELETE_ERROR", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func userToJSON(u *pgclient.User) gin.H {
	return gin.H{
		"id":        u.ID,
		"username":  u.Username,
		"role":      u.Role,
		"createdAt": u.CreatedAt,
		"updatedAt": u.UpdatedAt,
	}
}

func validRole(r string) bool {
	return r == "member" || r == "admin"
}
