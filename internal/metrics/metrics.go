package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HttpRequestsTotal — счётчик total-запросов (в т.ч. /metrics).
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HttpRequestDuration — гистограмма длительности запросов.
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)

	// HttpRequestsInFlight — gauge активных запросов прямо сейчас.
	HttpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)

	// UsersCreatedTotal - счетчик созданных пользователей
	UsersCreatedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "users_created_total",
			Help: "Total number of users created",
		},
	)

	// ProfilesCreatedTotal - счетчик созданных профилей
	ProfilesCreatedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "profiles_created_total",
			Help: "Total number of profiles created",
		},
	)
)

func init() {
	prometheus.MustRegister(HttpRequestsTotal)
	prometheus.MustRegister(HttpRequestDuration)
	prometheus.MustRegister(HttpRequestsInFlight)
	prometheus.MustRegister(UsersCreatedTotal)
	prometheus.MustRegister(ProfilesCreatedTotal)
}

// Handler возвращает http.Handler для /metrics ( promhttp ).
func Handler() http.Handler {
	return promhttp.Handler()
}
