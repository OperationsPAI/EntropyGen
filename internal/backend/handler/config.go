package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ConfigHandler serves public configuration values to the frontend.
type ConfigHandler struct {
	giteaBaseURL string
}

func NewConfigHandler(giteaBaseURL string) *ConfigHandler {
	return &ConfigHandler{giteaBaseURL: giteaBaseURL}
}

// @Summary      Get platform config
// @Tags         config
// @Produce      json
// @Success      200  {object}  ConfigResponse
// @Router       /config [get]
//
// Get returns public configuration.
func (h *ConfigHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"gitea_base_url": h.giteaBaseURL,
	})
}
