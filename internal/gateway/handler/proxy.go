package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/entropyGen/entropyGen/internal/gateway/audit"
	"github.com/entropyGen/entropyGen/internal/gateway/gatewayctx"
)

const proxyMaxBodySize = 64 * 1024 // 64KB, same as audit package

// ProxyHandler routes requests to LiteLLM or Gitea based on path prefix.
type ProxyHandler struct {
	litellmProxy *httputil.ReverseProxy
	giteaProxy   *httputil.ReverseProxy
	eventWriter  *audit.EventWriter
}

// NewProxyHandler creates a ProxyHandler that routes to LiteLLM and Gitea.
func NewProxyHandler(litellmAddr, giteaAddr string, ew *audit.EventWriter) *ProxyHandler {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	makeProxy := func(rawURL string) *httputil.ReverseProxy {
		target, _ := url.Parse(rawURL)
		rp := httputil.NewSingleHostReverseProxy(target)
		rp.Transport = transport
		rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			if strings.Contains(err.Error(), "context deadline exceeded") {
				http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
			} else {
				http.Error(w, "Bad Gateway", http.StatusBadGateway)
			}
		}
		return rp
	}

	return &ProxyHandler{
		litellmProxy: makeProxy(litellmAddr),
		giteaProxy:   makeProxy(giteaAddr),
		eventWriter:  ew,
	}
}

// ServeHTTP routes the request to the appropriate upstream based on path prefix.
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	agentID, _ := r.Context().Value(gatewayctx.AgentID).(string)
	agentRole, _ := r.Context().Value(gatewayctx.AgentRole).(string)
	traceID := uuid.New().String()
	origPath := r.URL.Path

	var (
		proxy     *httputil.ReverseProxy
		eventType string
		isLLM     bool
		reqBody   []byte
	)

	switch {
	case strings.HasPrefix(origPath, "/v1/"):
		proxy = p.litellmProxy
		eventType = "gateway.llm_inference"
		isLLM = true
	case strings.HasPrefix(origPath, "/api/v1/"):
		proxy = p.giteaProxy
		eventType = "gateway.gitea_api"
	case strings.HasPrefix(origPath, "/git/"):
		// Strip /git prefix: /git/owner/repo.git -> /owner/repo.git
		r2 := r.Clone(r.Context())
		r2.URL.Path = strings.TrimPrefix(origPath, "/git")
		r2.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, "/git")
		if r2.URL.Path == "" {
			r2.URL.Path = "/"
		}
		r = r2
		proxy = p.giteaProxy
		eventType = "gateway.git_http"
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Capture request body for LLM inference (for training data export)
	if isLLM && r.Body != nil {
		reqBody, _ = io.ReadAll(io.LimitReader(r.Body, proxyMaxBodySize+1))
		r.Body = io.NopCloser(bytes.NewReader(reqBody))
		r.ContentLength = int64(len(reqBody))
	}

	rec := newResponseRecorder(w, isLLM)
	proxy.ServeHTTP(rec, r)

	latencyMs := time.Since(start).Milliseconds()

	p.eventWriter.EnqueueRequest(audit.RequestRecord{
		TraceID:   traceID,
		EventType: eventType,
		AgentID:   agentID,
		AgentRole: agentRole,
		Method:    r.Method,
		Path:      origPath,
		Status:    rec.statusCode,
		LatencyMs: latencyMs,
		ReqBody:   reqBody,
		RespBody:  rec.body(),
		IsLLM:     isLLM,
	})
}

// responseRecorder captures the status code and optionally the response body.
type responseRecorder struct {
	http.ResponseWriter
	statusCode  int
	captureBody bool
	buf         *bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter, captureBody bool) *responseRecorder {
	rec := &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		captureBody:    captureBody,
	}
	if captureBody {
		rec.buf = &bytes.Buffer{}
	}
	return rec
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.captureBody && r.buf != nil && r.buf.Len() < proxyMaxBodySize {
		remaining := proxyMaxBodySize - r.buf.Len()
		if len(b) <= remaining {
			r.buf.Write(b)
		} else {
			r.buf.Write(b[:remaining])
		}
	}
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) body() []byte {
	if r.buf == nil {
		return nil
	}
	return r.buf.Bytes()
}
