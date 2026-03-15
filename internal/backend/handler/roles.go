package handler

import (
	"archive/zip"
	"io"
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
	ct := c.ContentType()
	if strings.HasPrefix(ct, "multipart/form-data") {
		h.createFromMultipart(c)
		return
	}
	h.createFromJSON(c)
}

func (h *RoleHandler) createFromJSON(c *gin.Context) {
	var req k8sclient.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	h.doCreate(c, req)
}

func (h *RoleHandler) createFromMultipart(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", "name is required", ""))
		return
	}

	req := k8sclient.CreateRoleRequest{
		Name:        name,
		Description: c.PostForm("description"),
		Role:        c.PostForm("role"),
	}

	// Parse zip if uploaded
	fh, err := c.FormFile("file")
	if err == nil && fh != nil {
		f, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError("ZIP_READ_ERROR", err.Error(), ""))
			return
		}
		defer f.Close()

		files, err := parseZipFiles(f, fh.Size)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError("ZIP_PARSE_ERROR", err.Error(), ""))
			return
		}
		req.Files = files
	}

	h.doCreate(c, req)
}

func (h *RoleHandler) doCreate(c *gin.Context, req k8sclient.CreateRoleRequest) {
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

// parseZipFiles reads a zip archive and returns a map of filepath → content.
// Paths preserve directory structure using "/" separators (e.g. "skills/gitea-api/SKILL.md").
func parseZipFiles(r io.ReaderAt, size int64) (map[string]string, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	files := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// Clean the path and strip any top-level wrapper directory
		name := strings.ReplaceAll(f.Name, "\\", "/")
		name = strings.TrimPrefix(name, "/")
		// Remove leading "./" if present
		name = strings.TrimPrefix(name, "./")
		if name == "" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}

		// Store with "/" path separators directly — no __ encoding needed
		files[name] = string(content)
	}
	return files, nil
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

// fileParam extracts the file path from the *filepath catch-all parameter.
func fileParam(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("filepath"), "/")
}

func (h *RoleHandler) GetFile(c *gin.Context) {
	filename := fileParam(c)
	file, err := h.client.GetFile(c.Request.Context(), c.Param("name"), filename)
	if err != nil {
		c.JSON(http.StatusNotFound,
			apiError("FILE_NOT_FOUND", "file not found", "file '"+filename+"' not found in role '"+c.Param("name")+"'"))
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
	filename := fileParam(c)
	file, err := h.client.PutFile(c.Request.Context(), c.Param("name"), filename, body.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("PUT_FILE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": file})
}

func (h *RoleHandler) DeleteFile(c *gin.Context) {
	filename := fileParam(c)
	if err := h.client.DeleteFile(c.Request.Context(), c.Param("name"), filename); err != nil {
		c.JSON(http.StatusInternalServerError, apiError("DELETE_FILE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *RoleHandler) RenameFile(c *gin.Context) {
	var body struct {
		OldName string `json:"old_name" binding:"required"`
		NewName string `json:"new_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, apiError("INVALID_REQUEST", err.Error(), ""))
		return
	}
	file, err := h.client.RenameFile(c.Request.Context(), c.Param("name"), body.OldName, body.NewName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("RENAME_FILE_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": file})
}
