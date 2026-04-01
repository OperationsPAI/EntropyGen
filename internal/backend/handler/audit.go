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

// AuditHandler handles audit data queries and JSONL export.
type AuditHandler struct {
	ch               *chclient.Client
	exportConcurrent int32 // atomic counter, max 2
}

func NewAuditHandler(ch *chclient.Client) *AuditHandler {
	return &AuditHandler{ch: ch}
}

func (h *AuditHandler) ListTraces(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit > 200 {
		limit = 200
	}
	if limit < 1 {
		limit = 1
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	f := chclient.TraceFilter{
		AgentID:     c.Query("agent_id"),
		RequestType: c.Query("request_type"),
		Status:      c.Query("status"),
		StartTime:   c.Query("start_time"),
		EndTime:     c.Query("end_time"),
		Limit:       limit,
		Page:        page,
	}

	result, err := h.ch.QueryTraces(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("QUERY_FAILED", err.Error(), ""))
		return
	}
	items := result.Items
	if items == nil {
		items = []chclient.AuditTrace{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
		"meta":    gin.H{"limit": limit, "count": result.Total, "page": page},
	})
}

func (h *AuditHandler) GetTrace(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
}

func (h *AuditHandler) TokenUsage(c *gin.Context) {
	agentID := c.Query("agent_id")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	results, err := h.ch.GetTokenUsage(c.Request.Context(), agentID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("QUERY_FAILED", err.Error(), ""))
		return
	}
	if results == nil {
		results = []chclient.TokenUsageSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

func (h *AuditHandler) AgentActivity(c *gin.Context) {
	agentID := c.Query("agent_id")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	results, err := h.ch.GetAgentActivity(c.Request.Context(), agentID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("QUERY_FAILED", err.Error(), ""))
		return
	}
	if results == nil {
		results = []chclient.AgentActivitySummary{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

func (h *AuditHandler) Operations(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
}

func (h *AuditHandler) ModelDistribution(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	results, err := h.ch.GetModelDistribution(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("QUERY_FAILED", err.Error(), ""))
		return
	}
	if results == nil {
		results = []chclient.ModelDistribution{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

func (h *AuditHandler) LatencyTrend(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	results, err := h.ch.GetLatencyTrend(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("QUERY_FAILED", err.Error(), ""))
		return
	}
	if results == nil {
		results = []chclient.LatencyPoint{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

func (h *AuditHandler) AgentRanking(c *gin.Context) {
	results, err := h.ch.GetAgentRanking(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError("QUERY_FAILED", err.Error(), ""))
		return
	}
	if results == nil {
		results = []chclient.AgentRanking{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

// Export streams audit traces as JSONL (training data format).
func (h *AuditHandler) Export(c *gin.Context) {
	if atomic.LoadInt32(&h.exportConcurrent) >= 2 {
		c.JSON(http.StatusTooManyRequests, apiError("EXPORT_BUSY", "max 2 concurrent exports", ""))
		return
	}
	atomic.AddInt32(&h.exportConcurrent, 1)
	defer atomic.AddInt32(&h.exportConcurrent, -1)

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
