package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/entropyGen/entropyGen/internal/backend/wspush"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSHandler handles WebSocket connections for realtime event push.
type WSHandler struct {
	pusher *wspush.Pusher
}

func NewWSHandler(pusher *wspush.Pusher) *WSHandler {
	return &WSHandler{pusher: pusher}
}

func (h *WSHandler) Handle(c *gin.Context) {
	agentID := c.Query("agent_id")
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError("WS_UPGRADE_FAILED", err.Error(), ""))
		return
	}
	client := h.pusher.Register(conn, agentID)
	defer h.pusher.Unregister(client)
	// Read pump: blocks until client disconnects
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
