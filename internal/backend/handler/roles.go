package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
)

type RoleHandler struct {
	client *k8sclient.RoleClient
}

func NewRoleHandler(client *k8sclient.RoleClient) *RoleHandler {
	return &RoleHandler{client: client}
}

func (h *RoleHandler) List(c *gin.Context) {
	roles, err := h.client.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("LIST_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": roles})
}

func (h *RoleHandler) Create(c *gin.Context) {
	var req k8sclient.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	role, err := h.client.Create(c.Request.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, apiError("ROLE_EXISTS", err.Error(), ""))
			return
		}
		c.JSON(http.StatusInternalServerError, apiError("CREATE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": role})
}

func (h *RoleHandler) Get(c *gin.Context) {
	role, err := h.client.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		c.JSON(http.StatusNotFound,
			apiError("ROLE_NOT_FOUND", "role not found", "role '"+c.Param("name")+"' not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": role})
}

func (h *RoleHandler) Update(c *gin.Context) {
	var body struct {
		Description string `json:"description" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	role, err := h.client.UpdateDescription(c.Request.Context(), c.Param("name"), body.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("UPDATE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": role})
}

func (h *RoleHandler) Delete(c *gin.Context) {
	if err := h.client.Delete(c.Request.Context(), c.Param("name")); err != nil {
		if strings.Contains(err.Error(), "agents are using") {
			c.JSON(http.StatusConflict, apiError("ROLE_IN_USE", err.Error(), ""))
			return
		}
		c.JSON(http.StatusInternalServerError, apiError("DELETE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *RoleHandler) ListFiles(c *gin.Context) {
	files, err := h.client.ListFiles(c.Request.Context(), c.Param("name"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("LIST_FILES_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": files})
}

func (h *RoleHandler) GetFile(c *gin.Context) {
	file, err := h.client.GetFile(c.Request.Context(), c.Param("name"), c.Param("filename"))
	if err != nil {
		c.JSON(http.StatusNotFound,
			apiError("FILE_NOT_FOUND", "file not found", "file '"+c.Param("filename")+"' not found in role '"+c.Param("name")+"'"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": file})
}

func (h *RoleHandler) PutFile(c *gin.Context) {
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	file, err := h.client.PutFile(c.Request.Context(), c.Param("name"), c.Param("filename"), body.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("PUT_FILE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": file})
}

func (h *RoleHandler) DeleteFile(c *gin.Context) {
	if err := h.client.DeleteFile(c.Request.Context(), c.Param("name"), c.Param("filename")); err != nil {
		c.JSON(http.StatusInternalServerError, apiError("DELETE_FILE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *RoleHandler) RenameFile(c *gin.Context) {
	var body struct {
		NewName string `json:"new_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	file, err := h.client.RenameFile(c.Request.Context(), c.Param("name"), c.Param("filename"), body.NewName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("RENAME_FILE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": file})
}
