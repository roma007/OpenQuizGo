package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "openquizgo_http_requests_total",
		Help: "HTTP 请求数",
	}, []string{"method", "route", "status"})

	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "openquizgo_http_request_duration_seconds",
		Help:    "HTTP 请求耗时",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.5, 1, 2, 5},
	}, []string{"method", "route"})

	httpInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "openquizgo_http_in_flight_requests",
		Help: "当前处理中的请求数",
	})

	quizSessions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "openquizgo_quiz_sessions_created_total",
		Help: "创建的答题会话数",
	})

	quizAnswers = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "openquizgo_quiz_answers_total",
		Help: "提交的答案数",
	}, []string{"result"})

	grantsIssued = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "openquizgo_watch_grants_issued_total",
		Help: "签发的观看授权数",
	})

	heartbeatWatched = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "openquizgo_watch_heartbeat_seconds_total",
		Help: "心跳计费的观看秒数",
	})

	reportsGenerated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "openquizgo_reports_generated_total",
		Help: "生成的成长报告数",
	})
)

func init() {
	prometheus.MustRegister(
		httpRequests, httpDuration, httpInFlight,
		quizSessions, quizAnswers, grantsIssued, heartbeatWatched, reportsGenerated,
	)
}

// Handler 返回 Prometheus 抓取端点。
func Handler() http.Handler {
	return promhttp.Handler()
}

// ObserveHTTP 记录一次请求。
func ObserveHTTP(method, route string, status int, duration time.Duration) {
	httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	httpDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

// IncInFlight 调整处理中请求数。
func IncInFlight() { httpInFlight.Inc() }

// DecInFlight 调整处理中请求数。
func DecInFlight() { httpInFlight.Dec() }

// IncQuizSession 记录会话创建。
func IncQuizSession() { quizSessions.Inc() }

// IncQuizAnswer 记录答案提交。
func IncQuizAnswer(correct bool) {
	result := "wrong"
	if correct {
		result = "correct"
	}
	quizAnswers.WithLabelValues(result).Inc()
}

// IncGrantIssued 记录授权签发。
func IncGrantIssued() { grantsIssued.Inc() }

// AddWatchedSeconds 记录心跳计费秒数。
func AddWatchedSeconds(seconds int) {
	if seconds > 0 {
		heartbeatWatched.Add(float64(seconds))
	}
}

// IncReportGenerated 记录报告生成。
func IncReportGenerated() { reportsGenerated.Inc() }
