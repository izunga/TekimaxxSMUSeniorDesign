package middleware

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type clientWindow struct {
	ResetAt time.Time
	Count   int
}

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]clientWindow
	limit   int
	window  time.Duration
}

func NewRateLimiterFromEnv() *RateLimiter {
	limit := readEnvInt("RATE_LIMIT_REQUESTS", 120)
	windowSeconds := readEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60)
	if limit <= 0 || windowSeconds <= 0 {
		return nil
	}

	return &RateLimiter{
		clients: make(map[string]clientWindow),
		limit:   limit,
		window:  time.Duration(windowSeconds) * time.Second,
	}
}

func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	if r == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/healthz" || req.URL.Path == "/readyz" || req.URL.Path == "/metrics" {
			next.ServeHTTP(w, req)
			return
		}

		key := clientKey(req)
		now := time.Now()

		r.mu.Lock()
		window := r.clients[key]
		if window.ResetAt.IsZero() || now.After(window.ResetAt) {
			window = clientWindow{
				ResetAt: now.Add(r.window),
				Count:   0,
			}
		}
		window.Count++
		r.clients[key] = window
		remaining := r.limit - window.Count
		resetAfter := int(time.Until(window.ResetAt).Seconds())
		allowed := window.Count <= r.limit
		r.mu.Unlock()

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(r.limit))
		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.Itoa(resetAfter))

		if !allowed {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func clientKey(req *http.Request) string {
	if forwarded := strings.TrimSpace(req.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if req.RemoteAddr != "" {
		return req.RemoteAddr
	}
	return "unknown"
}

func readEnvInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
