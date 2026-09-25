package httpapi

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type metricBucket struct {
	UpperSeconds float64
	Count        uint64
}

type httpMetrics struct {
	mu sync.Mutex

	inFlight uint64
	requests map[string]uint64

	durationCount uint64
	durationSum   float64
	buckets       []metricBucket
}

func newHTTPMetrics() *httpMetrics {
	return &httpMetrics{
		requests: make(map[string]uint64),
		buckets: []metricBucket{
			{UpperSeconds: 0.01},
			{UpperSeconds: 0.05},
			{UpperSeconds: 0.1},
			{UpperSeconds: 0.25},
			{UpperSeconds: 0.5},
			{UpperSeconds: 1},
			{UpperSeconds: 2.5},
			{UpperSeconds: 5},
		},
	}
}

func statusClass(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500:
		return "5xx"
	default:
		return "other"
	}
}

func (m *httpMetrics) begin() {
	m.mu.Lock()
	m.inFlight++
	m.mu.Unlock()
}

func (m *httpMetrics) finish(method string, status int, duration time.Duration) {
	seconds := duration.Seconds()
	key := strings.ToUpper(method) + "|" + statusClass(status)

	m.mu.Lock()
	if m.inFlight > 0 {
		m.inFlight--
	}
	m.requests[key]++
	m.durationCount++
	m.durationSum += seconds
	for i := range m.buckets {
		if seconds <= m.buckets[i].UpperSeconds {
			m.buckets[i].Count++
		}
	}
	m.mu.Unlock()
}

func (m *httpMetrics) render() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var b strings.Builder
	b.WriteString("# HELP myankafe_finance_http_requests_in_flight Current HTTP requests being served.\n")
	b.WriteString("# TYPE myankafe_finance_http_requests_in_flight gauge\n")
	fmt.Fprintf(&b, "myankafe_finance_http_requests_in_flight %d\n", m.inFlight)

	b.WriteString("# HELP myankafe_finance_http_requests_total Total HTTP requests by method and status class.\n")
	b.WriteString("# TYPE myankafe_finance_http_requests_total counter\n")
	keys := make([]string, 0, len(m.requests))
	for key := range m.requests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.SplitN(key, "|", 2)
		fmt.Fprintf(
			&b,
			"myankafe_finance_http_requests_total{method=%q,status_class=%q} %d\n",
			parts[0],
			parts[1],
			m.requests[key],
		)
	}

	b.WriteString("# HELP myankafe_finance_http_request_duration_seconds HTTP request duration.\n")
	b.WriteString("# TYPE myankafe_finance_http_request_duration_seconds histogram\n")
	for _, bucket := range m.buckets {
		fmt.Fprintf(
			&b,
			"myankafe_finance_http_request_duration_seconds_bucket{le=%q} %d\n",
			strconv.FormatFloat(bucket.UpperSeconds, 'f', -1, 64),
			bucket.Count,
		)
	}
	fmt.Fprintf(&b, "myankafe_finance_http_request_duration_seconds_bucket{le=%q} %d\n", "+Inf", m.durationCount)
	fmt.Fprintf(&b, "myankafe_finance_http_request_duration_seconds_sum %.9f\n", m.durationSum)
	fmt.Fprintf(&b, "myankafe_finance_http_request_duration_seconds_count %d\n", m.durationCount)

	return b.String()
}

func (s *Server) observeRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.Metrics.begin()

		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		duration := time.Since(start)
		s.Metrics.finish(r.Method, status, duration)

		slog.InfoContext(
			r.Context(),
			"http_request",
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"status_class", statusClass(status),
			"duration_ms", float64(duration.Microseconds())/1000,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(s.Config.MetricsBearerToken)

	if s.Config.AppEnv == "production" && token == "" {
		http.NotFound(w, r)
		return
	}
	if token != "" {
		const prefix = "Bearer "
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if len(authz) <= len(prefix) || !strings.EqualFold(authz[:len(prefix)], prefix) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			fail(w, http.StatusUnauthorized, fmt.Errorf("metrics authentication required"))
			return
		}
		got := strings.TrimSpace(authz[len(prefix):])
		if len(got) != len(token) || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			fail(w, http.StatusUnauthorized, fmt.Errorf("metrics authentication required"))
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(s.Metrics.render()))
}
