package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/m0zgen/tgn-relay/internal/auth"
	"github.com/m0zgen/tgn-relay/internal/config"
	"github.com/m0zgen/tgn-relay/internal/metrics"
	"github.com/m0zgen/tgn-relay/internal/telegram"
)

type TelegramSender interface {
	SendMessage(ctx context.Context, token string, req telegram.SendMessageRequest) (*telegram.APIResponse, error)
}

type Server struct {
	cfg *config.Config
	tg  TelegramSender
	log *slog.Logger
}

type sendRequest struct {
	Group                 string `json:"group"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview *bool  `json:"disable_web_page_preview,omitempty"`
	DisableNotification   *bool  `json:"disable_notification,omitempty"`
}

type directRequest struct {
	Token                 string `json:"token"`
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
	DisableNotification   bool   `json:"disable_notification,omitempty"`
}

func NewServer(cfg *config.Config, tg TelegramSender, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, tg: tg, log: logger}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("GET /metrics", metrics.Handler())

	mux.HandleFunc("POST /api/v1/send", s.withAuth(s.sendByGroup))
	mux.HandleFunc("POST /api/v1/direct", s.withAuth(s.direct))
	return s.accessLog(mux)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.CheckIP(r, s.cfg.Security.AllowIPs) {
			writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "ip forbidden"})
			return
		}
		if !auth.CheckKey(r, s.cfg.Security.RelayKeys) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, s.cfg.Security.MaxTextBytes+2048)
		next(w, r)
	}
}

func (s *Server) sendByGroup(w http.ResponseWriter, r *http.Request) {
	var req sendRequest
	if err := decodeRequest(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	req.Group = strings.TrimSpace(req.Group)
	req.Text = strings.TrimSpace(req.Text)
	if req.Group == "" || req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "group and text are required"})
		return
	}
	if int64(len(req.Text)) > s.cfg.Security.MaxTextBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"ok": false, "error": "text too large"})
		return
	}

	group, ok := s.cfg.Groups[req.Group]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "group not found"})
		return
	}

	parseMode := group.ParseMode
	if req.ParseMode != "" {
		parseMode = req.ParseMode
	}
	preview := group.DisableWebPagePreview
	if req.DisableWebPagePreview != nil {
		preview = *req.DisableWebPagePreview
	}
	silent := group.DisableNotification
	if req.DisableNotification != nil {
		silent = *req.DisableNotification
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Telegram.TimeoutDuration()+2*time.Second)
	defer cancel()

	_, err := s.tg.SendMessage(ctx, group.Token, telegram.SendMessageRequest{
		ChatID:                group.ChatID,
		Text:                  req.Text,
		ParseMode:             parseMode,
		DisableWebPagePreview: preview,
		DisableNotification:   silent,
	})
	if err != nil {
		if errors.Is(err, telegram.ErrQueueFull) {
			s.log.Error("telegram queue full", "mode", "group", "group", req.Group)
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"ok":    false,
				"error": "telegram queue full",
			})
			return
		}

		s.log.Error("send enqueue failed", "mode", "group", "group", req.Group, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "telegram send failed"})
		return
	}

	metrics.IncMessageEnqueued("group", req.Group)

	// s.log.Info("send ok", "mode", "group", "group", req.Group, "bytes", len(req.Text))
	// writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	s.log.Info("send queued", "mode", "group", "group", req.Group, "bytes", len(req.Text))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "queued": true})
}

func (s *Server) direct(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Security.DirectModeEnabled {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "direct mode disabled"})
		return
	}

	var req directRequest
	if err := decodeRequest(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.ChatID = strings.TrimSpace(req.ChatID)
	req.Text = strings.TrimSpace(req.Text)
	if req.Token == "" || req.ChatID == "" || req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "token, chat_id and text are required"})
		return
	}
	if int64(len(req.Text)) > s.cfg.Security.MaxTextBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"ok": false, "error": "text too large"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Telegram.TimeoutDuration()+2*time.Second)
	defer cancel()

	_, err := s.tg.SendMessage(ctx, req.Token, telegram.SendMessageRequest{
		ChatID:                req.ChatID,
		Text:                  req.Text,
		ParseMode:             req.ParseMode,
		DisableWebPagePreview: req.DisableWebPagePreview,
		DisableNotification:   req.DisableNotification,
	})
	if err != nil {
		s.log.Error("send failed", "mode", "direct", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "telegram send failed"})
		return
	}

	metrics.IncMessageEnqueued("direct", "-")

	s.log.Info("send ok", "mode", "direct", "bytes", len(req.Text))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func decodeRequest(r *http.Request, dst any) error {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		return json.NewDecoder(r.Body).Decode(dst)
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	switch v := dst.(type) {
	case *sendRequest:
		v.Group = r.Form.Get("group")
		v.Text = r.Form.Get("text")
		v.ParseMode = r.Form.Get("parse_mode")
	case *directRequest:
		v.Token = r.Form.Get("token")
		v.ChatID = r.Form.Get("chat_id")
		v.Text = r.Form.Get("text")
		v.ParseMode = r.Form.Get("parse_mode")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		
		metrics.ObserveHTTPRequest(r.Method, r.URL.Path, rw.status, time.Since(start))

		// Never log full URI: direct mode may receive token and old proxy mode had token in path.
		s.log.Info("http request", "method", r.Method, "path", r.URL.Path, "status", rw.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
