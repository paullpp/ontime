package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ontime/server/internal/api/middleware"
	"github.com/ontime/server/internal/api/respond"
	"github.com/ontime/server/internal/db"
	"github.com/ontime/server/internal/models"
	"github.com/ontime/server/internal/scheduler"
)

type TripHandler struct {
	store     *db.Store
	scheduler *scheduler.Scheduler
}

func NewTripHandler(store *db.Store, sched *scheduler.Scheduler) *TripHandler {
	return &TripHandler{store: store, scheduler: sched}
}

// GET /api/v1/trips
func (h *TripHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	trips, err := h.store.GetActiveTripsByUserID(r.Context(), userID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "list trips failed")
		return
	}
	if trips == nil {
		trips = []*models.Trip{}
	}
	respond.JSON(w, http.StatusOK, trips)
}

// POST /api/v1/trips
func (h *TripHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		OriginLat        float64   `json:"origin_lat"`
		OriginLng        float64   `json:"origin_lng"`
		OriginName       string    `json:"origin_name"`
		DestinationLat   float64   `json:"destination_lat"`
		DestinationLng   float64   `json:"destination_lng"`
		DestinationName  string    `json:"destination_name"`
		DesiredArrivalAt time.Time `json:"desired_arrival_at"`
		WarningMinutes   int       `json:"warning_minutes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.DestinationName == "" || body.DesiredArrivalAt.IsZero() {
		respond.Error(w, http.StatusBadRequest, "destination_name and desired_arrival_at are required")
		return
	}
	if body.DesiredArrivalAt.Before(time.Now()) {
		respond.Error(w, http.StatusBadRequest, "desired_arrival_at must be in the future")
		return
	}

	trip, err := h.store.CreateTrip(r.Context(), db.CreateTripParams{
		UserID:           userID,
		OriginLat:        body.OriginLat,
		OriginLng:        body.OriginLng,
		OriginName:       body.OriginName,
		DestinationLat:   body.DestinationLat,
		DestinationLng:   body.DestinationLng,
		DestinationName:  body.DestinationName,
		DesiredArrivalAt: body.DesiredArrivalAt,
		WarningMinutes:   body.WarningMinutes,
		NextPollAt:       time.Now(), // poll immediately
	})
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "create trip failed")
		return
	}

	// Enqueue in Redis scheduler.
	if err := h.scheduler.Schedule(r.Context(), trip.ID, trip.NextPollAt); err != nil {
		// Non-fatal: worker will catch it on next tick via DB fallback.
		_ = err
	}

	respond.JSON(w, http.StatusCreated, trip)
}

// GET /api/v1/trips/{id}
func (h *TripHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tripID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid trip id")
		return
	}

	trip, err := h.store.GetTripByIDAndUserID(r.Context(), tripID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "trip not found")
		} else {
			respond.Error(w, http.StatusInternalServerError, "get trip failed")
		}
		return
	}

	respond.JSON(w, http.StatusOK, trip)
}

// PUT /api/v1/trips/{id}
func (h *TripHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tripID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid trip id")
		return
	}

	var body struct {
		DesiredArrivalAt time.Time `json:"desired_arrival_at"`
		WarningMinutes   int       `json:"warning_minutes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	trip, err := h.store.UpdateTrip(r.Context(), tripID, userID, db.UpdateTripParams{
		DesiredArrivalAt: body.DesiredArrivalAt,
		WarningMinutes:   body.WarningMinutes,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "trip not found")
		} else {
			respond.Error(w, http.StatusInternalServerError, "update trip failed")
		}
		return
	}

	// Reschedule immediately so the worker picks up the change.
	_ = h.scheduler.Schedule(r.Context(), trip.ID, time.Now())

	respond.JSON(w, http.StatusOK, trip)
}

// DELETE /api/v1/trips/{id}
func (h *TripHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tripID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid trip id")
		return
	}

	if err := h.store.CancelTrip(r.Context(), tripID, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "trip not found or already cancelled")
		} else {
			respond.Error(w, http.StatusInternalServerError, "cancel trip failed")
		}
		return
	}

	_ = h.scheduler.Unschedule(r.Context(), tripID)
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/trips/{id}/activate
func (h *TripHandler) Activate(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tripID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid trip id")
		return
	}

	if err := h.store.ActivateTrip(r.Context(), tripID, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "trip not found or cannot be activated")
		} else {
			respond.Error(w, http.StatusInternalServerError, "activate trip failed")
		}
		return
	}

	_ = h.scheduler.Schedule(r.Context(), tripID, time.Now())
	w.WriteHeader(http.StatusNoContent)
}

