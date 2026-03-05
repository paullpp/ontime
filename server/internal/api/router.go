package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/ontime/server/internal/api/handlers"
	mw "github.com/ontime/server/internal/api/middleware"
	"github.com/ontime/server/internal/api/respond"
	"github.com/ontime/server/internal/auth"
	"github.com/ontime/server/internal/db"
	"github.com/ontime/server/internal/scheduler"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

func NewRouter(
	store *db.Store,
	jwtSvc *auth.JWTService,
	appleVerify auth.AppleVerifier,
	sched *scheduler.Scheduler,
	rdb *redis.Client,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware.
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(mw.PerIPRateLimiter(rate.Limit(30), 60)) // 30 req/s, burst 60

	// Health probes.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		respond.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	authH := handlers.NewAuthHandler(store, jwtSvc, appleVerify, rdb)
	deviceH := handlers.NewDeviceHandler(store)
	tripH := handlers.NewTripHandler(store, sched)

	authMiddleware := mw.Authenticate(jwtSvc, rdb)

	r.Route("/api/v1", func(r chi.Router) {
		// Auth (unauthenticated).
		r.Post("/auth/apple", authH.SignInWithApple)
		r.Post("/auth/refresh", authH.Refresh)

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)

			r.Delete("/auth/logout", authH.Logout)
			r.Delete("/auth/logout-all", authH.LogoutAll)

			r.Post("/devices", deviceH.Register)
			r.Delete("/devices/{id}", deviceH.Deregister)

			r.Get("/trips", tripH.List)
			r.Post("/trips", tripH.Create)
			r.Get("/trips/{id}", tripH.Get)
			r.Put("/trips/{id}", tripH.Update)
			r.Delete("/trips/{id}", tripH.Cancel)
			r.Post("/trips/{id}/activate", tripH.Activate)
		})
	})

	return r
}
