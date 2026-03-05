package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/ontime/server/internal/api/middleware"
	"github.com/ontime/server/internal/api/respond"
	"github.com/ontime/server/internal/auth"
	"github.com/ontime/server/internal/db"
	"github.com/redis/go-redis/v9"
)

type AuthHandler struct {
	store       *db.Store
	jwtSvc      *auth.JWTService
	appleVerify auth.AppleVerifier
	rdb         *redis.Client
}

func NewAuthHandler(store *db.Store, jwtSvc *auth.JWTService, appleVerify auth.AppleVerifier, rdb *redis.Client) *AuthHandler {
	return &AuthHandler{store: store, jwtSvc: jwtSvc, appleVerify: appleVerify, rdb: rdb}
}

// POST /api/v1/auth/apple
func (h *AuthHandler) SignInWithApple(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IdentityToken string `json:"identity_token"`
		DeviceToken   string `json:"device_token"` // optional APNs token on sign-in
	}
	if err := decodeJSON(r, &body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.IdentityToken == "" {
		respond.Error(w, http.StatusBadRequest, "identity_token required")
		return
	}

	appleSub, email, err := h.appleVerify.Verify(r.Context(), body.IdentityToken)
	if err != nil {
		respond.Error(w, http.StatusUnauthorized, "invalid identity token")
		return
	}

	user, err := h.store.UpsertUser(r.Context(), appleSub, email)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "upsert user failed")
		return
	}

	accessToken, err := h.jwtSvc.IssueAccessToken(user.ID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "issue access token failed")
		return
	}

	// Register device if provided.
	var deviceID string
	if body.DeviceToken != "" {
		device, err := h.store.RegisterDevice(r.Context(), user.ID, body.DeviceToken)
		if err == nil {
			deviceID = device.ID.String()
		}
	}

	// Issue refresh token.
	rawRefresh, err := generateToken()
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "generate refresh token failed")
		return
	}
	expiresAt := time.Now().Add(auth.RefreshTokenTTL)
	// deviceID is optional for refresh tokens (can be zero UUID).
	devUUID, _ := parseUUIDOrZero(deviceID)
	if err := h.store.CreateRefreshToken(r.Context(), user.ID, devUUID, db.HashToken(rawRefresh), expiresAt); err != nil {
		respond.Error(w, http.StatusInternalServerError, "create refresh token failed")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": rawRefresh,
		"expires_in":    int(auth.AccessTokenTTL.Seconds()),
		"user":          user,
	})
}

// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &body); err != nil || body.RefreshToken == "" {
		respond.Error(w, http.StatusBadRequest, "refresh_token required")
		return
	}

	rt, err := h.store.GetRefreshToken(r.Context(), db.HashToken(body.RefreshToken))
	if err != nil {
		respond.Error(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Rotate: delete old, issue new.
	if err := h.store.DeleteRefreshToken(r.Context(), db.HashToken(body.RefreshToken)); err != nil {
		respond.Error(w, http.StatusInternalServerError, "rotate refresh token failed")
		return
	}

	accessToken, err := h.jwtSvc.IssueAccessToken(rt.UserID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "issue access token failed")
		return
	}

	rawRefresh, err := generateToken()
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "generate refresh token failed")
		return
	}
	expiresAt := time.Now().Add(auth.RefreshTokenTTL)
	if err := h.store.CreateRefreshToken(r.Context(), rt.UserID, rt.DeviceID, db.HashToken(rawRefresh), expiresAt); err != nil {
		respond.Error(w, http.StatusInternalServerError, "create refresh token failed")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": rawRefresh,
		"expires_in":    int(auth.AccessTokenTTL.Seconds()),
	})
}

// DELETE /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Add current access token to denylist with TTL = remaining lifetime.
	tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	claims, err := h.jwtSvc.Verify(tokenStr)
	if err == nil && claims.ExpiresAt != nil {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			h.rdb.Set(r.Context(), "denylist:"+tokenStr, 1, ttl)
		}
	}

	_ = userID
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/v1/auth/logout-all
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.store.DeleteAllRefreshTokensByUserID(r.Context(), userID); err != nil {
		respond.Error(w, http.StatusInternalServerError, "logout all failed")
		return
	}

	// Also denylist current access token.
	tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	claims, err := h.jwtSvc.Verify(tokenStr)
	if err == nil && claims.ExpiresAt != nil {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			h.rdb.Set(r.Context(), "denylist:"+tokenStr, 1, ttl)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
