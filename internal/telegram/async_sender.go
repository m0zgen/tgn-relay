package telegram

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/m0zgen/tgn-relay/internal/metrics"
)

var ErrQueueFull = errors.New("telegram queue is full")

type AsyncSenderConfig struct {
	QueueSize int
	Interval  time.Duration
}

type queueItem struct {
	Token string
	Req   SendMessageRequest
}

type AsyncSender struct {
	client *Client
	log    *slog.Logger

	queue chan queueItem

	interval time.Duration

	// UnixNano timestamp.
	// При 429 ставим паузу до этого момента.
	pausedUntil atomic.Int64
}

func NewAsyncSender(client *Client, cfg AsyncSenderConfig, logger *slog.Logger) *AsyncSender {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 1000
	}

	if cfg.Interval <= 0 {
		cfg.Interval = time.Second
	}

	if logger == nil {
		logger = slog.Default()
	}

	s := &AsyncSender{
		client:   client,
		log:      logger,
		queue:    make(chan queueItem, cfg.QueueSize),
		interval: cfg.Interval,
	}

	metrics.SetQueueCapacity(cap(s.queue))
	metrics.SetQueueDepth(len(s.queue))

	return s
}

// SendMessage реализует тот же интерфейс, что и обычный Client,
// но реально только кладет сообщение в очередь.
func (s *AsyncSender) SendMessage(ctx context.Context, token string, req SendMessageRequest) (*APIResponse, error) {
	if token == "" {
		return nil, errors.New("token is empty")
	}
	if req.ChatID == "" {
		return nil, errors.New("chat_id is empty")
	}
	if req.Text == "" {
		return nil, errors.New("text is empty")
	}

	item := queueItem{
		Token: token,
		Req:   req,
	}

	select {
	case s.queue <- item:
		metrics.SetQueueDepth(len(s.queue))

		return &APIResponse{
			OK:          true,
			Description: "queued",
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()

	default:
		metrics.IncQueueFull()
		metrics.SetQueueDepth(len(s.queue))

		return nil, ErrQueueFull
	}
}

func (s *AsyncSender) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Info("telegram async sender started", "queue_size", cap(s.queue), "interval", s.interval.String())

	for {
		select {
		case <-ctx.Done():
			s.log.Info("telegram async sender stopped")
			return

		case item := <-s.queue:
			metrics.SetQueueDepth(len(s.queue))

			s.waitIfPaused(ctx)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			s.sendWithRetryAfter(ctx, item)
		}
	}
}

func (s *AsyncSender) waitIfPaused(ctx context.Context) {
	for {
		untilUnix := s.pausedUntil.Load()
		if untilUnix <= 0 {
			return
		}

		until := time.Unix(0, untilUnix)
		now := time.Now()

		if !until.After(now) {
			return
		}

		wait := time.Until(until)

		s.log.Warn("telegram sender paused", "wait", wait.String())

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			return
		}
	}
}

func (s *AsyncSender) sendWithRetryAfter(ctx context.Context, item queueItem) {
	for {
		_, err := s.client.SendMessage(ctx, item.Token, item.Req)
		if err == nil {
			metrics.IncTelegramSent()
			metrics.SetTelegramPausedUntil(time.Time{})

			s.log.Info("telegram queued message sent", "chat_id", item.Req.ChatID, "bytes", len(item.Req.Text))
			return
		}

		var rl *RateLimitError
		if errors.As(err, &rl) {
			retryAfter := rl.RetryAfter
			if retryAfter <= 0 {
				retryAfter = time.Second
			}

			until := time.Now().Add(retryAfter)
			s.pausedUntil.Store(until.UnixNano())

			metrics.IncTelegramRateLimited()
			metrics.SetTelegramRetryAfter(retryAfter)
			metrics.SetTelegramPausedUntil(until)

			s.log.Warn(
				"telegram rate limited",
				"retry_after", retryAfter.String(),
				"chat_id", item.Req.ChatID,
			)

			timer := time.NewTimer(retryAfter)
			select {
			case <-ctx.Done():
				timer.Stop()
				metrics.IncTelegramFailed("context_cancelled")
				return
			case <-timer.C:
				continue
			}
		}

		reason := classifySendError(err)
		metrics.IncTelegramFailed(reason)

		s.log.Error("telegram queued message failed", "chat_id", item.Req.ChatID, "reason", reason, "error", err)
		return
	}
}

// helper
func classifySendError(err error) string {
	if err == nil {
		return "none"
	}

	if errors.Is(err, context.Canceled) {
		return "context_cancelled"
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "context_deadline"
	}

	var rl *RateLimitError
	if errors.As(err, &rl) {
		return "rate_limited"
	}

	return "telegram_error"
}