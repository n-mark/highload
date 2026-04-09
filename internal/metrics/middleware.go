package metrics

import (
	"net/http"
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
	return path
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
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
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
