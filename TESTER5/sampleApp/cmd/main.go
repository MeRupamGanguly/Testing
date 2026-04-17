package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"domainconcern/sampleApp/config"
	"domainconcern/sampleApp/handler"
	repository "domainconcern/sampleApp/repostory"

	"domainconcern/sampleApp/service"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	db, err := repository.NewPostgresConnection(cfg)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	orderRepo := repository.NewPostgresOrderRepository(db)
	orderService := service.NewOrderService(orderRepo, cfg.DefaultCurrency)
	orderHandler := handler.NewOrderHandler(orderService)
	router := handler.NewRouter(orderHandler)

	srv := &http.Server{
		Addr:         cfg.ServerAddr(),
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("starting server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
