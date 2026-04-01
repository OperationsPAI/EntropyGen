package handler

import (
	"bytes"
	"context"
	"encoding/json"
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

// @Summary      List LLM models
// @Tags         llm
// @Produce      json
// @Success      200  {object}  object  "LiteLLM model list (proxied)"
// @Failure      502  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /llm/models [get]
func (h *LLMHandler) ListModels(c *gin.Context) { h.proxy(c, "GET", "/model/info", nil) }

// @Summary      LLM service health
// @Tags         llm
// @Produce      json
// @Success      200  {object}  object  "LiteLLM health (proxied)"
// @Failure      502  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /llm/health [get]
func (h *LLMHandler) Health(c *gin.Context) { h.proxy(c, "GET", "/health", nil) }

// @Summary      Chat completion (test)
// @Tags         llm
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "OpenAI chat completion request"
// @Success      200   {object}  object  "LiteLLM response (proxied)"
// @Failure      502   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /llm/chat [post]
//
// Chat proxies a chat completion request to LiteLLM for end-to-end testing.
func (h *LLMHandler) Chat(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	h.proxy(c, "POST", "/chat/completions", body)
}

// createModelRequest is the frontend DTO for adding a model.
type createModelRequest struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
	BaseURL  string `json:"baseUrl,omitempty"`
	RPM      int    `json:"rpm"`
	TPM      int    `json:"tpm"`
}

// @Summary      Create LLM model
// @Tags         llm
// @Accept       json
// @Produce      json
// @Param        body  body      createModelRequest  true  "Model config"
// @Success      200   {object}  object  "LiteLLM response (proxied)"
// @Failure      502   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /llm/models [post]
func (h *LLMHandler) CreateModel(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)

	var req createModelRequest
	if err := json.Unmarshal(body, &req); err != nil {
		// Not our DTO format — pass through as-is (might be raw LiteLLM format).
		h.proxy(c, "POST", "/model/new", body)
		return
	}

	// Build the model identifier: provider/name (e.g. "openai/gpt-4o").
	modelID := req.Name
	if req.Provider != "" {
		modelID = req.Provider + "/" + req.Name
	}

	litellmPayload := map[string]any{
		"model_name": req.Name,
		"litellm_params": map[string]any{
			"model":   modelID,
			"api_key": req.APIKey,
		},
		"model_info": map[string]any{
			"rpm": req.RPM,
			"tpm": req.TPM,
		},
	}

	if req.BaseURL != "" {
		litellmPayload["litellm_params"].(map[string]any)["api_base"] = req.BaseURL
	}

	out, _ := json.Marshal(litellmPayload)
	h.proxy(c, "POST", "/model/new", out)
}

// @Summary      Check single model health
// @Tags         llm
// @Produce      json
// @Param        id  path      string  true  "Model ID"
// @Success      200  {object}  HealthModelResponse
// @Security     BearerAuth
// @Router       /llm/health/{id} [post]
//
// HealthModel checks a single model by sending a minimal chat completion.
func (h *LLMHandler) HealthModel(c *gin.Context) {
	model := c.Param("id")
	payload, _ := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens": 1,
	})

	start := time.Now()
	resp, err := h.doLiteLLM(c.Request.Context(), "POST", "/chat/completions", payload)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		c.JSON(http.StatusOK, gin.H{"status": "unhealthy", "latency_ms": latency, "error": string(body)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy", "latency_ms": latency})
}

// doLiteLLM sends a request to LiteLLM and returns the raw response.
func (h *LLMHandler) doLiteLLM(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.litellmAddr+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if h.masterKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.masterKey)
	}
	return h.httpClient.Do(req)
}

// @Summary      Update LLM model
// @Tags         llm
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Model ID"
// @Param        body  body      object  true  "Model update"
// @Success      200   {object}  object  "LiteLLM response (proxied)"
// @Failure      502   {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /llm/models/{id} [put]
func (h *LLMHandler) UpdateModel(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	h.proxy(c, "POST", "/model/update", body)
}

// deleteModelRequest is used to build the LiteLLM /model/delete payload.
type deleteModelRequest struct {
	ID string `json:"id"`
}

// @Summary      Delete LLM model
// @Tags         llm
// @Produce      json
// @Param        id  path      string  true  "Model ID"
// @Success      200  {object}  object  "LiteLLM response (proxied)"
// @Failure      502  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /llm/models/{id} [delete]
func (h *LLMHandler) DeleteModel(c *gin.Context) {
	id := c.Param("id")
	payload, _ := json.Marshal(deleteModelRequest{ID: id})
	h.proxy(c, "POST", "/model/delete", payload)
}

func (h *LLMHandler) proxy(c *gin.Context, method, path string, body []byte) {
	resp, err := h.doLiteLLM(c.Request.Context(), method, path, body)
	if err != nil {
		c.JSON(http.StatusBadGateway,
			apiError("LITELLM_UNAVAILABLE", fmt.Sprintf("litellm unreachable: %v", err), ""))
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", respBody)
}
