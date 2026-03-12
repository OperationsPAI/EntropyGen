package observer

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	wsPingInterval = 30 * time.Second
	wsWriteWait    = 10 * time.Second
)

// WSHub manages WebSocket clients and broadcasts file change events.
type WSHub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
	watcher *Watcher
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub(watcher *Watcher) *WSHub {
	return &WSHub{
		clients: make(map[*wsClient]struct{}),
		watcher: watcher,
	}
}

// Run listens for watcher events and broadcasts them to all connected clients.
func (h *WSHub) Run(ctx context.Context, completionsDir string) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-h.watcher.Events():
			if !ok {
				return
			}
			h.broadcastEvent(evt, completionsDir)
		}
	}
}

// broadcastEvent converts a watcher event to a WebSocket message and sends it.
func (h *WSHub) broadcastEvent(evt interface{}, completionsDir string) {
	var msg []byte
	var err error

	switch e := evt.(type) {
	case JSONLLineEvent:
		// Read the latest lines from the JSONL file
		sessionPath := filepath.Join(completionsDir, e.SessionFile)
		lastLine := readLastLine(sessionPath)
		if lastLine == nil {
			return
		}
		sessionID := strings.TrimSuffix(e.SessionFile, ".jsonl")
		msg, err = json.Marshal(map[string]interface{}{
			"type":       "jsonl",
			"session_id": sessionID,
			"data":       json.RawMessage(lastLine),
		})
	case FileChangeEvent:
		msg, err = json.Marshal(map[string]interface{}{
			"type":   "file_change",
			"path":   e.Path,
			"action": e.Action,
		})
	default:
		return
	}

	if err != nil {
		slog.Warn("ws: marshal error", "err", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// slow client, skip
		}
	}
}

// HandleUpgrade handles a WebSocket upgrade request (used by the Gin handler).
func (h *WSHub) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws: upgrade failed", "err", err)
		return
	}

	c := &wsClient{
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go h.writePump(c)
	h.readPump(c)
}

// readPump blocks until the client disconnects, then unregisters.
func (h *WSHub) readPump(c *wsClient) {
	defer func() {
		h.mu.Lock()
		if _, ok := h.clients[c]; ok {
			delete(h.clients, c)
			close(c.send)
		}
		h.mu.Unlock()
		c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

// writePump sends queued messages and pings to the client.
func (h *WSHub) writePump(c *wsClient) {
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readLastLine reads the last non-empty line from a file.
func readLastLine(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Seek to near end of file to find last line
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return nil
	}

	// Read from the end to find the last line
	const readSize = 64 * 1024
	offset := info.Size() - readSize
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var lastLine []byte
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 {
			lastLine = make([]byte, len(line))
			copy(lastLine, line)
		}
	}
	return lastLine
}
