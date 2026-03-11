package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/entropyGen/entropyGen/internal/common/chclient"
)

var exportConcurrent int32 // atomic counter, max 2

// AuditHandler handles audit data queries and JSONL export.
type AuditHandler struct {
	ch *chclient.Client
}

func NewAuditHandler(ch *chclient.Client) *AuditHandler {
	return &AuditHandler{ch: ch}
}

func (h *AuditHandler) ListTraces(c *gin.Context) {
	agentID := c.Query("agent_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit > 200 {
		limit = 200
	}
	if limit < 1 {
		limit = 1
	}
	traces, err := h.ch.GetRecentTraces(c.Request.Context(), agentID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("QUERY_FAILED", err.Error(), ""))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    traces,
		"meta":    gin.H{"limit": limit, "count": len(traces)},
	})
}

func (h *AuditHandler) GetTrace(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
}

func (h *AuditHandler) TokenUsage(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
}

func (h *AuditHandler) AgentActivity(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
}

func (h *AuditHandler) Operations(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
}

// Export streams audit traces as JSONL (training data format).
func (h *AuditHandler) Export(c *gin.Context) {
	if atomic.LoadInt32(&exportConcurrent) >= 2 {
		c.JSON(http.StatusTooManyRequests, apiError("EXPORT_BUSY", "max 2 concurrent exports", ""))
		return
	}
	atomic.AddInt32(&exportConcurrent, 1)
	defer atomic.AddInt32(&exportConcurrent, -1)

	agentID := c.Query("agent_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "1000"))
	if limit > 10000 {
		limit = 10000
	}

	traces, err := h.ch.GetRecentTraces(c.Request.Context(), agentID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("EXPORT_FAILED", err.Error(), ""))
		return
	}

	c.Header("Content-Type", "application/x-ndjson")
	c.Header("X-Total-Count", strconv.Itoa(len(traces)))
	c.Status(http.StatusOK)

	for i, t := range traces {
		if t.RequestBody == "" || t.ResponseBody == "" {
			continue
		}
		var reqBody map[string]json.RawMessage
		if err := json.Unmarshal([]byte(t.RequestBody), &reqBody); err != nil {
			continue
		}
		messages, ok := reqBody["messages"]
		if !ok {
			continue
		}
		var respBody map[string]json.RawMessage
		if err := json.Unmarshal([]byte(t.ResponseBody), &respBody); err != nil {
			continue
		}
		choices, ok := respBody["choices"]
		if !ok {
			continue
		}
		var choiceArr []map[string]json.RawMessage
		if err := json.Unmarshal(choices, &choiceArr); err != nil || len(choiceArr) == 0 {
			continue
		}
		msgObj, ok := choiceArr[0]["message"]
		if !ok {
			continue
		}
		line := fmt.Sprintf(`{"messages":%s,"response":%s}`, string(messages), string(msgObj))
		fmt.Fprintln(c.Writer, line)
		if (i+1)%100 == 0 {
			c.Writer.Flush()
		}
	}
}
