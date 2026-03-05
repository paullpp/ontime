package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/ontime/server/internal/api/respond"
	"github.com/ontime/server/internal/auth"
	"github.com/redis/go-redis/v9"
)

type contextKey string

const UserIDKey contextKey = "userID"

// Authenticate validates the Bearer JWT and injects the user ID into the context.
// It also checks the Redis denylist for logged-out tokens.
func Authenticate(jwtSvc *auth.JWTService, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("Authorization")
			if !strings.HasPrefix(raw, "Bearer ") {
				respond.Error(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			tokenStr := strings.TrimPrefix(raw, "Bearer ")

			// Check denylist.
			denied, err := rdb.Exists(r.Context(), "denylist:"+tokenStr).Result()
			if err == nil && denied > 0 {
				respond.Error(w, http.StatusUnauthorized, "token revoked")
				return
			}

			claims, err := jwtSvc.Verify(tokenStr)
			if err != nil {
				respond.Error(w, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromCtx extracts the authenticated user ID from the request context.
func UserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return id, ok
}
