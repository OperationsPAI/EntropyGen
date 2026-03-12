package handler

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	observeProxyTimeout   = 30 * time.Second
	observeWSReadBufSize  = 1024
	observeWSWriteBufSize = 1024
	sidecarPort           = "8081"
)

// ObserveHandler proxies requests to the Agent Observer Sidecar
// running inside each Agent Pod.
type ObserveHandler struct {
	namespace string
}

// NewObserveHandler creates a handler that proxies observe requests
// to the sidecar at agent-{name}.{namespace}.svc:8081.
func NewObserveHandler(namespace string) *ObserveHandler {
	return &ObserveHandler{namespace: namespace}
}

// SidecarAddr returns the in-cluster address for a given agent's sidecar.
func SidecarAddr(agentName, namespace string) string {
	return fmt.Sprintf("agent-%s.%s.svc:%s", agentName, namespace, sidecarPort)
}

// SidecarPath strips the /api/agents/:name/observe prefix and returns
// the remaining path to forward to the sidecar.
func SidecarPath(fullPath, agentName string) string {
	prefix := "/api/agents/" + agentName + "/observe"
	path := strings.TrimPrefix(fullPath, prefix)
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	return path
}

// Proxy handles both HTTP reverse proxy and WebSocket relay to the sidecar.
func (h *ObserveHandler) Proxy(c *gin.Context) {
	agentName := c.Param("name")
	if agentName == "" {
		c.JSON(http.StatusBadRequest, apiError("INVALID_AGENT", "agent name is required", ""))
		return
	}

	addr := SidecarAddr(agentName, h.namespace)
	forwardPath := SidecarPath(c.Request.URL.Path, agentName)

	if isWebSocketUpgrade(c.Request) {
		h.proxyWebSocket(c, addr, forwardPath)
		return
	}

	h.proxyHTTP(c, addr, forwardPath)
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func (h *ObserveHandler) proxyHTTP(c *gin.Context, addr, path string) {
	target := &url.URL{
		Scheme: "http",
		Host:   addr,
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = path
			req.URL.RawQuery = c.Request.URL.RawQuery
			req.Host = target.Host
		},
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: observeProxyTimeout}).DialContext,
			ResponseHeaderTimeout: observeProxyTimeout,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("observe proxy error", "agent", c.Param("name"), "err", err)
			if c.Writer.Written() {
				return
			}
			c.JSON(http.StatusBadGateway, apiError("PROXY_ERROR", "sidecar unreachable", err.Error()))
		},
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

var observeWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  observeWSReadBufSize,
	WriteBufferSize: observeWSWriteBufSize,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (h *ObserveHandler) proxyWebSocket(c *gin.Context, addr, path string) {
	agentName := c.Param("name")

	// Upgrade frontend connection
	clientConn, err := observeWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("observe ws upgrade failed", "agent", agentName, "err", err)
		return
	}
	defer clientConn.Close()

	// Dial sidecar
	sidecarURL := fmt.Sprintf("ws://%s%s", addr, path)
	sidecarConn, _, err := websocket.DefaultDialer.Dial(sidecarURL, nil)
	if err != nil {
		slog.Error("observe ws dial sidecar failed", "agent", agentName, "url", sidecarURL, "err", err)
		clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "sidecar unreachable"))
		return
	}
	defer sidecarConn.Close()

	done := make(chan struct{})

	// sidecar → client
	go func() {
		defer close(done)
		for {
			msgType, msg, err := sidecarConn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					slog.Debug("observe ws sidecar read closed", "agent", agentName, "err", err)
				}
				return
			}
			if err := clientConn.WriteMessage(msgType, msg); err != nil {
				slog.Debug("observe ws client write failed", "agent", agentName, "err", err)
				return
			}
		}
	}()

	// client → sidecar
	go func() {
		for {
			msgType, msg, err := clientConn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					slog.Debug("observe ws client read closed", "agent", agentName, "err", err)
				}
				sidecarConn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := sidecarConn.WriteMessage(msgType, msg); err != nil {
				slog.Debug("observe ws sidecar write failed", "agent", agentName, "err", err)
				return
			}
		}
	}()

	<-done
}
