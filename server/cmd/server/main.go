package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ontime/server/internal/api"
	"github.com/ontime/server/internal/auth"
	"github.com/ontime/server/internal/config"
	"github.com/ontime/server/internal/db"
	"github.com/ontime/server/internal/maps"
	"github.com/ontime/server/internal/notifications"
	"github.com/ontime/server/internal/scheduler"
	"github.com/ontime/server/internal/worker"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── PostgreSQL ────────────────────────────────────────────────────────────
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()
	store := db.NewStore(pool)

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdbOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}
	rdb := redis.NewClient(rdbOpts)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}

	sched := scheduler.New(rdb)

	// ── Auth ──────────────────────────────────────────────────────────────────
	jwtSvc := auth.NewJWTService(cfg.JWTSecret)

	var appleVerify auth.AppleVerifier
	if cfg.MockAuth {
		log.Println("[startup] using mock Apple verifier")
		appleVerify = auth.NewMockAppleVerifier()
	} else {
		appleVerify, err = auth.NewAppleVerifier(ctx)
		if err != nil {
			return fmt.Errorf("init apple verifier: %w", err)
		}
	}

	// ── Google Maps ───────────────────────────────────────────────────────────
	var mapsClient maps.RoutesClient
	if cfg.MockMaps {
		log.Println("[startup] using mock maps client")
		mapsClient = maps.NewMockClient()
	} else {
		mapsClient = maps.NewGoogleClient(cfg.GoogleMapsAPIKey)
	}

	// ── APNs ──────────────────────────────────────────────────────────────────
	var notifier notifications.Notifier
	if cfg.MockAPNS {
		log.Println("[startup] using mock APNs notifier")
		notifier = notifications.NewMockNotifier()
	} else {
		notifier, err = notifications.NewAPNSNotifier(
			cfg.APNSKeyID, cfg.APNSTeamID, cfg.APNSKeyFile, cfg.APNSBundleID,
		)
		if err != nil {
			return fmt.Errorf("init apns notifier: %w", err)
		}
	}

	// ── Worker ────────────────────────────────────────────────────────────────
	processor := worker.NewProcessor(store, mapsClient, notifier, sched)
	supervisor := worker.NewSupervisor(processor, store, sched)
	go supervisor.Start(ctx)

	// ── HTTP server ───────────────────────────────────────────────────────────
	router := api.NewRouter(store, jwtSvc, appleVerify, sched, rdb)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[startup] listening on :%d (env=%s)", cfg.Port, cfg.Environment)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Println("[shutdown] draining...")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutCancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		log.Println("[shutdown] complete")
	}
	return nil
}
