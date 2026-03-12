package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// LLMHandler proxies LLM model configuration requests to LiteLLM.
type LLMHandler struct {
	litellmAddr string
	masterKey   string
	httpClient  *http.Client
}

func NewLLMHandler(litellmAddr, masterKey string) *LLMHandler {
	return &LLMHandler{
		litellmAddr: litellmAddr,
		masterKey:   masterKey,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *LLMHandler) ListModels(c *gin.Context) { h.proxy(c, "GET", "/model/info", nil) }
func (h *LLMHandler) Health(c *gin.Context)      { h.proxy(c, "GET", "/health", nil) }

func (h *LLMHandler) CreateModel(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	h.proxy(c, "POST", "/model/new", body)
}

func (h *LLMHandler) UpdateModel(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	h.proxy(c, "POST", "/model/update", body)
}

func (h *LLMHandler) DeleteModel(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	h.proxy(c, "POST", "/model/delete", body)
}

func (h *LLMHandler) proxy(c *gin.Context, method, path string, body []byte) {
	url := h.litellmAddr + path
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), method, url, bodyReader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("PROXY_ERROR", err.Error(), ""))
		return
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if h.masterKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.masterKey)
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway,
			apiError("LITELLM_UNAVAILABLE", fmt.Sprintf("litellm unreachable: %v", err), ""))
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", respBody)
}
