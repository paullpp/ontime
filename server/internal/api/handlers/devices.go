package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ontime/server/internal/api/middleware"
	"github.com/ontime/server/internal/api/respond"
	"github.com/ontime/server/internal/db"
)

type DeviceHandler struct {
	store *db.Store
}

func NewDeviceHandler(store *db.Store) *DeviceHandler {
	return &DeviceHandler{store: store}
}

// POST /api/v1/devices
func (h *DeviceHandler) Register(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		APNSToken string `json:"apns_token"`
	}
	if err := decodeJSON(r, &body); err != nil || body.APNSToken == "" {
		respond.Error(w, http.StatusBadRequest, "apns_token required")
		return
	}

	device, err := h.store.RegisterDevice(r.Context(), userID, body.APNSToken)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "register device failed")
		return
	}

	respond.JSON(w, http.StatusCreated, device)
}

// DELETE /api/v1/devices/{id}
func (h *DeviceHandler) Deregister(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	deviceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid device id")
		return
	}

	if err := h.store.DeactivateDevice(r.Context(), deviceID, userID); err != nil {
		respond.Error(w, http.StatusInternalServerError, "deregister device failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
