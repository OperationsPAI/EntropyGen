package wspush

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

const pingInterval = 30 * time.Second

// Client represents a connected WebSocket client.
type Client struct {
	conn    *websocket.Conn
	agentID string // "" = receive all
	send    chan []byte
}

// Pusher manages WebSocket connections and broadcasts events from Redis Streams.
type Pusher struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

// NewPusher creates a new Pusher hub.
func NewPusher() *Pusher {
	return &Pusher{clients: make(map[*Client]struct{})}
}

// Register adds a WebSocket connection to the hub and starts its write pump.
func (p *Pusher) Register(conn *websocket.Conn, agentID string) *Client {
	c := &Client{
		conn:    conn,
		agentID: agentID,
		send:    make(chan []byte, 256),
	}
	p.mu.Lock()
	p.clients[c] = struct{}{}
	p.mu.Unlock()
	go c.writePump()
	return c
}

// Unregister removes a WebSocket client from the hub and closes its send channel.
func (p *Pusher) Unregister(c *Client) {
	p.mu.Lock()
	if _, ok := p.clients[c]; ok {
		delete(p.clients, c)
		close(c.send)
	}
	p.mu.Unlock()
}

func (p *Pusher) broadcast(event *models.Event, data []byte) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for c := range p.clients {
		if c.agentID != "" && c.agentID != event.AgentID {
			continue
		}
		select {
		case c.send <- data:
		default:
			// slow client: skip this message
		}
	}
}

// Run consumes events from Redis Streams and broadcasts to connected clients.
func (p *Pusher) Run(ctx context.Context, reader *redisclient.StreamReader) {
	streams := []string{"events:gateway", "events:gitea", "events:k8s"}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		for _, stream := range streams {
			msgs, err := reader.Read(ctx, stream, 50, time.Second)
			if err != nil {
				slog.Warn("wspush: read failed", "stream", stream, "err", err)
				continue
			}
			ids := make([]string, 0, len(msgs))
			for _, msg := range msgs {
				if msg.Event == nil {
					ids = append(ids, msg.ID)
					continue
				}
				data, _ := json.Marshal(msg.Event)
				p.broadcast(msg.Event, data)
				ids = append(ids, msg.ID)
			}
			if len(ids) > 0 {
				_ = reader.ACK(ctx, stream, ids...)
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
