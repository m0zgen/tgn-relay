package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/m0zgen/tgn-relay/internal/config"
	"github.com/m0zgen/tgn-relay/internal/httpapi"
	"github.com/m0zgen/tgn-relay/internal/telegram"
	"github.com/m0zgen/tgn-relay/internal/version"
)

func main() {
	configPath := flag.String("config", "config.yml", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		os.Stdout.WriteString(version.String() + "\n")
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	// root app context
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	tg := telegram.NewClient(cfg.Telegram.APIURL, cfg.Telegram.TimeoutDuration())
	tgSender := telegram.NewAsyncSender(tg, telegram.AsyncSenderConfig{
		QueueSize: cfg.Telegram.QueueSize,
		Interval:  cfg.Telegram.SendIntervalDuration(),
	}, logger)

	go tgSender.Run(appCtx)

	api := httpapi.NewServer(cfg, tgSender, logger)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("tgn-relay started", "listen", cfg.Listen, "version", version.String())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("tgn-relay shutting down")

	appCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
}
