package metrics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Middleware собирает Prometheus-метрики для каждого проходящего запроса.
// Он НЕ оборачивает сам эндпоинт /metrics — иначе метрики метрик
// размножаются экспоненциально.
func Middleware(next http.Handler) http.Handler {
	metricsHandler := Handler()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Эндпоинт метрик отдаём без instrumentation
		if r.URL.Path == "/metrics" {
			metricsHandler.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		HttpRequestsInFlight.Inc()
		next.ServeHTTP(ww, r)
		HttpRequestsInFlight.Dec()

		duration := time.Since(start).Seconds()
		path := normalizePath(r.URL.Path)

		HttpRequestsTotal.WithLabelValues(r.Method, path, codeToString(ww.statusCode)).Inc()
		HttpRequestDuration.WithLabelValues(r.Method, path, codeToString(ww.statusCode)).Observe(duration)
	})
}

// normalizePath приводит пути к обобщённому виду (например /user/42 → /user/:id).
func normalizePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		// Если часть — это число (ID), заменяем на :id
		if _, err := strconv.Atoi(part); err == nil && part != "" {
			parts[i] = ":id"
		}
		// Если это UUID (пример: 123e4567-e89b-12d3-a456-426614174000)
		if matched, _ := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, part); matched {
			parts[i] = ":uuid"
		}
		// Можно добавить другие паттерны: хэши, slug и т.д.
	}
	return strings.Join(parts, "/")
}

// responseWriter обёртка для перехвата status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
		rw.ResponseWriter.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) Flush() {
	if fl, ok := rw.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support Hijack")
}

func codeToString(code int) string {
	return codeToClass(code)
}

func codeToClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}
