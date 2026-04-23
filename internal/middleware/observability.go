package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

type Metrics struct {
	totalRequests atomic.Uint64
	inFlight      atomic.Int64
	mu            sync.Mutex
	byRoute       map[string]uint64
	byStatus      map[int]uint64
}

func NewMetrics() *Metrics {
	return &Metrics{
		byRoute:  make(map[string]uint64),
		byStatus: make(map[int]uint64),
	}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.inFlight.Add(1)
		defer m.inFlight.Add(-1)

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		duration := time.Since(start)
		requestID := GetRequestID(r.Context())
		m.record(r.URL.Path, recorder.status)
		writeLog(map[string]any{
			"ts":          time.Now().UTC().Format(time.RFC3339Nano),
			"level":       "info",
			"request_id":  requestID,
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      recorder.status,
			"duration_ms": duration.Milliseconds(),
			"remote_addr": r.RemoteAddr,
		})
	})
}

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte("# HELP tekimax_requests_total Total number of HTTP requests handled.\n"))
		_, _ = w.Write([]byte("# TYPE tekimax_requests_total counter\n"))
		_, _ = w.Write([]byte("tekimax_requests_total " + strconv.FormatUint(m.totalRequests.Load(), 10) + "\n"))
		_, _ = w.Write([]byte("# HELP tekimax_inflight_requests Number of in-flight HTTP requests.\n"))
		_, _ = w.Write([]byte("# TYPE tekimax_inflight_requests gauge\n"))
		_, _ = w.Write([]byte("tekimax_inflight_requests " + strconv.FormatInt(m.inFlight.Load(), 10) + "\n"))

		for route, count := range m.byRoute {
			_, _ = w.Write([]byte(`tekimax_requests_by_route{path="` + route + `"} ` + strconv.FormatUint(count, 10) + "\n"))
		}
		for status, count := range m.byStatus {
			_, _ = w.Write([]byte(`tekimax_requests_by_status{status="` + strconv.Itoa(status) + `"} ` + strconv.FormatUint(count, 10) + "\n"))
		}
	})
}

func (m *Metrics) record(path string, status int) {
	m.totalRequests.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byRoute[path]++
	m.byStatus[status]++
}

func writeLog(fields map[string]any) {
	payload, err := json.Marshal(fields)
	if err != nil {
		return
	}
	println(string(payload))
}
