package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "tgn_relay"

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	messagesEnqueuedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "messages_enqueued_total",
			Help:      "Total number of messages accepted into local queue.",
		},
		[]string{"mode", "group"},
	)

	telegramMessagesSentTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "telegram_messages_sent_total",
			Help:      "Total number of messages successfully sent to Telegram.",
		},
	)

	telegramMessagesFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "telegram_messages_failed_total",
			Help:      "Total number of Telegram message send failures.",
		},
		[]string{"reason"},
	)

	telegramRateLimitedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "telegram_rate_limited_total",
			Help:      "Total number of Telegram 429 rate limit responses.",
		},
	)

	telegramRetryAfterSeconds = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "telegram_retry_after_seconds",
			Help:      "Last Telegram retry_after value in seconds.",
		},
	)

	telegramPausedUntilTimestamp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "telegram_paused_until_timestamp",
			Help:      "Unix timestamp until which Telegram sender is paused.",
		},
	)

	queueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "queue_depth",
			Help:      "Current Telegram sender queue depth.",
		},
	)

	queueCapacity = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "queue_capacity",
			Help:      "Telegram sender queue capacity.",
		},
	)

	queueFullTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "queue_full_total",
			Help:      "Total number of rejected messages because local queue was full.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDurationSeconds,
		messagesEnqueuedTotal,
		telegramMessagesSentTotal,
		telegramMessagesFailedTotal,
		telegramRateLimitedTotal,
		telegramRetryAfterSeconds,
		telegramPausedUntilTimestamp,
		queueDepth,
		queueCapacity,
		queueFullTotal,
	)
}

func Handler() http.Handler {
	return promhttp.Handler()
}

func ObserveHTTPRequest(method, path string, status int, duration time.Duration) {
	statusStr := strconv.Itoa(status)

	httpRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	httpRequestDurationSeconds.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
}

func IncMessageEnqueued(mode, group string) {
	if group == "" {
		group = "-"
	}

	messagesEnqueuedTotal.WithLabelValues(mode, group).Inc()
}

func IncTelegramSent() {
	telegramMessagesSentTotal.Inc()
}

func IncTelegramFailed(reason string) {
	if reason == "" {
		reason = "unknown"
	}

	telegramMessagesFailedTotal.WithLabelValues(reason).Inc()
}

func IncTelegramRateLimited() {
	telegramRateLimitedTotal.Inc()
}

func SetTelegramRetryAfter(d time.Duration) {
	telegramRetryAfterSeconds.Set(d.Seconds())
}

func SetTelegramPausedUntil(t time.Time) {
	if t.IsZero() {
		telegramPausedUntilTimestamp.Set(0)
		return
	}

	telegramPausedUntilTimestamp.Set(float64(t.Unix()))
}

func SetQueueDepth(n int) {
	queueDepth.Set(float64(n))
}

func SetQueueCapacity(n int) {
	queueCapacity.Set(float64(n))
}

func IncQueueFull() {
	queueFullTotal.Inc()
}
